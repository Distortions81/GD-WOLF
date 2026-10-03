package main

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"testing"

	"gd-wolf/internal/wl6"
)

func floorLightTestGame() (*game, float64) {
	g := alphaRenderTestGame()
	g.hdTexturesEnabled, g.hdAssetRoot = true, "test-pack"
	g.sprites.Pages = make([]wl6.Sprite, shapeSPR_STAT_14+1)
	g.sprites.Pages[shapeSPR_STAT_4] = alphaRenderTestSprite(color.NRGBA{A: 255})
	g.staticSprites = []staticSprite{{x: 3.5, y: 4.5, shapenum: shapeSPR_STAT_4, alive: true}}
	for x := range g.zbuffer {
		g.zbuffer[x] = 6.5
	}
	g.prepareRaycastDirections(0, 64, 1, 0, 0, math.Tan(fov/2))
	return g, 64 / (2 * math.Tan(fov/2))
}

func TestFixtureLightBrightensFloorWithoutReplacingIt(t *testing.T) {
	for _, shape := range []int{shapeSPR_STAT_3, shapeSPR_STAT_4, shapeSPR_STAT_14} {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			g, projection := floorLightTestGame()
			g.staticSprites[0].shapenum = shape
			g.sprites.Pages[shape] = alphaRenderTestSprite(color.NRGBA{A: 255})
			before := slices.Clone(g.gameplayFrame32)
			g.drawLightFixtureFloorGlows(1, 0, projection)
			i := 45*64 + 32
			old, lit := before[i], g.gameplayFrame32[i]
			r := int(byte(lit)) - int(byte(old))
			gr := int(byte(lit>>8)) - int(byte(old>>8))
			b := int(byte(lit>>16)) - int(byte(old>>16))
			if r <= 0 || r < gr || gr <= b || byte(lit>>24) != 255 {
				t.Fatalf("floor light changed %#08x to %#08x, want subtle opaque warm illumination", old, lit)
			}
			for _, i := range []int{10*64 + 32, 45 * 64, 63*64 + 32} {
				if g.gameplayFrame32[i] != before[i] {
					t.Fatalf("light changed pixel %d outside its projected floor footprint", i)
				}
			}
		})
	}
}

func TestFixtureFloorLightMovesWithWorldProjection(t *testing.T) {
	g, projection := floorLightTestGame()
	g.playerY = 3.5
	g.drawLightFixtureFloorGlows(1, 0, projection)
	if i := 45*64 + 32; g.gameplayFrame32[i] != g.gameplayBackground32[i] {
		t.Fatal("light remained at screen center after moving sideways")
	}
	if i := 45*64 + 59; g.gameplayFrame32[i] == g.gameplayBackground32[i] {
		t.Fatal("light did not move to the projected world position")
	}
}

func TestFixtureFloorLightIsOccludedByWalls(t *testing.T) {
	g, projection := floorLightTestGame()
	for x := range g.zbuffer {
		g.zbuffer[x] = 1
	}
	before := slices.Clone(g.gameplayFrame32)
	g.drawLightFixtureFloorGlows(1, 0, projection)
	if !slices.Equal(g.gameplayFrame32, before) {
		t.Fatal("light was drawn through the wall in front of it")
	}
}

func TestFixtureFloorLightRequiresHDReplacement(t *testing.T) {
	for _, classic := range []bool{true, false} {
		g, projection := floorLightTestGame()
		if classic {
			g.sprites.Pages[shapeSPR_STAT_4].IndexedFormat = true
		} else {
			g.hdTexturesEnabled = false
		}
		before := slices.Clone(g.gameplayFrame32)
		g.drawLightFixtureFloorGlows(1, 0, projection)
		if !slices.Equal(g.gameplayFrame32, before) {
			t.Fatal("procedural light was added to the original sprite's baked-in floor patch")
		}
	}
}
