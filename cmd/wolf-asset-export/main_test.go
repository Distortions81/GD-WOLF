package main

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestExportedWallPagesIncludeDistinctElevatorSwitches(t *testing.T) {
	for _, tc := range []struct {
		page int
		want bool
	}{
		{0, true}, {1, false}, {40, true}, {41, true}, {42, true}, {43, true},
		{98, true}, {99, false}, {100, true}, {101, false}, {102, true}, {103, false},
	} {
		if got := isExportedWallPage(tc.page); got != tc.want {
			t.Fatalf("page %d exported = %v, want %v", tc.page, got, tc.want)
		}
	}
}

func TestExportIndexedSprite(t *testing.T) {
	files, _, err := wl6.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := exportSprites(files, out, "files", 1); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(out, "sprites", "shape-050.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("guard bounds = %v", img.Bounds())
	}
	set, err := files.LoadSpriteSet()
	if err != nil {
		t.Fatal(err)
	}
	walls, err := files.LoadWallSet()
	if err != nil {
		t.Fatal(err)
	}
	covered := [64][64]bool{}
	for x, column := range set.Pages[50].Columns {
		for _, post := range column.Posts {
			for i, index := range post.Indexed {
				y := post.StartY + i
				covered[x][y] = true
				rgba := walls.Palette[index]
				want := color.NRGBA{R: byte(rgba >> 24), G: byte(rgba >> 16), B: byte(rgba >> 8), A: byte(rgba)}
				got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				if got != want {
					t.Fatalf("pixel (%d,%d) = %v, want palette color %v", x, y, got, want)
				}
			}
		}
	}
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			if _, _, _, alpha := img.At(x, y).RGBA(); !covered[x][y] && alpha != 0 {
				t.Fatalf("background pixel (%d,%d) is not transparent", x, y)
			}
		}
	}
}

func TestPictureIsPlaceholder(t *testing.T) {
	if !pictureIsPlaceholder(nil) {
		t.Fatal("nil picture should be treated as placeholder")
	}

	if !pictureIsPlaceholder(&wl6.Picture{Width: 2, Height: 2, Data: []byte{0, 0, 0, 0}}) {
		t.Fatal("all-zero picture should be treated as placeholder")
	}

	if pictureIsPlaceholder(&wl6.Picture{Width: 2, Height: 2, Data: []byte{0, 0, 1, 0}}) {
		t.Fatal("picture with non-zero pixels should not be treated as placeholder")
	}
}
