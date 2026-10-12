/*
  Open Charts - in-sim viewer. ES5 on purpose (MSFS embedded browser, Chrome ~49).

  One page, two modes for the same chart:
    overlay  the chart warped over OpenStreetMap tiles (Leaflet map, Web Mercator)
    chart    the chart image alone, aircraft placed with the chart's georeference (CRS.Simple)
  Search finds charts by ICAO, type, words ("sao paulo", "ils 17r"); a chart can be pinned to the
  current group, and pinned charts are one tap away in the bar.

  Debug: /viewer.html?chart=<id>&mode=chart|overlay&at=<lat>,<lon>[,<heading>]&lang=pt|en
*/
(function () {
  'use strict';

  var POLL_MS = 500;            // position polling while the aircraft moves
  var POLL_IDLE_MS = 1500;      // ... and while it has not changed for a few polls (parked)
  var FOLLOW_DEADZONE_PX = 36;  // the map follows the aircraft only beyond this distance from the centre
  var TILE = 256;
  var MESH = 8;
  var OPACITY_STEPS = [1, 0.75, 0.5, 0.25];   // chart opacity: the one button steps through these
  var CHART_MIN_ZOOM = -5;
  var CHART_MAX_ZOOM = 2;
  var OVERLAY_MIN_ZOOM = 3;
  var OVERLAY_MAX_ZOOM = 16;

  var STR = {
    en: {
      search: 'Search charts: SBSP, ILS, sao paulo...',
      noPins: 'Search a chart and pin it to this group.',
      noResults: 'No charts found.',
      pin: 'Pin', pinned: 'Pinned',
      modeOverlay: 'On map', modeChart: 'Chart only',
      group: 'Group', newGroup: 'New group',
      waitSim: 'Waiting for MSFS (SimConnect)...',
      waitPos: 'Connected - waiting for position (start a flight)',
      unreachable: 'Server unreachable',
      pick: 'Pick a chart (search above)',
      offChart: 'OFF CHART',
      loading: 'Loading chart...',
      loadFail: 'Could not load the chart',
      fixed: 'FIXED',
      simNote: 'FLIGHT SIMULATION ONLY - NOT FOR REAL-WORLD AVIATION',
      starPin: 'Pin this chart to the group', starUnpin: 'Remove this chart from the group',
      thisGroup: 'Charts in this group', noPinsHere: 'No charts pinned in this group.', remove: 'Remove',
      viewWithMap: 'With map', viewChartOnly: 'Chart only', blendMap: 'Map', blendChart: 'Chart',
      blendTitle: 'Chart over the map', otherGroup: 'Other',
      prevChart: 'Scroll left', nextChart: 'Scroll right', closeChart: 'Close chart',
      showPinned: 'Pinned charts: hide the list', hidePinned: 'Pinned charts: show the list'
    },
    pt: {
      search: 'Buscar cartas: SBSP, ILS, sao paulo...',
      noPins: 'Busque uma carta e fixe neste grupo.',
      noResults: 'Nenhuma carta encontrada.',
      pin: 'Fixar', pinned: 'Fixada',
      modeOverlay: 'No mapa', modeChart: 'Só a carta',
      group: 'Grupo', newGroup: 'Novo grupo',
      waitSim: 'Aguardando o MSFS (SimConnect)...',
      waitPos: 'Conectado - aguardando posição (inicie um voo)',
      unreachable: 'Servidor inacessível',
      pick: 'Escolha uma carta (busca acima)',
      offChart: 'FORA DA CARTA',
      loading: 'Carregando carta...',
      loadFail: 'Não foi possível carregar a carta',
      fixed: 'FIXO',
      simNote: 'APENAS SIMULAÇÃO AÉREA - PROIBIDO O USO EM AVIAÇÃO REAL',
      starPin: 'Fixar esta carta no grupo', starUnpin: 'Tirar esta carta do grupo',
      thisGroup: 'Cartas deste grupo', noPinsHere: 'Nenhuma carta fixada neste grupo.', remove: 'Tirar',
      viewWithMap: 'Com mapa', viewChartOnly: 'S\u00f3 a carta', blendMap: 'Mapa', blendChart: 'Carta',
      blendTitle: 'Carta sobre o mapa', otherGroup: 'Outras',
      prevChart: 'Rolar para a esquerda', nextChart: 'Rolar para a direita', closeChart: 'Fechar carta',
      showPinned: 'Cartas fixadas: esconder a lista', hidePinned: 'Cartas fixadas: mostrar a lista'
    }
  };

  var el = function (id) { return document.getElementById(id); };
  var qEl = el('q');
  var pinsEl = el('pins');
  var resultsEl = el('results');
  var groupsEl = el('groups');
  var kbdEl = el('kbd');
  var statusEl = el('status');
  var followBtn = el('btn-follow');
  var groupBtn = el('btn-group');
  var viewOverlayBtn = el('view-overlay');
  var viewChartBtn = el('view-chart');
  var blendBtn = el('btn-blend');
  var closeBtn = el('btn-close');
  var stripBtn = el('btn-strip');
  var stripEl = document.querySelector('.strip');
  var pinPrev = el('pin-prev');
  var pinNext = el('pin-next');
  var attribEl = el('attribution');
  var starBtn = el('btn-star');
  var simNoteEl = el('simnote');

  var params = (function () {
    var out = {};
    window.location.search.replace(/^\?/, '').split('&').forEach(function (kv) {
      if (!kv) return;
      var i = kv.indexOf('=');
      out[decodeURIComponent(i < 0 ? kv : kv.slice(0, i))] = i < 0 ? '' : decodeURIComponent(kv.slice(i + 1));
    });
    return out;
  })();

  function store(key, val) {
    try {
      if (val === undefined) return window.localStorage.getItem('oc.' + key);
      window.localStorage.setItem('oc.' + key, val);
    } catch (e) { /* storage may be blocked */ }
    return null;
  }

  var lang = (params.lang || store('lang') || (navigator.language || 'en')).toLowerCase().indexOf('pt') === 0 ? 'pt' : 'en';
  if (params.lang === 'en') lang = 'en';
  function t(k) { return STR[lang][k] || STR.en[k] || k; }

  var basemap = 'standard';   // from the settings, through /api/viewer-state (standard, light, dark, none)
  if (/^(standard|light|dark|none)$/.test(params.basemap || '')) basemap = params.basemap;
  var mode = params.mode || store('mode') || 'overlay';
  if (mode !== 'chart' && mode !== 'overlay') mode = 'overlay';

  var debugAt = null;
  if (params.at) {
    var pa = params.at.split(',').map(Number);
    if (pa.length >= 2 && isFinite(pa[0]) && isFinite(pa[1])) {
      debugAt = { lat: pa[0], lon: pa[1], headingTrue: isFinite(pa[2]) ? pa[2] : 0 };
    }
  }

  // What this page does, counted and sent to the server every 10 s (reportPerf). Used to find
  // out what makes the simulator's browser flicker; see /api/client-perf.
  var perf = {};
  function bump(k) { perf[k] = (perf[k] || 0) + 1; }
  var idlePolls = 0;
  var lastPollSig = '';

  var map = null;
  var marker = null;
  var planeRot = null;
  var chartLayer = null;
  var active = null;            // { meta, img }
  var groups = { current: '', groups: [] };
  var pinned = [];              // chart objects of the current group, in pin order
  var follow = true;
  var followJustEnabled = false;
  var lastKey = null;
  var lastFix = null;
  var lastStatusText = null;
  var lastStatusKind = null;
  var lastW = 0;
  var lastH = 0;
  var opacityIdx = 1;
  if (params.blend !== undefined && /^[0-3]$/.test(params.blend)) opacityIdx = parseInt(params.blend, 10);
  try { var oi = parseInt(window.localStorage.getItem('oc.blend2'), 10); if (oi >= 0 && oi < 4) opacityIdx = oi; } catch (e) { /* ignore */ }
  var needsFit = false;
  var searchTimer = null;
  var searchSeq = 0;

  function setStatus(text, kind) {
    if (text === lastStatusText && kind === lastStatusKind) return;
    lastStatusText = text;
    lastStatusKind = kind;
    statusEl.textContent = text;
    statusEl.className = 'status' + (kind ? ' ' + kind : '');
  }

  if (!window.L || typeof window.L.map !== 'function' || !window.LMMGeo) {
    setStatus('Leaflet or geo.js failed to load', 'bad');
    return;
  }

  // ---- HTTP ----
  function request(method, url, body, cb) {
    var xhr = new XMLHttpRequest();
    xhr.open(method, url);
    xhr.timeout = 8000;
    if (body !== null) xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) return;
      var data = null;
      if (xhr.status >= 200 && xhr.status < 300) {
        try { data = JSON.parse(xhr.responseText); } catch (e) { data = {}; }
      }
      if (cb) cb(data);
    };
    xhr.send(body === null ? null : JSON.stringify(body));
  }
  function getJson(url, cb) { request('GET', url, null, cb); }

  // The sim's font has no middle dot (U+00B7): it shows as a box.
  function label(s) { return String(s).replace(/\s*·\s*/g, ' - '); }

  function clear(node) { while (node.firstChild) node.removeChild(node.firstChild); }
  function button(cls, text, onclick) {
    var b = document.createElement('button');
    b.type = 'button';
    b.className = cls;
    b.textContent = text;
    b.addEventListener('click', onclick);
    return b;
  }

  // ---------------------------------------------------------------------------
  // Warping layer (overlay mode): see overlay.js in the prototype for the maths.
  // ---------------------------------------------------------------------------
  function tileToLatLon(z, x, y) {
    var n = Math.pow(2, z);
    var lon = x / n * 360 - 180;
    var m = Math.PI * (1 - 2 * y / n);
    var lat = Math.atan((Math.exp(m) - Math.exp(-m)) / 2) * 180 / Math.PI;
    return { lat: lat, lon: lon };
  }

  function drawTriangle(ctx, img, s0, s1, s2, d0, d1, d2, frame) {
    var det = (s1.x - s0.x) * (s2.y - s0.y) - (s2.x - s0.x) * (s1.y - s0.y);
    if (Math.abs(det) < 1e-9) return;
    var a = ((d1.x - d0.x) * (s2.y - s0.y) - (d2.x - d0.x) * (s1.y - s0.y)) / det;
    var c = ((d2.x - d0.x) * (s1.x - s0.x) - (d1.x - d0.x) * (s2.x - s0.x)) / det;
    var b = ((d1.y - d0.y) * (s2.y - s0.y) - (d2.y - d0.y) * (s1.y - s0.y)) / det;
    var d = ((d2.y - d0.y) * (s1.x - s0.x) - (d1.y - d0.y) * (s2.x - s0.x)) / det;
    var e = d0.x - a * s0.x - c * s0.y;
    var f = d0.y - b * s0.x - d * s0.y;
    var cx = (d0.x + d1.x + d2.x) / 3;
    var cy = (d0.y + d1.y + d2.y) / 3;
    function grow(pt) {
      var dx = pt.x - cx;
      var dy = pt.y - cy;
      var len = Math.sqrt(dx * dx + dy * dy) || 1;
      return { x: pt.x + dx / len * 0.6, y: pt.y + dy / len * 0.6 };
    }
    var g0 = grow(d0);
    var g1 = grow(d1);
    var g2 = grow(d2);
    var sx0 = Math.max(frame.left, Math.min(s0.x, s1.x, s2.x) - 1);
    var sy0 = Math.max(frame.top, Math.min(s0.y, s1.y, s2.y) - 1);
    var sx1 = Math.min(frame.right, Math.max(s0.x, s1.x, s2.x) + 1);
    var sy1 = Math.min(frame.bottom, Math.max(s0.y, s1.y, s2.y) + 1);
    if (sx1 <= sx0 || sy1 <= sy0) return;
    ctx.save();
    ctx.beginPath();
    ctx.moveTo(g0.x, g0.y);
    ctx.lineTo(g1.x, g1.y);
    ctx.lineTo(g2.x, g2.y);
    ctx.closePath();
    ctx.clip();
    ctx.setTransform(a, b, c, d, e, f);
    ctx.drawImage(img, sx0, sy0, sx1 - sx0, sy1 - sy0, sx0, sy0, sx1 - sx0, sy1 - sy0);
    ctx.restore();
  }

  // Base map tiles are fetched with XHR and shown as data: URIs. In the simulator a tile image that
  // came from the network was dropped from its image cache while the game's own UI was redrawn and
  // came back black; the chart tiles (data: URIs) never did. A data: URI keeps the bytes in the
  // element itself, so there is nothing to lose.
  function toBase64(buf) {
    var bytes = new Uint8Array(buf);
    var s = '';
    for (var i = 0; i < bytes.length; i += 0x4000) {
      s += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x4000));
    }
    return window.btoa(s);
  }

  var DataTileLayer = L.TileLayer.extend({
    initialize: function (url, options) {
      L.TileLayer.prototype.initialize.call(this, url, options);
      this.on('tileunload', function (ev) {
        if (ev.tile && ev.tile._xhr) { try { ev.tile._xhr.abort(); } catch (e) { /* done */ } }
      });
    },
    createTile: function (coords, done) {
      var tile = document.createElement('img');
      tile.alt = '';
      tile.setAttribute('role', 'presentation');
      var url = this.getTileUrl(coords);
      var tries = 0;
      var settled = false;
      function fin(err) { if (!settled) { settled = true; done(err || null, tile); } }
      function retry() {
        tile._xhr = null;
        if (tries >= 5) { bump('baseTileError'); fin('failed'); return; }
        window.setTimeout(load, 700 * tries);   // a blip or upstream throttling: ask again
      }
      function load() {
        tries += 1;
        var x = new XMLHttpRequest();
        tile._xhr = x;
        x.open('GET', url);
        x.responseType = 'arraybuffer';
        x.timeout = 20000;
        x.onload = function () {
          if (x.status !== 200 || !x.response) { retry(); return; }
          var type = x.getResponseHeader('Content-Type') || 'image/png';
          tile.onload = function () { fin(null); };
          tile.onerror = function () { retry(); };
          tile.src = 'data:' + type + ';base64,' + toBase64(x.response);
        };
        x.onerror = retry;
        x.ontimeout = retry;
        x.send();
      }
      load();
      return tile;
    }
  });

  var BLANK_TILE = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';

  var WarpLayer = L.GridLayer.extend({
    initialize: function (meta, img, options) {
      L.GridLayer.prototype.initialize.call(this, options);
      this._meta = meta;
      this._img = img;
      // The chart's opacity is baked into each tile (not a CSS opacity on the layer): in the
      // simulator a translucent container made the tiles flicker and turn black.
      this._alpha = options.chartAlpha === undefined ? 1 : options.chartAlpha;
    },
    setAlpha: function (a) {
      this._alpha = a;
      this.redraw();
    },
    createTile: function (coords, done) {
      bump('chartTileDrawn');
      var dpr = Math.min(3, window.devicePixelRatio || 1);   // draw at device pixels: crisp lines
      var tile = document.createElement('canvas');
      tile.width = Math.round(TILE * dpr);
      tile.height = Math.round(TILE * dpr);
      tile.className = 'leaflet-tile';
      var meta = this._meta;
      var img = this._img;
      var frame = meta.mapFramePx;
      var step = TILE * dpr / MESH;
      var src = [];
      var minX = Infinity;
      var minY = Infinity;
      var maxX = -Infinity;
      var maxY = -Infinity;
      var i;
      var j;
      for (j = 0; j <= MESH; j++) {
        src[j] = [];
        for (i = 0; i <= MESH; i++) {
          var ll = tileToLatLon(coords.z, coords.x + i / MESH, coords.y + j / MESH);
          var pt = LMMGeo.toPixel(meta, ll.lat, ll.lon);
          src[j][i] = { x: pt.px, y: pt.py };
          if (pt.px < minX) minX = pt.px;
          if (pt.px > maxX) maxX = pt.px;
          if (pt.py < minY) minY = pt.py;
          if (pt.py > maxY) maxY = pt.py;
        }
      }
      var inside = !(maxX < frame.left || minX > frame.right || maxY < frame.top || minY > frame.bottom);
      if (inside) {
        var out = tile.getContext('2d');
        var ctx = out;
        var work = null;
        if (this._alpha < 1) {
          work = document.createElement('canvas');   // drawn opaque, then blended once
          work.width = tile.width;
          work.height = tile.height;
          ctx = work.getContext('2d');
        }
        ctx.imageSmoothingEnabled = true;
        if ('imageSmoothingQuality' in ctx) ctx.imageSmoothingQuality = 'high';
        for (j = 0; j < MESH; j++) {
          for (i = 0; i < MESH; i++) {
            var p00 = { x: i * step, y: j * step };
            var p10 = { x: (i + 1) * step, y: j * step };
            var p11 = { x: (i + 1) * step, y: (j + 1) * step };
            var p01 = { x: i * step, y: (j + 1) * step };
            drawTriangle(ctx, img, src[j][i], src[j][i + 1], src[j + 1][i + 1], p00, p10, p11, frame);
            drawTriangle(ctx, img, src[j][i], src[j + 1][i + 1], src[j + 1][i], p00, p11, p01, frame);
          }
        }
        if (work) {
          out.globalAlpha = this._alpha;
          out.drawImage(work, 0, 0);
        }
      }
      // The finished canvas is turned into a plain image. A canvas keeps its pixels in graphics
      // memory, and the simulator's browser loses them (tiles flicker and turn black while the
      // aircraft is parked); an image is decoded again by the browser whenever it is needed.
      var tileImg = document.createElement('img');
      tileImg.className = 'leaflet-tile chart-tile';
      tileImg.alt = '';
      var settled = false;
      function finish(err) { if (!settled) { settled = true; done(err || null, tileImg); } }
      tileImg.onload = function () { finish(null); };
      tileImg.onerror = function () { finish(null); };
      var url = BLANK_TILE;
      if (inside) {
        try { url = tile.toDataURL('image/png'); } catch (e) { url = BLANK_TILE; }
      }
      tileImg.src = url;
      window.setTimeout(function () { finish(null); }, 1500);   // never leave a tile pending
      return tileImg;
    }
  });

  // ---------------------------------------------------------------------------
  // Map (rebuilt when the mode changes: the two modes use different CRS)
  // ---------------------------------------------------------------------------
  var PLANE_SVG =
    '<svg viewBox="0 0 64 64"><path d="M32 4L35 10L36 24L58 40L58 45L36 38L35 50L42 57L42 61' +
    'L32 58L22 61L22 57L29 50L28 38L6 45L6 40L28 24L29 10Z"/></svg>';

  function hasSize() {
    var s = map.getSize();
    return s.x > 0 && s.y > 0;
  }

  function chartPixelToLatLng(pt) { return L.latLng(active.meta.image.height - pt.py, pt.px); }

  function buildMap() {
    if (map) {
      map.off();
      map.remove();
      map = null;
    }
    chartLayer = null;
    marker = null;
    planeRot = null;
    lastKey = null;

    var common = {
      zoomControl: false, attributionControl: false, zoomAnimation: false, fadeAnimation: false,
      markerZoomAnimation: false, inertia: false, tap: false, touchZoom: true, bounceAtZoomLimits: false, keyboard: false,
      doubleClickZoom: false, boxZoom: false
    };
    if (mode === 'chart') {
      common.crs = L.CRS.Simple;
      common.minZoom = CHART_MIN_ZOOM;
      common.maxZoom = CHART_MAX_ZOOM;
      common.zoomSnap = 0.25;
      map = L.map('map', common);
      map.setView([0, 0], 0);
    } else {
      common.minZoom = OVERLAY_MIN_ZOOM;
      common.maxZoom = OVERLAY_MAX_ZOOM;
      map = L.map('map', common).setView([-23.55, -46.63], 9);
      var base = basemap === 'none' ? null : new DataTileLayer(basemap === 'standard' ? '/tiles/{z}/{x}/{y}.png?f=rgb' : '/tiles/' + basemap + '/{z}/{x}/{y}.jpg?f=rgb', {
        minZoom: OVERLAY_MIN_ZOOM, maxZoom: OVERLAY_MAX_ZOOM, tileSize: 256, updateWhenIdle: false, keepBuffer: 2,
        detectRetina: true   // phones and tablets: tiles of the next zoom level, drawn at half size
      });
      if (base) {
        base.on('tileload', function () { bump('baseTileLoaded'); });
        base.on('tileunload', function () { bump('baseTileUnloaded'); });
        base.addTo(map);
      }
    }
    var icon = L.divIcon({
      className: 'plane-icon',
      html: '<div class="plane-rot">' + PLANE_SVG + '</div>',
      iconSize: [40, 40],
      iconAnchor: [20, 20]
    });
    marker = L.marker([0, 0], { icon: icon, interactive: false, keyboard: false, zIndexOffset: 1000 });
    map.on('dragstart', function () { setFollow(false); });
    map.on('move', function () { bump('mapMove'); });
    map.on('zoomend', function () { bump('mapZoomEnd'); });
    map.on('resize', function () { bump('mapResize'); });

    attribEl.style.display = mode === 'overlay' ? '' : 'none';
    updateDisplayPanel();
    mapBackground();
    applyChart();
    if (lastFix) showAircraft(lastFix);
  }

  function mapBackground() {
    el('map').style.background = mode === 'chart' ? '#ffffff' : (basemap === 'dark' ? '#1c2024' : (basemap === 'none' ? '#eef1f3' : '#dfe6ea'));
  }

  // Put the active chart on the freshly built map.
  function applyChart() {
    if (!active) {
      setStatus(t('pick'), 'warn');
      return;
    }
    var meta = active.meta;
    if (mode === 'chart') {
      var b = L.latLngBounds([0, 0], [meta.image.height, meta.image.width]);
      chartLayer = L.imageOverlay('/charts/' + encodeURIComponent(meta.id) + '/' + meta.image.file, b).addTo(map);
      map.setMaxBounds(b.pad(0.5));
    } else {
      chartLayer = new WarpLayer(meta, active.img, {
        tileSize: TILE, chartAlpha: OPACITY_STEPS[opacityIdx], minZoom: OVERLAY_MIN_ZOOM,
        maxZoom: OVERLAY_MAX_ZOOM, updateWhenIdle: false, keepBuffer: 1, zIndex: 10
      }).addTo(map);
    }
    fitChart();
  }

  function fitChart() {
    if (!active || !map) return;
    if (!hasSize()) { needsFit = true; return; }
    needsFit = false;
    var meta = active.meta;
    if (mode === 'chart') {
      map.fitBounds(L.latLngBounds([0, 0], [meta.image.height, meta.image.width]), { animate: false });
    } else {
      var pts = meta.geoCorners.map(function (g) { return L.latLng(g.lat, g.lon); });
      map.fitBounds(L.latLngBounds(pts), { animate: false });
    }
  }

  function setFollow(on) {
    if (on && !follow) followJustEnabled = true;
    follow = on;
    followBtn.className = on ? 'active' : '';
  }

  function rotatePlane(deg) {
    if (!planeRot) {
      var e = marker && marker.getElement ? marker.getElement() : null;
      planeRot = e ? e.querySelector('.plane-rot') : null;
    }
    if (!planeRot) return;
    var v = 'rotate(' + deg.toFixed(1) + 'deg)';
    planeRot.style.transform = v;
    planeRot.style.webkitTransform = v;
  }

  function chartHeading(lat, lon, headingTrue) {
    var h = LMMGeo.rad(headingTrue);
    var dLat = 200 * Math.cos(h) / 111320;
    var dLon = 200 * Math.sin(h) / (111320 * Math.cos(LMMGeo.rad(lat)));
    var a = LMMGeo.toPixel(active.meta, lat, lon);
    var b = LMMGeo.toPixel(active.meta, lat + dLat, lon + dLon);
    return Math.atan2(b.px - a.px, -(b.py - a.py)) * 180 / Math.PI;
  }

  function showAircraft(p) {
    lastFix = p;
    if (!map || !marker) return;
    var ll;
    var rot = p.headingTrue;
    var inside = true;
    if (mode === 'chart') {
      if (!active) return;
      var pt = LMMGeo.toPixel(active.meta, p.lat, p.lon);
      var f = active.meta.mapFramePx;
      inside = pt.px >= f.left && pt.px <= f.right && pt.py >= f.top && pt.py <= f.bottom;
      ll = chartPixelToLatLng(pt);
      rot = chartHeading(p.lat, p.lon, p.headingTrue);
    } else {
      ll = L.latLng(p.lat, p.lon);
    }
    // Redraw the aircraft only when it moved by about half a pixel or turned by 2 degrees: a parked
    // aircraft still reports tiny changes, and every redraw is work for the simulator's browser.
    var lp = map.latLngToLayerPoint(ll);
    var key = Math.round(lp.x * 2) + '|' + Math.round(lp.y * 2) + '|' + Math.round(rot / 2);
    var changed = key !== lastKey;
    if (!map.hasLayer(marker)) {
      marker.addTo(map);
      planeRot = null;
      changed = true;
    }
    if (changed) {
      lastKey = key;
      bump('markerMoved');
      marker.setLatLng(ll);
      rotatePlane(rot);
    }
    var name = active ? label(active.meta.name) + '  |  ' : '';
    setStatus(name + (p.simulated ? t('fixed') + ' ' : '') + p.lat.toFixed(4) + ', ' + p.lon.toFixed(4) +
      '  HDG ' + Math.round(p.headingTrue) + (inside ? '' : '  -  ' + t('offChart')), inside ? 'ok' : 'warn');

    // Follow: the map moves only when the aircraft gets away from the centre (a dead zone), not for
    // every small step. In the simulator the map pane moves through left/top (no 3D), which
    // repaints every tile; doing that for each position update made the tiles flicker.
    if (follow && inside) {
      var c = map.latLngToContainerPoint(ll);
      var s = map.getSize();
      var dx = c.x - s.x / 2;
      var dy = c.y - s.y / 2;
      if (followJustEnabled || Math.abs(dx) > FOLLOW_DEADZONE_PX || Math.abs(dy) > FOLLOW_DEADZONE_PX) {
        followJustEnabled = false;
        bump('followPan');
        map.panBy([dx, dy], { animate: false });
      }
    }
  }

  function poll() {
    if (debugAt) {
      showAircraft({ lat: debugAt.lat, lon: debugAt.lon, headingTrue: debugAt.headingTrue, simulated: true });
      return;
    }
    getJson('/api/position?t=' + Date.now(), function (p) {
      bump('positionPolls');
      var next = POLL_MS;
      if (p && p.valid) {
        // A parked aircraft does not need four requests a second: slow down while nothing changes.
        var sig = p.lat.toFixed(5) + '|' + p.lon.toFixed(5) + '|' + Math.round(p.headingTrue);
        idlePolls = sig === lastPollSig ? idlePolls + 1 : 0;
        lastPollSig = sig;
        if (idlePolls >= 4) next = POLL_IDLE_MS;
      }
      if (!p) {
        setStatus(t('unreachable'), 'bad');
      } else if (!p.simConnected) {
        setStatus(t('waitSim'), 'warn');
      } else if (!p.valid) {
        setStatus(t('waitPos'), 'warn');
      } else {
        showAircraft(p);
      }
      window.setTimeout(poll, next);
    });
  }

  // ---------------------------------------------------------------------------
  // Charts: open, search, pin
  // ---------------------------------------------------------------------------
  // The EFB app always loads the same URL, so the state is kept on the server and mirrored in
  // the page URL (reload, or opening the same URL in a browser, comes back to the same view).
  // A remembered chart may have been deleted or renamed: fall back to the first pinned chart
  // (or just wait for a choice) instead of leaving the viewer on an error.
  function loadFailed(id, fallback) {
    setStatus(t('loadFail'), 'bad');
    if (fallback) return;
    var next = null;
    for (var i = 0; i < pinned.length; i++) if (pinned[i].id !== id) { next = pinned[i].id; break; }
    if (next) openChart(next, true);
  }

  var saveTimer = null;
  function saveState() {
    if (!active) return;
    var chartId = active.meta.id;
    try {
      if (window.history && window.history.replaceState) {
        var q = '?chart=' + encodeURIComponent(chartId) + '&mode=' + mode + '&lang=' + lang;
        window.history.replaceState(null, '', window.location.pathname + q);
      }
    } catch (e) { /* ignore: some embedded browsers block it */ }
    if (debugAt) return;
    if (saveTimer) window.clearTimeout(saveTimer);
    saveTimer = window.setTimeout(function () {
      request('PUT', '/api/viewer-state', { chart: chartId, mode: mode, lang: lang }, null);
    }, 300);
  }

  function openChart(id, fallback) {
    setStatus(t('loading'), 'warn');
    hidePanels();
    // v=<time>: never trust a chart.json cached by an older server (it kept names for an hour)
    getJson('/charts/' + encodeURIComponent(id) + '/chart.json?v=' + Date.now(), function (meta) {
      if (!meta) { loadFailed(id, fallback); return; }
      var img = new Image();
      img.onload = function () {
        active = { meta: meta, img: img };
        store('chart', id);
        setFollow(!debugAt);
        buildMap();
        renderPins();
        updateCloseButton();
        saveState();
      };
      img.onerror = function () { loadFailed(id, fallback); };
      img.src = '/charts/' + encodeURIComponent(id) + '/' + meta.image.file;
    });
  }

  function isPinned(id) {
    for (var i = 0; i < pinned.length; i++) if (pinned[i].id === id) return true;
    return false;
  }

  function currentGroup() {
    for (var i = 0; i < groups.groups.length; i++) {
      if (groups.groups[i].id === groups.current) return groups.groups[i];
    }
    return null;
  }

  function loadGroups(cb) {
    getJson('/api/groups', function (st) {
      if (st) groups = st;
      var fl = currentGroup();
      groupBtn.textContent = fl ? fl.name : '-';
      groupBtn.title = t('group');
      if (!fl || !fl.pins.length) {
        pinned = [];
        lastSignature = signatureOf(groups, []);
        renderPins();
        if (cb) cb();
        return;
      }
      getJson('/api/charts?group=' + encodeURIComponent(fl.id) + '&limit=200', function (list) {
        var byId = {};
        (list || []).forEach(function (c) { byId[c.id] = c; });
        pinned = [];
        fl.pins.forEach(function (id) { if (byId[id]) pinned.push(byId[id]); });   // pin order
        lastSignature = signatureOf(groups, list || []);
        syncActiveName();
        renderPins();
        if (cb) cb();
      });
    });
  }

  // A chart renamed or edited in the manager shows its new name here without reloading the page.
  function syncActiveName() {
    if (!active) return;
    for (var i = 0; i < pinned.length; i++) {
      if (pinned[i].id === active.meta.id && pinned[i].name && pinned[i].name !== active.meta.name) {
        active.meta.name = pinned[i].name;
        if (lastFix) showAircraft(lastFix);
      }
    }
  }

  // Cheap periodic check: reload the group and its pinned charts only when something changed
  // (names, pins, current group), so the strip is not rebuilt for nothing.
  var lastSignature = '';
  function signatureOf(st, list) {
    return st.current + '|' + JSON.stringify(st.groups.map(function (f) { return [f.id, f.name, f.pins]; })) + '|' +
      JSON.stringify((list || []).map(function (c) { return [c.id, c.name]; }));
  }
  function softRefresh() {
    // Not gated on document.hidden: inside the simulator the frame may report itself hidden
    // even while it is on screen. Two tiny requests every 15 s are cheap.
    if (lastSignature === '') return;   // nothing on screen yet
    getJson('/api/groups', function (st) {
      if (!st) return;
      var cur = null;
      for (var i = 0; i < st.groups.length; i++) if (st.groups[i].id === st.current) cur = st.groups[i];
      var check = function (list) {
        if (signatureOf(st, list) !== lastSignature) loadGroups();
      };
      if (!cur || !cur.pins.length) { check([]); return; }
      getJson('/api/charts?limit=200&group=' + encodeURIComponent(cur.id), function (list) {
        if (list) check(list);
      });
    });
  }

  function togglePin(chart, done) {
    var fl = currentGroup();
    if (!fl) return;
    var url = '/api/groups/' + encodeURIComponent(fl.id) + '/pins/' + encodeURIComponent(chart.id);
    request(isPinned(chart.id) ? 'DELETE' : 'PUT', url, null, function () {
      loadGroups(done);
    });
  }

  // ---- display panel: "With map" / "Chart only" and, for the map view, the chart opacity
  function updateDisplayPanel() {
    viewOverlayBtn.className = mode === 'overlay' ? 'active' : '';
    viewChartBtn.className = mode === 'chart' ? 'active' : '';
    blendBtn.style.display = mode === 'overlay' ? '' : 'none';
    var lvl = blendBtn.querySelector('.lvl');
    if (lvl) lvl.setAttribute('fill-opacity', String(OPACITY_STEPS[opacityIdx]));
    blendBtn.title = t('blendTitle') + ': ' + Math.round(OPACITY_STEPS[opacityIdx] * 100) + '%';
  }

  function setMode(next) {
    if (next === mode) return;
    mode = next;
    store('mode', mode);
    buildMap();
    saveState();
  }

  // one button: 100% -> 75% -> 50% -> 25% -> 100% of the chart over the map
  function cycleBlend() { setOpacity((opacityIdx + 1) % OPACITY_STEPS.length); }

  function setOpacity(idx) {
    if (idx < 0 || idx >= OPACITY_STEPS.length) return;
    opacityIdx = idx;
    store('blend2', String(idx));
    if (chartLayer && chartLayer.setAlpha && mode === 'overlay') chartLayer.setAlpha(OPACITY_STEPS[opacityIdx]);
    updateDisplayPanel();
  }

  // ---- pinned charts, grouped by airport
  function viewWithMapTitle() { return lang === 'pt' ? 'Carta sobre o mapa do OpenStreetMap' : 'Chart over the OpenStreetMap map'; }
  function viewChartOnlyTitle() { return lang === 'pt' ? 'Apenas a carta, sem mapa' : 'The chart alone, no map'; }

  function chipLabel(c) {
    var title = c.title || '';
    var typeIsProcedure = c.type === 'IAC' || c.type === 'SID' || c.type === 'STAR';
    var parts = [];
    if (c.type && !typeIsProcedure && title.toUpperCase().indexOf(c.type) < 0) parts.push(c.type);
    if (title) parts.push(title);
    return parts.join(' ') || c.name;
  }

  // [{key, label, items:[chart]}] in order of first appearance, charts keep their pin order
  function pinnedByAirport() {
    var airports = [];
    var byKey = {};
    pinned.forEach(function (c) {
      var key = c.icao || '';
      if (!byKey[key]) {
        byKey[key] = { key: key, label: key || t('otherGroup'), items: [] };
        airports.push(byKey[key]);
      }
      byKey[key].items.push(c);
    });
    return airports;
  }

  function activeIndex() {
    if (!active) return -1;
    for (var i = 0; i < pinned.length; i++) if (pinned[i].id === active.meta.id) return i;
    return -1;
  }

  // The arrows only move the strip so that every chart can be reached; choosing a chart is
  // always a tap on the chart itself. Disabled at either end.
  function updateStripArrows() {
    var max = pinsEl.scrollWidth - pinsEl.clientWidth;
    pinPrev.disabled = pinsEl.scrollLeft <= 1;
    pinNext.disabled = pinsEl.scrollLeft >= max - 1;
  }

  // Eased scroll (about a quarter of a second) over roughly half the strip: short enough to
  // follow with the eye, so the user sees that the strip is moving.
  var stripAnim = null;
  function scrollStrip(dir) {
    if (stripAnim) window.cancelAnimationFrame(stripAnim);
    var max = pinsEl.scrollWidth - pinsEl.clientWidth;
    var from = pinsEl.scrollLeft;
    var to = Math.max(0, Math.min(max, from + dir * Math.max(100, Math.round(pinsEl.clientWidth * 0.5))));
    var t0 = null;
    var DURATION = 260;
    var frames = 0;
    var done = false;
    function finish() {
      if (done) return;
      done = true;
      stripAnim = null;
      pinsEl.scrollLeft = to;
      updateStripArrows();
    }
    function step(ts) {
      if (done) return;
      frames += 1;
      if (t0 === null) t0 = ts;
      var k = Math.min(1, (ts - t0) / DURATION);
      var eased = 1 - Math.pow(1 - k, 3);
      pinsEl.scrollLeft = from + (to - from) * eased;
      if (k < 1) {
        stripAnim = window.requestAnimationFrame(step);
      } else {
        finish();
      }
    }
    stripAnim = window.requestAnimationFrame(step);
    // Frames are not delivered while the page reports itself hidden: do not leave the arrow
    // dead, jump to the target instead.
    window.setTimeout(function () { if (frames === 0) finish(); }, 150);
    window.setTimeout(finish, DURATION + 400);
  }

  // Leave the chart on screen. A chart that is not pinned has no chip to tap, so this is the
  // way out; the viewer remembers that nothing was open.
  function closeChart() {
    if (!active) return;
    active = null;
    lastKey = null;
    store('chart', '');
    if (!debugAt) request('PUT', '/api/viewer-state', { chart: '', mode: mode, lang: lang }, null);
    try {
      if (window.history && window.history.replaceState) {
        window.history.replaceState(null, '', window.location.pathname + '?mode=' + mode + '&lang=' + lang);
      }
    } catch (e) { /* ignore */ }
    buildMap();
    renderPins();
    updateCloseButton();
  }

  function updateCloseButton() {
    closeBtn.style.display = active ? '' : 'none';
  }

  // The star pins/unpins the chart on screen for the current group.
  function updateStar() {
    var on = !!active && isPinned(active.meta.id);
    starBtn.disabled = !active;
    starBtn.className = 'tool' + (on ? ' on' : '');
    starBtn.title = on ? t('starUnpin') : t('starPin');
  }

  // The list of pinned charts can be hidden to give the screen to the chart; it is remembered.
  var stripVisible = true;
  try { stripVisible = window.localStorage.getItem('oc.strip') !== '0'; } catch (e) { /* ignore */ }
  function applyStripVisibility() {
    stripEl.style.display = stripVisible ? '' : 'none';
    stripBtn.className = stripVisible ? 'active' : '';
    stripBtn.title = stripVisible ? t('showPinned') : t('hidePinned');
    if (stripVisible) updateStripArrows();
  }


  // true while a chart is being opened by tapping its chip: the strip must not move then
  var openedFromStrip = false;

  function renderPins() {
    updateStar();
    var keepScroll = pinsEl.scrollLeft;
    clear(pinsEl);
    if (!pinned.length) {
      var hint = document.createElement('span');
      hint.className = 'hint';
      hint.textContent = t('noPins');
      pinsEl.appendChild(hint);
      pinPrev.disabled = true;
      pinNext.disabled = true;
      return;
    }
    var activeChip = null;
    pinnedByAirport().forEach(function (g) {
      var wrap = document.createElement('span');
      wrap.className = 'group';
      var lbl = document.createElement('span');
      lbl.className = 'glabel';
      lbl.textContent = g.label;
      wrap.appendChild(lbl);
      g.items.forEach(function (c) {
        var isActive = !!active && active.meta.id === c.id;
        var b = button('chip' + (isActive ? ' active' : ''), chipLabel(c), function () {
          openedFromStrip = true;
          openChart(c.id);
        });
        b.title = label(c.name);
        if (isActive) activeChip = b;
        wrap.appendChild(b);
      });
      pinsEl.appendChild(wrap);
    });
    // Same position as before the re-render: tapping a chip never moves the strip.
    pinsEl.scrollLeft = keepScroll;
    updateStripArrows();
    // A chart opened some other way (search, first load) is brought into view, but only when it
    // is not already visible. Widths settle after layout and after the web font loads, so this is
    // checked again shortly after.
    if (activeChip && !openedFromStrip) {
      var reveal = function () {
        var viewL = pinsEl.scrollLeft;
        var viewR = viewL + pinsEl.clientWidth;
        var l = activeChip.offsetLeft;
        var r = l + activeChip.offsetWidth;
        if (l < viewL + 4 || r > viewR - 4) pinsEl.scrollLeft = Math.max(0, l - 56);
      };
      reveal();
      window.setTimeout(function () { reveal(); updateStripArrows(); }, 60);
      window.setTimeout(function () { reveal(); updateStripArrows(); }, 500);
    }
    openedFromStrip = false;
  }

  function renderResults(list) {
    clear(resultsEl);
    resultsEl.className = 'panel';
    if (!list.length) {
      var e = document.createElement('div');
      e.className = 'empty';
      e.textContent = t('noResults');
      resultsEl.appendChild(e);
      return;
    }
    list.forEach(function (c) {
      var row = document.createElement('div');
      row.className = 'item';
      var pb = button('pinbtn' + (isPinned(c.id) ? ' on' : ''), isPinned(c.id) ? t('pinned') : t('pin'), function (ev) {
        if (ev.stopPropagation) ev.stopPropagation();
        togglePin(c, function () { renderResults(list); });
      });
      row.appendChild(pb);
      row.appendChild(document.createTextNode(label(c.name)));
      row.addEventListener('click', function () { openChart(c.id); });
      resultsEl.appendChild(row);
    });
  }

  function runSearch() {
    var q = qEl.value.replace(/^\s+|\s+$/g, '');
    if (!q) { resultsEl.className = 'panel hidden'; return; }
    var seq = ++searchSeq;
    getJson('/api/charts?limit=12&q=' + encodeURIComponent(q), function (list) {
      if (seq !== searchSeq || list === null) return;
      groupsEl.className = 'panel hidden';
      renderResults(list);
    });
  }

  function scheduleSearch() {
    if (searchTimer) window.clearTimeout(searchTimer);
    searchTimer = window.setTimeout(runSearch, 180);
  }

  function hidePanels() {
    resultsEl.className = 'panel hidden';
    groupsEl.className = 'panel hidden';
  }

  // ---- groups panel ----
  function showGroups() {
    if (groupsEl.className.indexOf('hidden') < 0) { groupsEl.className = 'panel hidden'; return; }
    resultsEl.className = 'panel hidden';
    drawGroups();
  }

  function drawGroups() {
    clear(groupsEl);
    groupsEl.className = 'panel';
    groups.groups.forEach(function (f) {
      var row = document.createElement('div');
      row.className = 'item' + (f.id === groups.current ? ' current' : '');
      row.textContent = f.name + '  (' + f.pins.length + ')';
      row.addEventListener('click', function () {
        request('PUT', '/api/groups/current', { id: f.id }, function () {
          groupsEl.className = 'panel hidden';
          loadGroups();
        });
      });
      groupsEl.appendChild(row);
    });
    var add = document.createElement('div');
    add.className = 'item';
    add.textContent = '+ ' + t('newGroup');
    add.addEventListener('click', function () {
      var n = groups.groups.length + 1;
      request('POST', '/api/groups', { name: t('group') + ' ' + n }, function () {
        groupsEl.className = 'panel hidden';
        loadGroups();
      });
    });
    groupsEl.appendChild(add);

    // The charts pinned to the current group, each with a button to take it out.
    var head = document.createElement('div');
    head.className = 'item head';
    head.textContent = t('thisGroup') + ' (' + pinned.length + ')';
    groupsEl.appendChild(head);
    if (!pinned.length) {
      var none = document.createElement('div');
      none.className = 'empty';
      none.textContent = t('noPinsHere');
      groupsEl.appendChild(none);
    }
    pinned.forEach(function (c) {
      var r = document.createElement('div');
      r.className = 'item' + (active && active.meta.id === c.id ? ' current' : '');
      var x = button('pinbtn', t('remove'), function (ev) {
        if (ev.stopPropagation) ev.stopPropagation();
        togglePin({ id: c.id }, function () { drawGroups(); });
      });
      r.appendChild(x);
      r.appendChild(document.createTextNode(label(c.name)));
      r.addEventListener('click', function () { openChart(c.id); });
      groupsEl.appendChild(r);
    });
  }

  // ---- on-screen keyboard (typing may not reach the iframe inside the EFB) ----
  function buildKeyboard() {
    var rows = ['1234567890', 'QWERTYUIOP', 'ASDFGHJKL', 'ZXCVBNM'];
    rows.forEach(function (r) {
      var d = document.createElement('div');
      d.className = 'krow';
      r.split('').forEach(function (ch) {
        d.appendChild(button('', ch, function () { type(ch.toLowerCase()); }));
      });
      kbdEl.appendChild(d);
    });
    var last = document.createElement('div');
    last.className = 'krow';
    last.appendChild(button('k-wide', '____', function () { type(' '); }));
    last.appendChild(button('', '<', function () { qEl.value = qEl.value.slice(0, -1); scheduleSearch(); }));
    last.appendChild(button('', 'C', function () { qEl.value = ''; runSearch(); }));
    kbdEl.appendChild(last);
  }

  function type(ch) {
    qEl.value += ch;
    scheduleSearch();
  }

  // ---- wiring ----
  function checkSize() {
    var w = document.documentElement.clientWidth;
    var h = document.documentElement.clientHeight;
    if (w !== lastW || h !== lastH) {
      lastW = w;
      lastH = h;
      if (map) {
        map.invalidateSize({ animate: false });
        if (needsFit) fitChart();
      }
    }
  }

  qEl.placeholder = t('search');
  viewOverlayBtn.setAttribute('aria-label', t('viewWithMap'));
  viewChartBtn.setAttribute('aria-label', t('viewChartOnly'));
  closeBtn.setAttribute('aria-label', t('closeChart'));
  closeBtn.addEventListener('click', closeChart);
  stripBtn.addEventListener('click', function () {
    stripVisible = !stripVisible;
    store('strip', stripVisible ? '1' : '0');
    applyStripVisibility();
  });
  applyStripVisibility();
  updateCloseButton();
  blendBtn.addEventListener('click', cycleBlend);
  viewOverlayBtn.title = viewWithMapTitle();
  viewChartBtn.title = viewChartOnlyTitle();
  pinPrev.title = t('prevChart');
  pinNext.title = t('nextChart');
  simNoteEl.textContent = t('simNote');
  starBtn.addEventListener('click', function () {
    if (active) togglePin({ id: active.meta.id }, function () { renderPins(); });
  });
  updateStar();
  qEl.addEventListener('input', scheduleSearch);
  qEl.addEventListener('keyup', function (ev) { if (ev.keyCode === 27) { qEl.value = ''; hidePanels(); } });
  el('btn-kbd').addEventListener('click', function () {
    var hidden = kbdEl.className.indexOf('hidden') >= 0;
    kbdEl.className = hidden ? 'kbd' : 'kbd hidden';
  });
  groupBtn.addEventListener('click', showGroups);
  viewOverlayBtn.addEventListener('click', function () { setMode('overlay'); });
  viewChartBtn.addEventListener('click', function () { setMode('chart'); });
  pinPrev.addEventListener('click', function () { scrollStrip(-1); });
  pinNext.addEventListener('click', function () { scrollStrip(1); });
  pinsEl.addEventListener('scroll', updateStripArrows);
  el('btn-zoom-in').addEventListener('click', function () { map.zoomIn(1, { animate: false }); });
  el('btn-zoom-out').addEventListener('click', function () { map.zoomOut(1, { animate: false }); });
  followBtn.addEventListener('click', function () { setFollow(!follow); });
  el('btn-fit').addEventListener('click', function () { setFollow(false); fitChart(); });
  // The simulator keeps the EFB app open, so the page cannot be closed and reopened: this reloads it.
  el('btn-reload').title = lang === 'pt' ? 'Recarregar' : 'Reload';
  el('btn-reload').addEventListener('click', function () { window.location.reload(); });
  window.addEventListener('resize', checkSize);

  window.LMM_DEBUG = {
    map: function () { return map; },
    active: function () { return active; },
    mode: function () { return mode; }
  };

  if (params.q) { qEl.value = params.q; }
  window.setInterval(softRefresh, 15000);

  // Count the changes made to the map's DOM (nodes added/removed, style/class/src attributes).
  function watchDom() {
    if (typeof MutationObserver === 'undefined') return;
    var mo = new MutationObserver(function (list) {
      for (var i = 0; i < list.length; i++) {
        if (list[i].type === 'childList') {
          perf.domChildren = (perf.domChildren || 0) + list[i].addedNodes.length + list[i].removedNodes.length;
        } else {
          bump('domAttr');
        }
      }
    });
    mo.observe(el('map'), { subtree: true, childList: true, attributes: true, attributeFilter: ['style', 'class', 'src'] });
  }

  // Every 10 s: tell the server what this page did (counts only, nothing about the user).
  // The state of the map as the browser really has it (not what our code believes): how many tiles
  // exist, how many have an image, how many are visible, frames painted, timer stalls, errors.
  var frames = 0;
  var maxGap = 0;
  var lastTick = 0;
  var lastError = '';
  function rafLoop() { frames += 1; if (window.requestAnimationFrame) window.requestAnimationFrame(rafLoop); }
  function tickGap() {
    var n = new Date().getTime();
    if (lastTick) maxGap = Math.max(maxGap, n - lastTick - 250);
    lastTick = n;
  }
  window.onerror = function (msg, src, line) {
    bump('jsErrors');
    lastError = String(msg).slice(0, 120) + ' @' + String(src || '').split('/').pop() + ':' + line;
  };
  function health(snapshot) {
    var imgs = document.querySelectorAll('#map .leaflet-tile-pane img.leaflet-tile');
    var total = 0;
    var ok = 0;
    var vis = 0;
    var chart = 0;
    var chartOk = 0;
    var box = el('map').getBoundingClientRect();
    for (var i = 0; i < imgs.length; i++) {
      var im = imgs[i];
      var isChart = im.className.indexOf('chart-tile') >= 0;
      var good = im.complete && im.naturalWidth > 0;
      if (isChart) { chart += 1; if (good) chartOk += 1; } else { total += 1; if (good) ok += 1; }
      var r = im.getBoundingClientRect();
      if (r.right > box.left && r.left < box.right && r.bottom > box.top && r.top < box.bottom) vis += 1;
    }
    snapshot.h_baseTiles = total;
    snapshot.h_baseOk = ok;
    snapshot.h_chartTiles = chart;
    snapshot.h_chartOk = chartOk;
    snapshot.h_inView = vis;
    snapshot.h_frames = frames;
    snapshot.h_maxTimerGapMs = Math.round(maxGap);
    snapshot.h_hidden = document.hidden ? 1 : 0;
    // What this browser really does with the CSS the map library uses (not what its version says).
    var probe = document.createElement('div');
    probe.style.mixBlendMode = 'plus-lighter';
    snapshot.h_plusLighterAccepted = probe.style.mixBlendMode === 'plus-lighter' ? 1 : 0;
    probe.style.mixBlendMode = 'multiply';
    snapshot.h_blendModesKnown = probe.style.mixBlendMode === 'multiply' ? 1 : 0;
    snapshot.h_cssSupportsApi = (window.CSS && typeof window.CSS.supports === 'function') ? 1 : 0;
    var oneTile = document.querySelector('#map img.leaflet-tile');
    if (oneTile) {
      var bm = window.getComputedStyle(oneTile).mixBlendMode;
      snapshot.h_tileBlendNormal = bm === 'normal' ? 1 : 0;
    }
    var pane = document.querySelector('#map .leaflet-map-pane');
    if (pane) {
      var pr = pane.getBoundingClientRect();
      snapshot.h_paneLeft = Math.round(pr.left);
      snapshot.h_paneTop = Math.round(pr.top);
      snapshot.h_paneStyleLeft = parseInt(pane.style.left, 10) || 0;
      snapshot.h_paneStyleTop = parseInt(pane.style.top, 10) || 0;
    }
    snapshot.h_zoom = map ? map.getZoom() : -1;
    snapshot.h_layers = document.querySelectorAll('#map .leaflet-layer').length;
    snapshot.h_tileContainers = document.querySelectorAll('#map .leaflet-tile-container').length;
    snapshot.h_allImgs = document.querySelectorAll('img').length;
    snapshot.h_w = Math.round(box.width);
    snapshot.h_h = Math.round(box.height);
    frames = 0;
    maxGap = 0;
  }
  function reportPerf(mark) {
    var snapshot = perf;
    perf = {};
    health(snapshot);
    var note = lastError;
    lastError = '';
    if (mark === true) note = 'MARK (user says the map is dark) ' + note;
    request('POST', '/api/client-perf', { ua: navigator.userAgent, seconds: 10, mode: mode, counts: snapshot, note: note }, null);
  }
  rafLoop();
  window.setInterval(tickGap, 250);
  window.setInterval(function () { reportPerf(false); }, 10000);
  // Tapping the status line tells the server "the map looks dark right now" (see /api/client-perf).
  statusEl.addEventListener('click', function () { reportPerf(true); });

  // Tell the server what this browser can do (WebGL, pixel ratio...). Read in the manager's Setup.
  // Only facts about the browser; nothing about the user.
  function reportClient() {
    var info = { ua: navigator.userAgent, dpr: window.devicePixelRatio || 1, width: window.innerWidth,
      height: window.innerHeight, touch: 'ontouchstart' in window || (navigator.maxTouchPoints || 0) > 0,
      webgl: false, webgl2: false, renderer: '' };
    try {
      var c = document.createElement('canvas');
      var gl = c.getContext('webgl') || c.getContext('experimental-webgl');
      info.webgl = !!gl;
      if (gl) {
        var ext = gl.getExtension('WEBGL_debug_renderer_info');
        info.renderer = String(ext ? gl.getParameter(ext.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER) || '');
      }
      info.webgl2 = !!document.createElement('canvas').getContext('webgl2');
    } catch (e) { /* no WebGL */ }
    request('POST', '/api/client-info', info, null);
  }
  window.setTimeout(reportClient, 2500);

  function start(saved) {
    saved = saved || {};
    if (!params.mode && (saved.mode === 'chart' || saved.mode === 'overlay')) mode = saved.mode;
    if (!params.basemap && /^(standard|light|dark|none)$/.test(saved.basemap || '')) basemap = saved.basemap;
    if (!params.lang && (saved.lang === 'pt' || saved.lang === 'en')) {
      lang = saved.lang;
      qEl.placeholder = t('search');
      simNoteEl.textContent = t('simNote');
    }
    buildKeyboard();
    buildMap();
    checkSize();
    window.setInterval(checkSize, 1000);
    poll();
    watchDom();
    loadGroups(function () {
      if (params.q) runSearch();
      var want = params.chart || saved.chart || store('chart');
      var closedByUser = !params.chart && saved.chart === '' && !!saved.mode;
      if (want) {
        openChart(want);
      } else if (pinned.length && !closedByUser) {
        openChart(pinned[0].id);
      }
    });
  }

  // Wait briefly for the saved state; never block the viewer on it.
  var started = false;
  function startOnce(saved) { if (!started) { started = true; start(saved); } }
  window.setTimeout(function () { startOnce(null); }, 1500);
  getJson('/api/viewer-state', function (st) { startOnce(st); });
})();
