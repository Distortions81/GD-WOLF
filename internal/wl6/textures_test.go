package wl6

import (
	"math/bits"
	"os"
	"reflect"
	"testing"
)

func TestExtractGamePalBytes(t *testing.T) {
	path, err := findRepoFile("WOLFSRC/OBJ/GAMEPAL.OBJ")
	if err != nil {
		t.Fatalf("find GAMEPAL.OBJ: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read GAMEPAL.OBJ: %v", err)
	}

	raw, err := extractGamePalBytes(data)
	if err != nil {
		t.Fatalf("extract palette: %v", err)
	}
	if len(raw) != 768 {
		t.Fatalf("palette byte count = %d, want 768", len(raw))
	}
	if got := raw[:len(gamePalPrefix)]; string(got) != string(gamePalPrefix) {
		t.Fatal("palette prefix did not match expected Wolfenstein 3D VGA palette")
	}
}

func TestLoadWallSet(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	set, err := files.LoadWallSet()
	if err != nil {
		t.Fatalf("load wall set: %v", err)
	}

	if len(set.AllPages) == 0 {
		t.Fatal("expected wall pages")
	}
	if got := len(set.AllPages[0].Pixels); got != wallTextureArea {
		t.Fatalf("page pixel count = %d, want %d", got, wallTextureArea)
	}
	if set.Horizontal[1].Pixels == nil {
		t.Fatal("expected horizontal wall texture for tile 1")
	}
	if set.Vertical[1].Pixels == nil {
		t.Fatal("expected vertical wall texture for tile 1")
	}
	if got := set.Vertical[1].Pixel(0, 0); got != dimPackedRGBA(set.Horizontal[1].Pixel(0, 0), WallShadeScale) {
		t.Fatalf("vertical pixel = %#08x, want dimmed horizontal %#08x", got, dimPackedRGBA(set.Horizontal[1].Pixel(0, 0), WallShadeScale))
	}
}

func TestLoadWallSetUsesExplicitVerticalForElevatorWalls(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	set, err := files.LoadWallSet()
	if err != nil {
		t.Fatalf("load wall set: %v", err)
	}

	for _, tile := range []int{sharewareElevatorWallTile, sharewareElevatorWallUsedTile} {
		hPage := (tile - 1) * 2
		vPage := hPage + 1
		if hPage >= len(set.AllPages) || vPage >= len(set.AllPages) {
			t.Fatalf("tile %d pages out of range", tile)
		}
		if !reflect.DeepEqual(set.Vertical[tile].Pixels, set.AllPages[vPage].Pixels) {
			t.Fatalf("tile %d vertical texture should use explicit page %d", tile, vPage)
		}
	}
}

func TestDimWallTextureScalesRGBAndPreservesAlpha(t *testing.T) {
	src := WallTexture{
		Page:   7,
		Width:  1,
		Height: 1,
		Pixels: []uint32{bits.ReverseBytes32(0xc8643280)},
	}

	dimmed := dimWallTexture(src, WallShadeScale)
	if dimmed.Page != 7 {
		t.Fatalf("page = %d, want 7", dimmed.Page)
	}
	want := dimPackedRGBA(0xc8643280, WallShadeScale)
	if got := dimmed.Pixel(0, 0); got != want {
		t.Fatalf("pixel = %#08x, want %#08x", got, want)
	}
}
