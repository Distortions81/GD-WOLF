package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestBuildWallTextureFromImagePreservesPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.SetRGBA(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	img.SetRGBA(0, 1, color.RGBA{R: 4, G: 5, B: 6, A: 255})
	img.SetRGBA(0, 2, color.RGBA{R: 7, G: 8, B: 9, A: 255})
	img.SetRGBA(1, 0, color.RGBA{R: 10, G: 11, B: 12, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 13, G: 14, B: 15, A: 255})
	img.SetRGBA(1, 2, color.RGBA{R: 16, G: 17, B: 18, A: 255})

	tex := buildWallTextureFromImage(7, img)
	if tex.Page != 7 {
		t.Fatalf("page = %d, want 7", tex.Page)
	}
	width, height := tex.Size()
	if width != 2 || height != 3 {
		t.Fatalf("size = %dx%d, want 2x3", width, height)
	}
	tests := []struct {
		x, y int
		want uint32
	}{
		{0, 0, 0x010203ff},
		{0, 1, 0x040506ff},
		{0, 2, 0x070809ff},
		{1, 0, 0x0a0b0cff},
		{1, 1, 0x0d0e0fff},
		{1, 2, 0x101112ff},
	}
	for _, tc := range tests {
		if got := tex.Pixel(tc.x, tc.y); got != tc.want {
			t.Fatalf("pixel(%d,%d) = %#08x, want %#08x", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestBuildSpriteFromImageBuildsSparseColumns(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	img.SetRGBA(0, 0, color.RGBA{R: 20, G: 21, B: 22, A: 255})
	img.SetRGBA(0, 2, color.RGBA{R: 30, G: 31, B: 32, A: 255})
	img.SetRGBA(0, 3, color.RGBA{R: 40, G: 41, B: 42, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 50, G: 51, B: 52, A: 255})

	sprite := buildSpriteFromImage(img)
	if sprite.Width != 2 || sprite.Height != 4 {
		t.Fatalf("size = %dx%d, want 2x4", sprite.Width, sprite.Height)
	}
	if got := len(sprite.Columns[0].Posts); got != 2 {
		t.Fatalf("column 0 post count = %d, want 2", got)
	}
	if got := len(sprite.Columns[1].Posts); got != 1 {
		t.Fatalf("column 1 post count = %d, want 1", got)
	}
	if sprite.Columns[0].Posts[0].StartY != 0 || len(sprite.Columns[0].Posts[0].Pixels) != 1 {
		t.Fatalf("column 0 post 0 = %+v, want one pixel at y=0", sprite.Columns[0].Posts[0])
	}
	if sprite.Columns[0].Posts[1].StartY != 2 || len(sprite.Columns[0].Posts[1].Pixels) != 2 {
		t.Fatalf("column 0 post 1 = %+v, want two pixels at y=2", sprite.Columns[0].Posts[1])
	}
	if sprite.Columns[1].Posts[0].StartY != 1 || len(sprite.Columns[1].Posts[0].Pixels) != 1 {
		t.Fatalf("column 1 post 0 = %+v, want one pixel at y=1", sprite.Columns[1].Posts[0])
	}
	if got := sprite.Pixels[1*sprite.Width+0]; got != 0 {
		t.Fatalf("transparent pixel = %#08x, want 0", got)
	}
}

func TestBuildSpriteFromImagePreservesStraightAlphaColors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})

	sprite := buildSpriteFromImage(img)
	if sprite.Width != 1 || sprite.Height != 1 {
		t.Fatalf("size = %dx%d, want 1x1", sprite.Width, sprite.Height)
	}
	if got := sprite.Pixels[0]; got != 0xc8643280 {
		t.Fatalf("pixel = %#08x, want %#08x", got, uint32(0xc8643280))
	}
	if got := sprite.Columns[0].Posts[0].Pixels[0]; got != 0xc8643280 {
		t.Fatalf("post pixel = %#08x, want %#08x", got, uint32(0xc8643280))
	}
}

func TestBlendRGBAOverBuffer(t *testing.T) {
	buf := []byte{10, 20, 30, 255}
	blendRGBAOverBuffer(buf, 0, 0xc8643200)
	if got := [4]byte{buf[0], buf[1], buf[2], buf[3]}; got != [4]byte{10, 20, 30, 255} {
		t.Fatalf("alpha 0 blend = %v, want unchanged", got)
	}

	blendRGBAOverBuffer(buf, 0, 0xff0000ff)
	if got := [4]byte{buf[0], buf[1], buf[2], buf[3]}; got != [4]byte{255, 0, 0, 255} {
		t.Fatalf("alpha 255 blend = %v, want opaque red", got)
	}

	buf = []byte{10, 20, 30, 255}
	blendRGBAOverBuffer(buf, 0, 0x6496c880)
	if got := [4]byte{buf[0], buf[1], buf[2], buf[3]}; got != [4]byte{55, 85, 115, 255} {
		t.Fatalf("alpha 128 blend = %v, want [55 85 115 255]", got)
	}
}

func TestApplyTopLeftColorKeyClearsMatchingPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	key := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	img.SetRGBA(0, 0, key)
	img.SetRGBA(1, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	img.SetRGBA(2, 0, key)
	img.SetRGBA(0, 1, key)
	img.SetRGBA(1, 1, color.RGBA{R: 40, G: 50, B: 60, A: 128})
	img.SetRGBA(2, 1, color.RGBA{R: 70, G: 80, B: 90, A: 255})

	applyTopLeftColorKey(img)

	if got := img.RGBAAt(0, 0); got != (color.RGBA{}) {
		t.Fatalf("top-left pixel = %+v, want transparent", got)
	}
	if got := img.RGBAAt(2, 0); got != (color.RGBA{}) {
		t.Fatalf("matching keyed pixel = %+v, want transparent", got)
	}
	if got := img.RGBAAt(0, 1); got != (color.RGBA{}) {
		t.Fatalf("matching keyed pixel = %+v, want transparent", got)
	}
	if got := img.RGBAAt(1, 0); got != (color.RGBA{R: 200, G: 100, B: 50, A: 255}) {
		t.Fatalf("non-key pixel = %+v, want preserved", got)
	}
	if got := img.RGBAAt(1, 1); got != (color.RGBA{R: 40, G: 50, B: 60, A: 128}) {
		t.Fatalf("alpha pixel = %+v, want preserved", got)
	}
}

func TestShouldUseHDAssetsRequiresToggle(t *testing.T) {
	g := &game{
		hdAssetRoot:       "hd-assets",
		hdTexturesEnabled: true,
		renderMode:        renderModeUltra,
	}
	if !g.shouldUseHDAssets() {
		t.Fatal("shouldUseHDAssets = false, want true")
	}
	g.hdTexturesEnabled = false
	if g.shouldUseHDAssets() {
		t.Fatal("shouldUseHDAssets = true, want false when toggle is off")
	}
}

func TestDimWallImageScalesRGBAndPreservesAlpha(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 200, G: 100, B: 60, A: 128})

	dimmed := dimWallImage(img, wl6.WallShadeScale)
	got := dimmed.RGBAAt(0, 0)
	want := color.RGBA{
		R: scaleColorByte(200, wl6.WallShadeScale),
		G: scaleColorByte(100, wl6.WallShadeScale),
		B: scaleColorByte(60, wl6.WallShadeScale),
		A: 128,
	}
	if got != want {
		t.Fatalf("dimmed pixel = %+v, want %+v", got, want)
	}
}

