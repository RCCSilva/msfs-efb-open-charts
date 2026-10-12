// Package geo holds the map-projection maths used to place latitude/longitude on a chart image.
//
// Supported: WGS84 Transverse Mercator (UTM), Lambert Conformal Conic (1 or 2 standard
// parallels, ellipsoidal) and plain geographic. The formulas are the same ones validated
// against PROJ (< 0.1 mm over 200 random points) in the Python prototype.
package geo

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// Kind of projection.
const (
	KindTM         = "tm"
	KindLCC        = "lcc"
	KindGeographic = "geographic"
)

// Projection converts (lat, lon) in degrees to projected coordinates (x, y) in metres.
type Projection struct {
	Kind string

	// Transverse Mercator / Lambert
	Lon0, Lat0, Lat1, Lat2 float64
	K0, FalseE, FalseN     float64
	A, InvF                float64 // ellipsoid (Lambert uses it; TM is fixed WGS84)

	lcc *lccConsts
}

type lccConsts struct{ n, bigF, rho0, e float64 }

// ---- WGS84 Transverse Mercator (Kruger series) ----
var (
	wgsF     = 1 / 298.257223563
	wgsN     = wgsF / (2 - wgsF)
	wgsAA    = 6378137.0 / (1 + wgsN) * (1 + wgsN*wgsN/4 + math.Pow(wgsN, 4)/64)
	wgsAlpha = [3]float64{
		wgsN/2 - 2*wgsN*wgsN/3 + 5*math.Pow(wgsN, 3)/16,
		13*wgsN*wgsN/48 - 3*math.Pow(wgsN, 3)/5,
		61 * math.Pow(wgsN, 3) / 240,
	}
)

func rad(d float64) float64 { return d * math.Pi / 180 }

// Forward projects a position.
func (p *Projection) Forward(lat, lon float64) (x, y float64) {
	switch p.Kind {
	case KindTM:
		return p.tm(lat, lon)
	case KindLCC:
		return p.lccForward(lat, lon)
	default: // geographic: lon/lat as plain x/y
		return lon, lat
	}
}

func (p *Projection) tm(lat, lon float64) (float64, float64) {
	phi := rad(lat)
	lam := rad(lon - p.Lon0)
	c := 2 * math.Sqrt(wgsN) / (1 + wgsN)
	t := math.Sinh(math.Atanh(math.Sin(phi)) - c*math.Atanh(c*math.Sin(phi)))
	xiP := math.Atan2(t, math.Cos(lam))
	etaP := math.Atanh(math.Sin(lam) / math.Sqrt(1+t*t))
	xi, eta := xiP, etaP
	for j := 1; j <= 3; j++ {
		a := wgsAlpha[j-1]
		xi += a * math.Sin(2*float64(j)*xiP) * math.Cosh(2*float64(j)*etaP)
		eta += a * math.Cos(2*float64(j)*xiP) * math.Sinh(2*float64(j)*etaP)
	}
	return p.FalseE + p.K0*wgsAA*eta, p.FalseN + p.K0*wgsAA*xi
}

// ---- Lambert Conformal Conic ----
func (p *Projection) initLCC() {
	f := 1 / p.InvF
	e := math.Sqrt(f * (2 - f))
	m := func(phi float64) float64 { return math.Cos(phi) / math.Sqrt(1-math.Pow(e*math.Sin(phi), 2)) }
	t := func(phi float64) float64 {
		s := e * math.Sin(phi)
		return math.Tan(math.Pi/4-phi/2) / math.Pow((1-s)/(1+s), e/2)
	}
	p0, p1, p2 := rad(p.Lat0), rad(p.Lat1), rad(p.Lat2)
	var n float64
	if math.Abs(p1-p2) < 1e-12 {
		n = math.Sin(p1)
	} else {
		n = (math.Log(m(p1)) - math.Log(m(p2))) / (math.Log(t(p1)) - math.Log(t(p2)))
	}
	bigF := m(p1) / (n * math.Pow(t(p1), n))
	p.lcc = &lccConsts{n: n, bigF: bigF, rho0: p.A * bigF * math.Pow(t(p0), n), e: e}
}

func (p *Projection) lccForward(lat, lon float64) (float64, float64) {
	c := p.lcc
	phi := rad(lat)
	s := c.e * math.Sin(phi)
	t := math.Tan(math.Pi/4-phi/2) / math.Pow((1-s)/(1+s), c.e/2)
	rho := p.A * c.bigF * math.Pow(t, c.n)
	theta := c.n * rad(lon-p.Lon0)
	return p.FalseE + p.K0*rho*math.Sin(theta), p.FalseN + p.K0*(c.rho0-rho*math.Cos(theta))
}

