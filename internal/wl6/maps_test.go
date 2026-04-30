package wl6

import "testing"

func TestMapsFixture(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	maps, err := files.Maps()
	if err != nil {
		t.Fatalf("parse maps: %v", err)
	}
	if len(maps) == 0 {
		t.Fatal("expected at least one map")
	}

	if maps[0].Name == "" {
		t.Fatal("expected first map to have a name")
	}

	data, err := files.LoadMap(maps[0].Index)
	if err != nil {
		t.Fatalf("load first map: %v", err)
	}

	want := int(data.Header.Width) * int(data.Header.Height)
	if got := len(data.Planes[0]); got != want {
		t.Fatalf("plane 0 size = %d, want %d", got, want)
	}
	if got := len(data.Planes[1]); got != want {
		t.Fatalf("plane 1 size = %d, want %d", got, want)
	}
}
