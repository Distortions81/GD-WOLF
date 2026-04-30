package wl6

import "testing"

func TestOpenEmbeddedShareware(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}
	if got, want := files.Variant.Ext, "WL1"; got != want {
		t.Fatalf("variant ext = %q, want %q", got, want)
	}

	maps, err := files.Maps()
	if err != nil {
		t.Fatalf("list shareware maps: %v", err)
	}
	if got, want := len(maps), 10; got != want {
		t.Fatalf("shareware map count = %d, want %d", got, want)
	}

	data, err := files.LoadMap(0)
	if err != nil {
		t.Fatalf("load shareware map 0: %v", err)
	}
	if data.Width() <= 0 || data.Height() <= 0 {
		t.Fatalf("invalid shareware map size %dx%d", data.Width(), data.Height())
	}

	pic, err := files.LoadPicture(files.Variant.PausedPicChunk)
	if err != nil {
		t.Fatalf("load shareware paused picture: %v", err)
	}
	if pic.Width <= 0 || pic.Height <= 0 {
		t.Fatalf("invalid shareware paused picture size %dx%d", pic.Width, pic.Height)
	}
}
