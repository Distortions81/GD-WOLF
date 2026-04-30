//go:build js

package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"
	"syscall/js"
	"time"
)

const browserSaveSlotPrefix = "gd-wolf.savegame."

func autosaveSlotPath(index int) string {
	return fmt.Sprintf("%sautosave-%d%s", browserSaveSlotPrefix, index+1, saveFileExtension)
}

func browserLocalStorage() (js.Value, bool) {
	global := js.Global()
	if global.IsUndefined() || global.IsNull() {
		return js.Undefined(), false
	}
	storage := global.Get("localStorage")
	if storage.IsUndefined() || storage.IsNull() {
		return js.Undefined(), false
	}
	return storage, true
}

func browserStorageCall(storage js.Value, method string, args ...any) (result js.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("browser storage %s failed: %v", method, recovered)
		}
	}()
	result = storage.Call(method, args...)
	return result, nil
}

func browserSaveSlotKey(name string) string {
	stamp := time.Now().Format("20060102-150405")
	return browserSaveSlotPrefix + stamp + "-" + sanitizeSaveFileComponent(name) + saveFileExtension
}

func browserUniqueSaveSlotKey(storage js.Value, name string) (string, error) {
	base := browserSaveSlotKey(name)
	path := base
	for i := 1; ; i++ {
		item, err := browserStorageCall(storage, "getItem", path)
		if err != nil {
			return "", err
		}
		if item.IsNull() || item.IsUndefined() {
			return path, nil
		}
		path = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(base, saveFileExtension), i, saveFileExtension)
	}
}

func (g *game) listSaveSlots() ([]saveSlotSummary, error) {
	storage, ok := browserLocalStorage()
	if !ok {
		return nil, nil
	}

	length := storage.Get("length").Int()
	autosaves := autosavePlaceholderSummaries()
	slots := make([]saveSlotSummary, 0, length)
	for i := 0; i < length; i++ {
		key, keyErr := browserStorageCall(storage, "key", i)
		if keyErr != nil {
			return nil, keyErr
		}
		path := key.String()
		if !strings.HasPrefix(path, browserSaveSlotPrefix) || !strings.HasSuffix(path, saveFileExtension) {
			continue
		}
		item, itemErr := browserStorageCall(storage, "getItem", path)
		if itemErr != nil {
			return nil, itemErr
		}
		if item.IsNull() || item.IsUndefined() {
			continue
		}
		decoded, decodeErr := base64.StdEncoding.DecodeString(item.String())
		if decodeErr != nil {
			continue
		}
		save, parseErr := parseSaveGame(decoded)
		if parseErr != nil {
			continue
		}
		slots = append(slots, saveSlotSummary{
			Used:      true,
			Path:      path,
			Name:      save.Name,
			MapIndex:  save.MapIndex,
			Timestamp: save.Timestamp,
			Thumbnail: save.Thumbnail,
		})
		for i := range autosaves {
			if path != autosaves[i].Path {
				continue
			}
			autosaves[i].Used = true
			autosaves[i].MapIndex = save.MapIndex
			autosaves[i].Timestamp = save.Timestamp
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

func (g *game) writeSaveSlot(existingPath, name string, data []byte) (string, error) {
	storage, ok := browserLocalStorage()
	if !ok {
		return "", errSaveUnsupported
	}
	path := existingPath
	if path == "" {
		var err error
		path, err = browserUniqueSaveSlotKey(storage, name)
		if err != nil {
			return "", err
		}
	}
	if _, err := browserStorageCall(storage, "setItem", path, base64.StdEncoding.EncodeToString(data)); err != nil {
		return "", err
	}
	return path, nil
}

func (g *game) readSaveSlot(path string) ([]byte, error) {
	storage, ok := browserLocalStorage()
	if !ok {
		return nil, errSaveUnsupported
	}
	item, err := browserStorageCall(storage, "getItem", path)
	if err != nil {
		return nil, err
	}
	if item.IsNull() || item.IsUndefined() {
		return nil, fmt.Errorf("save slot %q not found", path)
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(item.String())
	if decodeErr != nil {
		return nil, fmt.Errorf("save slot %q is not valid base64", path)
	}
	return decoded, nil
}

var (
	browserSaveImportCallback          js.Func
	browserSaveImportCallbackInstalled bool
)

func (g *game) installBrowserSaveBridge() {
	if g == nil {
		return
	}
	if browserSaveImportCallbackInstalled {
		return
	}
	browserSaveImportCallback = js.FuncOf(func(this js.Value, args []js.Value) any {
		var importedPath string
		if len(args) > 0 {
			importedPath = args[0].String()
		}
		if err := g.reloadSaveSlots(); err != nil {
			log.Printf("save import refresh failed: %v", err)
			g.setNotice("Import refresh failed")
			return nil
		}
		if importedPath != "" && g.saveSlotIndexByPath(importedPath) < 0 {
			if storage, ok := browserLocalStorage(); ok {
				if _, err := browserStorageCall(storage, "removeItem", importedPath); err != nil {
					log.Printf("save import cleanup failed: %v", err)
				}
			}
			g.setNotice("Imported file is not a compatible save")
			return nil
		}
		if importedPath != "" {
			g.selectLoadSlotByPath(importedPath)
		}
		g.setNotice("Save imported")
		return nil
	})
	js.Global().Set("gdwolfOnSaveImport", browserSaveImportCallback)
	browserSaveImportCallbackInstalled = true
}

func (g *game) supportsBrowserSaveActions() bool {
	global := js.Global()
	if global.IsUndefined() || global.IsNull() {
		return false
	}
	return global.Get("gdwolfImportSave").Type() == js.TypeFunction &&
		global.Get("gdwolfExportSave").Type() == js.TypeFunction &&
		global.Get("gdwolfExportAllSaves").Type() == js.TypeFunction
}

func (g *game) triggerBrowserSaveImport() error {
	if !g.supportsBrowserSaveActions() {
		return errSaveUnsupported
	}
	js.Global().Call("gdwolfImportSave")
	return nil
}

func (g *game) triggerBrowserSaveExport(path string) error {
	if path == "" {
		return fmt.Errorf("save slot path is empty")
	}
	if !g.supportsBrowserSaveActions() {
		return errSaveUnsupported
	}
	js.Global().Call("gdwolfExportSave", path)
	return nil
}

func (g *game) triggerBrowserSaveExportAll() error {
	if !g.supportsBrowserSaveActions() {
		return errSaveUnsupported
	}
	js.Global().Call("gdwolfExportAllSaves")
	return nil
}
