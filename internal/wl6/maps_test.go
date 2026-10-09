package wl6

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"
)

func TestParseCompactMapHead(t *testing.T) {
	for _, count := range []int{1, 10, 60, 100} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			raw := make([]byte, 2+4*count)
			binary.LittleEndian.PutUint16(raw, 0xabcd)
			for i := 0; i < count; i++ {
				binary.LittleEndian.PutUint32(raw[2+4*i:], uint32(38*i))
			}
			tag, offsets, err := parseMapHead(raw)
			if err != nil || tag != 0xabcd {
				t.Fatalf("tag=%x, error=%v", tag, err)
			}
			for i, got := range offsets {
				want := uint32(0xffffffff)
				if i < count {
					want = uint32(38 * i)
				}
				if got != want {
					t.Fatalf("offset %d = %d, want %d", i, got, want)
				}
			}
		})
	}
	for _, size := range []int{0, 1, 2, 3, 4, 5, 7, 8, 9, 239, 240, 241, 399, 400, 401} {
		if _, _, err := parseMapHead(make([]byte, size)); err == nil {
			t.Errorf("accepted incomplete MAPHEAD of %d bytes", size)
		}
	}
}

func TestMapsCompactHeader(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	want, err := files.LoadMap(0)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve support for the full editor structure's trailing tileinfo.
	files.MapHead = append(append([]byte(nil), files.MapHead...), 1, 2, 3)
	if _, err := files.LoadMap(0); err != nil {
		t.Fatal(err)
	}
	files.MapHead = files.MapHead[:6]
	maps, err := files.Maps()
	if err != nil || len(maps) != 1 || maps[0].Index != 0 {
		t.Fatalf("compact map list=%v, error=%v", maps, err)
	}
	got, err := files.LoadMap(0)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("compact header changed map decoding: %v", err)
	}
	if _, err := files.LoadMap(1); err == nil {
		t.Fatal("missing compact-header map slot accepted")
	}
}

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
