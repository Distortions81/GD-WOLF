package wl6

import "testing"

func TestEmbeddedSharewareDemos(t *testing.T) {
	f, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	for i, count := range []int{1152, 1284, 671, 633} {
		d, err := f.LoadDemo(i)
		if err != nil {
			t.Fatal(err)
		}
		if d.Map != i*2 || len(d.Commands) != count {
			t.Fatalf("demo %d: map=%d commands=%d", i, d.Map, len(d.Commands))
		}
	}
	d, _ := f.LoadDemo(0)
	if d.Commands[1].ControlX != 1 || d.Commands[1].ControlY != -63 {
		t.Fatalf("signed demo controls decoded incorrectly: %+v", d.Commands[1])
	}
}

func TestParseDemoRejectsMalformedInput(t *testing.T) {
	for _, data := range [][]byte{nil, {0, 7, 0, 0}, {0, 8, 0, 0, 0, 0, 0}, {0, 8, 0, 0, 0, 0, 0, 0}, {60, 7, 0, 0, 0, 0, 0}} {
		if _, err := ParseDemo(data); err == nil {
			t.Fatalf("accepted malformed demo %v", data)
		}
	}
	if _, err := ParseDemo([]byte{0, 7, 0, 19, 0, 255, 128}); err != nil {
		t.Fatal(err)
	}
}
