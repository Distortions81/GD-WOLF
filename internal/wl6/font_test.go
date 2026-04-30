package wl6

import "testing"

func TestLoadFontShareware(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}
	font, err := files.LoadFont(0)
	if err != nil {
		t.Fatalf("load font 0: %v", err)
	}
	if font.Height <= 0 {
		t.Fatalf("font height=%d want positive", font.Height)
	}
	if font.Width['A'] == 0 {
		t.Fatal("expected non-zero width for 'A'")
	}
}
