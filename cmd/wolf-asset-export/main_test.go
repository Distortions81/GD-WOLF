package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

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
