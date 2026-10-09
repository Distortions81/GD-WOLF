package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

// External recordings contain inputs, not map data. The runtime harness must
// still load the recording's map from the explicitly selected data directory.
func loadWolfDemoRuntimeInput(files *wl6.Files, demoIndex int, demoFile string) (*wl6.Demo, error) {
	if demoFile == "" {
		return files.LoadDemo(demoIndex)
	}
	raw, err := os.ReadFile(demoFile)
	if err != nil {
		return nil, fmt.Errorf("read external demo: %w", err)
	}
	demo, err := wl6.ParseDemo(raw)
	if err != nil {
		return nil, fmt.Errorf("parse external demo %s: %w", demoFile, err)
	}
	if _, err := files.LoadMap(demo.Map); err != nil {
		return nil, fmt.Errorf("external demo map %d in %s: %w", demo.Map, files.Variant.Name, err)
	}
	return demo, nil
}

func TestWolfDemoRuntimeExternalInput(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("signed inputs and nonzero unused header", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "third-party.wl1")
		if err := os.WriteFile(path, []byte{0, 7, 0, 0xd8, 3, 255, 128}, 0600); err != nil {
			t.Fatal(err)
		}
		demo, err := loadWolfDemoRuntimeInput(files, 3, path)
		if err != nil {
			t.Fatal(err)
		}
		if demo.Map != 0 || len(demo.Commands) != 1 || demo.Commands[0] != (wl6.DemoCommand{Buttons: 3, ControlX: -1, ControlY: -128}) {
			t.Fatalf("unexpected external input: %+v", demo)
		}
	})
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"truncated", []byte{0, 10, 0, 0, 0, 0, 0}},
		{"unavailable map", []byte{59, 7, 0, 0, 0, 0, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "demo.wl1")
			if err := os.WriteFile(path, test.raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadWolfDemoRuntimeInput(files, 0, path); err == nil {
				t.Fatal("invalid external recording accepted")
			}
		})
	}
}
