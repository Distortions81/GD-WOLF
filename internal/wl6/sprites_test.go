package wl6

import "testing"

func TestLoadSpriteSet(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	set, err := files.LoadSpriteSet()
	if err != nil {
		t.Fatalf("load sprite set: %v", err)
	}
	if len(set.Pages) == 0 {
		t.Fatal("expected sprite pages")
	}

	first := set.Pages[0]
	if first.Width != 64 || first.Height != 64 {
		t.Fatalf("first sprite size = %dx%d, want 64x64", first.Width, first.Height)
	}
	if !first.IndexedFormat {
		t.Fatal("expected stock sprite to use indexed format")
	}
	if len(first.Pixels) != 0 {
		t.Fatalf("expected stock sprite RGBA plane to be empty, got %d pixels", len(first.Pixels))
	}

	opaque := 0
	for _, column := range first.Columns {
		for _, post := range column.Posts {
			opaque += len(post.Indexed)
		}
	}
	if opaque == 0 {
		t.Fatal("expected at least one opaque pixel in first sprite")
	}
}

func TestSpriteByShape(t *testing.T) {
	set := &SpriteSet{
		Pages: []Sprite{
			{Width: 1, Height: 1},
			{},
			{Width: 2, Height: 2, IndexedFormat: true, Columns: []SpriteColumn{
				{Posts: []SpritePost{{StartY: 0, EndY: 2, Indexed: []byte{1, 2}}}},
			}},
		},
	}
	if _, ok := set.SpriteByShape(-1); ok {
		t.Fatal("expected negative shape lookup to fail")
	}
	if _, ok := set.SpriteByShape(3); ok {
		t.Fatal("expected out of range shape lookup to fail")
	}
	if _, ok := set.SpriteByShape(1); ok {
		t.Fatal("expected empty shape slot lookup to fail")
	}
	sprite, ok := set.SpriteByShape(2)
	if !ok {
		t.Fatal("expected in range shape lookup to succeed")
	}
	if sprite.Width != 2 || sprite.Height != 2 {
		t.Fatalf("sprite = %+v, want 2x2", *sprite)
	}
}

func TestLoadSpriteSetPreservesSparseShapeIndexes(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}

	set, err := files.LoadSpriteSet()
	if err != nil {
		t.Fatalf("load sprite set: %v", err)
	}

	for _, shape := range []int{186, 296, 408, 416} {
		sprite, ok := set.SpriteByShape(shape)
		if !ok {
			t.Fatalf("expected shape %d to exist", shape)
		}
		if sprite.Width != 64 || sprite.Height != 64 {
			t.Fatalf("shape %d size = %dx%d, want 64x64", shape, sprite.Width, sprite.Height)
		}
	}

	for _, shape := range []int{187, 250, 307, 399} {
		if _, ok := set.SpriteByShape(shape); ok {
			t.Fatalf("expected sparse shape %d to be missing", shape)
		}
	}
}
