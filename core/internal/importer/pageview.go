package importer

import (
	"fmt"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
)

// pageView is a PDF page as it is shown: its box and its /Rotate. PDF points are in user space (origin
// at the bottom left, y up); the picture is in the shown page (origin at the top left, y down), and a
// page marked /Rotate 90, 180 or 270 is shown turned clockwise by that much.
type pageView struct {
	box [4]float64 // x0, y0, x1, y1 in user space
	// W, H are the sides of the shown page in points (swapped for 90 and 270).
	W, H float64
	// u = au*x + bu*y + cu and v = av*x + bv*y + cv, in points of the shown page.
	au, bu, cu, av, bv, cv float64
}

func newPageView(box [4]float64, rotate int) (pageView, error) {
	x0, y0, x1, y1 := box[0], box[1], box[2], box[3]
	pv := pageView{box: box, W: x1 - x0, H: y1 - y0}
	switch ((rotate % 360) + 360) % 360 {
	case 0:
		pv.au, pv.cu, pv.bv, pv.cv = 1, -x0, -1, y1
	case 90:
		pv.W, pv.H = pv.H, pv.W
		pv.bu, pv.cu, pv.av, pv.cv = 1, -y0, 1, -x0
	case 180:
		pv.au, pv.cu, pv.bv, pv.cv = -1, x1, 1, -y0
	case 270:
		pv.W, pv.H = pv.H, pv.W
		pv.bu, pv.cu, pv.av, pv.cv = -1, y1, -1, x1
	default:
		return pv, fmt.Errorf("page rotation %d is not a multiple of 90", rotate)
	}
	return pv, nil
}

// point is a point of user space in points of the shown page.
func (pv pageView) point(x, y float64) (u, v float64) {
	return pv.au*x + pv.bu*y + pv.cu, pv.av*x + pv.bv*y + pv.cv
}

// compose chains (E,N) -> viewport (U,V) -> user space -> shown page -> pixels. aff maps (E,N) to the
// viewport's normalised (U,V); vp is the viewport's x0, y0, width and height; sx and sy are the pixels
// per point of the shown page.
func (pv pageView) compose(aff geo.Affine, vp [4]float64, sx, sy float64) geo.Affine {
	x0, y0, bw, bh := vp[0], vp[1], vp[2], vp[3]
	return geo.Affine{
		A: sx * (pv.au*bw*aff.A + pv.bu*bh*aff.D),
		B: sx * (pv.au*bw*aff.B + pv.bu*bh*aff.E),
		C: sx * (pv.au*(x0+bw*aff.C) + pv.bu*(y0+bh*aff.F) + pv.cu),
		D: sy * (pv.av*bw*aff.A + pv.bv*bh*aff.D),
		E: sy * (pv.av*bw*aff.B + pv.bv*bh*aff.E),
		F: sy * (pv.av*(x0+bw*aff.C) + pv.bv*(y0+bh*aff.F) + pv.cv),
	}
}
