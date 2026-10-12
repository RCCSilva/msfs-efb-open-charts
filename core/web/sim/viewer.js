/*
  Open Charts - in-sim viewer. ES5 on purpose (MSFS embedded browser, Chrome ~49).

  One page, two modes for the same chart:
    overlay  the chart warped over OpenStreetMap tiles (Leaflet map, Web Mercator)
    chart    the chart image alone, aircraft placed with the chart's georeference (CRS.Simple)
  Search finds charts by ICAO, type, words ("sao paulo", "ils 17r"); a chart can be pinned to the
  current flight, and pinned charts are one tap away in the bar.

  Debug: /viewer.html?chart=<id>&mode=chart|overlay&at=<lat>,<lon>[,<heading>]&lang=pt|en
*/
(function () {
  'use strict';

  var POLL_MS = 250;
  var TILE = 256;
  var MESH = 8;
  var OPACITY_STEPS = [0.35, 0.6, 0.85, 1];
  var CHART_MIN_ZOOM = -5;
  var CHART_MAX_ZOOM = 2;
  var OVERLAY_MIN_ZOOM = 3;
  var OVERLAY_MAX_ZOOM = 16;

  var STR = {
    en: {
      search: 'Search charts: SBSP, ILS, sao paulo...',
      noPins: 'Search a chart and pin it to this flight.',
      noResults: 'No charts found.',
      pin: 'Pin', pinned: 'Pinned',
      modeOverlay: 'On map', modeChart: 'Chart only',
      flight: 'Flight', newFlight: 'New flight',
      waitSim: 'Waiting for MSFS (SimConnect)...',
      waitPos: 'Connected - waiting for position (start a flight)',
      unreachable: 'Server unreachable',
      pick: 'Pick a chart (search above)',
      offChart: 'OFF CHART',
      loading: 'Loading chart...',
      loadFail: 'Could not load the chart',
      fixed: 'FIXED',
      simNote: 'FLIGHT SIMULATION ONLY - NOT FOR REAL-WORLD AVIATION',
      starPin: 'Pin this chart to the flight', starUnpin: 'Remove this chart from the flight',
      thisFlight: 'Charts in this flight', noPinsHere: 'No charts pinned in this flight.', remove: 'Remove'
    },
    pt: {
      search: 'Buscar cartas: SBSP, ILS, sao paulo...',
      noPins: 'Busque uma carta e fixe neste voo.',
      noResults: 'Nenhuma carta encontrada.',
      pin: 'Fixar', pinned: 'Fixada',
      modeOverlay: 'No mapa', modeChart: 'Só a carta',
      flight: 'Voo', newFlight: 'Novo voo',
      waitSim: 'Aguardando o MSFS (SimConnect)...',
      waitPos: 'Conectado - aguardando posição (inicie um voo)',
      unreachable: 'Servidor inacessível',
      pick: 'Escolha uma carta (busca acima)',
      offChart: 'FORA DA CARTA',
      loading: 'Carregando carta...',
      loadFail: 'Não foi possível carregar a carta',
      fixed: 'FIXO',
      simNote: 'APENAS SIMULAÇÃO AÉREA - PROIBIDO O USO EM AVIAÇÃO REAL',
      starPin: 'Fixar esta carta no voo', starUnpin: 'Tirar esta carta do voo',
      thisFlight: 'Cartas deste voo', noPinsHere: 'Nenhuma carta fixada neste voo.', remove: 'Tirar'
    }
  };

  var el = function (id) { return document.getElementById(id); };
  var qEl = el('q');
  var pinsEl = el('pins');
  var resultsEl = el('results');
  var flightsEl = el('flights');
  var kbdEl = el('kbd');
  var statusEl = el('status');
  var followBtn = el('btn-follow');
  var modeBtn = el('btn-mode');
  var flightBtn = el('btn-flight');
  var opacityBtn = el('btn-opacity');
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

  var mode = params.mode || store('mode') || 'overlay';
  if (mode !== 'chart' && mode !== 'overlay') mode = 'overlay';

  var debugAt = null;
  if (params.at) {
    var pa = params.at.split(',').map(Number);
    if (pa.length >= 2 && isFinite(pa[0]) && isFinite(pa[1])) {
      debugAt = { lat: pa[0], lon: pa[1], headingTrue: isFinite(pa[2]) ? pa[2] : 0 };
    }
  }

  var map = null;
  var marker = null;
  var planeRot = null;
  var chartLayer = null;
  var active = null;            // { meta, img }
  var flights = { current: '', flights: [] };
  var pinned = [];              // chart objects of the current flight, in pin order
  var follow = true;
  var followJustEnabled = false;
  var lastKey = null;
  var lastFix = null;
  var lastStatusText = null;
  var lastStatusKind = null;
  var lastW = 0;
  var lastH = 0;
  var opacityIdx = 2;
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

  var WarpLayer = L.GridLayer.extend({
    initialize: function (meta, img, options) {
      L.GridLayer.prototype.initialize.call(this, options);
      this._meta = meta;
      this._img = img;
    },
    createTile: function (coords, done) {
      var tile = document.createElement('canvas');
      tile.width = TILE;
      tile.height = TILE;
      tile.className = 'leaflet-tile';
      var meta = this._meta;
      var img = this._img;
      var frame = meta.mapFramePx;
      var step = TILE / MESH;
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
        var ctx = tile.getContext('2d');
        ctx.imageSmoothingEnabled = true;
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
      }
      window.setTimeout(function () { done(null, tile); }, 0);
      return tile;
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
      markerZoomAnimation: false, inertia: false, tap: false, touchZoom: false, keyboard: false,
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
      L.tileLayer('/tiles/{z}/{x}/{y}.png', {
        minZoom: OVERLAY_MIN_ZOOM, maxZoom: OVERLAY_MAX_ZOOM, tileSize: 256, updateWhenIdle: false, keepBuffer: 2
      }).addTo(map);
    }
    var icon = L.divIcon({
      className: 'plane-icon',
      html: '<div class="plane-rot">' + PLANE_SVG + '</div>',
      iconSize: [40, 40],
      iconAnchor: [20, 20]
    });
    marker = L.marker([0, 0], { icon: icon, interactive: false, keyboard: false, zIndexOffset: 1000 });
    map.on('dragstart', function () { setFollow(false); });

    attribEl.style.display = mode === 'overlay' ? '' : 'none';
    opacityBtn.style.display = mode === 'overlay' ? '' : 'none';
    modeBtn.textContent = mode === 'overlay' ? t('modeOverlay') : t('modeChart');
    mapBackground();
    applyChart();
    if (lastFix) showAircraft(lastFix);
  }

  function mapBackground() {
    el('map').style.background = mode === 'chart' ? '#ffffff' : '#dfe6ea';
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
        tileSize: TILE, opacity: OPACITY_STEPS[opacityIdx], minZoom: OVERLAY_MIN_ZOOM,
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
    var key = ll.lat.toFixed(5) + '|' + ll.lng.toFixed(5) + '|' + Math.round(rot);
    var changed = key !== lastKey;
    if (!map.hasLayer(marker)) {
      marker.addTo(map);
      planeRot = null;
      changed = true;
    }
    if (changed) {
      lastKey = key;
      marker.setLatLng(ll);
      rotatePlane(rot);
    }
    var name = active ? label(active.meta.name) + '  |  ' : '';
    setStatus(name + (p.simulated ? t('fixed') + ' ' : '') + p.lat.toFixed(4) + ', ' + p.lon.toFixed(4) +
      '  HDG ' + Math.round(p.headingTrue) + (inside ? '' : '  -  ' + t('offChart')), inside ? 'ok' : 'warn');

    if (follow && (changed || followJustEnabled) && inside) {
      followJustEnabled = false;
      if (mode === 'overlay') {
        map.setView(ll, map.getZoom(), { animate: false });
      } else {
        var c = map.latLngToContainerPoint(ll);
        var s = map.getSize();
        var dx = c.x - s.x / 2;
        var dy = c.y - s.y / 2;
        if (Math.abs(dx) >= 1 || Math.abs(dy) >= 1) map.panBy([dx, dy], { animate: false });
      }
    }
  }

  function poll() {
    if (debugAt) {
      showAircraft({ lat: debugAt.lat, lon: debugAt.lon, headingTrue: debugAt.headingTrue, simulated: true });
      return;
    }
    getJson('/api/position?t=' + Date.now(), function (p) {
      if (!p) {
        setStatus(t('unreachable'), 'bad');
      } else if (!p.simConnected) {
        setStatus(t('waitSim'), 'warn');
      } else if (!p.valid) {
        setStatus(t('waitPos'), 'warn');
      } else {
        showAircraft(p);
      }
      window.setTimeout(poll, POLL_MS);
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
    getJson('/charts/' + encodeURIComponent(id) + '/chart.json', function (meta) {
      if (!meta) { loadFailed(id, fallback); return; }
      var img = new Image();
      img.onload = function () {
        active = { meta: meta, img: img };
        store('chart', id);
        setFollow(!debugAt);
        buildMap();
        renderPins();
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

  function currentFlight() {
    for (var i = 0; i < flights.flights.length; i++) {
      if (flights.flights[i].id === flights.current) return flights.flights[i];
    }
    return null;
  }

  function loadFlights(cb) {
    getJson('/api/flights', function (st) {
      if (st) flights = st;
      var fl = currentFlight();
      flightBtn.textContent = t('flight') + ': ' + (fl ? fl.name : '-');
      if (!fl || !fl.pins.length) {
        pinned = [];
        renderPins();
        if (cb) cb();
        return;
      }
      getJson('/api/charts?flight=' + encodeURIComponent(fl.id) + '&limit=200', function (list) {
        var byId = {};
        (list || []).forEach(function (c) { byId[c.id] = c; });
        pinned = [];
        fl.pins.forEach(function (id) { if (byId[id]) pinned.push(byId[id]); });   // pin order
        renderPins();
        if (cb) cb();
      });
    });
  }

  function togglePin(chart, done) {
    var fl = currentFlight();
    if (!fl) return;
    var url = '/api/flights/' + encodeURIComponent(fl.id) + '/pins/' + encodeURIComponent(chart.id);
    request(isPinned(chart.id) ? 'DELETE' : 'PUT', url, null, function () {
      loadFlights(done);
    });
  }

  // The star pins/unpins the chart on screen for the current flight.
  function updateStar() {
    var on = !!active && isPinned(active.meta.id);
    starBtn.disabled = !active;
    starBtn.className = 'tool' + (on ? ' on' : '');
    starBtn.title = on ? t('starUnpin') : t('starPin');
  }

  function renderPins() {
    updateStar();
    clear(pinsEl);
    if (!pinned.length) {
      var h = document.createElement('span');
      h.className = 'hint';
      h.textContent = t('noPins');
      pinsEl.appendChild(h);
      return;
    }
    pinned.forEach(function (c) {
      var short = (c.icao ? c.icao + ' ' : '') + (c.type ? c.type + ' ' : '') + (c.title || '');
      var b = button('chip' + (active && active.meta.id === c.id ? ' active' : ''), (short.replace(/\s+$/, '') || c.name), function () { openChart(c.id); });
      pinsEl.appendChild(b);
    });
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
      flightsEl.className = 'panel hidden';
      renderResults(list);
    });
  }

  function scheduleSearch() {
    if (searchTimer) window.clearTimeout(searchTimer);
    searchTimer = window.setTimeout(runSearch, 180);
  }

  function hidePanels() {
    resultsEl.className = 'panel hidden';
    flightsEl.className = 'panel hidden';
  }

  // ---- flights panel ----
  function showFlights() {
    if (flightsEl.className.indexOf('hidden') < 0) { flightsEl.className = 'panel hidden'; return; }
    resultsEl.className = 'panel hidden';
    drawFlights();
  }

  function drawFlights() {
    clear(flightsEl);
    flightsEl.className = 'panel';
    flights.flights.forEach(function (f) {
      var row = document.createElement('div');
      row.className = 'item' + (f.id === flights.current ? ' current' : '');
      row.textContent = f.name + '  (' + f.pins.length + ')';
      row.addEventListener('click', function () {
        request('PUT', '/api/flights/current', { id: f.id }, function () {
          flightsEl.className = 'panel hidden';
          loadFlights();
        });
      });
      flightsEl.appendChild(row);
    });
    var add = document.createElement('div');
    add.className = 'item';
    add.textContent = '+ ' + t('newFlight');
    add.addEventListener('click', function () {
      var n = flights.flights.length + 1;
      request('POST', '/api/flights', { name: t('flight') + ' ' + n }, function () {
        flightsEl.className = 'panel hidden';
        loadFlights();
      });
    });
    flightsEl.appendChild(add);

    // The charts pinned to the current flight, each with a button to take it out.
    var head = document.createElement('div');
    head.className = 'item head';
    head.textContent = t('thisFlight') + ' (' + pinned.length + ')';
    flightsEl.appendChild(head);
    if (!pinned.length) {
      var none = document.createElement('div');
      none.className = 'empty';
      none.textContent = t('noPinsHere');
      flightsEl.appendChild(none);
    }
    pinned.forEach(function (c) {
      var r = document.createElement('div');
      r.className = 'item' + (active && active.meta.id === c.id ? ' current' : '');
      var x = button('pinbtn', '\u2715 ' + t('remove'), function (ev) {
        if (ev.stopPropagation) ev.stopPropagation();
        togglePin({ id: c.id }, function () { drawFlights(); });
      });
      r.appendChild(x);
      r.appendChild(document.createTextNode(label(c.name)));
      r.addEventListener('click', function () { openChart(c.id); });
      flightsEl.appendChild(r);
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
    last.appendChild(button('', '⌫', function () { qEl.value = qEl.value.slice(0, -1); scheduleSearch(); }));
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
  flightBtn.addEventListener('click', showFlights);
  modeBtn.addEventListener('click', function () {
    mode = mode === 'overlay' ? 'chart' : 'overlay';
    store('mode', mode);
    buildMap();
    saveState();
  });
  el('btn-zoom-in').addEventListener('click', function () { map.zoomIn(1, { animate: false }); });
  el('btn-zoom-out').addEventListener('click', function () { map.zoomOut(1, { animate: false }); });
  followBtn.addEventListener('click', function () { setFollow(!follow); });
  el('btn-fit').addEventListener('click', function () { setFollow(false); fitChart(); });
  opacityBtn.addEventListener('click', function () {
    opacityIdx = (opacityIdx + 1) % OPACITY_STEPS.length;
    if (chartLayer && chartLayer.setOpacity && mode === 'overlay') chartLayer.setOpacity(OPACITY_STEPS[opacityIdx]);
  });
  window.addEventListener('resize', checkSize);

  window.LMM_DEBUG = {
    map: function () { return map; },
    active: function () { return active; },
    mode: function () { return mode; }
  };

  if (params.q) { qEl.value = params.q; }
  function start(saved) {
    saved = saved || {};
    if (!params.mode && (saved.mode === 'chart' || saved.mode === 'overlay')) mode = saved.mode;
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
    loadFlights(function () {
      if (params.q) runSearch();
      var want = params.chart || saved.chart || store('chart');
      if (want) {
        openChart(want);
      } else if (pinned.length) {
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