func TestApplyHDWallOverridesLoadsHorizontalAndAutoDimsVertical(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "walls", "horizontal")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 200, G: 100, B: 60, A: 255})
	file, err := os.Create(filepath.Join(path, "tile-01.png"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("Encode: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	g := &game{
		hdAssetRoot:       root,
		hdTexturesEnabled: true,
		renderMode:        renderModeUltra,
		walls: &wl6.WallSet{
			Horizontal: [64]wl6.WallTexture{1: {Page: 7}},
			Vertical:   [64]wl6.WallTexture{1: {Page: 8}},
		},
	}

	g.applyHDWallOverrides()

	if got := g.walls.Horizontal[1].Pixel(0, 0); got != 0xc8643cff {
		t.Fatalf("horizontal pixel = %#08x, want %#08x", got, uint32(0xc8643cff))
	}
	wantDimmed := dimPackedRGBA(0xc8643cff, wl6.WallShadeScale)
	if got := g.walls.Vertical[1].Pixel(0, 0); got != uint32(wantDimmed) {
		t.Fatalf("vertical pixel = %#08x, want %#08x", got, uint32(wantDimmed))
	}
}

func TestApplyHDWallOverridesLoadsEvenWallPageAndAutoDimsNextPage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "walls", "pages")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 200, G: 100, B: 60, A: 255})
	file, err := os.Create(filepath.Join(path, "page-000.png"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("Encode: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	g := &game{
		hdAssetRoot:       root,
		hdTexturesEnabled: true,
		renderMode:        renderModeUltra,
		walls: &wl6.WallSet{
			AllPages: []wl6.WallTexture{
				{Page: 0},
				{Page: 1},
			},
		},
	}

	g.applyHDWallOverrides()

	if got := g.walls.AllPages[0].Pixel(0, 0); got != 0xc8643cff {
		t.Fatalf("page 0 pixel = %#08x, want %#08x", got, uint32(0xc8643cff))
	}
	wantDimmed := dimPackedRGBA(0xc8643cff, wl6.WallShadeScale)
	if got := g.walls.AllPages[1].Pixel(0, 0); got != uint32(wantDimmed) {
		t.Fatalf("page 1 pixel = %#08x, want %#08x", got, uint32(wantDimmed))
	}
}

func TestApplyHDWallOverridesKeepsDimmedVerticalForStockFallback(t *testing.T) {
	root := t.TempDir()

	g := &game{
		hdAssetRoot:       root,
		hdTexturesEnabled: true,
		renderMode:        renderModeUltra,
		walls: &wl6.WallSet{
			AllPages: []wl6.WallTexture{
				buildWallTextureFromImage(0, solidRGBA(200, 100, 60, 255)),
				buildWallTextureFromImage(1, solidRGBA(10, 20, 30, 255)),
			},
		},
	}

	g.applyHDWallOverrides()

	if got := g.walls.Horizontal[1].Pixel(0, 0); got != 0xc8643cff {
		t.Fatalf("horizontal pixel = %#08x, want %#08x", got, uint32(0xc8643cff))
	}
	wantDimmed := dimPackedRGBA(0xc8643cff, wl6.WallShadeScale)
	if got := g.walls.Vertical[1].Pixel(0, 0); got != uint32(wantDimmed) {
		t.Fatalf("vertical pixel = %#08x, want %#08x", got, uint32(wantDimmed))
	}
}

func solidRGBA(r, g, b, a uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: r, G: g, B: b, A: a})
	return img
}