// JSON returns the projection in the shape the viewer pages expect (chart.json "projection").
func (p *Projection) JSON() map[string]any {
	switch p.Kind {
	case KindTM:
		return map[string]any{"type": KindTM, "lon0": p.Lon0, "k0": p.K0,
			"falseEasting": p.FalseE, "falseNorthing": p.FalseN}
	case KindLCC:
		return map[string]any{"type": KindLCC, "a": p.A, "invF": p.InvF, "lon0": p.Lon0,
			"lat0": p.Lat0, "lat1": p.Lat1, "lat2": p.Lat2, "k0": p.K0,
			"falseEasting": p.FalseE, "falseNorthing": p.FalseN}
	default:
		return map[string]any{"type": KindGeographic}
	}
}

// ---- WKT ----

func wktParam(wkt, name string) (float64, bool) {
	re := regexp.MustCompile(`(?i)PARAMETER\[\\?"` + regexp.QuoteMeta(name) + `\\?"\s*,\s*(-?\d+(?:\.\d+)?)\]`)
	m := re.FindStringSubmatch(wkt)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	return v, err == nil
}

// FromWKT builds a projection from the /GCS WKT of a geospatial PDF.
// Anything that is not UTM/Transverse Mercator, Lambert or geographic is refused.
func FromWKT(wkt string) (*Projection, error) {
	switch {
	case regexp.MustCompile(`Transverse[_ ]Mercator`).MatchString(wkt):
		p := &Projection{Kind: KindTM}
		var ok1, ok2, ok3, ok4 bool
		p.Lon0, ok1 = wktParam(wkt, "Central_Meridian")
		p.K0, ok2 = wktParam(wkt, "Scale_Factor")
		p.FalseE, ok3 = wktParam(wkt, "False_Easting")
		p.FalseN, ok4 = wktParam(wkt, "False_Northing")
		if !(ok1 && ok2 && ok3 && ok4) {
			return nil, fmt.Errorf("Transverse Mercator WKT is missing parameters")
		}
		return p, nil

	case regexp.MustCompile(`Lambert_Conformal_Conic`).MatchString(wkt):
		p := &Projection{Kind: KindLCC, A: 6378137.0, InvF: 298.257223563, K0: 1}
		if m := regexp.MustCompile(`SPHEROID\[\\?"[^"\\]*\\?"\s*,\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)`).FindStringSubmatch(wkt); m != nil {
			p.A, _ = strconv.ParseFloat(m[1], 64)
			p.InvF, _ = strconv.ParseFloat(m[2], 64)
		}
		var ok0, ok1 bool
		p.Lon0, ok0 = wktParam(wkt, "Central_Meridian")
		p.Lat0, ok1 = wktParam(wkt, "Latitude_Of_Origin")
		var ok2 bool
		p.Lat1, ok2 = wktParam(wkt, "Standard_Parallel_1")
		if v, ok := wktParam(wkt, "Standard_Parallel_2"); ok {
			p.Lat2 = v
		} else {
			p.Lat2 = p.Lat1
		}
		if v, ok := wktParam(wkt, "Scale_Factor"); ok {
			p.K0 = v
		}
		p.FalseE, _ = wktParam(wkt, "False_Easting")
		p.FalseN, _ = wktParam(wkt, "False_Northing")
		if !(ok0 && ok1 && ok2) {
			return nil, fmt.Errorf("Lambert WKT is missing parameters")
		}
		p.initLCC()
		return p, nil

	case !regexp.MustCompile(`(?i)PROJECTION`).MatchString(wkt):
		return &Projection{Kind: KindGeographic}, nil
	}
	return nil, fmt.Errorf("unsupported projection (supported: Transverse Mercator/UTM, Lambert Conformal Conic, geographic)")
}

// ---- Affine fit ----

// Affine is  u = A*x + B*y + C ;  v = D*x + E*y + F.
type Affine struct{ A, B, C, D, E, F float64 }

func solve3(a [3][3]float64, b [3]float64) [3]float64 {
	var m [3][4]float64
	for i := 0; i < 3; i++ {
		copy(m[i][:3], a[i][:])
		m[i][3] = b[i]
	}
	for c := 0; c < 3; c++ {
		p := c
		for r := c + 1; r < 3; r++ {
			if math.Abs(m[r][c]) > math.Abs(m[p][c]) {
				p = r
			}
		}
		m[c], m[p] = m[p], m[c]
		for r := c + 1; r < 3; r++ {
			f := m[r][c] / m[c][c]
			for k := c; k < 4; k++ {
				m[r][k] -= f * m[c][k]
			}
		}
	}
	var x [3]float64
	for r := 2; r >= 0; r-- {
		s := m[r][3]
		for k := r + 1; k < 3; k++ {
			s -= m[r][k] * x[k]
		}
		x[r] = s / m[r][r]
	}
	return x
}

