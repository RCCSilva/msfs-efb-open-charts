package chartfox

import "math"

// GeoRef is a georeference as ChartFox describes it (https://chartfox.org/developers/docs/datatypes): a transform
// between a point of the chart and EPSG:3857 coordinates.
//
// Not validated against a real chart yet: the only georeferenced sample so far is the documentation's invented one, and
// the charts imported so far carry their own georeference in the PDF, which the importer already reads. Do not use it to
// place a chart before checking it against one.
type GeoRef struct {
	TX           float64 `json:"tx"`
	TY           float64 `json:"ty"`
	K            float64 `json:"k"`
	Angle        float64 `json:"transform_angle"` // degrees
	PageRotation int     `json:"pdf_page_rotation"`
	Page         int     `json:"page"`
}

// ChartPoint turns EPSG:3857 coordinates into a chart position, in pixels of a render `height` pixels tall, measured
// from the top-left corner of the chart as rotated by the georeference.
func (g GeoRef) ChartPoint(x, y, height float64) (cx, cy float64) {
	a := g.Angle * math.Pi / 180
	kLocal := g.K / height
	xt, yt := x-g.TX, y-g.TY
	cx = (math.Cos(a)*xt + math.Sin(a)*yt) / kLocal
	cy = (math.Sin(a)*xt - math.Cos(a)*yt) / kLocal
	return cx, cy
}

// World is the inverse of ChartPoint: it gives the EPSG:3857 coordinates of a chart position. The matrix of the
// transform is its own inverse (a reflection), so the same terms apply.
func (g GeoRef) World(cx, cy, height float64) (x, y float64) {
	a := g.Angle * math.Pi / 180
	kLocal := g.K / height
	px, py := cx*kLocal, cy*kLocal
	x = g.TX + math.Cos(a)*px + math.Sin(a)*py
	y = g.TY + math.Sin(a)*px - math.Cos(a)*py
	return x, y
}
