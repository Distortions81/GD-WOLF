//go:build ignore

// Remake the plain wood panel texture with continuous curves and analytic shading.
// Run from the repository root:
// GOCACHE=/tmp/gdwolf-site-build-cache go run art/walls-hd/v1/remake_wood.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func smooth(lo, hi, v float64) float64 {
	t := clamp((v-lo)/(hi-lo), 0, 1)
	return t * t * (3 - 2*t)
}
func mix(a, b, t float64) float64 { return a + (b-a)*t }

// All grain functions operate in normalized continuous coordinates. The board
// crossing the texture boundary uses the same coordinates at both ends.
func wood(x, y float64) [3]float64 {
	wrapped := math.Mod(x+7, 64)
	board, u, width := 0, wrapped, 16.0
	if wrapped >= 16 {
		board = 1 + int((wrapped-16)/12)
		u = math.Mod(wrapped-16, 12)
		width = 12
	}
	face := u / width
	phase := float64(board) * 1.49
	flow := face + 0.06*math.Sin(y*0.049+phase) +
		0.045*math.Sin(y*0.127+phase*0.7)
	// Curved growth rings and fine fibres; no random cells or enlarged pixels.
	rings := math.Sin(2 * math.Pi * (3.9*flow + 0.28*math.Cos(y*0.096+phase)))
	grain := math.Sin(2 * math.Pi * (75*flow + 0.04*math.Sin(y*0.45+phase)))
	finer := math.Sin(2 * math.Pi * (139*flow + 0.08*math.Sin(y*0.71+phase)))
	broad := 0.055*rings + 0.07*math.Cos(2*math.Pi*(1.8*flow)+phase)
	knots := math.Pow(math.Max(0, math.Cos(2*math.Pi*(1.55*flow+
		0.32*math.Cos(y*0.074+phase)))), 8)
	shade := 1 + broad - 0.11*knots + 0.017*grain + 0.006*finer
	c := [3]float64{112 * shade, 73 * shade, 37 * shade}

	// Narrow original recesses at x=9,21,33,45,57; warm beveled edges.
	distance := math.Min(u, width-u)
	recess := 1 - smooth(0.28, 0.65, distance)
	for i, dark := range [3]float64{43, 30, 14} {
		c[i] = mix(c[i], dark, recess)
	}
	highlight := math.Exp(-math.Pow((u-0.9)/0.35, 2)) * 0.37
	shadow := math.Exp(-math.Pow((width-u-0.82)/0.5, 2)) * 0.34
	for i := range c {
		c[i] = c[i] * (1 + highlight - shadow)
	}
	// A thin top rail across the full wall height, as in the original.
	if y < 3 {
		t := y / 3
		railGrain := 0.015 * math.Sin(2*math.Pi*(27*x/64+
			0.2*math.Sin(y*1.7)))
		railShade := 0.90 + 0.14*math.Sin(math.Pi*t) + railGrain
		c = [3]float64{117 * railShade, 78 * railShade, 40 * railShade}
		edge := 1 - smooth(2.65, 3, y)
		for i := range c {
			c[i] *= 0.75 + 0.25*edge
		}
	} else {
		shadow := 1 - 0.32*math.Exp(-(y-3)/0.4)
		for i := range c {
			c[i] *= shadow
		}
	}
	return c
}

func main() {
	const size = 1024
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var sum [3]float64
			// Four analytic samples per pixel antialias the beveled joins.
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					c := wood((float64(x)+(float64(sx)+0.5)/2)*64/size,
						(float64(y)+(float64(sy)+0.5)/2)*64/size)
					for i := range c {
						sum[i] += c[i] / 4
					}
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(clamp(sum[0], 0, 255) + 0.5),
				uint8(clamp(sum[1], 0, 255) + 0.5), uint8(clamp(sum[2], 0, 255) + 0.5), 255})
		}
	}
	f, err := os.Create("art/walls-hd/v1/alternatives/tile-12-direct.png")
	if err != nil {
		panic(err)
	}
	if err = png.Encode(f, img); err != nil {
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
}
