//go:build ignore

// Run from the repository root: go run ./art/e1f1-hd/register_sprite_frames.go
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
	atlases := []atlas{
		{"guard-movement", 8, 5, shapeRange(50, 40)},
		{"guard-actions", 4, 4, []int{-1, -1, -1, -1, -1, 96, 97, 98, 90, 94, 91, 92, 93, 95, -1, -1}},
	}
	for phase := 0; phase < 4; phase++ {
		atlases = append(atlases, atlas{fmt.Sprintf("dog-movement-%d", phase), 4, 2, shapeRange(99+phase*8, 8)})
	}
	atlases = append(atlases,
		atlas{"dog-actions", 4, 2, []int{131, 132, 133, 134, 135, 136, 137, -1}},
		atlas{"scenery-1", 3, 2, []int{2, 3, 4, 5, 6, 8}},
		atlas{"scenery-2", 3, 2, []int{10, 11, 13, 14, 15, -1}},
		atlas{"scenery-3", 3, 2, []int{18, 21, 25, 26, 27, 28}},
		atlas{"scenery-4", 3, 2, []int{29, 31, 32, 33, 35, 37}},
		atlas{"scenery-5", 3, 1, []int{38, 39, 41}},
		atlas{"player-marker", 1, 1, []int{408}},
	)
	var registrations []registration
	for _, spec := range atlases {
		im := readPNG(filepath.Join(*root, "revision-2", "atlases", spec.Name+".png"))
		ycuts := boundaries(im, im.Bounds(), spec.Rows, false)
		for row := 0; row < spec.Rows; row++ {
			rowRegion := image.Rect(0, ycuts[row], im.Bounds().Dx(), ycuts[row+1])
			xcuts := boundaries(im, rowRegion, spec.Columns, true)
			for column := 0; column < spec.Columns; column++ {
				shape := spec.Shapes[row*spec.Columns+column]
				if shape < 0 || shape == 6 {
					continue
				}
				name := fmt.Sprintf("shape-%03d.png", shape)
				region := image.Rect(xcuts[column], ycuts[row], xcuts[column+1], ycuts[row+1])
				original := readPNG(filepath.Join(*root, "references", "sprites", name))
				content := []image.Rectangle{occupiedBounds(im, region).Inset(-2).Intersect(region)}
				target := originalObjectBounds(original, shape)
				target = image.Rect(target.Min.X*8, target.Min.Y*8, target.Max.X*8, target.Max.Y*8)
				targets := []image.Rectangle{target}
				output := image.NewNRGBA(image.Rect(0, 0, 512, 512))
				assetimage.ResampleCutout(output, target, im, content[0])
				writePNG(filepath.Join(*root, "sprites", name), output)
				registrations = append(registrations, registration{shape, "revision-2/atlases/" + spec.Name + ".png", region, content, targets})
			}
		}
	}
	// Hanging fixtures omit their old opaque floor patches. Keep their original
	// placement; the renderer supplies light on the actual horizontal floor.
	for _, shape := range []int{6, 16} {
		name := fmt.Sprintf("shape-%03d.png", shape)
		im := readPNG(filepath.Join(*root, "edits", name))
		source := occupiedBounds(im, im.Bounds()).Inset(-2).Intersect(im.Bounds())
		var target image.Rectangle
		if shape == 6 {
			original := readPNG(filepath.Join(*root, "references", "sprites", name))
			target = verticalParts(original, original.Bounds())[0]
			target = image.Rect(target.Min.X*16, target.Min.Y*16, target.Max.X*16, target.Max.Y*16)
		} else {
			original := readPNG(filepath.Join(*root, "edits", "references", "shape-016-before.png"))
			// The physical dome ends at 17% of the approved lamp canvas; its
			// old cone and detached light pool are excluded from registration.
			target = occupiedBounds(original, image.Rect(0, 0, original.Bounds().Dx(), original.Bounds().Dy()*17/100))
		}
		output := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
		assetimage.ResampleCutout(output, target, im, source)
		writePNG(filepath.Join(*root, "sprites", name), output)
		registrations = append(registrations, registration{shape, "edits/" + name, im.Bounds(), []image.Rectangle{source}, []image.Rectangle{target}})
	}
	data, err := json.MarshalIndent(registrations, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(*root, "registrations.json"), append(data, '\n'), 0644))
	fmt.Printf("Registered %d transparent HD sprite frames.\n", len(registrations))
}

func shapeRange(first, count int) []int {
	shapes := make([]int, count)
	for i := range shapes {
		shapes[i] = first + i
	}
	return shapes
}

// Register the physical object, excluding the original opaque ground shadow.
// These scenery rectangles were reviewed against the decoded 64px originals.
func originalObjectBounds(im image.Image, shape int) image.Rectangle {
	if box, ok := map[int]image.Rectangle{
		3:  image.Rect(20, 32, 45, 64),
		4:  image.Rect(7, 31, 56, 64),
		5:  image.Rect(21, 23, 44, 62),
		10: image.Rect(17, 10, 49, 64),
		13: image.Rect(24, 9, 42, 64),
		14: image.Rect(24, 33, 43, 64),
		15: image.Rect(11, 38, 54, 64),
		18: image.Rect(15, 1, 55, 64),
		25: image.Rect(21, 48, 42, 63),
		26: image.Rect(16, 55, 44, 64),
		27: image.Rect(18, 54, 45, 64),
		28: image.Rect(27, 53, 37, 63),
		29: image.Rect(12, 47, 51, 59),
		31: image.Rect(25, 45, 38, 63),
		32: image.Rect(27, 48, 39, 63),
		33: image.Rect(17, 50, 43, 64),
		35: image.Rect(20, 34, 41, 55),
		37: image.Rect(17, 31, 48, 61),
		38: image.Rect(9, 39, 51, 64),
		39: image.Rect(9, 39, 51, 64),
		41: image.Rect(24, 3, 42, 64),
	}[shape]; ok {
		return box
	}
	if shape >= 99 && shape <= 137 {
		// The classic dog shadow is achromatic along the last eight rows;
		// retain the colored paws, tail and blood when finding body bounds.
		b := image.Rectangle{}
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				r, g, bl, a := im.At(x, y).RGBA()
				if a < 32768 || y >= 56 && r == g && g == bl {
					continue
				}
				b = b.Union(image.Rect(x, y, x+1, y+1))
			}
		}
		return b
	}
	return occupiedBounds(im, im.Bounds())
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
