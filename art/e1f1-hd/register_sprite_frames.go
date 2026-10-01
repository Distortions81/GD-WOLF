//go:build ignore

// Run from the repository root: go run ./art/e1f1-hd/register_sprite_frames.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

type atlas struct {
	Name          string
	Columns, Rows int
	Shapes        []int
}

type registration struct {
	Shape         int               `json:"shape"`
	Atlas         string            `json:"atlas"`
	SourceRegion  image.Rectangle   `json:"sourceRegion"`
	SourceContent []image.Rectangle `json:"sourceContent"`
	RenderBounds  []image.Rectangle `json:"renderBounds"`
}

func main() {
	root := flag.String("root", "art/e1f1-hd", "directory containing atlases and original sprite references")
	flag.Parse()
	var movement []int
	for shape := 99; shape <= 130; shape++ {
		movement = append(movement, shape)
	}
	atlases := []atlas{
		{"dog-movement", 8, 4, movement},
		{"dog-actions", 4, 2, []int{131, 132, 133, 134, 135, 136, 137, -1}},
		{"scenery-1", 3, 2, []int{2, 3, 4, 5, 6, 8}},
		{"scenery-2", 3, 2, []int{10, 11, 13, 14, 15, -1}},
		{"scenery-3", 3, 2, []int{18, 21, 25, 26, 27, 28}},
		{"scenery-4", 3, 2, []int{29, 31, 32, 33, 35, 37}},
		{"scenery-5", 3, 1, []int{38, 39, 41}},
		{"player-marker", 1, 1, []int{408}},
	}
	var registrations []registration
	for _, spec := range atlases {
		im := readPNG(filepath.Join(*root, "atlases", spec.Name+".png"))
		ycuts := boundaries(im, im.Bounds(), spec.Rows, false)
		for row := 0; row < spec.Rows; row++ {
			rowRegion := image.Rect(0, ycuts[row], im.Bounds().Dx(), ycuts[row+1])
			xcuts := boundaries(im, rowRegion, spec.Columns, true)
			for column := 0; column < spec.Columns; column++ {
				shape := spec.Shapes[row*spec.Columns+column]
				if shape < 0 {
					continue
				}
				name := fmt.Sprintf("shape-%03d.png", shape)
				region := image.Rect(xcuts[column], ycuts[row], xcuts[column+1], ycuts[row+1])
				original := readPNG(filepath.Join(*root, "references", "sprites", name))
				content := []image.Rectangle{occupiedBounds(im, region)}
				targets := []image.Rectangle{occupiedBounds(original, original.Bounds())}
				// Keep a hanging chandelier and its detached floor shadow aligned
				// independently; registering their combined bounds stretches the lamp.
				if shape == 6 {
					content = verticalParts(im, region)
					targets = verticalParts(original, original.Bounds())
					if len(content) != 2 || len(targets) != 2 {
						panic(fmt.Sprintf("chandelier parts: generated %v, original %v, cell %v", content, targets, region))
					}
				}
				if shape == 35 {
					// The new medallion has no detached floor line. Register its
					// circular face to the original medallion, not the distant line.
					targets = verticalParts(original, original.Bounds())[:1]
				}
				output := image.NewNRGBA(image.Rect(0, 0, 256, 256))
				for part, source := range content {
					target := targets[part]
					target = image.Rect(target.Min.X*4, target.Min.Y*4, target.Max.X*4, target.Max.Y*4)
					targets[part] = target
					for y := target.Min.Y; y < target.Max.Y; y++ {
						for x := target.Min.X; x < target.Max.X; x++ {
							sx := source.Min.X + (2*(x-target.Min.X)+1)*source.Dx()/(2*target.Dx())
							sy := source.Min.Y + (2*(y-target.Min.Y)+1)*source.Dy()/(2*target.Dy())
							output.SetNRGBA(x, y, color.NRGBAModel.Convert(im.At(sx, sy)).(color.NRGBA))
						}
					}
				}
				writePNG(filepath.Join(*root, "sprites", name), output)
				registrations = append(registrations, registration{shape, spec.Name + ".png", region, content, targets})
			}
		}
	}
	data, err := json.MarshalIndent(registrations, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(*root, "registrations.json"), append(data, '\n'), 0644))
	fmt.Printf("Registered %d new transparent 256x256 sprite frames.\n", len(registrations))
}

// Generated sheets are not guaranteed to use equal cells. Find the nearest
// empty alpha scan line to each nominal divider, separately for each row.
func boundaries(im image.Image, region image.Rectangle, count int, vertical bool) []int {
	start, end := region.Min.Y, region.Max.Y
	if vertical {
		start, end = region.Min.X, region.Max.X
	}
	result := []int{start}
	for i := 1; i < count; i++ {
		nominal := start + i*(end-start)/count
		found := -1
		for offset := 0; offset <= (end-start)/count/2; offset++ {
			for _, p := range []int{nominal - offset, nominal + offset} {
				if p <= result[len(result)-1] || p >= end {
					continue
				}
				n := 0
				if vertical {
					for y := region.Min.Y; y < region.Max.Y; y++ {
						if visible(im, p, y) {
							n++
						}
					}
				} else {
					for x := region.Min.X; x < region.Max.X; x++ {
						if visible(im, x, p) {
							n++
						}
					}
				}
				if n == 0 {
					found = p
					break
				}
			}
			if found >= 0 {
				break
			}
		}
		if found < 0 {
			panic(fmt.Sprintf("no transparent divider near %d in %v", nominal, region))
		}
		result = append(result, found)
	}
	return append(result, end)
}

func verticalParts(im image.Image, region image.Rectangle) []image.Rectangle {
	content := occupiedBounds(im, region)
	bestStart, bestLength, currentStart, currentLength := 0, 0, 0, 0
	for y := content.Min.Y; y < content.Max.Y; y++ {
		n := 0
		for x := content.Min.X; x < content.Max.X; x++ {
			if visible(im, x, y) {
				n++
			}
		}
		if n == 0 {
			if currentLength == 0 {
				currentStart = y
			}
			currentLength++
			if currentLength > bestLength {
				bestStart, bestLength = currentStart, currentLength
			}
		} else {
			currentLength = 0
		}
	}
	if bestLength < content.Dy()/5 {
		return []image.Rectangle{content}
	}
	cut := bestStart + bestLength/2
	return []image.Rectangle{occupiedBounds(im, image.Rect(region.Min.X, region.Min.Y, region.Max.X, cut)), occupiedBounds(im, image.Rect(region.Min.X, cut, region.Max.X, region.Max.Y))}
}

func visible(im image.Image, x, y int) bool { _, _, _, a := im.At(x, y).RGBA(); return a >= 32768 }
func occupiedBounds(im image.Image, region image.Rectangle) image.Rectangle {
	b := image.Rectangle{}
	seen := false
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if visible(im, x, y) {
				p := image.Rect(x, y, x+1, y+1)
				if !seen {
					b = p
					seen = true
				} else {
					b = b.Union(p)
				}
			}
		}
	}
	if !seen {
		panic(fmt.Sprintf("empty sprite region %v", region))
	}
	return b
}
func readPNG(path string) image.Image {
	f, e := os.Open(path)
	must(e)
	defer f.Close()
	im, e := png.Decode(f)
	must(e)
	return im
}
func writePNG(path string, im image.Image) {
	f, e := os.Create(path)
	must(e)
	must(png.Encode(f, im))
	must(f.Close())
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
