package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"

	"gd-wolf/internal/wl6"
)

// Compose wall variants at pack-load time. The raycaster still samples a single
// cached texture, while the source art shares materials and decorations.
type hdWallCompositionManifest struct {
	Horizontal map[int]hdWallComposition `json:"horizontal"`
}

type hdWallComposition struct {
	Base   string        `json:"base"`
	Layers []hdWallLayer `json:"layers,omitempty"`
}

type hdWallLayer struct {
	Image string      `json:"image,omitempty"`
	Color *[4]uint8   `json:"color,omitempty"`
	Rect  *[4]float64 `json:"rect,omitempty"` // x, y, width, height in wall units.
}

func (g *game) applyHDWallCompositions() []string {
	if !g.shouldUseHDAssets() || g.walls == nil {
		return nil
	}
	path := filepath.Join(g.hdAssetRoot, "walls", "compositions.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	var manifest hdWallCompositionManifest
	if err == nil {
		err = json.Unmarshal(data, &manifest)
	}
	if err != nil {
		log.Printf("hd-assets: wall compositions failed: %v", err)
		return nil
	}
	// Shared inputs are decoded once and never modified by composition.
	images := map[string]image.Image{}
	load := func(name string) (image.Image, error) {
		if !filepath.IsLocal(name) {
			return nil, fmt.Errorf("wall image path must be relative to walls/: %q", name)
		}
		if img, ok := images[name]; ok {
			return img, nil
		}
		img, ok := g.loadHDPNG("walls", name)
		if !ok {
			return nil, fmt.Errorf("missing wall composition image %q", name)
		}
		images[name] = img
		return img, nil
	}
	ids := make([]int, 0, len(manifest.Horizontal))
	for id := range manifest.Horizontal {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var applied []string
	for _, id := range ids {
		if id <= 0 || id >= len(g.walls.Horizontal) {
			log.Printf("hd-assets: invalid composition wall tile %d", id)
			continue
		}
		img, err := composeHDWall(manifest.Horizontal[id], load)
		if err != nil {
			log.Printf("hd-assets: wall tile %d composition failed: %v", id, err)
			continue
		}
		g.walls.Horizontal[id] = buildWallTextureFromImage(g.walls.Horizontal[id].Page, img)
		if !wl6.WallTileUsesExplicitVertical(id) {
			g.walls.Vertical[id] = buildWallTextureFromImage(g.walls.Vertical[id].Page, dimWallImage(img, wl6.WallShadeScale))
		}
		applied = append(applied, fmt.Sprintf("walls/compositions.json tile %d", id))
	}
	return applied
}

func composeHDWall(spec hdWallComposition, load func(string) (image.Image, error)) (*image.RGBA, error) {
	base, err := load(spec.Base)
	if err != nil {
		return nil, err
	}
	bounds := base.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), base, bounds.Min, draw.Src)
	for _, layer := range spec.Layers {
		if (layer.Image == "") == (layer.Color == nil) {
			return nil, fmt.Errorf("layer must have exactly one image or color")
		}
		r := [4]float64{0, 0, 1, 1}
		if layer.Rect != nil {
			r = *layer.Rect
		}
		for _, v := range r {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("invalid layer rectangle %v", r)
			}
		}
		if r[0] < 0 || r[1] < 0 || r[2] <= 0 || r[3] <= 0 || r[0]+r[2] > 1 || r[1]+r[3] > 1 {
			return nil, fmt.Errorf("layer rectangle must fit inside the wall: %v", r)
		}
		rect := image.Rect(int(math.Round(r[0]*float64(bounds.Dx()))), int(math.Round(r[1]*float64(bounds.Dy()))),
			int(math.Round((r[0]+r[2])*float64(bounds.Dx()))), int(math.Round((r[1]+r[3])*float64(bounds.Dy()))))
		if layer.Color != nil {
			c := layer.Color
			draw.Draw(dst, rect, image.NewUniform(color.NRGBA{R: c[0], G: c[1], B: c[2], A: c[3]}), image.Point{}, draw.Over)
			continue
		}
		src, err := load(layer.Image)
		if err != nil {
			return nil, err
		}
		drawHDWallLayer(dst, rect, src)
	}
	return dst, nil
}

func drawHDWallLayer(dst *image.RGBA, rect image.Rectangle, src image.Image) {
	bounds := src.Bounds()
	if bounds.Size() == rect.Size() {
		draw.Draw(dst, rect, src, bounds.Min, draw.Over)
		return
	}
	// Filter premultiplied channels, so transparent edges do not pick up the
	// RGB of invisible background pixels. Scaling happens only at pack load.
	resized := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			sx := float64(bounds.Min.X) + (float64(x)+0.5)*float64(bounds.Dx())/float64(rect.Dx()) - 0.5
			sy := float64(bounds.Min.Y) + (float64(y)+0.5)*float64(bounds.Dy())/float64(rect.Dy()) - 0.5
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			fx, fy := sx-float64(x0), sy-float64(y0)
			var channels [4]float64
			for yy := 0; yy < 2; yy++ {
				for xx := 0; xx < 2; xx++ {
					wx, wy := 1-fx, 1-fy
					if xx == 1 {
						wx = fx
					}
					if yy == 1 {
						wy = fy
					}
					px, py := maxInt(bounds.Min.X, minInt(bounds.Max.X-1, x0+xx)), maxInt(bounds.Min.Y, minInt(bounds.Max.Y-1, y0+yy))
					r, g, b, a := src.At(px, py).RGBA()
					for c, v := range [4]uint32{r, g, b, a} {
						channels[c] += float64(v) * wx * wy
					}
				}
			}
			resized.SetRGBA(x, y, color.RGBA{R: uint8(math.Round(channels[0] / 257)), G: uint8(math.Round(channels[1] / 257)),
				B: uint8(math.Round(channels[2] / 257)), A: uint8(math.Round(channels[3] / 257))})
		}
	}
	draw.Draw(dst, rect, resized, image.Point{}, draw.Over)
}
