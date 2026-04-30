package wl6

import "testing"

func TestLoadMusicChunkShareware(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}

	chunk, err := files.LoadMusicChunk(files.Variant.MenuSong)
	if err != nil {
		t.Fatalf("load menu song: %v", err)
	}
	if len(chunk.Events) == 0 {
		t.Fatal("expected music chunk events")
	}
	first := chunk.Events[0]
	if first.Reg > 0xff {
		t.Fatalf("first reg=%#x out of OPL range", first.Reg)
	}
	if chunk.Name == "" {
		t.Fatal("expected parsed music chunk name")
	}
}

func TestLoadMusicChunkIntroShareware(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}

	chunk, err := files.LoadMusicChunk(files.Variant.IntroSong)
	if err != nil {
		t.Fatalf("load intro song: %v", err)
	}
	if len(chunk.Events) < 16 {
		t.Fatalf("intro song event count=%d want at least 16", len(chunk.Events))
	}
}
