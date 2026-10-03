//go:build ignore

// Split each continuous A/B artwork into the two square runtime wall tiles.
// Run from the repository root: go run ./art/walls-hd/v1/split_wall_pairs.go
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"gd-wolf/internal/assetimage"
)

func main() {
	const root = "art/walls-hd/v1"
	type pairRegistration struct {
		Source     string             `json:"source"`
		SourceSize [2]int             `json:"sourceSize"`
		Tiles      [2]int             `json:"tiles"`
		Canvas     int                `json:"canvas"`
		Regions    [2]image.Rectangle `json:"sourceRegions"`
	}
	var registrations []pairRegistration
	for _, pair := range []struct {
		name  string
		tiles [2]int
	}{
		{"gray", [2]int{1, 2}}, {"blue", [2]int{8, 9}},
	} {
		name := "pairs/" + pair.name + "-ab.png"
		f, err := os.Open(filepath.Join(root, name))
		must(err)
		src, err := png.Decode(f)
		f.Close()
		must(err)
		bounds := src.Bounds()
		if bounds.Dx() != bounds.Dy()*2 {
			panic(fmt.Sprintf("%s must contain two square tiles in a 2:1 image, got %v", name, bounds))
		}
		size := max(1024, bounds.Dy())
		reg := pairRegistration{Source: name, SourceSize: [2]int{bounds.Dx(), bounds.Dy()}, Tiles: pair.tiles, Canvas: size}
		strip := image.NewNRGBA(image.Rect(0, 0, size*4, size))
		for half, tile := range pair.tiles {
			region := image.Rect(bounds.Min.X+half*bounds.Dy(), bounds.Min.Y, bounds.Min.X+(half+1)*bounds.Dy(), bounds.Max.Y)
			reg.Regions[half] = region
			out := image.NewNRGBA(image.Rect(0, 0, size, size))
			assetimage.ResampleCutout(out, out.Bounds(), src, region)
			for repeat := range 2 {
				x := (half + repeat*2) * size
				draw.Draw(strip, image.Rect(x, 0, x+size, size), out, image.Point{}, draw.Src)
			}
			path := filepath.Join(root, "walls/horizontal", fmt.Sprintf("tile-%02d.png", tile))
			f, err := os.Create(path)
			must(err)
			must(png.Encode(f, out))
			must(f.Close())
		}
		f, err = os.Create(filepath.Join(root, "pairs", pair.name+"-abab-preview.png"))
		must(err)
		must(png.Encode(f, strip))
		must(f.Close())
		registrations = append(registrations, reg)
		fmt.Printf("Registered %s A/B pair as tiles %02d/%02d (%dpx).\n", pair.name, pair.tiles[0], pair.tiles[1], size)
	}
	data, err := json.MarshalIndent(registrations, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(root, "pairs/registrations.json"), append(data, '\n'), 0644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
