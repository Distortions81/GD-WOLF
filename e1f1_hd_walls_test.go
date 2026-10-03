package main

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestHDWallDecorationsAreTransparentAndShared(t *testing.T) {
	root := filepath.Join("art", "walls-hd", "v1", "walls")
	data, err := os.ReadFile(filepath.Join(root, "compositions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest hdWallCompositionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	uses := map[string]int{}
	for _, spec := range manifest.Horizontal {
		for _, layer := range spec.Layers {
			if layer.Image != "" {
				uses[layer.Image]++
			}
		}
	}
	for _, name := range []string{"decorations/portrait-silhouette.png", "decorations/iron-bars.png"} {
		if uses[name] != 2 {
			t.Errorf("%s used %d times, want one shared image in both wall variants", name, uses[name])
		}
	}
	for name := range uses {
		f, err := os.Open(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		if b.Dx() < 1024 || b.Dy() < 1024 {
			t.Errorf("%s is still low resolution: %v", name, b.Size())
		}
		for _, p := range [][2]int{{b.Min.X, b.Min.Y}, {b.Max.X - 1, b.Min.Y}, {b.Min.X, b.Max.Y - 1}, {b.Max.X - 1, b.Max.Y - 1}} {
			if _, _, _, a := img.At(p[0], p[1]).RGBA(); a != 0 {
				t.Errorf("%s retains an opaque background at %v", name, p)
			}
		}
	}
}

func TestFirstLevelHDWallCoverage(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	walls, err := files.LoadWallSet()
	if err != nil {
		t.Fatal(err)
	}
	data, err := files.LoadMap(0)
	if err != nil {
		t.Fatal(err)
	}
	g := &game{walls: walls, hdAssetRoot: filepath.Join("art", "walls-hd", "v1"), hdTexturesEnabled: true, renderMode: renderModeUltra}
	g.applyHDWallOverrides()
	ids := map[int]bool{}
	for _, tile := range data.Level().Tiles {
		if tile.RenderWall && tile.RawWall > 0 && tile.RawWall < 64 {
			ids[int(tile.RawWall)] = true
		}
	}
	// Activating the level's elevator changes its control-panel wall to tile22.
	ids[int(wolfElevatorUsedTile)] = true
	check := func(label string, texture wl6.WallTexture) {
		t.Helper()
		w, h := texture.Size()
		if w < 1024 || h < 1024 || len(texture.Indices) != 0 {
			t.Errorf("%s still uses a low-resolution texture: %dx%d indexed=%v", label, w, h, len(texture.Indices) > 0)
		}
	}
	for id := range ids {
		check(wallTileFileName(id)+" horizontal", g.walls.Horizontal[id])
		check(wallTileFileName(id)+" vertical", g.walls.Vertical[id])
	}
	// The door renderer looks up raw page numbers, not horizontal tile50/51.
	for page := 98; page <= 103; page++ {
		check(wallPageFileName(page), g.walls.AllPages[page])
	}
}

func TestHDHangingFixturesHaveNoBakedFloorPatch(t *testing.T) {
	for _, shape := range []int{shapeSPR_STAT_4, shapeSPR_STAT_14} {
		path := filepath.Join("art", "e1f1-hd", "sprites", spriteShapeFileName(shape))
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		bounds := img.Bounds()
		for y := bounds.Min.Y + bounds.Dy()/2; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
					t.Fatalf("%s still paints a floor patch at (%d,%d)", path, x, y)
				}
			}
		}
	}
}
