package wl6

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Files holds the minimal Wolfenstein 3D data files needed to read maps and graphics.
type Files struct {
	MapHead   []byte
	GameMaps  []byte
	VSwap     []byte
	VGADict   []byte
	VGAHead   []byte
	VGAGraph  []byte
	AudioHead []byte
	AudioT    []byte
	Variant   VariantSpec
}

var defaultDataDirs = []string{
	"data",
	filepath.Join("wolf3d", "WOLF3D"),
}

// OpenDefault loads data from the first known local data directory, falling back
// to embedded shareware data when nothing is found on disk.
func OpenDefault() (*Files, string, error) {
	cwd, err := os.Getwd()
	if err == nil {
		for _, dir := range candidateDataDirs(cwd) {
			files, err := Open(dir)
			if err == nil {
				return files, dir, nil
			}
		}
	}

	files, err := OpenEmbeddedShareware()
	if err == nil {
		return files, "embedded shareware", nil
	}

	return nil, "", fmt.Errorf("could not find Wolfenstein 3D data directory; tried %v and embedded shareware fallback failed: %w", defaultDataDirs, err)
}

// Open loads the required data files from a Wolfenstein 3D data directory.
func Open(dir string) (*Files, error) {
	fsys := os.DirFS(dir)
	spec, ok := detectVariant(fsys, ".")
	if !ok {
		return nil, fmt.Errorf("could not find supported Wolfenstein 3D data files in %s", dir)
	}
	files, err := openFromFS(fsys, ".", spec)
	if err != nil {
		return nil, fmt.Errorf("open %s data from %s: %w", spec.Name, dir, err)
	}
	return files, nil
}

func openFromFS(fsys fs.FS, root string, spec VariantSpec) (*Files, error) {
	mapHead, err := fs.ReadFile(fsys, pathFor(root, "MAPHEAD."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read MAPHEAD.%s: %w", spec.Ext, err)
	}
	gameMaps, err := fs.ReadFile(fsys, pathFor(root, "GAMEMAPS."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read GAMEMAPS.%s: %w", spec.Ext, err)
	}
	vswap, err := fs.ReadFile(fsys, pathFor(root, "VSWAP."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read VSWAP.%s: %w", spec.Ext, err)
	}
	vgaDict, err := fs.ReadFile(fsys, pathFor(root, "VGADICT."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read VGADICT.%s: %w", spec.Ext, err)
	}
	vgaHead, err := fs.ReadFile(fsys, pathFor(root, "VGAHEAD."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read VGAHEAD.%s: %w", spec.Ext, err)
	}
	vgaGraph, err := fs.ReadFile(fsys, pathFor(root, "VGAGRAPH."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read VGAGRAPH.%s: %w", spec.Ext, err)
	}
	audioHead, err := fs.ReadFile(fsys, pathFor(root, "AUDIOHED."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read AUDIOHED.%s: %w", spec.Ext, err)
	}
	audioT, err := fs.ReadFile(fsys, pathFor(root, "AUDIOT."+spec.Ext))
	if err != nil {
		return nil, fmt.Errorf("read AUDIOT.%s: %w", spec.Ext, err)
	}

	return &Files{
		MapHead:   mapHead,
		GameMaps:  gameMaps,
		VSwap:     vswap,
		VGADict:   vgaDict,
		VGAHead:   vgaHead,
		VGAGraph:  vgaGraph,
		AudioHead: audioHead,
		AudioT:    audioT,
		Variant:   spec,
	}, nil
}

func detectVariant(fsys fs.FS, root string) (VariantSpec, bool) {
	for _, spec := range supportedVariants {
		if hasDataFiles(fsys, root, spec.Ext) {
			return spec, true
		}
	}
	return VariantSpec{}, false
}

func hasDataFiles(fsys fs.FS, root, ext string) bool {
	required := [...]string{
		"MAPHEAD." + ext,
		"GAMEMAPS." + ext,
		"VSWAP." + ext,
		"VGADICT." + ext,
		"VGAHEAD." + ext,
		"VGAGRAPH." + ext,
		"AUDIOHED." + ext,
		"AUDIOT." + ext,
	}
	for _, name := range required {
		if _, err := fs.Stat(fsys, pathFor(root, name)); err != nil {
			return false
		}
	}
	return true
}

func pathFor(root, name string) string {
	if root == "." || root == "" {
		return name
	}
	return filepath.ToSlash(filepath.Join(root, name))
}

func candidateDataDirs(start string) []string {
	seen := map[string]bool{}
	var dirs []string

	current := start
	for {
		for _, rel := range defaultDataDirs {
			candidate := filepath.Clean(filepath.Join(current, rel))
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			dirs = append(dirs, candidate)
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return dirs
}
