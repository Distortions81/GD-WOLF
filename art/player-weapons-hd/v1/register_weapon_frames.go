//go:build ignore

// Run from the repo root: go run ./art/player-weapons-hd/v1/register_weapon_frames.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"gd-wolf/internal/assetimage"
)

type frameMetadata struct {
	Shape         int             `json:"shape"`
	Weapon        string          `json:"weapon"`
	Phase         string          `json:"phase"`
	Atlas         string          `json:"atlas"`
	SourceRegion  image.Rectangle `json:"sourceRegion"`
	SourceContent image.Rectangle `json:"sourceContent"`
	RenderBounds  image.Rectangle `json:"renderBounds"`
}

func main() {
	root := flag.String("root", "art/player-weapons-hd/v1", "directory containing weapon atlases and original references")
	flag.Parse()
	const canvas = 512
	names := []string{"knife", "pistol", "machinegun", "chaingun"}
	phases := []string{"ready", "attack-1", "attack-2", "attack-3", "attack-4"}
	var frames []frameMetadata
	for weapon, name := range names {
		atlas := readPNG(filepath.Join(*root, name+".png"))
		cuts := cellBoundaries(atlas)
		for phase := 0; phase < 5; phase++ {
			shape := 416 + weapon*5 + phase
			filename := fmt.Sprintf("shape-%03d.png", shape)
			region := image.Rect(cuts[phase], 0, cuts[phase+1], atlas.Bounds().Dy())
			content := occupiedBounds(atlas, region).Inset(-2).Intersect(region)
			original := readPNG(filepath.Join(*root, "references", "sprites", filename))
			originalBounds := occupiedBounds(original, original.Bounds())
			target := image.Rect(originalBounds.Min.X*8, originalBounds.Min.Y*8, originalBounds.Max.X*8, originalBounds.Max.Y*8)
			output := image.NewNRGBA(image.Rect(0, 0, canvas, canvas))
			// Keep smooth alpha edges while restoring the original screen placement.
			assetimage.ResampleCutout(output, target, atlas, content)
			validateFrame(output)
			writePNG(filepath.Join(*root, "sprites", filename), output)
			frames = append(frames, frameMetadata{shape, name, phases[phase], name + ".png", region, content, target})
		}
	}
	metadata := struct {
		Canvas int             `json:"canvas"`
		Frames []frameMetadata `json:"frames"`
	}{canvas, frames}
	data, err := json.MarshalIndent(metadata, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(*root, "frames.json"), append(data, '\n'), 0o644))
	fmt.Printf("Registered and validated %d transparent %dx%d weapon frames.\n", len(frames), canvas, canvas)
}

// Find each split in the alpha gap nearest the nominal fifth-of-width boundary.
func cellBoundaries(img image.Image) []int {
	width := img.Bounds().Dx()
	cuts := []int{0}
	for boundary := 1; boundary < 5; boundary++ {
		nominal := boundary * width / 5
		best, bestCount := nominal, opaqueColumnCount(img, nominal)
		for offset := 1; offset <= width/25; offset++ {
			for _, x := range []int{nominal - offset, nominal + offset} {
				count := opaqueColumnCount(img, x)
				if count < bestCount {
					best, bestCount = x, count
				}
			}
		}
		if bestCount != 0 {
			panic(fmt.Sprintf("overlapping figures near column %d; atlas needs review", nominal))
		}
		cuts = append(cuts, best)
	}
	return append(cuts, width)
}

func opaqueColumnCount(img image.Image, x int) int {
	count := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		_, _, _, alpha := img.At(x, y).RGBA()
		if alpha >= 32768 {
			count++
		}
	}
	return count
}

func occupiedBounds(img image.Image, region image.Rectangle) image.Rectangle {
	result := image.Rectangle{}
	count := 0
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha >= 32768 {
				pixel := image.Rect(x, y, x+1, y+1)
				if count == 0 {
					result = pixel
				} else {
					result = result.Union(pixel)
				}
				count++
			}
		}
	}
	if count == 0 {
		panic(fmt.Sprintf("empty sprite region %v", region))
	}
	return result
}

func validateFrame(img *image.NRGBA) {
	opaque, clear := 0, 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			alpha := img.NRGBAAt(x, y).A
			if alpha == 0 {
				clear++
			}
			if alpha >= 128 {
				opaque++
			}
		}
	}
	if opaque < 100 || clear < 100 {
		panic("frame lacks visible content or transparent background")
	}
}

func readPNG(path string) image.Image {
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	img, err := png.Decode(f)
	must(err)
	return img
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	must(png.Encode(f, img))
	must(f.Close())
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
