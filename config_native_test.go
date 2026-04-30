//go:build !js

package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestPersistentConfigRoundTrip(t *testing.T) {
	cfg := persistentConfig{
		SFXVolume:          0.35,
		MusicVolume:        0.8,
		MouseSensitivity:   defaultMouseLook * 1.5,
		TurnSpeed:          defaultTurnSpeed * 0.75,
		MapMoveSpeed:       defaultMapMoveSpeed * 1.25,
		RenderMode:         renderModeHQ.label(),
		RenderModePrompted: true,
		HDTexturesEnabled:  false,
		VsyncEnabled:       false,
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

	if got.SFXVolume != cfg.SFXVolume || got.MusicVolume != cfg.MusicVolume || got.MouseSensitivity != cfg.MouseSensitivity || got.TurnSpeed != cfg.TurnSpeed || got.MapMoveSpeed != cfg.MapMoveSpeed || got.RenderMode != cfg.RenderMode || got.RenderModePrompted != cfg.RenderModePrompted || got.HDTexturesEnabled != cfg.HDTexturesEnabled || got.VsyncEnabled != cfg.VsyncEnabled {
		t.Fatalf(
			"config values = (%v, %v, %v, %v, %v, %v, %v, %v, %v), want (%v, %v, %v, %v, %v, %v, %v, %v, %v)",
			got.SFXVolume,
			got.MusicVolume,
			got.MouseSensitivity,
			got.TurnSpeed,
			got.MapMoveSpeed,
			got.RenderMode,
			got.RenderModePrompted,
			got.HDTexturesEnabled,
			got.VsyncEnabled,
			cfg.SFXVolume,
			cfg.MusicVolume,
			cfg.MouseSensitivity,
			cfg.TurnSpeed,
			cfg.MapMoveSpeed,
			cfg.RenderMode,
			cfg.RenderModePrompted,
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
}
