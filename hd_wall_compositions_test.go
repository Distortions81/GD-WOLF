package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestHDWallCompositionsReuseDecorationsAndReplaceStaleVariants(t *testing.T) {
	root := t.TempDir()
	write := func(name string, c color.NRGBA) {
		t.Helper()
		path := filepath.Join(root, "walls", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	write("horizontal/tile-01.png", color.NRGBA{100, 100, 100, 255})
	write("horizontal/tile-03.png", color.NRGBA{0, 0, 255, 255}) // Older baked variant.
	write("decorations/shared.png", color.NRGBA{200, 0, 0, 128})
	manifest := hdWallCompositionManifest{Horizontal: map[int]hdWallComposition{
		3:    {Base: "horizontal/tile-01.png", Layers: []hdWallLayer{{Image: "decorations/shared.png"}}},
		4:    {Base: "horizontal/tile-01.png", Layers: []hdWallLayer{{Image: "decorations/shared.png", Rect: &[4]float64{0, 0, .5, .5}}}},
		5:    {Base: "horizontal/tile-01.png", Layers: []hdWallLayer{{Image: "decorations/missing.png"}}},
		6:    {Base: "../outside.png"},
		9999: {Base: "horizontal/tile-01.png"},
	}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "walls", "compositions.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	walls := &wl6.WallSet{}
	for _, id := range []int{3, 4, 5, 6} {
		walls.Horizontal[id] = buildWallTextureFromImage((id-1)*2, solidRGBA(10, 20, 30, 255))
	}
	g := &game{walls: walls, hdAssetRoot: root, hdTexturesEnabled: true, renderMode: renderModeUltra}
	g.applyHDWallOverrides()
	for _, id := range []int{3, 4} {
		if got := g.walls.Horizontal[id].Pixel(0, 0); got != 0x963131ff {
			t.Fatalf("composed wall %d = %#08x, want shared translucent decoration on gray", id, got)
		}
	}
	if got := g.walls.Horizontal[4].Pixel(3, 3); got != 0x646464ff {
		t.Fatalf("outside placed decoration = %#08x, want untouched shared base", got)
	}
	if got := g.walls.Horizontal[1].Pixel(0, 0); got != 0x646464ff {
		t.Fatalf("composition mutated shared base: %#08x", got)
	}
	if got := g.walls.Vertical[3].Pixel(0, 0); got == g.walls.Horizontal[3].Pixel(0, 0) || byte(got) != 255 {
		t.Fatalf("shaded composed wall = %#08x, want dimmed opaque orientation", got)
	}
	for _, id := range []int{5, 6} {
		if got := g.walls.Horizontal[id].Pixel(0, 0); got != 0x0a141eff {
			t.Fatalf("failed composition replaced wall %d with partial artwork: %#08x", id, got)
		}
	}
}

func TestHDWallLayerScalingIgnoresInvisibleBackdropColor(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 3, 1))
	for x := 0; x < 3; x++ {
		dst.SetRGBA(x, 0, color.RGBA{20, 30, 40, 255})
	}
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{200, 0, 0, 128})
	src.SetNRGBA(1, 0, color.NRGBA{0, 255, 0, 0})
	drawHDWallLayer(dst, dst.Bounds(), src)
	if got := dst.RGBAAt(1, 0); got.R < 64 || got.R > 66 || got.G > 30 || got.A != 255 {
		t.Fatalf("filtered edge = %v, want red blended over the base without green fringe", got)
	}
	if got := dst.RGBAAt(2, 0); got != (color.RGBA{20, 30, 40, 255}) {
		t.Fatalf("transparent decoration replaced base: %v", got)
	}
}
