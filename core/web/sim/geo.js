/*
  Local Moving Map - shared georeference maths. ES5 on purpose (MSFS embedded browser).

  Turns a latitude/longitude into a pixel of a prepared chart image, using the
  chart.json written by charts/prepare_chart.py (projection + affine "toPixel").
  Same formulas as charts/prepare_chart.py (validated against PROJ to < 0.1 mm).

  Exposes window.LMMGeo = { rad, toPixel(chart, lat, lon) }.
*/
(function () {
  'use strict';

  // ---- WGS84 Transverse Mercator (Kruger series) ----
  var WGS_F = 1 / 298.257223563;
  var N = WGS_F / (2 - WGS_F);
  var AA = 6378137.0 / (1 + N) * (1 + N * N / 4 + Math.pow(N, 4) / 64);
  var ALPHA = [
    N / 2 - 2 * N * N / 3 + 5 * Math.pow(N, 3) / 16,
    13 * N * N / 48 - 3 * Math.pow(N, 3) / 5,
    61 * Math.pow(N, 3) / 240
  ];

  function atanh(x) { return 0.5 * Math.log((1 + x) / (1 - x)); }
  function sinh(x) { return (Math.exp(x) - Math.exp(-x)) / 2; }
  function cosh(x) { return (Math.exp(x) + Math.exp(-x)) / 2; }
  function rad(d) { return d * Math.PI / 180; }

  function tmForward(lat, lon, lon0, k0, fe, fn) {
    var phi = rad(lat);
    var lam = rad(lon - lon0);
    var c = 2 * Math.sqrt(N) / (1 + N);
    var t = sinh(atanh(Math.sin(phi)) - c * atanh(c * Math.sin(phi)));
    var xiP = Math.atan2(t, Math.cos(lam));
    var etaP = atanh(Math.sin(lam) / Math.sqrt(1 + t * t));
    var xi = xiP;
    var eta = etaP;
    for (var j = 1; j <= 3; j++) {
      xi += ALPHA[j - 1] * Math.sin(2 * j * xiP) * cosh(2 * j * etaP);
      eta += ALPHA[j - 1] * Math.cos(2 * j * xiP) * sinh(2 * j * etaP);
    }
    return { x: fe + k0 * AA * eta, y: fn + k0 * AA * xi };
  }

  // ---- Lambert Conformal Conic (ellipsoidal, 1 or 2 standard parallels) ----
  var lccCache = null;   // constants depend only on the projection, computed once

  function lccConstants(pr) {
    var f = 1 / pr.invF;
    var e = Math.sqrt(f * (2 - f));
    function m(phi) { return Math.cos(phi) / Math.sqrt(1 - Math.pow(e * Math.sin(phi), 2)); }
    function t(phi) {
      var s = e * Math.sin(phi);
      return Math.tan(Math.PI / 4 - phi / 2) / Math.pow((1 - s) / (1 + s), e / 2);
    }
    var p0 = rad(pr.lat0);
    var p1 = rad(pr.lat1);
    var p2 = rad(pr.lat2);
    var n = Math.abs(p1 - p2) < 1e-12
      ? Math.sin(p1)
      : (Math.log(m(p1)) - Math.log(m(p2))) / (Math.log(t(p1)) - Math.log(t(p2)));
    var bigF = m(p1) / (n * Math.pow(t(p1), n));
    return { n: n, F: bigF, rho0: pr.a * bigF * Math.pow(t(p0), n), e: e };
  }

  function lccForward(lat, lon, pr) {
    if (!lccCache || lccCache.owner !== pr) {
      lccCache = lccConstants(pr);
      lccCache.owner = pr;
    }
    var c = lccCache;
    var phi = rad(lat);
    var s = c.e * Math.sin(phi);
    var t = Math.tan(Math.PI / 4 - phi / 2) / Math.pow((1 - s) / (1 + s), c.e / 2);
    var rho = pr.a * c.F * Math.pow(t, c.n);
    var theta = c.n * rad(lon - pr.lon0);
    return {
      x: pr.falseEasting + pr.k0 * rho * Math.sin(theta),
      y: pr.falseNorthing + pr.k0 * (c.rho0 - rho * Math.cos(theta))
    };
  }

  // lat/lon -> image pixel {px, py} (py measured from the top of the image)
  function toPixel(chart, lat, lon) {
    var pr = chart.projection;
    var e;
    var n;
    if (pr.type === 'tm') {
      var r = tmForward(lat, lon, pr.lon0, pr.k0, pr.falseEasting, pr.falseNorthing);
      e = r.x;
      n = r.y;
    } else if (pr.type === 'lcc') {
      var q = lccForward(lat, lon, pr);
      e = q.x;
      n = q.y;
    } else { // geographic
      e = lon;
      n = lat;
    }
    var m = chart.toPixel;
    return { px: m.a * e + m.b * n + m.c, py: m.d * e + m.e * n + m.f };
  }

  window.LMMGeo = { rad: rad, toPixel: toPixel };
})();
