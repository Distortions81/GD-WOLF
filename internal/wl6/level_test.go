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

func TestDoorUsesOriginalMapAreaSide(t *testing.T) {
	m := &MapData{Header: MapHeader{Width: 5, Height: 5}}
	m.Planes[0] = make([]uint16, 25)
	m.Planes[1] = make([]uint16, 25)
	for i := range m.Planes[0] {
		m.Planes[0][i] = areaTile
	}
	m.Planes[0][2*5+1] = areaTile + 3 // left of vertical door
	m.Planes[0][2*5+2] = 90           // vertical door
	m.Planes[0][3*5+3] = areaTile + 5 // above horizontal door
	m.Planes[0][4*5+3] = 91           // horizontal door
	level := m.Level()
	if got := level.Tile(2, 2).Area; got != 3 {
		t.Fatalf("vertical door area = %d, want left area 3", got)
	}
	if got := level.Tile(3, 4).Area; got != 5 {
		t.Fatalf("horizontal door area = %d, want above area 5", got)
	}
}
