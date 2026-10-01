//go:build ignore

// Redraw stone contours from continuous rounded shapes, rather than enlarged
// 64px outlines. Run from the repository root with:
// GOCACHE=/tmp/gdwolf-site-build-cache go run art/walls-hd/v1/remake_stone.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type block struct{ x, y, w, h, radius, seed, cornerCut float64 }

var stone01 = []block{
	{44, 1, 28, 12, .65, 1, 0}, // wraps into the partial upper-left stone
	{11, 0, 16, 23, .75, 2, 0},
	{29, 0, 13, 10, .55, 3, 0},
	{50, 16, 25, 12, .7, 4, 0}, // wraps across the horizontal texture boundary
	{30, 12, 12, 11, .6, 5, 0},
	{1, 30, 9, 19, .65, 6, 0},
	{12, 26, 10, 24, .7, 7, 0},
	{25, 25, 16, 16, .8, 8, 0},
	{44, 17, 5, 22, .6, 9, 0},
	{51, 30, 11, 9, .6, 10, 0},
	{26, 43, 36, 10, .7, 11, 0},
	{1, 54, 32, 8, .7, 12, 0},
	{36, 56, 9, 6, .6, 13, 0},
	{48, 57, 15, 6, .6, 14, 0},
}
var stone02 = []block{
	{1, 1, 21, 11, .7, 21, 0},
	{24, 0, 15, 43, .8, 22, 8}, // one broad, smoothly curved diagonal corner break
	{41, 1, 23, 12, .7, 23, 0},
	{53, 16, 30, 11, .7, 24, 0},
	{42, 17, 11, 9, .7, 25, 0},
	{2, 29, 20, 11, .7, 26, 0},
	{34, 34, 6, 10, .6, 27, 0},
	{42, 29, 20, 15, .75, 28, 0},
	{1, 43, 16, 19, .75, 29, 0},
	{19, 45, 24, 15, .75, 30, 0},
	{46, 47, 16, 12, .7, 31, 0},
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func blend(a, b, t float64) float64   { return a + (b-a)*t }
func smooth(lo, hi, v float64) float64 {
	t := clamp((v-lo)/(hi-lo), 0, 1)
	return t * t * (3 - 2*t)
}
func smoothMax(a, b, k float64) float64 {
	h := clamp(.5+.5*(a-b)/k, 0, 1)
	return blend(b, a, h) + k*h*(1-h)
}
func roundedRect(x, y, cx, cy, hw, hh, r float64) float64 {
	dx := math.Abs(x-cx) - hw + r
	dy := math.Abs(y-cy) - hh + r
	return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) +
		math.Min(math.Max(dx, dy), 0) - r
}
func (b block) distance(x, y float64) float64 {
	// A pair of low-frequency curves gives a natural cut edge without any
	// copied staircase geometry or small square notches.
	wx := x + .09*math.Sin(y*.37+b.seed) + .035*math.Sin(y*.83+b.seed*2)
	wy := y + .08*math.Sin(x*.31+b.seed*.7) + .04*math.Sin(x*.71+b.seed)
	d := roundedRect(wx, wy, b.x+b.w/2, b.y+b.h/2, b.w/2, b.h/2, b.radius)
	if b.cornerCut > 0 {
		diagonal := ((wx - (b.x + b.w)) + (wy - (b.y + b.h)) + b.cornerCut) / math.Sqrt2
		d = smoothMax(d, diagonal, .75)
	}
	return d
}
func stone(x, y float64, blocks []block) [3]float64 {
	// Very quiet mortar and fine continuous surface variation.
	c := [3]float64{58, 58, 58}
	for _, b := range blocks {
		for _, offset := range []float64{-64, 0, 64} {
			px := x + offset
			if px < b.x-1 || px > b.x+b.w+1 || y < b.y-1 || y > b.y+b.h+1 {
				continue
			}
			d := b.distance(px, y)
			shadow := .19 * math.Exp(-math.Max(d, 0)/.35) * (1 - smooth(-.1, .45, d))
			for i := range c {
				c[i] *= 1 - shadow
			}
			a := 1 - smooth(-.045, .045, d)
			if a <= 0 {
				continue
			}
			nx := b.distance(px+.02, y) - b.distance(px-.02, y)
			ny := b.distance(px, y+.02) - b.distance(px, y-.02)
			normalLen := math.Hypot(nx, ny)
			if normalLen > 0 {
				nx /= normalLen
				ny /= normalLen
			}
			face := smooth(0, .9, -d)
			lighting := (-.55*nx - .83*ny)
			bevelShade := 1 + .29*lighting - .055
			shade := blend(bevelShade, 1, face)
			broad := 2.2*math.Sin(px*.21+y*.17+b.seed) +
				1.2*math.Sin(px*.47-y*.29+b.seed*.9)
			fine := .7*math.Sin(px*13.21+y*8.17+b.seed) +
				.45*math.Sin(px*26.13-y*17.31+b.seed)
			value := (173 + 4*math.Sin(b.seed*.8) + broad + fine) * shade
			// Slight quiet wear along the broad smooth bevel, not chips.
			for i := range c {
				c[i] = blend(c[i], value, a)
			}
		}
	}
	return c
}
func circle(x, y, cx, cy, r float64) float64 { return math.Hypot(x-cx, y-cy) - r }
func object(c *[3]float64, target [3]float64, d float64) {
	a := 1 - smooth(-.045, .045, d)
	for i := range c {
		c[i] = blend(c[i], target[i], a)
	}
}
func banner(x, y float64, c [3]float64) [3]float64 {
	// Plain unmarked cloth, matching the user's requested replacement.
	const left = 13.0
	const right = 51.0
	const top = 8.0
	const bottom = 57.0
	d := roundedRect(x, y, (left+right)/2, (top+bottom)/2, (right-left)/2, (bottom-top)/2, .13)
	if x > left-.7 && x < right+.8 && y > top-.3 && y < bottom+.8 {
		shadow := .28 * math.Exp(-math.Max(d, 0)/.22)
		for i := range c {
			c[i] *= 1 - shadow
		}
	}
	// Thin gold rod and spherical finials, with analytic shading.
	rod := roundedRect(x, y, 32, 6.8, 25, .48, .42)
	rodShade := .73 + .48*math.Exp(-math.Pow((y-6.6)/.30, 2))
	object(&c, [3]float64{173 * rodShade, 130 * rodShade, 58 * rodShade}, rod)
	for _, cx := range []float64{7, 57} {
		cd := circle(x, y, cx, 6.8, .94)
		ux := (x - cx) / .94
		uy := (y - 6.8) / .94
		z := math.Sqrt(math.Max(0, 1-ux*ux-uy*uy))
		diffuse := math.Max(0, -.4*ux-.5*uy+.7*z)
		highlight := math.Exp(-math.Pow((ux+.32)/.28, 2) - math.Pow((uy+.37)/.28, 2))
		shade := .55 + .56*diffuse + .3*highlight
		object(&c, [3]float64{184 * shade, 139 * shade, 63 * shade}, cd)
	}
	if d < .045 {
		u := (x - left) / (right - left)
		folds := .065*math.Sin(u*math.Pi*8) + .035*math.Sin(u*math.Pi*17+.05*y)
		foldShadow := .12*math.Exp(-math.Pow((u-.16)/.055, 2)) +
			.1*math.Exp(-math.Pow((u-.77)/.065, 2))
		fabric := .004*math.Sin(x*21+y*7) + .003*math.Sin(x*13-y*23)
		shade := 1 + folds - foldShadow + fabric
		hemDistance := math.Min(math.Min(x-left, right-x), bottom-y)
		shade *= .92 + .08*smooth(.14, .55, hemDistance)
		stitch := .96 + .04*smooth(.025, .065, math.Abs(hemDistance-.25))
		object(&c, [3]float64{178 * shade * stitch, 12 * shade, 18 * shade}, d)
	}
	return c
}
func render(tile int) error {
	const size = 1024
	blocks := stone01
	if tile == 2 {
		blocks = stone02
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Coordinates are centered native HD samples, not a resized low-res grid.
			px := (float64(x) + .5) * 64 / size
			py := (float64(y) + .5) * 64 / size
			c := stone(px, py, blocks)
			if tile == 3 {
				c = banner(px, py, c)
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(clamp(c[0], 0, 255) + .5),
				uint8(clamp(c[1], 0, 255) + .5), uint8(clamp(c[2], 0, 255) + .5), 255})
		}
	}
	path := filepath.Join("art/walls-hd/v1/alternatives", fmt.Sprintf("tile-%02d-direct.png", tile))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err = png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
func main() {
	for _, tile := range []int{1, 2, 3} {
		if err := render(tile); err != nil {
			panic(err)
		}
	}
}
