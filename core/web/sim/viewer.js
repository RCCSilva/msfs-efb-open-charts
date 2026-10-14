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
  // Chart-only mode shows a reduced copy of the chart picture that matches the zoom (the server makes
  // chart@2.png and chart@4.png): sharper than the browser shrinking a big picture, and much faster to
  // decode in the simulator. 1 = the full picture, 2 = half size, 4 = quarter size.
  var CHART_LEVELS = [4, 2, 1];

  var STR = {
    en: {
      search: 'Paste a route: SBSJ BOBI3A DAGUB UZ6 ... SBGR',
      noPins: 'No charts pinned to this group.',
      routePaste: 'Paste a route', routeTitle: 'Route', routePlaces: 'places', routeProblems: 'to check',
      routeNone: 'No route yet', routePasteShort: 'Paste', routeFrom: 'from', routeTo: 'to',
      pinnedTitle: 'Pinned charts', searchTitle: 'Search charts', searchPlaceholder: 'SBSP, ILS, sao paulo...',
      searchHint: 'Type to search. With a route, the charts of its airports appear here.', noResults: 'No charts found.',
      noChartsAirport: 'No charts of this airport in the library.', remove: 'Remove',
      routeWorking: 'Reading the route...', routeResolve: 'Resolve', routeFailed: 'Could not read the route',
      routeClear: 'Clear', routeCancel: 'Close', routeDirect: 'Direct to', routeRemove: 'Remove', routeMenu: 'Options',
      routeBoxTitle: 'Paste the route: SBSJ BOBI3A DAGUB UZ6 ... SBGR',
      pin: 'Pin', pinned: 'Pinned',
      modeOverlay: 'On map', modeChart: 'Chart only',
      waitSim: 'Waiting for MSFS (SimConnect)...',
      waitPos: 'Connected - waiting for position (start a flight)',
      unreachable: 'Server unreachable',
      pick: 'Pick a chart (search above)',
      offChart: 'OFF CHART',
      loading: 'Loading chart',
      viewOnly: 'View-only chart: no georeference, no aircraft position',
      viewOnlyMap: 'This chart has no georeference: it cannot be placed on the map',
      loadingSlow: 'Big charts take a while',
      loadFail: 'Could not load the chart',
      fixed: 'FIXED',
      simNote: 'FLIGHT SIMULATION ONLY - NOT FOR REAL-WORLD AVIATION',
      regionalTitle: 'Regional zoom (about 30 km around the aircraft)',
      starPin: 'Pin this chart', starUnpin: 'Unpin this chart', remove: 'Unpin',
      multiTitle: 'Several charts on the map', multiOn: 'Several charts: on', multiOff: 'Several charts: off',
      scrollUp: 'Up', scrollDown: 'Down',
      viewWithMap: 'With map', viewChartOnly: 'Chart only', blendMap: 'Map', blendChart: 'Chart',
      blendTitle: 'Chart over the map', otherGroup: 'Other',
      prevChart: 'Scroll left', nextChart: 'Scroll right', closeChart: 'Close chart',
      showPinned: 'Pinned charts: hide the list', hidePinned: 'Pinned charts: show the list'
    },
    pt: {
      search: 'Cole a rota: SBSJ BOBI3A DAGUB UZ6 ... SBGR',
      noPins: 'Nenhuma carta fixada neste grupo.',
      routePaste: 'Colar a rota', routeTitle: 'Rota', routePlaces: 'pontos', routeProblems: 'a conferir',
      routeNone: 'Nenhuma rota ainda', routePasteShort: 'Colar', routeFrom: 'origem', routeTo: 'destino',
      pinnedTitle: 'Cartas fixadas', searchTitle: 'Buscar cartas', searchPlaceholder: 'SBSP, ILS, sao paulo...',
      searchHint: 'Digite para buscar. Com uma rota, as cartas dos aeroportos dela aparecem aqui.', noResults: 'Nenhuma carta encontrada.',
      noChartsAirport: 'Nenhuma carta deste aeroporto na biblioteca.', remove: 'Remover',
      routeWorking: 'Lendo a rota...', routeResolve: 'Resolver', routeFailed: 'Não foi possível ler a rota',
      routeClear: 'Limpar', routeCancel: 'Fechar', routeDirect: 'Direto para', routeRemove: 'Remover', routeMenu: 'Opções',
      routeBoxTitle: 'Cole a rota: SBSJ BOBI3A DAGUB UZ6 ... SBGR',
      pin: 'Fixar', pinned: 'Fixada',
      modeOverlay: 'No mapa', modeChart: 'Só a carta',
      waitSim: 'Aguardando o MSFS (SimConnect)...',
      waitPos: 'Conectado - aguardando posição (inicie um voo)',
      unreachable: 'Servidor inacessível',
      pick: 'Escolha uma carta (busca acima)',
      offChart: 'FORA DA CARTA',
      loading: 'Carregando carta',
      viewOnly: 'Carta apenas para visualizar: sem georreferenciamento, sem posição da aeronave',
      viewOnlyMap: 'Esta carta não tem georreferenciamento: não dá para colocá-la sobre o mapa',
      loadingSlow: 'Cartas grandes demoram mais',
      loadFail: 'Não foi possível carregar a carta',
      fixed: 'FIXO',
      simNote: 'APENAS SIMULAÇÃO AÉREA - PROIBIDO O USO EM AVIAÇÃO REAL',
      regionalTitle: 'Zoom regional (cerca de 30 km ao redor da aeronave)',
      starPin: 'Fixar esta carta', starUnpin: 'Desafixar esta carta', remove: 'Desafixar',
      multiTitle: 'Varias cartas no mapa', multiOn: 'Varias cartas: ligado', multiOff: 'Varias cartas: desligado',
      scrollUp: 'Subir', scrollDown: 'Descer',
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
  var kbdEl = el('kbd');
  var statusEl = el('status');
  var followBtn = el('btn-follow');
  var multiBtn = el('btn-multi');
  var viewOverlayBtn = el('view-overlay');
  var viewChartBtn = el('view-chart');
  var blendBtn = el('btn-blend');
  var closeBtn = el('btn-close');
  var stripBtn = el('btn-strip');
  var searchBtn = el('btn-search');
  var routeDrawerEl = el('routedrawer');
  var chartDrawerEl = el('chartdrawer');
  var pinnedViewEl = el('cd-pinned');
  var searchViewEl = el('cd-search');
  var cqEl = el('cq');
  var kbd2El = el('kbd2');
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
  // How long opening a chart takes, phase by phase, measured in the browser that shows it (the
  // simulator's is much slower than a PC's). Sent once per chart to /api/client-perf.
  var loadTiming = null;
  function nowMs() { return new Date().getTime(); }
  function reportLoadTiming(t) {
    var counts = {
      ms_json: t.json - t.start, ms_image: t.image - t.json, ms_build: t.built - t.image,
      ms_tiles: t.tiles - t.built, ms_total: t.tiles - t.start,
      tilesDrawn: t.drawn, ms_drawingTiles: t.drawMs, imgW: t.w, imgH: t.h, level: t.level || 1
    };
    request('POST', '/api/client-perf', { ua: navigator.userAgent, seconds: 0, mode: 'chart-open:' + t.mode, counts: counts, note: t.id.slice(0, 80) }, null);
  }
  var lastPollSig = '';

  var map = null;
  var marker = null;
  var planeRot = null;
  var chartLayer = null;
  // several charts on the map at once (map view only): the others are under the active one
  var multi = store('multi') === '1';
  var extras = [];              // [{ meta, img, level }], oldest first
  var extraLayers = [];
  var MAX_EXTRAS = 4;
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
  function request(method, url, body, cb, timeoutMs) {
    var xhr = new XMLHttpRequest();
    xhr.open(method, url);
    xhr.timeout = timeoutMs || 8000;
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

  // A big chart is cut into a pyramid of tiles by the server (see importer/pyramid.go). Tile z of the
  // pyramid is shown at map zoom z: zoom 0 is the resolution of chart.png, zoom meta.tiles.maxZoom the
  // full one. the chart hangs from y = 0 downwards (see chartTop), so Leaflet's tile row is the picture's row.
  var ChartTileLayer = L.TileLayer.extend({
    initialize: function (meta, options) {
      this._meta = meta;
      L.TileLayer.prototype.initialize.call(this, '', options);
    },
    // Leaflet scales a tile level with a 3D transform, which this viewer turns off (L_DISABLE_3D);
    // without it a fractional zoom would show the tiles at the wrong size. A 2D transform does it.
    _setZoomTransform: function (level, center, zoom) {
      var scale = this._map.getZoomScale(zoom, level.zoom);
      var translate = level.origin.multiplyBy(scale).subtract(this._map._getNewPixelOrigin(center, zoom)).round();
      var v = 'translate(' + translate.x + 'px,' + translate.y + 'px) scale(' + scale + ')';
      level.el.style.webkitTransformOrigin = '0 0';
      level.el.style.transformOrigin = '0 0';
      level.el.style.webkitTransform = v;
      level.el.style.transform = v;
    },
    getTileUrl: function (coords) {
      var t = this._meta.tiles;
      var z = this._getZoomForUrl();
      var shift = t.maxZoom - z;
      var w = Math.ceil(t.width / Math.pow(2, shift));
      var h = Math.ceil(t.height / Math.pow(2, shift));
      var cols = Math.ceil(w / t.size);
      var rows = Math.ceil(h / t.size);
      var row = coords.y;
      if (coords.x < 0 || coords.x >= cols || row < 0 || row >= rows) return BLANK_TILE;
      return '/charts/' + encodeURIComponent(this._meta.id) + '/tiles/' + z + '/' + coords.x + '/' + row + '.png';
    }
  });

  // Tiles of a pyramid for the overlay: the pictures are kept a while (a pan reuses them).
  var tileImages = {};
  var tileImageOrder = [];
  function chartTileImage(url, cb) {
    var rec = tileImages[url];
    if (rec) {
      if (rec.ok) { cb(rec.img); } else { rec.waiting.push(cb); }
      return;
    }
    rec = tileImages[url] = { img: new Image(), ok: false, waiting: [cb] };
    tileImageOrder.push(url);
    if (tileImageOrder.length > 120) delete tileImages[tileImageOrder.shift()];
    rec.img.onload = function () {
      rec.ok = true;
      var w = rec.waiting;
      rec.waiting = [];
      for (var i = 0; i < w.length; i++) w[i](rec.img);
    };
    rec.img.onerror = function () {
      delete tileImages[url];
      var w = rec.waiting;
      rec.waiting = [];
      for (var i = 0; i < w.length; i++) w[i](null);
    };
    rec.img.src = url;
  }

  // The source of an overlay tile from the pyramid: the part of level z (picture pixels = overview
  // pixels * 2^z) that the tile needs, assembled in one canvas. Calls back with the canvas and its
  // origin (in level pixels), or with null when there is nothing to draw.
  function pyramidRegion(meta, z, x0, y0, x1, y1, cb) {
    var t = meta.tiles;
    var k = Math.pow(2, z);
    var rx0 = Math.max(0, Math.floor(x0 * k));
    var ry0 = Math.max(0, Math.floor(y0 * k));
    var rx1 = Math.min(Math.ceil(x1 * k), Math.ceil(t.width / Math.pow(2, t.maxZoom - z)));
    var ry1 = Math.min(Math.ceil(y1 * k), Math.ceil(t.height / Math.pow(2, t.maxZoom - z)));
    if (rx1 <= rx0 || ry1 <= ry0) { cb(null); return; }
    var cv = document.createElement('canvas');
    cv.width = rx1 - rx0;
    cv.height = ry1 - ry0;
    var cx = cv.getContext('2d');
    var c0 = Math.floor(rx0 / t.size), c1 = Math.floor((rx1 - 1) / t.size);
    var r0 = Math.floor(ry0 / t.size), r1 = Math.floor((ry1 - 1) / t.size);
    var pending = (c1 - c0 + 1) * (r1 - r0 + 1);
    function one(col, row) {
      var url = '/charts/' + encodeURIComponent(meta.id) + '/tiles/' + z + '/' + col + '/' + row + '.png';
      chartTileImage(url, function (img) {
        if (img) cx.drawImage(img, col * t.size - rx0, row * t.size - ry0);
        pending -= 1;
        if (pending === 0) cb({ canvas: cv, x: rx0, y: ry0, k: k });
      });
    }
    for (var row = r0; row <= r1; row++) for (var col = c0; col <= c1; col++) one(col, row);
  }

  var BLANK_TILE = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';

  var WarpLayer = L.GridLayer.extend({
    initialize: function (meta, img, options) {
      L.GridLayer.prototype.initialize.call(this, options);
      this._meta = meta;
      this._img = img;
    },
    createTile: function (coords, done) {
      bump('chartTileDrawn');
      var drawStart = loadTiming ? nowMs() : 0;
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
      var pyr = null;
      if (inside && meta.tiles) {
        // how many pixels of the picture a screen pixel needs: the level that gives about one to one
        var edge = Math.sqrt(Math.pow(src[0][MESH].x - src[0][0].x, 2) + Math.pow(src[0][MESH].y - src[0][0].y, 2)) || 1;
        var lz = Math.ceil(Math.log(TILE * dpr / edge) / Math.LN2 - 0.2);
        lz = Math.min(lz, meta.tiles.maxZoom);
        // never a region bigger than a few screens of pixels
        while (lz > 0 && Math.max(maxX - minX, maxY - minY) * Math.pow(2, lz) > TILE * dpr * 3) lz -= 1;
        if (lz > 0) pyr = lz;
      }
      if (pyr) {
        var rg = {
          x0: Math.max(frame.left, minX) - 1, y0: Math.max(frame.top, minY) - 1,
          x1: Math.min(frame.right, maxX) + 1, y1: Math.min(frame.bottom, maxY) + 1
        };
        var ctx2 = tile.getContext('2d');
        ctx2.imageSmoothingEnabled = true;
        if ('imageSmoothingQuality' in ctx2) ctx2.imageSmoothingQuality = 'high';
        var overviewFrame = frame;
        pyramidRegion(meta, pyr, rg.x0, rg.y0, rg.x1, rg.y1, function (region) {
          if (region) {
            var k = region.k;
            var whole = { left: 0, top: 0, right: region.canvas.width, bottom: region.canvas.height };
            var q = [];
            var jj, ii;
            for (jj = 0; jj <= MESH; jj++) {
              q[jj] = [];
              for (ii = 0; ii <= MESH; ii++) q[jj][ii] = { x: src[jj][ii].x * k - region.x, y: src[jj][ii].y * k - region.y };
            }
            for (jj = 0; jj < MESH; jj++) {
              for (ii = 0; ii < MESH; ii++) {
                var a00 = { x: ii * step, y: jj * step };
                var a10 = { x: (ii + 1) * step, y: jj * step };
                var a11 = { x: (ii + 1) * step, y: (jj + 1) * step };
                var a01 = { x: ii * step, y: (jj + 1) * step };
                drawTriangle(ctx2, region.canvas, q[jj][ii], q[jj][ii + 1], q[jj + 1][ii + 1], a00, a10, a11, whole);
                drawTriangle(ctx2, region.canvas, q[jj][ii], q[jj + 1][ii + 1], q[jj + 1][ii], a00, a11, a01, whole);
              }
            }
          }
          if (loadTiming) { loadTiming.drawn += 1; loadTiming.drawMs += nowMs() - drawStart; }
          // Never synchronously: with every picture already loaded this runs inside createTile, before
          // Leaflet has registered the tile, and a tile that is "ready" too early stays hidden for good.
          window.setTimeout(function () { done(null, tile); }, 0);
        });
        return tile;
      }
      if (inside) {
        var ctx = tile.getContext('2d');
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
      }
      if (loadTiming) { loadTiming.drawn += 1; loadTiming.drawMs += nowMs() - drawStart; }
      window.setTimeout(function () { done(null, tile); }, 0);
      return tile;
    }
  });

  // ---------------------------------------------------------------------------
  // Map (rebuilt when the mode changes: the two modes use different CRS)
  // ---------------------------------------------------------------------------
  // The aircraft symbol comes from the settings (the manager's Setup): a plane or a helicopter, in the
  // chosen colour. Both point up at heading 0.
  var HELI_SVG =
    '<svg viewBox="0 0 64 64"><path d="M29.5 36h5v20h-5z"/><path d="M23 54h18v5H23z"/>' +
    '<path d="M32 8c8.5 0 13 8 13 18.5S40.500 41 32 41s-13-4-13-14.500S23.500 8 32 8z"/>' +
    '<path d="M8 13l4-3.500 44 38-4 3.500z"/><path d="M56 13l-4-3.500-44 38 4 3.500z"/></svg>';
  var aircraft = 'plane';
  var aircraftHex = '#ffd21f';

  function aircraftSvg() {
    var svg = aircraft === 'helicopter' ? HELI_SVG : PLANE_SVG;
    return svg.replace('<svg ', '<svg style="fill:' + aircraftHex + '" ');
  }

  var PLANE_SVG =
    '<svg viewBox="0 0 64 64"><path d="M32 4L35 10L36 24L58 40L58 45L36 38L35 50L42 57L42 61' +
    'L32 58L22 61L22 57L29 50L28 38L6 45L6 40L28 24L29 10Z"/></svg>';

  function hasSize() {
    var s = map.getSize();
    return s.x > 0 && s.y > 0;
  }

  // In chart-only mode the picture lives in Leaflet's Simple CRS (y up). A chart with a tile pyramid
  // hangs from y = 0 downwards (latitude = -row), so that the tile grid, which Leaflet anchors at 0,
  // starts at the top of the picture, where the tiles were cut; any other chart stands on y = 0.
  function chartTop(meta) { return meta.tiles ? 0 : meta.image.height; }
  function chartBounds(meta) {
    var top = chartTop(meta);
    return L.latLngBounds([top - meta.image.height, 0], [top, meta.image.width]);
  }
  function chartPixelToLatLng(pt) { return L.latLng(chartTop(active.meta) - pt.py, pt.px); }

  function buildMap() {
    if (map) {
      map.off();
      map.remove();
      map = null;
    }
    chartLayer = null;
    extraLayers = [];
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
      common.maxZoom = active && active.meta.tiles ? Math.max(CHART_MAX_ZOOM, active.meta.tiles.maxZoom) : CHART_MAX_ZOOM;
      common.zoomSnap = 0.25;
      map = L.map('map', common);
      map.setView([0, 0], 0);
    } else {
      common.minZoom = OVERLAY_MIN_ZOOM;
      common.maxZoom = OVERLAY_MAX_ZOOM;
      map = L.map('map', common).setView([-23.55, -46.63], 9);
      var base = basemap === 'none' ? null : L.tileLayer(basemap === 'standard' ? '/tiles/{z}/{x}/{y}.png' : '/tiles/' + basemap + '/{z}/{x}/{y}.jpg', {
        minZoom: OVERLAY_MIN_ZOOM, maxZoom: OVERLAY_MAX_ZOOM, tileSize: 256, updateWhenIdle: false, keepBuffer: 2,
        detectRetina: true   // phones and tablets: tiles of the next zoom level, drawn at half size
      });
      if (base) {
        base.on('tileload', function () { bump('baseTileLoaded'); });
        base.on('tileunload', function () { bump('baseTileUnloaded'); });
        base.addTo(map);
      }
      // A tile that failed (network blip, upstream throttling) is asked again a few times
      // instead of staying black until the user moves or zooms the map.
      if (base) base.on('tileerror', function (ev) {
        bump('baseTileError');
        var img = ev.tile;
        var n = (img._ocRetry || 0) + 1;
        if (n > 4) return;
        img._ocRetry = n;
        var url = img.src.split('?')[0];
        window.setTimeout(function () {
          if (img.parentNode) img.src = url + '?r=' + n;
        }, 700 * n);
      });
    }
    var icon = L.divIcon({
      className: 'plane-icon',
      html: '<div class="plane-rot">' + aircraftSvg() + '</div>',
      iconSize: [40, 40],
      iconAnchor: [20, 20]
    });
    marker = L.marker([0, 0], { icon: icon, interactive: false, keyboard: false, zIndexOffset: 1000 });
    map.on('dragstart', function () { setFollow(false); pendingRegional = false; });
    setFollow(follow);
    if (mode === 'chart') map.on('zoomend', refineChartLevel);
    map.on('move', function () { bump('mapMove'); });
    map.on('zoomend', function () { bump('mapZoomEnd'); });
    map.on('resize', function () { bump('mapResize'); });

    attribEl.style.display = mode === 'overlay' ? '' : 'none';
    updateDisplayPanel();
    mapBackground();
    applyChart();
    drawRoute(new Date().getTime() < routeFitUntil);
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
      var b = chartBounds(meta);
      // The page already loaded and decoded this image (openChart): give that element to Leaflet.
      // Passing the URL made the browser fetch and decode the same big PNG a second time.
      if (meta.tiles) {
        chartLayer = new ChartTileLayer(meta, {
          tileSize: meta.tiles.size, minZoom: CHART_MIN_ZOOM, minNativeZoom: meta.tiles.minZoom || 0, maxNativeZoom: meta.tiles.maxZoom,
          maxZoom: meta.tiles.maxZoom, noWrap: true, bounds: b, updateWhenIdle: false, keepBuffer: 2
        }).addTo(map);
      } else {
        chartLayer = L.imageOverlay(active.img, b).addTo(map);
      }
      map.setMaxBounds(b.pad(0.75));
    } else {
      if (multi) {
        extras.forEach(function (e, i) {
          extraLayers.push(new WarpLayer(e.meta, e.img, {
            tileSize: TILE, opacity: OPACITY_STEPS[opacityIdx], minZoom: OVERLAY_MIN_ZOOM,
            maxZoom: OVERLAY_MAX_ZOOM, updateWhenIdle: false, keepBuffer: 1, zIndex: 5 + i
          }).addTo(map));
        });
      }
      chartLayer = new WarpLayer(meta, active.img, {
        tileSize: TILE, opacity: OPACITY_STEPS[opacityIdx], minZoom: OVERLAY_MIN_ZOOM,
        maxZoom: OVERLAY_MAX_ZOOM, updateWhenIdle: false, keepBuffer: 1, zIndex: 10
      }).addTo(map);
    }
    pendingRegional = !debugAt && isRegionalChart(meta);
    fitInitial();
  }

  // ---- regional zoom: about 30 km around the aircraft ----
  var REGIONAL_RADIUS_KM = 30;
  var pendingRegional = false;   // the chart is a big one and has not been framed around the aircraft yet

  function chartMetersPerPixel(meta) {
    if (meta.georef && meta.georef.metersPerPixel > 0) return meta.georef.metersPerPixel;
    var c = meta.geoCorners;
    var f = meta.mapFramePx;
    return L.latLng(c[0].lat, c[0].lon).distanceTo(L.latLng(c[1].lat, c[1].lon)) / Math.max(1, f.bottom - f.top);
  }

  // Clearly bigger than the regional area: only then is the whole chart "too far away" to start from.
  function isRegionalChart(meta) {
    if (meta.unreferenced) return false;
    var f = meta.mapFramePx;
    var span = Math.max(f.right - f.left, f.bottom - f.top) * chartMetersPerPixel(meta);
    return span > 2 * REGIONAL_RADIUS_KM * 1000 * 1.25;
  }

  // Frame the aircraft with REGIONAL_RADIUS_KM around it. False when that is not possible (no position
  // yet, or the aircraft is not on this chart): the caller falls back to the whole chart.
  function showRegional() {
    if (!map || !active || !lastFix || !hasSize()) return false;
    var meta = active.meta;
    if (meta.unreferenced) return false;
    var pt = LMMGeo.toPixel(meta, lastFix.lat, lastFix.lon);
    var f = meta.mapFramePx;
    if (pt.px < f.left || pt.px > f.right || pt.py < f.top || pt.py > f.bottom) return false;
    var half = REGIONAL_RADIUS_KM * 1000;
    if (mode === 'chart') {
      var hp = half / chartMetersPerPixel(meta);
      var c = chartPixelToLatLng(pt);
      map.fitBounds(L.latLngBounds([c.lat - hp, c.lng - hp], [c.lat + hp, c.lng + hp]), { animate: false });
    } else {
      map.fitBounds(L.latLng(lastFix.lat, lastFix.lon).toBounds(2 * half), { animate: false });
    }
    return true;
  }

  // How a chart is first shown: a regional chart around the aircraft, anything else as a whole.
  function fitInitial() {
    if (!active || !map) return;
    if (pendingRegional && showRegional()) { pendingRegional = false; return; }
    fitChart();
  }

  function fitChart() {
    if (!active || !map) return;
    if (!hasSize()) { needsFit = true; return; }
    needsFit = false;
    var meta = active.meta;
    if (mode === 'chart') {
      map.fitBounds(chartBounds(meta), { animate: false });
      // One level beyond the whole chart is enough: further out, the view is wider than the area the
      // map may show, the map is pushed back into it and the aircraft can no longer be centred.
      map.setMinZoom(Math.max(CHART_MIN_ZOOM, map.getZoom() - 1));
    } else {
      var pts = meta.geoCorners.map(function (g) { return L.latLng(g.lat, g.lon); });
      map.fitBounds(L.latLngBounds(pts), { animate: false });
    }
  }

  function setFollow(on) {
    if (on && !follow) followJustEnabled = true;
    follow = on;
    followBtn.className = on ? 'active' : '';
    // While following, pinch and wheel zoom around the centre of the screen (where the aircraft is):
    // zooming around the finger moved the aircraft away from the centre and every position update
    // then pulled the map back, which looked like flicker. Leaflet reads these options at each gesture.
    if (map) {
      map.options.touchZoom = on ? 'center' : true;
      map.options.scrollWheelZoom = on ? 'center' : true;
    }
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
    if (active && active.meta.unreferenced) { setStatus(t('viewOnly'), 'warn'); return; }
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
    // Short on purpose: the status tab may use only 30% of the width. The chart's name is the
    // highlighted chip in the strip, so it is not repeated here.
    setStatus((p.simulated ? t('fixed') + ' ' : '') + p.lat.toFixed(4) + ', ' + p.lon.toFixed(4) +
      '  HDG ' + Math.round(p.headingTrue) + (inside ? '' : '  -  ' + t('offChart')), inside ? 'ok' : 'warn');

    // A chart that covers a whole region opens at a regional zoom around the aircraft, not zoomed out
    // to the whole chart: done once, with the first position that falls on the chart.
    if (pendingRegional && inside && hasSize() && showRegional()) pendingRegional = false;

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
  // ---- loading box: its own element, so the position updates (which rewrite the status line) never
  // wipe it; it counts the seconds so a slow chart is visibly working, and it is gone only when the
  // chart is on screen or has failed.
  var loadingEl = el('loading');
  var loadingTimer = null;
  var loadingSince = 0;
  var loadingToken = null;
  function paintLoading() {
    var secs = Math.floor((new Date().getTime() - loadingSince) / 1000);
    var dots = ['.', '..', '...'][Math.floor((new Date().getTime() - loadingSince) / 400) % 3];
    clear(loadingEl);
    loadingEl.appendChild(document.createTextNode(t('loading') + dots + (secs >= 2 ? '  ' + secs + ' s' : '')));
    if (secs >= 4) {
      var sub = document.createElement('div');
      sub.className = 'sub';
      sub.textContent = t('loadingSlow');
      loadingEl.appendChild(sub);
    }
  }
  function showLoading(token) {
    loadingToken = token;
    loadingSince = new Date().getTime();
    loadingEl.className = 'loading';
    paintLoading();
    if (!loadingTimer) loadingTimer = window.setInterval(paintLoading, 400);
  }
  function hideLoading(token) {
    if (token !== undefined && token !== loadingToken) return;   // another chart is being opened meanwhile
    loadingEl.className = 'loading hidden';
    if (loadingTimer) { window.clearInterval(loadingTimer); loadingTimer = null; }
  }

  function loadFailed(id, fallback) {
    hideLoading();
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

  var modeBeforeViewOnly = null;

  function openChart(id, fallback) {
    setStatus(t('loading') + '...', 'warn');
    var loadToken = {};
    showLoading(loadToken);
    window.setTimeout(function () { hideLoading(loadToken); }, 120000);   // never leave it up forever
    // v=<time>: never trust a chart.json cached by an older server (it kept names for an hour)
    var timing = { id: id, mode: mode, start: nowMs(), json: 0, image: 0, built: 0, tiles: 0, drawn: 0, drawMs: 0, w: 0, h: 0 };
    getJson('/charts/' + encodeURIComponent(id) + '/chart.json?v=' + Date.now(), function (meta) {
      if (!meta) { loadFailed(id, fallback); return; }
      timing.json = nowMs();
      // A chart without georeferencing can only be looked at: chart-only mode, whatever was chosen
      // (the choice comes back with the next chart that has a position).
      if (meta.unreferenced) {
        if (mode === 'overlay') { modeBeforeViewOnly = mode; mode = 'chart'; }
      } else if (modeBeforeViewOnly) {
        mode = modeBeforeViewOnly;
        modeBeforeViewOnly = null;
      }
      var level = firstLevel(meta);
      var img = new Image();
      img.onload = function () {
        timing.image = nowMs();
        timing.w = img.naturalWidth;
        timing.h = img.naturalHeight;
        timing.level = level;
        keepPreviousOnMap(meta);
        active = { meta: meta, img: img, level: level };
        store('chart', id);
        setFollow(!debugAt);
        loadTiming = timing;
        buildMap();
        timing.built = nowMs();
        if (mode === 'chart') {
          // nothing more to load: the image element is already decoded
          timing.tiles = nowMs();
          loadTiming = null;
          reportLoadTiming(timing);
          hideLoading(loadToken);
        } else if (chartLayer && chartLayer.once) {
          chartLayer.once('load', function () {
            hideLoading(loadToken);
            if (loadTiming !== timing) return;   // another chart was opened meanwhile
            timing.tiles = nowMs();
            loadTiming = null;
            reportLoadTiming(timing);
          });
        } else {
          hideLoading(loadToken);
        }
        renderPins();
        updateCloseButton();
        saveState();
      };
      img.onerror = function () {
        if (level !== 1) {   // the reduced copy could not be made: use the full picture
          level = 1;
          img.src = chartImageUrl(meta, 1);
          return;
        }
        loadFailed(id, fallback);
      };
      img.src = chartImageUrl(meta, level);
    });
  }

  function chartImageUrl(meta, level) {
    var base = '/charts/' + encodeURIComponent(meta.id) + '/';
    return level === 1 || meta.image.file !== 'chart.png' ? base + meta.image.file : base + 'chart@' + level + '.png';
  }

  // The smallest copy that is still not shown enlarged at this scale (screen px per chart px).
  function levelForScale(scale) {
    var need = scale * (window.devicePixelRatio || 1) * 0.85;   // a little slack: a touch of enlargement is fine
    for (var i = 0; i < CHART_LEVELS.length; i++) {
      if (1 / CHART_LEVELS[i] >= need) return CHART_LEVELS[i];
    }
    return 1;
  }

  // The level to start with: the overlay mode draws from the full picture; chart-only mode starts at
  // the size the whole chart is first shown at.
  function firstLevel(meta) {
    if (meta.tiles) return 1;   // a pyramid brings its own levels
    if (mode !== 'chart') return 1;
    var m = el('map');
    var w = m.clientWidth || 468;
    var h = m.clientHeight || 696;
    return levelForScale(Math.min(w / meta.image.width, h / meta.image.height));
  }

  // Zoomed in: swap the chart picture for a finer copy (never back to a coarser one while it stays
  // open, so zooming out and in does not make the picture flash).
  var refining = false;
  function refineChartLevel() {
    if (mode !== 'chart' || !active || !map || refining || active.meta.tiles) return;
    var want = levelForScale(Math.pow(2, map.getZoom()));
    if (want >= active.level) return;
    refining = true;
    var id = active.meta.id;
    var meta = active.meta;
    var im = new Image();
    im.onload = function () {
      refining = false;
      if (!active || active.meta.id !== id || mode !== 'chart' || !map) return;
      var b = chartBounds(meta);
      var next = L.imageOverlay(im, b).addTo(map);
      if (chartLayer) map.removeLayer(chartLayer);
      chartLayer = next;
      active.img = im;
      active.level = want;
    };
    im.onerror = function () { refining = false; };
    im.src = chartImageUrl(meta, want);
  }

  // The overlay mode warps the full picture: bring it back before switching to it.
  function ensureFullImage(done) {
    if (!active || active.level === 1) { done(); return; }
    var id = active.meta.id;
    var meta = active.meta;
    setStatus(t('loading'), 'warn');
    var im = new Image();
    im.onload = function () {
      if (active && active.meta.id === id) { active.img = im; active.level = 1; }
      done();
    };
    im.onerror = function () { setStatus(t('loadFail'), 'bad'); };
    im.src = chartImageUrl(meta, 1);
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
    viewOverlayBtn.style.opacity = active && active.meta.unreferenced ? '0.35' : '';
    viewChartBtn.className = mode === 'chart' ? 'active' : '';
    // visibility, not display: the slot stays, so the buttons above never move when the mode changes
    blendBtn.style.visibility = mode === 'overlay' ? '' : 'hidden';
    multiBtn.style.visibility = mode === 'overlay' ? '' : 'hidden';
    multiBtn.className = multi ? 'active' : '';
    multiBtn.title = t('multiTitle') + ': ' + (multi ? t('multiOn') : t('multiOff'));
    var lvl = blendBtn.querySelector('.lvl');
    if (lvl) lvl.setAttribute('fill-opacity', String(OPACITY_STEPS[opacityIdx]));
    blendBtn.title = t('blendTitle') + ': ' + Math.round(OPACITY_STEPS[opacityIdx] * 100) + '%';
  }

  function setMode(next) {
    if (next === mode) return;
    if (next === 'overlay' && active && active.meta.unreferenced) { setStatus(t('viewOnlyMap'), 'warn'); return; }
    if (next === 'overlay' && active && active.level !== 1) {
      ensureFullImage(function () { setMode(next); });
      return;
    }
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
    if (chartLayer && chartLayer.setOpacity && mode === 'overlay') chartLayer.setOpacity(OPACITY_STEPS[opacityIdx]);
    extraLayers.forEach(function (l) { if (l.setOpacity) l.setOpacity(OPACITY_STEPS[opacityIdx]); });
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


  // Leave the chart on screen. A chart that is not pinned has no chip to tap, so this is the
  // way out; the viewer remembers that nothing was open.
  function closeChart() {
    if (!active) return;
    if (mode !== 'overlay') extras = [];
    if (extras.length) {   // the chart under it comes up
      active = extras.pop();
      store('chart', active.meta.id);
      buildMap();
      renderPins();
      updateCloseButton();
      saveState();
      return;
    }
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
    closeBtn.style.visibility = active ? '' : 'hidden';   // keeps its slot (see updateDisplayPanel)
  }

  // ---- several charts on the map ----
  function isOnMap(id) {
    for (var i = 0; i < extras.length; i++) if (extras[i].meta.id === id) return true;
    return false;
  }

  // With "several charts" on, the chart that was on screen stays under the one being opened.
  function keepPreviousOnMap(next) {
    extras = extras.filter(function (e) { return e.meta.id !== next.id; });
    if (!multi || mode !== 'overlay' || !active || next.unreferenced || active.meta.unreferenced ||
        active.meta.id === next.id || active.level !== 1) return;
    extras = extras.filter(function (e) { return e.meta.id !== active.meta.id; });
    extras.push(active);
    while (extras.length > MAX_EXTRAS) extras.shift();
  }

  function toggleMulti() {
    multi = !multi;
    store('multi', multi ? '1' : '0');
    if (!multi) extras = [];
    if (mode === 'overlay' && active) buildMap();
    updateDisplayPanel();
    renderPins();
  }

  // The pushpin of a pin button: filled when the chart is pinned.
  var PIN_SVG = '<svg viewBox="0 0 32 32"><path d="M11 4h10l-1 9 4 5H8l4-5z"/><path d="M16 18v10"/></svg>';
  function pinButton(on, title, handler) {
    var b = button('cx' + (on ? ' on' : ''), '', handler);
    b.innerHTML = PIN_SVG;
    b.title = title;
    return b;
  }

  // The button of the bar pins/unpins the chart on screen.
  function updateStar() {
    var on = !!active && isPinned(active.meta.id);
    starBtn.disabled = !active;
    starBtn.className = 'tool pinbig' + (on ? ' on' : '');
    starBtn.title = on ? t('starUnpin') : t('starPin');
    el('star-label').textContent = on ? t('pinned') : t('pin');
  }

  // ---- the drawers: the route on the left; on the right the pinned charts or the search ----
  var leftOpen = false;
  var rightView = '';          // '', 'pinned' or 'search'

  function updateDrawers() {
    routeDrawerEl.className = 'drawer left' + (leftOpen ? '' : ' hidden');
    chartDrawerEl.className = 'drawer right' + (rightView ? '' : ' hidden');
    pinnedViewEl.className = rightView === 'pinned' ? '' : 'hidden';
    searchViewEl.className = rightView === 'search' ? '' : 'hidden';
    el('cd-title').textContent = rightView === 'search' ? t('searchTitle') : t('pinnedTitle');
    stripBtn.className = 'cleanbtn stripbtn' + (rightView === 'pinned' ? ' active' : '');
    searchBtn.className = 'cleanbtn searchbtn' + (rightView === 'search' ? ' active' : '');
    routeBoxBtn.className = 'routebox' + (routePoints.length ? '' : ' empty') + (leftOpen ? ' on' : '');
    if (rightView === 'search') runChartSearch();
    if (rightView === 'pinned') renderPins();
  }

  function setLeftOpen(on) {
    leftOpen = on;
    if (on) rightView = '';
    hideRouteMenu();
    updateDrawers();
  }

  function setRightView(v) {
    rightView = rightView === v ? '' : v;
    if (rightView) leftOpen = false;
    hideRouteMenu();
    updateDrawers();
  }

  function closeDrawers() {
    if (!leftOpen && !rightView) return;
    leftOpen = false;
    rightView = '';
    updateDrawers();
  }

  function inside(node, container) {
    while (node) {
      if (node === container) return true;
      node = node.parentNode;
    }
    return false;
  }

  // ---- pinned charts: a vertical list grouped by airport (the group is chosen above it) ----
  function openFromDrawer(id) {
    closeDrawers();
    openChart(id);
  }

  function renderPins() {
    updateStar();
    clear(pinsEl);
    if (!pinned.length) {
      var hint = document.createElement('div');
      hint.className = 'empty';
      hint.textContent = t('noPins');
      pinsEl.appendChild(hint);
      return;
    }
    pinnedByAirport().forEach(function (g) {
      var head = document.createElement('div');
      head.className = 'dgroup';
      head.textContent = g.label;
      pinsEl.appendChild(head);
      g.items.forEach(function (c) {
        var isActive = !!active && active.meta.id === c.id;
        var row = document.createElement('div');
        row.className = 'crow' + (isActive ? ' active' : '') + (isOnMap(c.id) ? ' onmap' : '');
        row.title = label(c.name);
        row.appendChild(document.createTextNode(chipLabel(c)));
        row.addEventListener('click', function () { openFromDrawer(c.id); });
        row.appendChild(pinButton(true, t('remove'), function (ev) {
          if (ev.stopPropagation) ev.stopPropagation();
          togglePin({ id: c.id }, function () { renderPins(); });
        }));
        pinsEl.appendChild(row);
      });
    });
  }

  // ---- search for charts. With a route, the charts of its airports come first, without typing. ----
  var searchSeq = 0;
  var searchTimer = null;

  function routeAirports() {
    var out = [];
    routePoints.forEach(function (p) {
      if (p.kind === 'airport' && out.indexOf(p.ident) < 0) out.push(p.ident);
    });
    return out;
  }

  // the procedures the route uses (a SID or STAR name rides on its points as "via")
  function routeProcedures() {
    var out = [];
    routePoints.forEach(function (p) {
      if (p.via && /\d/.test(p.via) && out.indexOf(p.via) < 0) out.push(p.via);
    });
    return out;
  }

  function chartRow(c, onChange, relevant) {
    var isActive = !!active && active.meta.id === c.id;
    var row = document.createElement('div');
    row.className = 'crow' + (isActive ? ' active' : '') + (relevant ? ' relevant' : '') + (isOnMap(c.id) ? ' onmap' : '');
    row.appendChild(document.createTextNode(chipLabel(c)));
    var small = document.createElement('span');
    small.className = 'csmall';
    small.textContent = label(c.name);
    row.appendChild(small);
    row.addEventListener('click', function () { openFromDrawer(c.id); });
    row.appendChild(pinButton(isPinned(c.id), isPinned(c.id) ? t('remove') : t('pin'), function (ev) {
      if (ev.stopPropagation) ev.stopPropagation();
      togglePin(c, function () { if (onChange) onChange(); });
    }));
    return row;
  }

  function runChartSearch() {
    var box = el('results');
    var q = cqEl.value.replace(/^\s+|\s+$/g, '');
    var seq = ++searchSeq;
    clear(box);
    var again = function () { if (seq === searchSeq) runChartSearch(); };
    if (q) {
      getJson('/api/charts?limit=30&q=' + encodeURIComponent(q), function (list) {
        if (seq !== searchSeq || list === null) return;
        clear(box);
        if (!list.length) {
          var e = document.createElement('div');
          e.className = 'empty';
          e.textContent = t('noResults');
          box.appendChild(e);
        }
        list.forEach(function (c) { box.appendChild(chartRow(c, function () { runChartSearch(); }, false)); });
      });
      return;
    }
    var aps = routeAirports();
    if (!aps.length) {
      var h = document.createElement('div');
      h.className = 'empty';
      h.textContent = t('searchHint');
      box.appendChild(h);
      return;
    }
    var procs = routeProcedures();
    aps.forEach(function (icao, i) {
      var sec = document.createElement('div');
      var head = document.createElement('div');
      head.className = 'dgroup';
      head.textContent = icao + (aps.length < 2 ? '' : (i === 0 ? '  -  ' + t('routeFrom') : (i === aps.length - 1 ? '  -  ' + t('routeTo') : '')));
      sec.appendChild(head);
      box.appendChild(sec);
      getJson('/api/library?icao=' + encodeURIComponent(icao) + '&limit=80', function (r) {
        if (seq !== searchSeq || !r || !r.items) return;
        var items = r.items.slice();
        var mine = function (c) {
          var name = (c.name || '').toUpperCase();
          for (var k = 0; k < procs.length; k++) if (name.indexOf(procs[k].toUpperCase()) >= 0) return true;
          return false;
        };
        items.sort(function (a, b) { return (mine(b) ? 1 : 0) - (mine(a) ? 1 : 0); });
        if (!items.length) {
          var none = document.createElement('div');
          none.className = 'empty';
          none.textContent = t('noChartsAirport');
          sec.appendChild(none);
        }
        items.forEach(function (c) { sec.appendChild(chartRow(c, again, mine(c))); });
      });
    });
  }

  function scheduleChartSearch() {
    if (searchTimer) window.clearTimeout(searchTimer);
    searchTimer = window.setTimeout(runChartSearch, 200);
  }


  // ---- the route: pasted the way pilots write it, resolved by the server, kept as a list the user can edit ----
  var routePoints = [];     // the route as it is now: resolved, then edited by the user
  var routeProblems = [];   // what the last reading could not place, or placed with a doubt
  var routeText = '';       // what was pasted (or the places left after editing)
  var routeLayer = null;    // the drawn route
  var routeSeq = 0;
  var routeFitUntil = 0;    // a chart opening right after the route was read must not take the view away
  var routeOpen = true;     // the list of places is expanded
  var routeMenuEl = null;

  var routeModalEl = el('routemodal');
  var routeSumEl = el('routesum');
  var routeBoxBtn = el('btn-routebox');
  var routeHeadEl = el('routehead');
  var routeChipsEl = el('routechips');
  var routeProblemsEl = el('routeproblems');
  var routeGoBtn = el('route-go');
  var DOTS_SVG = '<svg viewBox="0 0 24 24"><circle cx="12" cy="5" r="2.3"/><circle cx="12" cy="12" r="2.3"/><circle cx="12" cy="19" r="2.3"/></svg>';

  // Where a place of the route goes on the map in use: on the world map its own position; on a
  // chart picture, through the chart's georeference (and nowhere when the chart has none).
  function routeLatLng(p) {
    if (mode === 'chart') {
      if (!active || active.meta.unreferenced) return null;
      return chartPixelToLatLng(LMMGeo.toPixel(active.meta, p.lat, p.lon));
    }
    return L.latLng(p.lat, p.lon);
  }

  function clearRoute() {
    if (routeLayer && map) map.removeLayer(routeLayer);
    routeLayer = null;
  }

  function drawRoute(fit) {
    clearRoute();
    if (!map || !routePoints.length) return;
    var group = L.layerGroup();
    var pts = [];
    var lastIdent = '';
    routePoints.forEach(function (p) {
      var ll = routeLatLng(p);
      if (!ll) return;
      pts.push(ll);
      var big = p.kind === 'airport';
      L.marker(ll, {
        interactive: false, keyboard: false,
        icon: L.divIcon({ className: 'rtpt' + (big ? ' rtairport' : ''), iconSize: [big ? 14 : 9, big ? 14 : 9] })
      }).addTo(group);
      if (p.ident !== lastIdent) {
        L.marker(ll, {
          interactive: false, keyboard: false,
          icon: L.divIcon({ className: 'rtlabel' + (big ? ' rtairport' : ''), html: p.ident, iconSize: null, iconAnchor: [-9, 6] })
        }).addTo(group);
      }
      lastIdent = p.ident;
    });
    if (!pts.length) return;
    L.polyline(pts, { color: '#c2185b', weight: 4, opacity: 0.9, interactive: false }).addTo(group);
    group.addTo(map);
    routeLayer = group;
    if (fit) {
      setFollow(false);
      pendingRegional = false;
      map.fitBounds(L.latLngBounds(pts), { padding: [48, 48], animate: false });
    }
  }

  function saveRoute() {
    store('routepts', JSON.stringify(routePoints));
    store('routetext', routeText);
    store('routeprob', JSON.stringify(routeProblems));
  }

  function loadRoute() {
    try {
      var pts = JSON.parse(store('routepts') || '[]');
      if (pts && pts.length && pts[0].lat !== undefined) routePoints = pts;
      routeText = store('routetext') || '';
      routeProblems = JSON.parse(store('routeprob') || '[]') || [];
    } catch (e) { routePoints = []; routeProblems = []; }
  }

  function hideRouteMenu() {
    if (routeMenuEl && routeMenuEl.parentNode) routeMenuEl.parentNode.removeChild(routeMenuEl);
    routeMenuEl = null;
  }

  // what the user did to the list: the map, the list and the saved copy follow
  function afterRouteEdit() {
    hideRouteMenu();
    routeProblems = [];
    routeText = routePoints.map(function (p) { return p.ident; }).join(' ');
    saveRoute();
    drawRoute(false);
    renderRoute();
  }

  function showRouteMenu(i, anchor) {
    hideRouteMenu();
    var m = document.createElement('div');
    m.className = 'rtmenu';
    if (i > 0) {
      m.appendChild(button('', t('routeDirect'), function () {
        routePoints = routePoints.slice(i);   // everything before it goes: the route starts here
        afterRouteEdit();
      }));
    }
    m.appendChild(button('', t('routeRemove'), function () {
      routePoints.splice(i, 1);
      afterRouteEdit();
    }));
    document.body.appendChild(m);
    var r = anchor.getBoundingClientRect();
    var w = document.documentElement.clientWidth;
    m.style.top = Math.round(r.bottom + 2) + 'px';
    m.style.left = Math.max(4, Math.min(Math.round(r.left), w - 160)) + 'px';
    routeMenuEl = m;
  }

  function renderRoute() {
    var n = routePoints.length;
    var first = n ? routePoints[0].ident : '';
    var last = n ? routePoints[n - 1].ident : '';
    routeBoxBtn.className = 'routebox' + (n ? '' : ' empty') + (leftOpen ? ' on' : '');
    routeSumEl.textContent = n ? first + (n > 1 ? ' > ' + last : '') : t('routePaste');
    clear(routeHeadEl);
    routeHeadEl.appendChild(document.createTextNode(n ? first + (n > 1 ? ' > ' + last : '') + '  -  ' + n + ' ' + t('routePlaces') : t('routeNone')));
    if (routeProblems.length) {
      var w = document.createElement('span');
      w.className = 'warn';
      w.textContent = routeProblems.length + ' ' + t('routeProblems');
      routeHeadEl.appendChild(w);
    }
    clear(routeChipsEl);
    routePoints.forEach(function (p, i) {
      var chip = document.createElement('div');
      chip.className = 'rtchip' + (p.kind === 'airport' ? ' rtap' : '');
      var name = document.createElement('span');
      name.className = 'rtname';
      name.textContent = p.ident;
      name.addEventListener('click', function () {
        var ll = routeLatLng(p);
        if (ll && map) {
          setFollow(false);
          map.setView(ll, mode === 'chart' ? map.getZoom() : Math.max(map.getZoom(), 10), { animate: false });
          closeDrawers();
        }
      });
      chip.appendChild(name);
      if (p.via) {
        var via = document.createElement('span');
        via.className = 'rtvia';
        via.textContent = p.via;
        chip.appendChild(via);
      }
      var more = document.createElement('button');
      more.type = 'button';
      more.className = 'rtmore';
      more.setAttribute('aria-label', t('routeMenu'));
      more.innerHTML = DOTS_SVG;
      more.addEventListener('click', function (ev) {
        if (ev.stopPropagation) ev.stopPropagation();
        showRouteMenu(i, more);
      });
      chip.appendChild(more);
      routeChipsEl.appendChild(chip);
    });
    if (rightView === 'search') runChartSearch();   // the charts of the route follow the route
  }

  function renderRouteProblems() {
    clear(routeProblemsEl);
    routeProblems.forEach(function (q) {
      var row = document.createElement('div');
      row.className = 'rp';
      var text = q.token + ': ' + q.message;
      if (q.candidates && q.candidates.length) {
        text += '  (' + q.candidates.map(function (c) { return c.ident + '/' + c.region; }).join(', ') + ')';
      }
      row.textContent = text;
      routeProblemsEl.appendChild(row);
    });
  }

  function openRouteBox() {
    hideRouteMenu();
    qEl.value = routeText;
    renderRouteProblems();
    routeModalEl.className = 'routemodal';
  }

  function closeRouteBox() {
    qEl.blur();
    routeModalEl.className = 'routemodal hidden';
  }

  function clearWholeRoute() {
    routeSeq += 1;
    routePoints = [];
    routeProblems = [];
    routeText = '';
    qEl.value = '';
    clearRoute();
    saveRoute();
    renderRoute();
    renderRouteProblems();
  }

  function resolveRoute() {
    var text = qEl.value.replace(/^\s+|\s+$/g, '');
    if (!text) { clearWholeRoute(); closeRouteBox(); return; }
    var seq = ++routeSeq;
    routeGoBtn.textContent = t('routeWorking');
    request('POST', '/api/route/resolve', { route: text }, function (res) {
      if (seq !== routeSeq) return;
      routeGoBtn.textContent = t('routeResolve');
      if (!res || !res.points) {
        routeProblems = [{ token: '', message: t('routeFailed') }];
        renderRouteProblems();
        return;
      }
      routePoints = res.points;
      routeProblems = res.problems || [];
      routeText = text;
      routeOpen = true;
      saveRoute();
      drawRoute(true);
      routeFitUntil = new Date().getTime() + 10000;
      renderRoute();
      renderRouteProblems();
      if (!routeProblems.length) closeRouteBox();   // with something to look at, the box stays open
    }, 90000);
  }

  // ---- on-screen keyboard (typing may not reach the iframe inside the EFB) ----
  function buildKeyboard(container, getInput, onChange, onOk) {
    var change = function () { if (onChange) onChange(); };
    var rows = ['1234567890', 'QWERTYUIOP', 'ASDFGHJKL', 'ZXCVBNM'];
    rows.forEach(function (r) {
      var d = document.createElement('div');
      d.className = 'krow';
      r.split('').forEach(function (ch) {
        d.appendChild(button('', ch, function () { getInput().value += ch.toLowerCase(); change(); }));
      });
      container.appendChild(d);
    });
    var last = document.createElement('div');
    last.className = 'krow';
    last.appendChild(button('k-wide', '____', function () { getInput().value += ' '; change(); }));
    last.appendChild(button('', '<', function () { var i = getInput(); i.value = i.value.slice(0, -1); change(); }));
    last.appendChild(button('', 'C', function () { getInput().value = ''; change(); }));
    last.appendChild(button('k-wide', 'OK', function () { if (onOk) onOk(); }));
    container.appendChild(last);
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
        if (needsFit) fitInitial();
      }
    }
  }

  viewOverlayBtn.setAttribute('aria-label', t('viewWithMap'));
  viewChartBtn.setAttribute('aria-label', t('viewChartOnly'));
  closeBtn.setAttribute('aria-label', t('closeChart'));
  closeBtn.addEventListener('click', closeChart);
  stripBtn.addEventListener('click', function () { setRightView('pinned'); });
  searchBtn.addEventListener('click', function () { setRightView('search'); });
  el('cd-close').addEventListener('click', closeDrawers);
  el('rd-close').addEventListener('click', closeDrawers);
  el('rd-paste').addEventListener('click', openRouteBox);
  cqEl.addEventListener('input', scheduleChartSearch);
  el('btn-kbd2').addEventListener('click', function () {
    kbd2El.className = kbd2El.className.indexOf('hidden') >= 0 ? 'kbd' : 'kbd hidden';
  });
  updateCloseButton();
  blendBtn.addEventListener('click', cycleBlend);
  viewOverlayBtn.title = viewWithMapTitle();
  viewChartBtn.title = viewChartOnlyTitle();
  simNoteEl.textContent = t('simNote');
  starBtn.addEventListener('click', function () {
    if (active) togglePin({ id: active.meta.id }, function () { renderPins(); });
  });
  updateStar();
  // a route is not searched while it is typed: it is resolved when it is pasted, on Enter, on the
  // button, or when the field is left
  qEl.addEventListener('paste', function () { window.setTimeout(resolveRoute, 0); });
  qEl.addEventListener('keydown', function (ev) { if (ev.keyCode === 13 && !ev.shiftKey) { ev.preventDefault(); resolveRoute(); } });
  el('btn-route').addEventListener('click', openRouteBox);
  // the box shows the route: a tap opens the list of its places (or, with no route, the box to paste one)
  routeBoxBtn.addEventListener('click', function () {
    if (!routePoints.length) { openRouteBox(); return; }
    setLeftOpen(!leftOpen);
  });
  routeGoBtn.addEventListener('click', resolveRoute);
  el('route-cancel').addEventListener('click', closeRouteBox);
  el('route-clear').addEventListener('click', clearWholeRoute);
  routeHeadEl.addEventListener('click', function () { if (routeProblems.length) openRouteBox(); });
  function applyRouteStrings() {
    routeGoBtn.textContent = t('routeResolve');
    el('route-clear').textContent = t('routeClear');
    el('route-cancel').textContent = t('routeCancel');
    el('routetitle').textContent = t('routeBoxTitle');
    el('rd-title').textContent = t('routeTitle');
    el('rd-paste').textContent = t('routePasteShort');
    cqEl.placeholder = t('searchPlaceholder');
    updateDrawers();
    qEl.placeholder = t('routePaste');
    renderRoute();
  }
  applyRouteStrings();
  // a tap outside the option menu closes it
  function closeMenuFromOutside(ev) {
    if (!routeMenuEl) return;
    var n = ev.target;
    while (n) { if (n === routeMenuEl) return; n = n.parentNode; }
    hideRouteMenu();
  }
  document.addEventListener('mousedown', closeMenuFromOutside, true);
  document.addEventListener('touchstart', closeMenuFromOutside, true);
  // A tap anywhere outside the bar (the map, the side buttons) closes the result and group lists, and
  // so does the viewer losing the focus (a click on the simulator's own interface): a list left open
  // after the search was abandoned only gets in the way. The on-screen keyboard is inside the bar.
  var barEl = el('bar');
  function closeListsFromOutside(ev) {
    var n = ev.target;
    // taps inside the bar, a drawer, the route box and the buttons of the drawers do not close anything
    if (inside(n, barEl) || inside(n, routeDrawerEl) || inside(n, chartDrawerEl) || inside(n, routeModalEl) ||
        inside(n, stripBtn) || inside(n, searchBtn) || (routeMenuEl && inside(n, routeMenuEl))) return;
    closeDrawers();
  }
  document.addEventListener('mousedown', closeListsFromOutside, true);
  document.addEventListener('touchstart', closeListsFromOutside, true);
  // Inside the EFB the page runs in a frame: tell the app when the search field is in use, so the
  // simulator sends the typed keys to it and not to its own key bindings (gear, flaps, camera...).
  var typing = qEl;   // the field that has the focus: the route or the chart search
  function tellHost(state) {
    try {
      if (window.parent && window.parent !== window) window.parent.postMessage({ oc: 'text-input', state: state, text: typing.value }, '*');
    } catch (e) { /* not in a frame */ }
  }
  qEl.addEventListener('focus', function () { typing = qEl; tellHost('focus'); });
  qEl.addEventListener('blur', function () { tellHost('blur'); });
  cqEl.addEventListener('focus', function () { typing = cqEl; tellHost('focus'); });
  cqEl.addEventListener('blur', function () { tellHost('blur'); });
  window.addEventListener('message', function (ev) {
    var d = ev && ev.data;
    if (!d || ev.source !== window.parent) return;
    if (d.oc === 'text-value' && typeof d.text === 'string') {
      typing.value = d.text.slice(0, 4000);
      if (typing === cqEl) scheduleChartSearch();
    }
    if (d.oc === 'text-blur') {
      typing.blur();
      if (typing === qEl && routeModalEl.className.indexOf('hidden') < 0) resolveRoute();
    }
  });
  qEl.addEventListener('keyup', function (ev) { if (ev.keyCode === 27) closeRouteBox(); });
  el('btn-kbd').addEventListener('click', function () {
    var hidden = kbdEl.className.indexOf('hidden') >= 0;
    kbdEl.className = hidden ? 'kbd' : 'kbd hidden';
  });
  multiBtn.addEventListener('click', toggleMulti);
  // the lists are short windows: buttons move them, a finger on the sim's screen scrolls badly
  function wireScroll(upId, downId, boxId) {
    var step = function (dir) {
      var box = el(boxId);
      box.scrollTop += dir * Math.max(60, Math.round(box.clientHeight * 0.8));
    };
    el(upId).addEventListener('click', function () { step(-1); });
    el(downId).addEventListener('click', function () { step(1); });
  }
  wireScroll('rd-up', 'rd-down', 'rd-scroll');
  wireScroll('cd-up', 'cd-down', 'cd-scroll');
  viewOverlayBtn.addEventListener('click', function () { setMode('overlay'); });
  viewChartBtn.addEventListener('click', function () { setMode('chart'); });
  el('btn-zoom-in').addEventListener('click', function () { map.zoomIn(1, { animate: false }); });
  el('btn-zoom-out').addEventListener('click', function () { map.zoomOut(1, { animate: false }); });
  followBtn.addEventListener('click', function () { setFollow(!follow); });
  el('btn-fit').addEventListener('click', function () { setFollow(false); pendingRegional = false; fitChart(); });
  el('btn-regional').title = t('regionalTitle');
  el('btn-regional').addEventListener('click', function () {
    pendingRegional = false;
    if (showRegional()) setFollow(true); else fitChart();
  });
  // The simulator keeps the EFB app open, so the page cannot be closed and reopened: this reloads it.
  el('btn-reload').title = lang === 'pt' ? 'Recarregar' : 'Reload';
  el('btn-reload').addEventListener('click', function () { window.location.reload(); });
  // Clean view: every control and the status line are hidden, so the chart (or the map) is seen on its
  // own. The button stays, and so does the black header band and the map credit.
  var cleanBtn = el('btn-clean');
  function setClean(on) {
    document.body.className = on ? 'clean' : '';
    cleanBtn.title = on ? (lang === 'pt' ? 'Mostrar controles' : 'Show controls') : (lang === 'pt' ? 'Ocultar controles' : 'Hide controls');
    store('clean', on ? '1' : '0');
    window.setTimeout(checkSize, 50);
  }
  cleanBtn.addEventListener('click', function () { setClean(document.body.className !== 'clean'); });
  setClean(store('clean') === '1');
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
  // Off by default (it watches the DOM and reports every 10 s). Open the viewer with ?perf=1 to turn
  // it on; the counters then appear in the program's log and at /api/client-perf.
  var PERF_ON = params.perf === '1';
  if (PERF_ON) {
    rafLoop();
    window.setInterval(tickGap, 250);
    window.setInterval(function () { reportPerf(false); }, 10000);
  }
  // Tapping the status line tells the server "the map looks dark right now" (see /api/client-perf).
  if (PERF_ON) statusEl.addEventListener('click', function () { reportPerf(true); });

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
    if (saved.aircraft === 'plane' || saved.aircraft === 'helicopter') aircraft = saved.aircraft;
    if (/^#[0-9a-fA-F]{6}$/.test(saved.aircraftHex || '')) aircraftHex = saved.aircraftHex;
    if (!params.lang && (saved.lang === 'pt' || saved.lang === 'en')) {
      lang = saved.lang;
      applyRouteStrings();
      simNoteEl.textContent = t('simNote');
    }
    buildKeyboard(kbdEl, function () { return qEl; }, null, resolveRoute);
    buildKeyboard(kbd2El, function () { return cqEl; }, scheduleChartSearch, runChartSearch);
    buildMap();
    checkSize();
    window.setInterval(checkSize, 1000);
    poll();
    if (PERF_ON) watchDom();
    loadGroups(function () {
      loadRoute();
      renderRoute();
      if (routePoints.length) {
        drawRoute(true);
        routeFitUntil = new Date().getTime() + 10000;
      } else if (params.q) {
        qEl.value = params.q;
        resolveRoute();
      }
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
