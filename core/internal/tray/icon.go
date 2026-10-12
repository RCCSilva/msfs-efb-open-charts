package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// icon draws a small aircraft on a blue disc and wraps the PNG in an ICO container (Windows
// accepts PNG-compressed entries), so no binary asset has to be kept in the repository.
func icon() []byte { return ICO() }

// ICO returns the application icon as an .ico file.
func ICO() []byte {
	const n = 32
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	blue := color.RGBA{37, 99, 235, 255}
	white := color.RGBA{255, 255, 255, 255}
	c := float64(n-1) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x)-c, float64(y)-c
			if dx*dx+dy*dy <= 15.2*15.2 {
				img.Set(x, y, blue)
			}
		}
	}
	// aircraft silhouette, pointing up: fuselage, wings, tailplane
	rect := func(x0, y0, x1, y1 int) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				img.Set(x, y, white)
			}
		}
	}
	rect(15, 6, 16, 25)  // fuselage
	rect(15, 5, 16, 5)   // nose
	rect(6, 14, 25, 17)  // wings
	rect(11, 22, 20, 24) // tailplane

	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)
	p := pngBuf.Bytes()

	var ico bytes.Buffer
	w := func(v any) { _ = binary.Write(&ico, binary.LittleEndian, v) }
	w(uint16(0)) // reserved
	w(uint16(1)) // type: icon
	w(uint16(1)) // one image
	ico.Write([]byte{n, n, 0, 0})
	w(uint16(1))  // planes
	w(uint16(32)) // bits per pixel
	w(uint32(len(p)))
	w(uint32(6 + 16)) // offset of the image data
	ico.Write(p)
	return ico.Bytes()
}
