package main

import (
	"image"
	"image/color"
	"math/bits"
	"testing"

	"gd-wolf/internal/wl6"
)

func alphaRenderTestGame() *game {
	level := blankLevel(9, 9)
	for y := 0; y < level.Height; y++ {
		for x := 0; x < level.Width; x++ {
			if x == 0 || y == 0 || x == level.Width-1 || y == level.Height-1 {
				setLevelTile(level, x, y, wl6.Tile{RawWall: 1, Solid: true})
			}
		}
	}
	g := testGameWithLevel(level)
	g.playerX, g.playerY = 1.5, 4.5
	g.playerA = 0
	g.renderThreads = 1
	g.renderMode = renderModeUltra
	g.layout.bufferWidth, g.layout.bufferHeight = 64, 64
	g.gameplayFrame32 = make([]uint32, 64*64)
	g.gameplayFrame = byteViewFromU32(g.gameplayFrame32)
	g.gameplayBackground32 = make([]uint32, 64*64)
	g.gameplayBackground = byteViewFromU32(g.gameplayBackground32)
	g.ceilingColor = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	g.floorColor = color.RGBA{R: 80, G: 80, B: 80, A: 255}
	g.rebuildGameplayBackground()
	g.wallColumns = make([]wallColumn, 64)
	g.cameraColumns = make([]float64, 64)
	g.rayDirXColumns = make([]float64, 64)
	g.rayDirYColumns = make([]float64, 64)
	g.zbuffer = make([]float64, 64)
	for x := range g.cameraColumns {
		g.cameraColumns[x] = 2*(float64(x)+0.5)/64 - 1
	}
	g.walls = &wl6.WallSet{}
	wall := wl6.WallTexture{Width: 1, Height: 1, Pixels: []uint32{bits.ReverseBytes32(0x00ff00ff)}}
	g.walls.Horizontal[1], g.walls.Vertical[1] = wall, wall
	g.sprites = &wl6.SpriteSet{Pages: make([]wl6.Sprite, 2)}
	g.playerDying = true // Leave the world unobstructed unless a test supplies a weapon.
	return g
}

func alphaRenderTestSprite(clr color.NRGBA) wl6.Sprite {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, clr)
	return buildSpriteFromImage(img)
}

func TestGameplayTranslucentSpriteCompositesOverWall(t *testing.T) {
	g := alphaRenderTestGame()
	g.sprites.Pages[0] = alphaRenderTestSprite(color.NRGBA{R: 255, A: 128})
	g.staticSprites = []staticSprite{{x: 3.5, y: 4.5, shapenum: 0, alive: true}}
	g.renderGameplayFrame()
	if got := bits.ReverseBytes32(g.gameplayFrame32[32*64+32]); got != 0x807f00ff {
		t.Fatalf("sprite over green wall = %#08x, want %#08x", got, uint32(0x807f00ff))
	}
}

func TestGameplayTranslucentSpritesCompositeFarToNear(t *testing.T) {
	g := alphaRenderTestGame()
	g.sprites.Pages[0] = alphaRenderTestSprite(color.NRGBA{R: 255, A: 128})
	g.sprites.Pages[1] = alphaRenderTestSprite(color.NRGBA{B: 255, A: 128})
	// Deliberately supply the near sprite first; rendering must sort by depth.
	g.staticSprites = []staticSprite{
		{x: 3.5, y: 4.5, shapenum: 1, alive: true},
		{x: 5.5, y: 4.5, shapenum: 0, alive: true},
	}
	g.renderGameplayFrame()
	if got := bits.ReverseBytes32(g.gameplayFrame32[32*64+32]); got != 0x3f3f80ff {
		t.Fatalf("blue edge over red edge over green wall = %#08x, want %#08x", got, uint32(0x3f3f80ff))
	}
}

func TestGameplaySpriteOrderUsesCameraDepth(t *testing.T) {
	g := alphaRenderTestGame()
	g.sprites.Pages[0] = alphaRenderTestSprite(color.NRGBA{R: 255, A: 128})
	g.sprites.Pages[1] = alphaRenderTestSprite(color.NRGBA{B: 255, A: 128})
	// The nearer blue billboard is farther away by radial distance because it
	// is off-center. Camera depth must determine their order at the overlap.
	g.staticSprites = []staticSprite{
		{x: 3.5, y: 4.95, shapenum: 1, alive: true},
		{x: 3.52, y: 4.5, shapenum: 0, alive: true},
	}
	g.renderGameplayFrame()
	if got := bits.ReverseBytes32(g.gameplayFrame32[32*64+32]); got != 0x3f3f80ff {
		t.Fatalf("off-center blue edge over red edge = %#08x, want %#08x", got, uint32(0x3f3f80ff))
	}
}

