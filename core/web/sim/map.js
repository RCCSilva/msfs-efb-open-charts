/* Local Moving Map - page script. ES5 on purpose (MSFS embedded browser). */
(function () {
  'use strict';

  var POLL_MS = 250;
  var DEFAULT_ZOOM = 11;
  var MIN_ZOOM = 3;
  var MAX_ZOOM = 17;

  var statusEl = document.getElementById('status');
  var followBtn = document.getElementById('btn-follow');

  var map = null;
  var marker = null;
  var planeRot = null;
  var follow = true;
  var gotFirstFix = false;
  var lastW = 0;
  var lastH = 0;

  var lastStatusText = null;
  var lastStatusKind = null;
  var lastLat = null;
  var lastLon = null;
  var lastHeading = null;

  // Only touch the DOM when something changed: redundant writes at 4 Hz make
  // MSFS' embedded browser repaint (visible as flicker).
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

  var PLANE_SVG =
    // Own artwork: top-down aircraft, nose up, straight-edged silhouette.
    '<svg viewBox="0 0 64 64"><path d="M32 4L35 10L36 24L58 40L58 45L36 38L35 50L42 57L42 61' +
    'L32 58L22 61L22 57L29 50L28 38L6 45L6 40L28 24L29 10Z"/></svg>';

  function createMap() {
    map = L.map('map', {
      zoomControl: false,
      attributionControl: false,
      minZoom: MIN_ZOOM,
      maxZoom: MAX_ZOOM,
      // Everything animated is a rendering risk in Coherent GT: turn it off.
      zoomAnimation: false,
      fadeAnimation: false,
      markerZoomAnimation: false,
      inertia: false,
      // Touch/keyboard handlers add nothing inside the sim.
      tap: false,
      touchZoom: false,
      keyboard: false,
      doubleClickZoom: false,
      boxZoom: false
    }).setView([0, 0], 2);

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
    marker = L.marker([0, 0], { icon: icon, interactive: false, keyboard: false });
    // Marker is added on the first valid fix so it never sits at 0,0.

    map.on('dragstart', function () {
      setFollow(false);
    });
  }

  var followJustEnabled = false;

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

  function applyPosition(p) {
    var ll = [p.lat, p.lon];
    var moved = (p.lat !== lastLat || p.lon !== lastLon);

    if (!gotFirstFix) {
      gotFirstFix = true;
      marker.addTo(map);
      map.setView(ll, DEFAULT_ZOOM, { animate: false });
      moved = true;
      lastHeading = null;
    }

    if (moved) {
      lastLat = p.lat;
      lastLon = p.lon;
      marker.setLatLng(ll);
    }

    if (p.headingTrue !== lastHeading) {
      lastHeading = p.headingTrue;
      rotatePlane(p.headingTrue);
    }

    // Recentre only when following AND the aircraft left the centre by >= 1px.
    // (map.setView() resets the whole view every call - never call it blindly.)
    if (follow && (moved || followJustEnabled)) {
      followJustEnabled = false;
      var c = map.latLngToContainerPoint(ll);
      var s = map.getSize();
      var dx = c.x - s.x / 2;
      var dy = c.y - s.y / 2;
      if (Math.abs(dx) >= 1 || Math.abs(dy) >= 1) {
        map.panBy([dx, dy], { animate: false });
      }
    }
  }

  function poll() {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/api/position?t=' + Date.now());
    xhr.timeout = 2000;
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) return;
      var p = null;
      if (xhr.status === 200) {
        try { p = JSON.parse(xhr.responseText); } catch (e) { p = null; }
      }
      if (!p) {
        setStatus('Server unreachable', 'bad');
      } else if (!p.simConnected) {
        setStatus('Waiting for MSFS (SimConnect)...', 'warn');
      } else if (!p.valid) {
        setStatus('Connected - waiting for position (start a flight)', 'warn');
      } else {
        var src = p.simulated ? 'SIMULATED ' : '';
        setStatus(src + p.lat.toFixed(4) + ', ' + p.lon.toFixed(4) +
          '  HDG ' + Math.round(p.headingTrue) + '°  GS ' + Math.round(p.groundSpeedKnots) + ' kt', 'ok');
        applyPosition(p);
      }
      window.setTimeout(poll, POLL_MS);
    };
    xhr.ontimeout = function () { /* readyState 4 handler above covers it */ };
    xhr.send();
  }

  // The EFB can resize/hide/show the frame without a reliable resize event:
  // poll the real size and tell Leaflet when it changes.
  function checkSize() {
    var w = document.documentElement.clientWidth;
    var h = document.documentElement.clientHeight;
    if (w !== lastW || h !== lastH) {
      lastW = w;
      lastH = h;
      if (map) map.invalidateSize({ animate: false });
    }
  }

  document.getElementById('btn-zoom-in').addEventListener('click', function () {
    map.zoomIn(1, { animate: false });
  });
  document.getElementById('btn-zoom-out').addEventListener('click', function () {
    map.zoomOut(1, { animate: false });
  });
  followBtn.addEventListener('click', function () {
    setFollow(!follow);
  });
  window.addEventListener('resize', checkSize);

  createMap();
  checkSize();
  window.setInterval(checkSize, 1000);
  poll();
})();
