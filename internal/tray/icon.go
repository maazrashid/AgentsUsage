package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func appIcon(size int, foreground color.RGBA) []byte {
	if size < 16 {
		size = 16
	}
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	unit := max(size/8, 2)
	gap := max(size/16, 1)
	base := size - size/5
	heights := []int{size * 3 / 8, size * 5 / 8, size * 4 / 8}
	left := (size - (3*unit + 2*gap)) / 2
	for index, height := range heights {
		x0 := left + index*(unit+gap)
		x1 := x0 + unit
		y0 := base - height
		for y := y0; y < base; y++ {
			for x := x0; x < x1; x++ {
				canvas.SetRGBA(x, y, foreground)
			}
		}
	}
	var output bytes.Buffer
	_ = png.Encode(&output, canvas)
	return output.Bytes()
}
