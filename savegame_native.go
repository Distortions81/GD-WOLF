//go:build !js

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func nativeSaveDir() (string, error) {
	return "saves", nil
}

func autosaveSlotPath(index int) string {
	return filepath.Join("saves", fmt.Sprintf("autosave-%d%s", index+1, saveFileExtension))
}

func (g *game) listSaveSlots() ([]saveSlotSummary, error) {
	dir, err := nativeSaveDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return autosavePlaceholderSummaries(), nil
		}
		return nil, err
	}
	autosaves := autosavePlaceholderSummaries()
	slots := make([]saveSlotSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != saveFileExtension {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		save, err := parseSaveGame(data)
		if err != nil {
			continue
		}
		timestamp := save.Timestamp
		if timestamp.IsZero() {
			if info, statErr := entry.Info(); statErr == nil {
				timestamp = info.ModTime()
			}
		}
		slots = append(slots, saveSlotSummary{
			Used:      true,
			Path:      path,
			Name:      save.Name,
			MapIndex:  save.MapIndex,
			Timestamp: timestamp,
			Thumbnail: save.Thumbnail,
		})
		for i := range autosaves {
			if path != autosaves[i].Path {
				continue
			}
			autosaves[i].Used = true
			autosaves[i].MapIndex = save.MapIndex
			autosaves[i].Timestamp = timestamp
			autosaves[i].Thumbnail = save.Thumbnail
			autosaves[i].Name = save.Name
			slots = slots[:len(slots)-1]
			break
		}
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Timestamp.Equal(slots[j].Timestamp) {
			return strings.ToLower(slots[i].Name) < strings.ToLower(slots[j].Name)
		}
		return slots[i].Timestamp.After(slots[j].Timestamp)
	})
	sort.SliceStable(autosaves, func(i, j int) bool {
		if autosaves[i].Used != autosaves[j].Used {
			return autosaves[i].Used
		}
		if autosaves[i].Timestamp.Equal(autosaves[j].Timestamp) {
			return autosaves[i].Path < autosaves[j].Path
		}
		return autosaves[i].Timestamp.After(autosaves[j].Timestamp)
	})
	for i := range autosaves {
		autosaves[i].AutosaveOrdinal = i + 1
	}
	return append(autosaves, slots...), nil
}

func uniqueSavePath(dir, name string) string {
	stamp := time.Now().Format("20060102-150405")
	base := fmt.Sprintf("%s-%s", stamp, sanitizeSaveFileComponent(name))
	path := filepath.Join(dir, base+saveFileExtension)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(dir, base+"-"+strconv.Itoa(i)+saveFileExtension)
	}
}

func (g *game) writeSaveSlot(existingPath, name string, data []byte) (string, error) {
	dir, err := nativeSaveDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := existingPath
	if path == "" {
		path = uniqueSavePath(dir, name)
	}
	return path, os.WriteFile(path, data, 0o644)
}

func (g *game) readSaveSlot(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (g *game) installBrowserSaveBridge() {
}

func (g *game) supportsBrowserSaveActions() bool {
	return false
}

func (g *game) triggerBrowserSaveImport() error {
	return errSaveUnsupported
}

func (g *game) triggerBrowserSaveExport(path string) error {
	_ = path
	return errSaveUnsupported
}

func (g *game) triggerBrowserSaveExportAll() error {
	return errSaveUnsupported
}
