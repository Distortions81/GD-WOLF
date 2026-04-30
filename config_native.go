//go:build !js

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type persistentConfig struct {
	SFXVolume          float64
	MusicVolume        float64
	MouseSensitivity   float64
	TurnSpeed          float64
	MapMoveSpeed       float64
	RenderMode         string
	RenderModePrompted bool
	HDTexturesEnabled  bool
	VsyncEnabled       bool
	Keybinds           map[string]persistentKeybind
}

type persistentKeybind struct {
	Primary   ebiten.Key
	Secondary ebiten.Key
}

func nativeConfigPath() (string, error) {
	return "config.toml", nil
}

func encodeConfigKey(key ebiten.Key) string {
	if key == keyUnbound {
		return "Unbound"
	}
	text, err := key.MarshalText()
	if err != nil {
		return "Unbound"
	}
	return string(text)
}

func decodeConfigKey(value string) (ebiten.Key, error) {
	if value == "Unbound" {
		return keyUnbound, nil
	}
	var key ebiten.Key
	if err := key.UnmarshalText([]byte(value)); err != nil {
		return keyUnbound, err
	}
	return normalizeBindingKey(key), nil
}

func (g *game) currentPersistentConfig() persistentConfig {
	cfg := persistentConfig{
		SFXVolume:          g.sfxVolume,
		MusicVolume:        g.musicVolume,
		MouseSensitivity:   g.mouseLook,
		TurnSpeed:          g.turnSpeed,
		MapMoveSpeed:       g.mapMoveSpeed,
		RenderMode:         g.renderMode.label(),
		RenderModePrompted: g.renderModePrompted,
		HDTexturesEnabled:  g.hdTexturesEnabled,
		VsyncEnabled:       g.vsyncEnabled,
		Keybinds:           make(map[string]persistentKeybind, len(g.keybinds)),
	}
	for _, binding := range g.keybinds {
		if binding.id == "" {
			continue
		}
		cfg.Keybinds[binding.id] = persistentKeybind{
			Primary:   binding.primary,
			Secondary: binding.secondary,
		}
	}
	return cfg
}

func (g *game) applyPersistentConfig(cfg persistentConfig) {
	g.sfxVolume = clampUnit(cfg.SFXVolume)
	g.musicVolume = clampUnit(cfg.MusicVolume)
	g.mouseLook = clampMouseLook(cfg.MouseSensitivity)
	g.turnSpeed = clampTurnSpeed(cfg.TurnSpeed)
	g.mapMoveSpeed = clampMapMoveSpeed(cfg.MapMoveSpeed)
	g.renderMode = parseRenderMode(cfg.RenderMode)
	g.renderModePrompted = cfg.RenderModePrompted
	g.hdTexturesEnabled = cfg.HDTexturesEnabled
	g.vsyncEnabled = cfg.VsyncEnabled
	ebiten.SetVsyncEnabled(g.vsyncEnabled)
	for i := range g.keybinds {
		if binding, ok := cfg.Keybinds[g.keybinds[i].id]; ok {
			g.keybinds[i].primary = binding.Primary
			g.keybinds[i].secondary = binding.Secondary
		}
	}
}

func marshalPersistentConfig(cfg persistentConfig) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[audio]\n")
	fmt.Fprintf(&buf, "sfx_volume = %.4f\n", cfg.SFXVolume)
	fmt.Fprintf(&buf, "music_volume = %.4f\n\n", cfg.MusicVolume)
	fmt.Fprintf(&buf, "[input]\n")
	fmt.Fprintf(&buf, "mouse_sensitivity = %.6f\n\n", cfg.MouseSensitivity)
	fmt.Fprintf(&buf, "turn_speed = %.6f\n\n", cfg.TurnSpeed)
	fmt.Fprintf(&buf, "map_move_speed = %.6f\n", cfg.MapMoveSpeed)
	fmt.Fprintf(&buf, "render_mode = %q\n", cfg.RenderMode)
	fmt.Fprintf(&buf, "render_mode_prompted = %t\n", cfg.RenderModePrompted)
	fmt.Fprintf(&buf, "hd_textures = %t\n", cfg.HDTexturesEnabled)
	fmt.Fprintf(&buf, "vsync = %t\n\n", cfg.VsyncEnabled)
	fmt.Fprintf(&buf, "[keybinds]\n\n")

	ids := make([]string, 0, len(cfg.Keybinds))
	for id := range cfg.Keybinds {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		binding := cfg.Keybinds[id]
		fmt.Fprintf(&buf, "[keybinds.%s]\n", id)
		fmt.Fprintf(&buf, "primary = %q\n", encodeConfigKey(binding.Primary))
		fmt.Fprintf(&buf, "secondary = %q\n\n", encodeConfigKey(binding.Secondary))
	}
	return buf.Bytes(), nil
}

func parsePersistentConfig(data []byte) (persistentConfig, error) {
	cfg := persistentConfig{
		SFXVolume:          0.5,
		MusicVolume:        1.0,
		MouseSensitivity:   defaultMouseLook,
		TurnSpeed:          defaultTurnSpeed,
		MapMoveSpeed:       defaultMapMoveSpeed,
		RenderMode:         renderModeUltra.label(),
		RenderModePrompted: false,
		HDTexturesEnabled:  true,
		VsyncEnabled:       true,
		Keybinds:           map[string]persistentKeybind{},
	}

	section := ""
	currentKeybind := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			currentKeybind = ""
			if strings.HasPrefix(section, "keybinds.") {
				currentKeybind = strings.TrimPrefix(section, "keybinds.")
				if _, ok := cfg.Keybinds[currentKeybind]; !ok {
					cfg.Keybinds[currentKeybind] = persistentKeybind{Primary: keyUnbound, Secondary: keyUnbound}
				}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch section {
		case "audio":
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return cfg, err
			}
			switch key {
			case "sfx_volume":
				cfg.SFXVolume = f
			case "music_volume":
				cfg.MusicVolume = f
			}
		case "input":
			if key == "render_mode" {
				text, err := strconv.Unquote(value)
				if err != nil {
					return cfg, err
				}
				cfg.RenderMode = parseRenderMode(text).label()
				continue
			}
			if key == "vsync" {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return cfg, err
				}
				cfg.VsyncEnabled = b
				continue
			}
			if key == "hd_textures" {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return cfg, err
				}
				cfg.HDTexturesEnabled = b
				continue
			}
			if key == "render_mode_prompted" {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return cfg, err
				}
				cfg.RenderModePrompted = b
				continue
			}
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return cfg, err
			}
			if key == "mouse_sensitivity" {
				cfg.MouseSensitivity = f
			}
			if key == "turn_speed" {
				cfg.TurnSpeed = f
			}
			if key == "map_move_speed" {
				cfg.MapMoveSpeed = f
				continue
			}
		default:
			if currentKeybind == "" {
				continue
			}
			text, err := strconv.Unquote(value)
			if err != nil {
				return cfg, err
			}
			binding := cfg.Keybinds[currentKeybind]
			parsed, err := decodeConfigKey(text)
			if err != nil {
				return cfg, err
			}
			switch key {
			case "primary":
				binding.Primary = parsed
			case "secondary":
				binding.Secondary = parsed
			}
			cfg.Keybinds[currentKeybind] = binding
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (g *game) loadPersistentConfig() error {
	path, err := nativeConfigPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cfg, err := parsePersistentConfig(data)
	if err != nil {
		return err
	}
	g.applyPersistentConfig(cfg)
	return nil
}

func (g *game) savePersistentConfig() error {
	path, err := nativeConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalPersistentConfig(g.currentPersistentConfig())
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
