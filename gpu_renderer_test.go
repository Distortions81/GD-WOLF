package main

import (
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gd-wolf/internal/wl6"
)

func TestSplitFixedRoundTrip(t *testing.T) {
	for _, value := range []uint32{0, 1, 0xffff, 0x10000, 0x12345678, 0xffffffff} {
		whole, fraction := splitFixed(value)
		got := uint32(whole)<<16 | uint32(fraction)
		if got != value {
			t.Fatalf("splitFixed(%#x) round trip = %#x", value, got)
		}
	}
}

func TestGPUCommandsKeepSourceOrder(t *testing.T) {
	first, second := new(ebiten.Image), new(ebiten.Image)
	var commands wolfGPUCommands
	commands.rect(first, 0, 0, 0, 9, 0, 0, 0, 1<<16)
	commands.rect(first, 1, 0, 1, 9, 0, 0, 0, 1<<16)
	commands.rect(second, 2, 0, 2, 9, 0, 0, 0, 1<<16)
	commands.rect(first, 3, 0, 3, 9, 0, 0, 0, 1<<16)
	if commands.used != 3 {
		t.Fatalf("batch count = %d, want 3 ordered source runs", commands.used)
	}
	if len(commands.batches[0].vertices) != 8 || len(commands.batches[0].indices) != 12 {
		t.Fatalf("first batch geometry = %d vertices, %d indices", len(commands.batches[0].vertices), len(commands.batches[0].indices))
	}
	if commands.batches[0].image != first || commands.batches[1].image != second || commands.batches[2].image != first {
		t.Fatal("texture batch order changed")
	}
}

func TestGPUClassicFrameMatchesSoftwareReference(t *testing.T) {
	if os.Getenv("GD_WOLF_GPU_TEST") == "" {
		t.Skip("set GD_WOLF_GPU_TEST=1 under a graphics-capable test session")
	}
	driver := &wolfGPUComparisonDriver{}
	driver.run = func() {
		compareGPUClassicFrames(t)
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

type wolfGPUComparisonDriver struct {
	done bool
	run  func()
}

func (d *wolfGPUComparisonDriver) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}

func (d *wolfGPUComparisonDriver) Layout(int, int) (int, int) { return 64, 64 }

func (d *wolfGPUComparisonDriver) Draw(*ebiten.Image) {
	if d.done {
		return
	}
	d.run()
	d.done = true
}

func compareGPUClassicFrames(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		tolerance int
		setup     func(*game)
	}{
		{name: "opaque-world"},
		{name: "translucent-sprite", tolerance: 1, setup: func(g *game) {
			g.sprites.Pages[0] = alphaRenderTestSprite(color.NRGBA{R: 255, A: 128})
			g.staticSprites = []staticSprite{{x: 3.5, y: 4.5, shapenum: 0, alive: true}}
		}},
		{name: "translucent-weapon", tolerance: 1, setup: func(g *game) {
			g.playerDying = false
			shape, ok := g.currentWeaponShape()
			if !ok {
				t.Fatal("missing ready weapon shape")
			}
			g.sprites.Pages = append(g.sprites.Pages, make([]wl6.Sprite, shape+1-len(g.sprites.Pages))...)
			g.sprites.Pages[shape] = alphaRenderTestSprite(color.NRGBA{B: 255, A: 128})
		}},
		{name: "hd-floor-light", tolerance: 2, setup: func(g *game) {
			shape := shapeSPR_STAT_3
			g.sprites.Pages = append(g.sprites.Pages, make([]wl6.Sprite, shape+1-len(g.sprites.Pages))...)
			g.sprites.Pages[shape] = alphaRenderTestSprite(color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			g.staticSprites = []staticSprite{{x: 3.5, y: 4.5, shapenum: shape, alive: true}}
			g.hdTexturesEnabled = true
			g.hdAssetRoot = "test-pack"
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			g := alphaRenderTestGame()
			if scenario.setup != nil {
				scenario.setup(g)
			}
			g.rebuildSpriteImageCache()
			g.gameplayBackgroundImage = ebiten.NewImage(g.layout.bufferWidth, g.layout.bufferHeight)
			g.gameplayBackgroundImage.WritePixels(g.gameplayBackground)

			g.renderGameplayFrame()
			expected := append([]byte(nil), g.gameplayFrame...)
			gpuFrame := g.renderGameplayFrameGPU()
			if gpuFrame == nil {
				t.Fatal("GPU renderer fell back to software")
			}
			actual := make([]byte, len(expected))
			gpuFrame.ReadPixels(actual)
			for i := range expected {
				difference := int(expected[i]) - int(actual[i])
				if difference < 0 {
					difference = -difference
				}
				if difference > scenario.tolerance {
					t.Fatalf("pixel channel %d differs: CPU=%d GPU=%d", i, expected[i], actual[i])
				}
			}
		})
	}
}
