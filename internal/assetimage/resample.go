// Package assetimage prepares transparent artwork for runtime asset canvases.
package assetimage

import (
	"image"
	"image/color"
	"math"
)

// ResampleCutout filters in premultiplied color space. Invisible backdrop RGB
// never bleeds into the visible edge; the result retains straight PNG alpha.
func ResampleCutout(dst *image.NRGBA, target image.Rectangle, src image.Image, source image.Rectangle) {
	for y := target.Min.Y; y < target.Max.Y; y++ {
		for x := target.Min.X; x < target.Max.X; x++ {
			sx := float64(source.Min.X) + (float64(x-target.Min.X)+0.5)*float64(source.Dx())/float64(target.Dx()) - 0.5
			sy := float64(source.Min.Y) + (float64(y-target.Min.Y)+0.5)*float64(source.Dy())/float64(target.Dy()) - 0.5
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			fx, fy := sx-float64(x0), sy-float64(y0)
			var channels [4]float64
			for yy := 0; yy < 2; yy++ {
				for xx := 0; xx < 2; xx++ {
					wx, wy := 1-fx, 1-fy
					if xx == 1 {
						wx = fx
					}
					if yy == 1 {
						wy = fy
					}
					px := max(source.Min.X, min(source.Max.X-1, x0+xx))
					py := max(source.Min.Y, min(source.Max.Y-1, y0+yy))
					r, g, b, a := src.At(px, py).RGBA()
					for c, v := range [4]uint32{r, g, b, a} {
						channels[c] += float64(v) * wx * wy
					}
				}
			}
			clr := color.NRGBA{}
			if channels[3] > 0 {
				clr = color.NRGBA{R: uint8(math.Round(channels[0] * 255 / channels[3])), G: uint8(math.Round(channels[1] * 255 / channels[3])), B: uint8(math.Round(channels[2] * 255 / channels[3])), A: uint8(math.Round(channels[3] / 257))}
			}
			dst.SetNRGBA(x, y, clr)
		}
	}
}