func TestGameplayTranslucentWeaponCompositesOverSpriteAndFloor(t *testing.T) {
	g := alphaRenderTestGame()
	g.sprites.Pages[0] = alphaRenderTestSprite(color.NRGBA{R: 255, A: 255})
	g.staticSprites = []staticSprite{{x: 3.5, y: 4.5, shapenum: 0, alive: true}}
	g.playerDying = false
	shape, ok := g.currentWeaponShape()
	if !ok {
		t.Fatal("missing ready weapon shape")
	}
	g.sprites.Pages = append(g.sprites.Pages, make([]wl6.Sprite, shape+1-len(g.sprites.Pages))...)
	g.sprites.Pages[shape] = alphaRenderTestSprite(color.NRGBA{B: 255, A: 128})
	g.renderGameplayFrame()
	if got := bits.ReverseBytes32(g.gameplayFrame32[32*64+32]); got != 0x7f0080ff {
		t.Fatalf("weapon edge over red sprite = %#08x, want %#08x", got, uint32(0x7f0080ff))
	}
	i := 60*64 + 32
	floor := bits.ReverseBytes32(g.gameplayBackground32[i])
	want := uint32(byte(floor>>24))*127/255<<24 |
		uint32(byte(floor>>16))*127/255<<16 |
		(255*128+uint32(byte(floor>>8))*127)/255<<8 | 255
	if got := bits.ReverseBytes32(g.gameplayFrame32[i]); got != want {
		t.Fatalf("weapon edge over floor = %#08x, want %#08x", got, want)
	}
}

func TestGameplayWallsOccludeSprites(t *testing.T) {
	g := alphaRenderTestGame()
	g.sprites.Pages[0] = alphaRenderTestSprite(color.NRGBA{R: 255, A: 128})
	g.staticSprites = []staticSprite{{x: 9.5, y: 4.5, shapenum: 0, alive: true}}
	g.renderGameplayFrame()
	if got := bits.ReverseBytes32(g.gameplayFrame32[32*64+32]); got != 0x00ff00ff {
		t.Fatalf("wall in front of sprite = %#08x, want opaque green", got)
	}
}

func TestUltraGameplayBackgroundIsOpaqueAndLit(t *testing.T) {
	g := alphaRenderTestGame()
	for i := 3; i < len(g.gameplayBackground); i += 4 {
		if g.gameplayBackground[i] != 255 {
			t.Fatalf("background alpha at pixel %d = %d, want 255", i/4, g.gameplayBackground[i])
		}
	}
	// Both surfaces become darker toward the horizon, with no transparent holes.
	if g.gameplayBackground[0] <= g.gameplayBackground[(30*64)*4] {
		t.Fatal("ceiling did not darken toward the horizon")
	}
	if g.gameplayBackground[(63*64)*4] <= g.gameplayBackground[(32*64)*4] {
		t.Fatal("floor did not darken toward the horizon")
	}
}

func TestRebuildBackgroundRefreshesGameplayLighting(t *testing.T) {
	g := &game{renderMode: renderModeUltra, walls: &wl6.WallSet{}}
	g.walls.Palette[g.ceilingColorIndex()] = 0x646464ff
	g.walls.Palette[0x19] = 0x505050ff
	g.ensureFrame(320, 240)
	// Simulate a map/palette change without reallocating render buffers.
	g.walls.Palette[g.ceilingColorIndex()] = 0xc86432ff
	g.walls.Palette[0x19] = 0x3264c8ff
	g.rebuildBackground()
	if got := bits.ReverseBytes32(g.gameplayBackground32[0]); got != 0xc86432ff {
		t.Fatalf("refreshed ceiling = %#08x, want %#08x", got, uint32(0xc86432ff))
	}
	i := len(g.gameplayBackground32) - g.layout.bufferWidth
	if got := bits.ReverseBytes32(g.gameplayBackground32[i]); got != 0x3264c8ff {
		t.Fatalf("refreshed floor = %#08x, want %#08x", got, uint32(0x3264c8ff))
	}
}

func TestPremultipliedRGBAForGPUUpload(t *testing.T) {
	for _, tc := range []struct{ input, want uint32 }{
		{0xc8643200, 0x00000000},
		{0xc8643280, 0x64321980},
		{0xc86432ff, 0xc86432ff},
		{0xff800001, 0x01010001},
	} {
		if got := premultipliedRGBA(tc.input); got != tc.want {
			t.Fatalf("GPU color for %#08x = %#08x, want %#08x", tc.input, got, tc.want)
		}
	}
}
