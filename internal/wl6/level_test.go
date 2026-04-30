package wl6

import "testing"

func TestLevelSemantics(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	data, err := files.LoadMap(0)
	if err != nil {
		t.Fatalf("load map: %v", err)
	}

	level := data.Level()
	if level.Width != 64 || level.Height != 64 {
		t.Fatalf("level size = %dx%d, want 64x64", level.Width, level.Height)
	}
	if len(level.PlayerStarts) == 0 {
		t.Fatal("expected at least one player start")
	}

	var foundArea, foundDoor bool
	for y := 0; y < level.Height; y++ {
		for x := 0; x < level.Width; x++ {
			tile := level.Tile(x, y)
			if tile.Area >= 0 {
				foundArea = true
			}
			if tile.Door != nil {
				foundDoor = true
			}
		}
	}

	if !foundArea {
		t.Fatal("expected at least one area tile")
	}
	if !foundDoor {
		t.Fatal("expected at least one door tile")
	}
}
