package tray

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"
)

func TestAppIconProducesValidPNG(t *testing.T) {
	data := appIcon(32, color.RGBA{R: 30, G: 220, B: 140, A: 255})
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 32 || decoded.Bounds().Dy() != 32 {
		t.Fatalf("unexpected icon bounds: %v", decoded.Bounds())
	}
}
