package assetimage

import (
	"image"
	"image/color"
	"testing"
)

func TestResampleCutoutRejectsInvisibleBackdropColors(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{G: 255})
	dst := image.NewNRGBA(image.Rect(0, 0, 7, 3))
	ResampleCutout(dst, image.Rect(1, 1, 6, 2), src, src.Bounds())
	partial := false
	for x := 1; x < 6; x++ {
		p := dst.NRGBAAt(x, 1)
		if p.A > 0 && (p.R != 255 || p.G != 0 || p.B != 0) {
			t.Fatalf("edge borrowed invisible green RGB: %v", p)
		}
		partial = partial || p.A > 0 && p.A < 255
	}
	if !partial {
		t.Fatal("filtered edge lost its antialiasing")
	}
	if dst.NRGBAAt(0, 1).A != 0 || dst.NRGBAAt(6, 1).A != 0 || dst.NRGBAAt(2, 0).A != 0 {
		t.Fatal("filter wrote outside registered rectangle")
	}
}
