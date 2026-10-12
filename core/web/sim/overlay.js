/*
  Local Moving Map - chart overlay on the OpenStreetMap base. ES5 on purpose.

  Each prepared chart (chart.png + chart.json) is warped, tile by tile, into Web
  Mercator and drawn by a Leaflet GridLayer on top of the OSM tiles.

  For every pixel of an OSM tile we go "backwards":  tile pixel -> lat/lon ->
  chart projection (UTM / Lambert, via geo.js) -> pixel of the chart image.
  Inside a tile this is nearly linear, so each tile is cut into a small mesh of
  triangles and every triangle is drawn with one affine transform (canvas).

  Debug: /overlay.html?chart=<id>&at=<lat>,<lon>[,<heading>]
*/
(function () {
  'use strict';

  var POLL_MS = 250;
  var MIN_ZOOM = 3;
  var MAX_ZOOM = 16;
  var TILE = 256;
  var MESH = 8;                 // cells per tile side (8x8 cells, 2 triangles each)
  var OPACITY_STEPS = [0.35, 0.6, 0.85, 1];

  var statusEl = document.getElementById('status');
  var listEl = document.getElementById('chart-list');
  var followBtn = document.getElementById('btn-follow');

  var map = null;
  var marker = null;
  var planeRot = null;
  var follow = true;
  var followJustEnabled = false;
  var lastKey = null;
  var lastStatusText = null;
  var lastStatusKind = null;
  var lastW = 0;
  var lastH = 0;
  var opacityIdx = 1;
  var charts = [];              // /api/charts
  var loaded = {};              // id -> { meta, img, layer, on }
  var lastFix = null;

  var params = (function () {
    var out = {};
    window.location.search.replace(/^\?/, '').split('&').forEach(function (kv) {
      if (!kv) return;
      var i = kv.indexOf('=');
      out[decodeURIComponent(i < 0 ? kv : kv.slice(0, i))] = i < 0 ? '' : decodeURIComponent(kv.slice(i + 1));
    });
    return out;
  })();

  var debugAt = null;
  if (params.at) {
    var p = params.at.split(',').map(Number);
    if (p.length >= 2 && isFinite(p[0]) && isFinite(p[1])) {
      debugAt = { lat: p[0], lon: p[1], headingTrue: isFinite(p[2]) ? p[2] : 0 };
    }
  }

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

  function getJson(url, cb) {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', url);
    xhr.timeout = 8000;
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) return;
      var data = null;
      if (xhr.status === 200) {
        try { data = JSON.parse(xhr.responseText); } catch (e) { data = null; }
      }
      cb(data);
    };
    xhr.send();
  }

  // ---------------------------------------------------------------------------
  // The warping tile layer
  // ---------------------------------------------------------------------------

  // Web Mercator tile fraction -> lat/lon
  function tileToLatLon(z, x, y) {
    var n = Math.pow(2, z);
    var lon = x / n * 360 - 180;
    var m = Math.PI * (1 - 2 * y / n);
    var lat = Math.atan((Math.exp(m) - Math.exp(-m)) / 2) * 180 / Math.PI;
    return { lat: lat, lon: lon };
  }

  // Draw source triangle (s0,s1,s2) of `img` onto dest triangle (d0,d1,d2), clipped to
  // the chart's map frame (so margins and legends are not painted over the map).
  function drawTriangle(ctx, img, s0, s1, s2, d0, d1, d2, frame) {
    var det = (s1.x - s0.x) * (s2.y - s0.y) - (s2.x - s0.x) * (s1.y - s0.y);
    if (Math.abs(det) < 1e-9) return;

    var a = ((d1.x - d0.x) * (s2.y - s0.y) - (d2.x - d0.x) * (s1.y - s0.y)) / det;
    var c = ((d2.x - d0.x) * (s1.x - s0.x) - (d1.x - d0.x) * (s2.x - s0.x)) / det;
    var b = ((d1.y - d0.y) * (s2.y - s0.y) - (d2.y - d0.y) * (s1.y - s0.y)) / det;
    var d = ((d2.y - d0.y) * (s1.x - s0.x) - (d1.y - d0.y) * (s2.x - s0.x)) / det;
    var e = d0.x - a * s0.x - c * s0.y;
    var f = d0.y - b * s0.x - d * s0.y;

    // Grow the destination triangle by ~0.6 px so neighbours overlap (no hairline seams).
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

    // Source rectangle actually needed (triangle bbox ∩ map frame), with a 1px margin.
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

  var ChartLayer = L.GridLayer.extend({
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

      // Sample the mesh: destination pixel -> lat/lon -> source pixel on the chart.
      var src = [];
      var minX = Infinity;
      var minY = Infinity;
      var maxX = -Infinity;
      var maxY = -Infinity;
      var j;
      var i;
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

      // Tile entirely outside the map frame: leave it transparent.
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
      // Leaflet expects the callback asynchronously.
      window.setTimeout(function () { done(null, tile); }, 0);
      return tile;
    }
  });

  // ---------------------------------------------------------------------------
  // Map
  // ---------------------------------------------------------------------------
  var PLANE_SVG =
    '<svg viewBox="0 0 64 64"><path d="M32 4L35 10L36 24L58 40L58 45L36 38L35 50L42 57L42 61' +
    'L32 58L22 61L22 57L29 50L28 38L6 45L6 40L28 24L29 10Z"/></svg>';

  function createMap() {
    map = L.map('map', {
      zoomControl: false,
      attributionControl: false,
      minZoom: MIN_ZOOM,
      maxZoom: MAX_ZOOM,
      zoomAnimation: false,
      fadeAnimation: false,
      markerZoomAnimation: false,
      inertia: false,
      tap: false,
      touchZoom: false,
      keyboard: false,
      doubleClickZoom: false,
      boxZoom: false
    }).setView([-23.55, -46.63], 9);

    L.tileLayer('/tiles/{z}/{x}/{y}.png', {
      minZoom: MIN_ZOOM,
      maxZoom: MAX_ZOOM,
      tileSize: 256,
      updateWhenIdle: false,
      keepBuffer: 2
    }).addTo(map);

    var icon = L.divIcon({
      className: 'plane-icon',
      html: '<div class="plane-rot">' + PLANE_SVG + '</div>',
      iconSize: [40, 40],
      iconAnchor: [20, 20]
    });
    marker = L.marker([0, 0], { icon: icon, interactive: false, keyboard: false, zIndexOffset: 1000 });

    map.on('dragstart', function () { setFollow(false); });
  }

  function setFollow(on) {
    if (on && !follow) followJustEnabled = true;
    follow = on;
    followBtn.className = on ? 'active' : '';
  }

  function rotatePlane(deg) {
    if (!planeRot) {
      var el = marker && marker.getElement ? marker.getElement() : null;
      planeRot = el ? el.querySelector('.plane-rot') : null;
    }
    if (!planeRot) return;
    var v = 'rotate(' + deg.toFixed(1) + 'deg)';
    planeRot.style.transform = v;
    planeRot.style.webkitTransform = v;
  }

  function activeNames() {
    var names = [];
    charts.forEach(function (c) {
      if (loaded[c.id] && loaded[c.id].on) names.push(c.name);
    });
    return names;
  }

  function showAircraft(p) {
    lastFix = p;
    var ll = L.latLng(p.lat, p.lon);
    var key = p.lat.toFixed(6) + '|' + p.lon.toFixed(6) + '|' + Math.round(p.headingTrue);
    var changed = key !== lastKey;
    if (!map.hasLayer(marker)) {
      marker.addTo(map);
      planeRot = null;
      changed = true;
    }
    if (changed) {
      lastKey = key;
      marker.setLatLng(ll);
      rotatePlane(p.headingTrue);
    }
    var names = activeNames();
    setStatus((names.length ? names.join(' + ') + '  |  ' : '') + (p.simulated ? 'FIXED ' : '') +
      p.lat.toFixed(4) + ', ' + p.lon.toFixed(4) + '  HDG ' + Math.round(p.headingTrue), 'ok');
    if (follow && (changed || followJustEnabled)) {
      followJustEnabled = false;
      map.setView(ll, map.getZoom(), { animate: false });
    }
  }

  function poll() {
    if (debugAt) {
      showAircraft({ lat: debugAt.lat, lon: debugAt.lon, headingTrue: debugAt.headingTrue, simulated: true });
      return;
    }
    getJson('/api/position?t=' + Date.now(), function (p) {
      if (!p) {
        setStatus('Server unreachable', 'bad');
      } else if (!p.simConnected) {
        setStatus('Waiting for MSFS (SimConnect)...', 'warn');
      } else if (!p.valid) {
        setStatus('Connected - waiting for position (start a flight)', 'warn');
      } else {
        showAircraft(p);
      }
      window.setTimeout(poll, POLL_MS);
    });
  }

  // ---------------------------------------------------------------------------
  // Chart list / toggling
  // ---------------------------------------------------------------------------
  function chartBounds(meta) {
    var pts = meta.geoCorners.map(function (g) { return L.latLng(g.lat, g.lon); });
    return L.latLngBounds(pts);
  }

  // The viewport can be 0x0 when the page loads (EFB app hidden / frame just created):
  // fitting then would pick the minimum zoom. Remember it and fit once there is a size.
  var pendingFit = null;

  function fitToChart(meta) {
    var s = map.getSize();
    if (s.x <= 0 || s.y <= 0) {
      pendingFit = meta;
      return;
    }
    pendingFit = null;
    map.fitBounds(chartBounds(meta), { animate: false });
  }

  function renderList() {
    listEl.innerHTML = '';
    charts.forEach(function (c) {
      var b = document.createElement('button');
      b.type = 'button';
      b.textContent = c.name;
      if (loaded[c.id] && loaded[c.id].on) b.className = 'active';
      b.addEventListener('click', function () { toggleChart(c.id, true); });
      listEl.appendChild(b);
    });
  }

  function toggleChart(id, fit) {
    var entry = loaded[id];
    if (entry && entry.layer) {
      entry.on = !entry.on;
      if (entry.on) { entry.layer.addTo(map); } else { map.removeLayer(entry.layer); }
      renderList();
      return;
    }
    setStatus('Loading chart ' + id + '...', 'warn');
    getJson('/charts/' + encodeURIComponent(id) + '/chart.json', function (meta) {
      if (!meta) { setStatus('Could not load chart "' + id + '"', 'bad'); return; }
      var img = new Image();
      img.onload = function () {
        var layer = new ChartLayer(meta, img, {
          tileSize: TILE,
          opacity: OPACITY_STEPS[opacityIdx],
          minZoom: MIN_ZOOM,
          maxZoom: MAX_ZOOM,
          updateWhenIdle: false,
          keepBuffer: 1,
          zIndex: 10
        });
        loaded[id] = { meta: meta, img: img, layer: layer, on: true };
        layer.addTo(map);
        if (fit && !debugAt) { setFollow(false); fitToChart(meta); }
        renderList();
        if (lastFix) showAircraft(lastFix);
      };
      img.onerror = function () { setStatus('Could not load the image of "' + id + '"', 'bad'); };
      img.src = '/charts/' + encodeURIComponent(id) + '/' + meta.image.file;
    });
  }

  function applyOpacity() {
    Object.keys(loaded).forEach(function (id) {
      if (loaded[id].layer) loaded[id].layer.setOpacity(OPACITY_STEPS[opacityIdx]);
    });
  }

  function checkSize() {
    var w = document.documentElement.clientWidth;
    var h = document.documentElement.clientHeight;
    if (w !== lastW || h !== lastH) {
      lastW = w;
      lastH = h;
      if (map) {
        map.invalidateSize({ animate: false });
        if (pendingFit) fitToChart(pendingFit);   // first real size after loading at 0x0
      }
    }
  }

  document.getElementById('btn-zoom-in').addEventListener('click', function () { map.zoomIn(1, { animate: false }); });
  document.getElementById('btn-zoom-out').addEventListener('click', function () { map.zoomOut(1, { animate: false }); });
  followBtn.addEventListener('click', function () { setFollow(!follow); });
  document.getElementById('btn-fit').addEventListener('click', function () {
    var firstOn = null;
    Object.keys(loaded).forEach(function (id) { if (!firstOn && loaded[id].on) firstOn = loaded[id]; });
    if (firstOn) { setFollow(false); fitToChart(firstOn.meta); }
  });
  document.getElementById('btn-opacity').addEventListener('click', function () {
    opacityIdx = (opacityIdx + 1) % OPACITY_STEPS.length;
    applyOpacity();
    setStatus('Chart opacity ' + Math.round(OPACITY_STEPS[opacityIdx] * 100) + '%', 'ok');
  });
  window.addEventListener('resize', checkSize);

  window.LMM_DEBUG = {
    map: function () { return map; },
    layers: function () { return loaded; },
    setOpacity: function (v) { Object.keys(loaded).forEach(function (id) { loaded[id].layer.setOpacity(v); }); }
  };

  createMap();
  checkSize();
  window.setInterval(checkSize, 1000);
  poll();

  getJson('/api/charts', function (list) {
    charts = list || [];
    renderList();
    var wanted = params.chart;
    if (wanted) {
      charts.forEach(function (c) { if (c.id === wanted) toggleChart(c.id, true); });
    }
  });
})();
