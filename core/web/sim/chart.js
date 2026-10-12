/*
  Local Moving Map - chart viewer. ES5 on purpose (MSFS embedded browser).

  Shows a pre-rendered chart image (charts/prepare_chart.py) and places the aircraft
  on it using the chart's georeference (chart.json: projection + lat/lon -> pixel affine).

  Debug: /charts.html?chart=<id>&at=<lat>,<lon>[,<heading>]  pins the aircraft at a
  fixed position instead of using /api/position (handy to check landmarks).
*/
(function () {
  'use strict';

  var POLL_MS = 250;
  var MIN_ZOOM = -5;
  var MAX_ZOOM = 2;

  var statusEl = document.getElementById('status');
  var listEl = document.getElementById('chart-list');
  var followBtn = document.getElementById('btn-follow');

  var map = null;
  var overlay = null;
  var marker = null;
  var planeRot = null;
  var chart = null;          // chart.json of the chart on screen
  var charts = [];           // /api/charts
  var follow = true;
  var followJustEnabled = false;
  var lastKey = null;
  var lastStatusText = null;
  var lastStatusKind = null;
  var lastW = 0;
  var lastH = 0;

  var params = (function () {
    var out = {};
    var q = window.location.search.replace(/^\?/, '');
    q.split('&').forEach(function (kv) {
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

  if (!window.L || typeof window.L.map !== 'function') {
    setStatus('Leaflet failed to load', 'bad');
    return;
  }

  // ---- georeference maths live in geo.js (shared with the overlay page) ----
  var rad = LMMGeo.rad;

  // lat/lon -> image pixel {px, py} (py measured from the top of the image)
  function geoToPixel(lat, lon) {
    return LMMGeo.toPixel(chart, lat, lon);
  }

  // Leaflet CRS.Simple has y pointing up: image pixel (px, py-from-top) -> [H - py, px]
  function pixelToLatLng(pt) {
    return L.latLng(chart.image.height - pt.py, pt.px);
  }

  // Direction of travel on the chart, in degrees clockwise from chart-up.
  // Projects a point ~200 m ahead so grid convergence is accounted for.
  function chartHeading(lat, lon, headingTrue) {
    var h = rad(headingTrue);
    var d = 200;
    var dLat = d * Math.cos(h) / 111320;
    var dLon = d * Math.sin(h) / (111320 * Math.cos(rad(lat)));
    var a = geoToPixel(lat, lon);
    var b = geoToPixel(lat + dLat, lon + dLon);
    return Math.atan2(b.px - a.px, -(b.py - a.py)) * 180 / Math.PI;
  }

  // ---- XHR helper ----
  function getJson(url, cb) {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', url);
    xhr.timeout = 5000;
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

  // ---- map ----
  var PLANE_SVG =
    '<svg viewBox="0 0 64 64"><path d="M32 4L35 10L36 24L58 40L58 45L36 38L35 50L42 57L42 61' +
    'L32 58L22 61L22 57L29 50L28 38L6 45L6 40L28 24L29 10Z"/></svg>';

  function createMap() {
    map = L.map('map', {
      crs: L.CRS.Simple,
      zoomControl: false,
      attributionControl: false,
      minZoom: MIN_ZOOM,
      maxZoom: MAX_ZOOM,
      zoomSnap: 0.5,
      zoomAnimation: false,
      fadeAnimation: false,
      markerZoomAnimation: false,
      inertia: false,
      tap: false,
      touchZoom: false,
      keyboard: false,
      doubleClickZoom: false,
      boxZoom: false
    });
    map.setView([0, 0], 0);
    map.on('dragstart', function () { setFollow(false); });

    var icon = L.divIcon({
      className: 'plane-icon',
      html: '<div class="plane-rot">' + PLANE_SVG + '</div>',
      iconSize: [40, 40],
      iconAnchor: [20, 20]
    });
    marker = L.marker([0, 0], { icon: icon, interactive: false, keyboard: false });
  }

  function setFollow(on) {
    if (on && !follow) followJustEnabled = true;
    follow = on;
    followBtn.className = on ? 'active' : '';
  }

  // The viewport can be 0x0 when the page loads (EFB app hidden / frame just created).
  // Fitting then would pick the minimum zoom, so remember to fit once a real size exists.
  var needsFit = true;

  function hasSize() {
    var s = map.getSize();
    return s.x > 0 && s.y > 0;
  }

  function fitWholeChart() {
    if (!chart) return;
    if (!hasSize()) {
      needsFit = true;
      return;
    }
    var b = L.latLngBounds([0, 0], [chart.image.height, chart.image.width]);
    map.fitBounds(b, { animate: false });
    needsFit = false;
  }

  function showChart(meta) {
    chart = meta;
    lastKey = null;
    if (overlay) map.removeLayer(overlay);
    var b = L.latLngBounds([0, 0], [chart.image.height, chart.image.width]);
    overlay = L.imageOverlay('/charts/' + encodeURIComponent(chart.id) + '/' + chart.image.file, b).addTo(map);
    map.setMaxBounds(b.pad(0.5));
    fitWholeChart();
    // The aircraft marker is added on the first fix inside showAircraft().
    if (map.hasLayer(marker)) map.removeLayer(marker);
    planeRot = null;
    renderList();
    setStatus(chart.name + '  -  waiting for position', 'warn');
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

  function showAircraft(p) {
    if (!chart) return;
    var pt = geoToPixel(p.lat, p.lon);
    var ll = pixelToLatLng(pt);

    // Inside the map frame (not just the page)?
    var f = chart.mapFramePx;
    var inside = pt.px >= f.left && pt.px <= f.right && pt.py >= f.top && pt.py <= f.bottom;

    var key = pt.px.toFixed(1) + '|' + pt.py.toFixed(1) + '|' + Math.round(p.headingTrue);
    var changed = key !== lastKey;

    if (!map.hasLayer(marker)) {
      marker.addTo(map);
      planeRot = null;
      changed = true;
    }
    if (changed) {
      lastKey = key;
      marker.setLatLng(ll);
      rotatePlane(chartHeading(p.lat, p.lon, p.headingTrue));
    }

    var src = p.simulated ? 'FIXED ' : '';
    var where = inside ? '' : '  -  OFF CHART';
    setStatus(chart.name + '  |  ' + src + p.lat.toFixed(4) + ', ' + p.lon.toFixed(4) +
      '  HDG ' + Math.round(p.headingTrue) + where, inside ? 'ok' : 'warn');

    if (follow && (changed || followJustEnabled)) {
      followJustEnabled = false;
      if (inside) {
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
      if (!chart) { // chart.json still loading
        window.setTimeout(poll, 200);
        return;
      }
      showAircraft({ lat: debugAt.lat, lon: debugAt.lon, headingTrue: debugAt.headingTrue, simulated: true });
      return; // static: no need to keep polling
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

  function renderList() {
    listEl.innerHTML = '';
    if (charts.length < 2) return; // nothing to choose from
    charts.forEach(function (c) {
      var b = document.createElement('button');
      b.type = 'button';
      b.textContent = c.name;
      if (chart && chart.id === c.id) b.className = 'active';
      b.addEventListener('click', function () { loadChart(c.id); });
      listEl.appendChild(b);
    });
  }

  function loadChart(id) {
    getJson('/charts/' + encodeURIComponent(id) + '/chart.json', function (meta) {
      if (!meta) {
        setStatus('Could not load chart "' + id + '"', 'bad');
        return;
      }
      showChart(meta);
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
        if (needsFit) fitWholeChart();   // first real size after loading at 0x0
      }
    }
  }

  document.getElementById('btn-zoom-in').addEventListener('click', function () { map.zoomIn(1, { animate: false }); });
  document.getElementById('btn-zoom-out').addEventListener('click', function () { map.zoomOut(1, { animate: false }); });
  followBtn.addEventListener('click', function () { setFollow(!follow); });
  document.getElementById('btn-fit').addEventListener('click', function () {
    setFollow(false);
    fitWholeChart();
  });
  window.addEventListener('resize', checkSize);

  // Read-only handle for checking the georeference from the browser console.
  window.LMM_DEBUG = {
    geoToPixel: function (lat, lon) { return chart ? geoToPixel(lat, lon) : null; },
    chart: function () { return chart; },
    map: function () { return map; }
  };

  createMap();
  checkSize();
  window.setInterval(checkSize, 1000);

  getJson('/api/charts', function (list) {
    if (!list || !list.length) {
      setStatus('No charts found. Run charts/prepare_chart.py (see README).', 'warn');
      return;
    }
    charts = list;
    var wanted = params.chart;
    var pick = list[0].id;
    list.forEach(function (c) { if (c.id === wanted) pick = c.id; });
    loadChart(pick);
    poll();
  });
})();
