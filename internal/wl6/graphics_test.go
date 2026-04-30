package wl6

import "testing"

func TestLoadPausedPicture(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	pic, err := files.LoadPicture(files.Variant.PausedPicChunk)
	if err != nil {
		t.Fatalf("load paused picture: %v", err)
	}
	if pic.Width <= 0 || pic.Height <= 0 {
		t.Fatalf("invalid paused picture size %dx%d", pic.Width, pic.Height)
	}
	if got, want := len(pic.Data), pic.Width*pic.Height; got != want {
		t.Fatalf("paused picture pixel count = %d, want %d", got, want)
	}
}
