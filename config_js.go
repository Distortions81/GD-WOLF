//go:build js

package main

import (
	"encoding/json"
	"fmt"
)

const browserResolutionConfigKey = "gd-wolf.resolution"

type browserResolutionConfig struct {
	RenderMode         string `json:"render_mode"`
	RenderModePrompted bool   `json:"render_mode_prompted"`
}

func (g *game) loadPersistentConfig() error {
	storage, available := browserLocalStorage()
	if !available {
		return nil
	}
	item, err := browserStorageCall(storage, "getItem", browserResolutionConfigKey)
	if err != nil {
		return err
	}
	if item.IsNull() || item.IsUndefined() {
		return nil
	}
	cfg := browserResolutionConfig{
		RenderMode:         g.renderMode.label(),
		RenderModePrompted: g.renderModePrompted,
	}
	if err := json.Unmarshal([]byte(item.String()), &cfg); err != nil {
		return fmt.Errorf("decode browser resolution settings: %w", err)
	}
	g.renderMode = parseRenderMode(cfg.RenderMode)
	g.renderModePrompted = cfg.RenderModePrompted
	return nil
}

func (g *game) savePersistentConfig() error {
	storage, available := browserLocalStorage()
	if !available {
		return nil
	}
	data, err := json.Marshal(browserResolutionConfig{
		RenderMode:         g.renderMode.label(),
		RenderModePrompted: g.renderModePrompted,
	})
	if err != nil {
		return err
	}
	_, err = browserStorageCall(storage, "setItem", browserResolutionConfigKey, string(data))
	return err
}