// FitAffine returns the least-squares affine map from src to dst (same length, >= 3).
// Coordinates are centred first: projected metres are ~1e6 and would make the normal
// equations badly conditioned.
func FitAffine(src, dst [][2]float64) (Affine, error) {
	if len(src) < 3 || len(src) != len(dst) {
		return Affine{}, fmt.Errorf("need at least 3 matching points")
	}
	var cx, cy float64
	for _, p := range src {
		cx += p[0]
		cy += p[1]
	}
	cx /= float64(len(src))
	cy /= float64(len(src))

	var ata [3][3]float64
	var bu, bv [3]float64
	for i, p := range src {
		row := [3]float64{p[0] - cx, p[1] - cy, 1}
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				ata[r][c] += row[r] * row[c]
			}
			bu[r] += row[r] * dst[i][0]
			bv[r] += row[r] * dst[i][1]
		}
	}
	u := solve3(ata, bu)
	v := solve3(ata, bv)
	return Affine{
		A: u[0], B: u[1], C: u[2] - u[0]*cx - u[1]*cy,
		D: v[0], E: v[1], F: v[2] - v[0]*cx - v[1]*cy,
	}, nil
}

// ---- constructors and inverse, for sources that carry the projection as numbers (GeoTIFF) ----

// NewLCC builds a Lambert Conformal Conic projection (degrees, metres). lat2 equal to lat1 is the
// one-parallel form.
func NewLCC(a, invF, lat0, lon0, lat1, lat2, k0, falseE, falseN float64) *Projection {
	p := &Projection{Kind: KindLCC, A: a, InvF: invF, Lat0: lat0, Lon0: lon0, Lat1: lat1, Lat2: lat2,
		K0: k0, FalseE: falseE, FalseN: falseN}
	p.initLCC()
	return p
}

// NewTM builds a (WGS84) Transverse Mercator projection, UTM when k0 is 0.9996.
func NewTM(lon0, k0, falseE, falseN float64) *Projection {
	return &Projection{Kind: KindTM, Lon0: lon0, K0: k0, FalseE: falseE, FalseN: falseN}
}

// NewGeographic is the plain latitude/longitude "projection" (x = lon, y = lat, in degrees).
func NewGeographic() *Projection { return &Projection{Kind: KindGeographic} }

// Inverse returns the latitude and longitude of projected coordinates. It exists for the
// Lambert projection and the geographic one; others report ok=false.
func (p *Projection) Inverse(x, y float64) (lat, lon float64, ok bool) {
	switch p.Kind {
	case KindGeographic:
		return y, x, true
	case KindLCC:
		if p.lcc == nil {
			return 0, 0, false
		}
		c := p.lcc
		xp := (x - p.FalseE) / p.K0
		yp := (y - p.FalseN) / p.K0
		d := c.rho0 - yp
		sgn := 1.0
		if c.n < 0 {
			sgn = -1
		}
		rho := sgn * math.Hypot(xp, d)
		theta := math.Atan2(sgn*xp, sgn*d)
		t := math.Pow(rho/(p.A*c.bigF), 1/c.n)
		phi := math.Pi/2 - 2*math.Atan(t)
		for i := 0; i < 8; i++ {
			s := c.e * math.Sin(phi)
			phi = math.Pi/2 - 2*math.Atan(t*math.Pow((1-s)/(1+s), c.e/2))
		}
		return phi * 180 / math.Pi, theta/c.n*180/math.Pi + p.Lon0, true
	case KindTM:
		return p.tmInverse(x, y)
	}
	return 0, 0, false
}

// tmInverse is the Kruger series of the Transverse Mercator projection, the inverse of tm.
func (p *Projection) tmInverse(x, y float64) (float64, float64, bool) {
	n := wgsN
	beta := [3]float64{
		n/2 - 2*n*n/3 + 37*math.Pow(n, 3)/96,
		n*n/48 + math.Pow(n, 3)/15,
		17 * math.Pow(n, 3) / 480,
	}
	delta := [3]float64{
		2*n - 2*n*n/3 - 2*math.Pow(n, 3),
		7*n*n/3 - 8*math.Pow(n, 3)/5,
		56 * math.Pow(n, 3) / 15,
	}
	xi := (y - p.FalseN) / (p.K0 * wgsAA)
	eta := (x - p.FalseE) / (p.K0 * wgsAA)
	xi0, eta0 := xi, eta
	for j := 1; j <= 3; j++ {
		b := beta[j-1]
		xi0 -= b * math.Sin(2*float64(j)*xi) * math.Cosh(2*float64(j)*eta)
		eta0 -= b * math.Cos(2*float64(j)*xi) * math.Sinh(2*float64(j)*eta)
	}
	chi := math.Asin(math.Sin(xi0) / math.Cosh(eta0))
	lat := chi
	for j := 1; j <= 3; j++ {
		lat += delta[j-1] * math.Sin(2*float64(j)*chi)
	}
	lon := p.Lon0 + math.Atan2(math.Sinh(eta0), math.Cos(xi0))*180/math.Pi
	return lat * 180 / math.Pi, lon, true
}
