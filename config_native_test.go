//go:build !js

package main

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestPersistentConfigRoundTrip(t *testing.T) {
	cfg := persistentConfig{
		SFXVolume:         0.35,
		MusicVolume:       0.8,
		MouseSensitivity:  defaultMouseLook * 1.5,
		TurnSpeed:         defaultTurnSpeed * 0.75,
		MapMoveSpeed:      defaultMapMoveSpeed * 1.25,
		RenderMode:        renderModeHQ.label(),
		HDTexturesEnabled: false,
		VsyncEnabled:      false,
		ModernDoors:       false,
		Keybinds: map[string]persistentKeybind{
			"forward": {
				Primary:   ebiten.KeyW,
				Secondary: ebiten.KeyUp,
			},
			"pause_back": {
				Primary:   ebiten.KeyEscape,
				Secondary: keyUnbound,
			},
		},
	}

	data, err := marshalPersistentConfig(cfg)
	if err != nil {
		t.Fatalf("marshalPersistentConfig: %v", err)
	}
	got, err := parsePersistentConfig(data)
	if err != nil {
		t.Fatalf("parsePersistentConfig: %v", err)
	}

	if got.SFXVolume != cfg.SFXVolume || got.MusicVolume != cfg.MusicVolume || got.MouseSensitivity != cfg.MouseSensitivity || got.TurnSpeed != cfg.TurnSpeed || got.MapMoveSpeed != cfg.MapMoveSpeed || got.RenderMode != cfg.RenderMode || got.HDTexturesEnabled != cfg.HDTexturesEnabled || got.VsyncEnabled != cfg.VsyncEnabled {
		t.Fatalf(
			"config values = (%v, %v, %v, %v, %v, %v, %v, %v), want (%v, %v, %v, %v, %v, %v, %v, %v)",
			got.SFXVolume,
			got.MusicVolume,
			got.MouseSensitivity,
			got.TurnSpeed,
			got.MapMoveSpeed,
			got.RenderMode,
			got.HDTexturesEnabled,
			got.VsyncEnabled,
			cfg.SFXVolume,
			cfg.MusicVolume,
			cfg.MouseSensitivity,
			cfg.TurnSpeed,
			cfg.MapMoveSpeed,
			cfg.RenderMode,
			cfg.HDTexturesEnabled,
			cfg.VsyncEnabled,
		)
	}
	for id, binding := range cfg.Keybinds {
		parsed, ok := got.Keybinds[id]
		if !ok {
			t.Fatalf("missing keybind %q", id)
		}
		if parsed != binding {
			t.Fatalf("keybind %q = %+v, want %+v", id, parsed, binding)
		}
	}
	if got.ModernDoors != cfg.ModernDoors {
		t.Fatalf("modern doors = %t, want %t", got.ModernDoors, cfg.ModernDoors)
	}
}

func TestPersistentConfigDoesNotSuppressLaunchPrompt(t *testing.T) {
	cfg, err := parsePersistentConfig([]byte("[input]\nrender_mode = \"DOS\"\nrender_mode_prompted = true\n"))
	if err != nil {
		t.Fatalf("parse config with old prompt setting: %v", err)
	}
	g := &game{}
	g.applyPersistentConfig(cfg)
	if g.renderMode != renderModeDOS {
		t.Fatalf("render mode = %v, want DOS", g.renderMode)
	}
	if got := g.nextTitleInputState(); got != uiStateRenderModePrompt {
		t.Fatalf("title input state = %v, want launch prompt", got)
	}
	g.renderModePrompted = true
	data, err := marshalPersistentConfig(g.currentPersistentConfig())
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if strings.Contains(string(data), "render_mode_prompted") {
		t.Fatal("session-only launch prompt flag was written to config")
	}
}

func TestPersistentConfigModernDoorDefaultAndApplication(t *testing.T) {
	cfg, err := parsePersistentConfig([]byte("[audio]\nsfx_volume = 0.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ModernDoors {
		t.Fatal("config without a door setting should retain modern doors")
	}
	g := &game{}
	g.applyPersistentConfig(cfg)
	if !g.modernDoors || !g.currentPersistentConfig().ModernDoors {
		t.Fatal("modern door setting was not applied")
	}
	if _, err := parsePersistentConfig([]byte("[gameplay]\nmodern_doors = broken\n")); err == nil {
		t.Fatal("invalid door boolean accepted")
	}
}
