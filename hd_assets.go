package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math/bits"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"gd-wolf/internal/wl6"
)

func locateHDAssetsRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	current := cwd
	for {
		candidate := filepath.Join(current, "hd-assets")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func (g *game) loadHDPictureImage(chunk int) (*ebiten.Image, bool) {
	img, ok := g.loadHDPNG("pictures", pictureChunkFileName(chunk))
	if !ok {
		return nil, false
	}
	return ebiten.NewImageFromImage(imageToRGBA(img)), true
}

func (g *game) applyHDWallOverrides() {
	if g == nil || g.walls == nil {
		return
	}

	var applied []string
	for page := wallOrientationPageSourceStart(); page < wallOrientationPageCount(); page += 2 {
		img, ok := g.loadHDPNG("walls", "pages", wallPageFileName(page))
		if !ok {
			continue
		}
		g.walls.AllPages[page] = buildWallTextureFromImage(g.walls.AllPages[page].Page, img)
		nextPage := page + 1
		if nextPage >= 0 && nextPage < len(g.walls.AllPages) {
			g.walls.AllPages[nextPage] = buildWallTextureFromImage(g.walls.AllPages[nextPage].Page, dimWallImage(img, wl6.WallShadeScale))
		}
		applied = append(applied, filepath.Join("walls", "pages", wallPageFileName(page))+" (next page auto-dimmed 50%)")
	}

	for page := range g.walls.AllPages {
		if isWallOrientationPage(page) {
			continue
		}
		img, ok := g.loadHDPNG("walls", "pages", wallPageFileName(page))
		if !ok {
			continue
		}
		g.walls.AllPages[page] = buildWallTextureFromImage(g.walls.AllPages[page].Page, img)
		applied = append(applied, filepath.Join("walls", "pages", wallPageFileName(page)))
	}

	for tile := 1; tile < len(g.walls.Horizontal); tile++ {
		hPage := (tile - 1) * 2
		vPage := hPage + 1
		if hPage < len(g.walls.AllPages) {
			g.walls.Horizontal[tile] = g.walls.AllPages[hPage]
			if wl6.WallTileUsesExplicitVertical(tile) && vPage < len(g.walls.AllPages) {
				g.walls.Vertical[tile] = g.walls.AllPages[vPage]
			} else {
				g.walls.Vertical[tile] = dimWallTextureCopy(g.walls.Horizontal[tile], wl6.WallShadeScale)
			}
			continue
		}
		if vPage < len(g.walls.AllPages) {
			g.walls.Vertical[tile] = g.walls.AllPages[vPage]
		}
	}

	for tile := 1; tile < len(g.walls.Horizontal); tile++ {
		hImg, hOK := g.loadHDPNG("walls", "horizontal", wallTileFileName(tile))
		if !hOK {
			continue
		}
		g.walls.Horizontal[tile] = buildWallTextureFromImage(g.walls.Horizontal[tile].Page, hImg)
		if wl6.WallTileUsesExplicitVertical(tile) {
			applied = append(applied, filepath.Join("walls", "horizontal", wallTileFileName(tile)))
			continue
		}
		g.walls.Vertical[tile] = buildWallTextureFromImage(g.walls.Vertical[tile].Page, dimWallImage(hImg, wl6.WallShadeScale))
		applied = append(applied,
			filepath.Join("walls", "horizontal", wallTileFileName(tile))+" (vertical auto-dimmed 50%)",
		)
	}

	if len(applied) > 0 {
		log.Printf("hd-assets: applied %d wall replacement(s) from %s: %v", len(applied), g.hdAssetRoot, applied)
	}
}

func (g *game) applyHDSpriteOverrides() {
	if g == nil || g.sprites == nil {
		return
	}

	var applied []string
	for shape := range g.sprites.Pages {
		img, ok := g.loadHDPNG("sprites", spriteShapeFileName(shape))
		if !ok {
			continue
		}
		g.sprites.Pages[shape] = buildSpriteFromImage(img)
		applied = append(applied, filepath.Join("sprites", spriteShapeFileName(shape)))
	}
	if len(applied) > 0 {
		log.Printf("hd-assets: applied %d sprite replacement(s) from %s: %v", len(applied), g.hdAssetRoot, applied)
	}
}

func (g *game) loadHDPNG(parts ...string) (image.Image, bool) {
	if g == nil || !g.shouldUseHDAssets() {
		return nil, false
	}
	path := filepath.Join(append([]string{g.hdAssetRoot}, parts...)...)
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		log.Printf("hd-assets: decode failed for %s: %v", path, err)
		return nil, false
	}
	return img, true
}

func (g *game) shouldUseHDAssets() bool {
	return g != nil && g.hdTexturesEnabled && g.hdAssetRoot != "" && g.renderMode == renderModeUltra
}

func imageToRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

func applyTopLeftColorKey(img *image.RGBA) {
	if img == nil {
		return
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return
	}
	key := img.RGBAAt(bounds.Min.X, bounds.Min.Y)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.RGBAAt(x, y) != key {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{})
		}
	}
}

func imageToNRGBA(src image.Image) *image.NRGBA {
	if nrgba, ok := src.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
	return dst
}

func dimWallImage(src image.Image, brightness float64) *image.RGBA {
	rgba := imageToRGBA(src)
	bounds := rgba.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	if brightness < 0 {
		brightness = 0
	}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			pixel := rgba.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
			dst.SetRGBA(x, y, color.RGBA{
				R: scaleColorByte(pixel.R, brightness),
				G: scaleColorByte(pixel.G, brightness),
				B: scaleColorByte(pixel.B, brightness),
				A: pixel.A,
			})
		}
	}
	return dst
}

func scaleColorByte(value uint8, scale float64) uint8 {
	scaled := float64(value) * scale
	if scaled <= 0 {
		return 0
	}
	if scaled >= 255 {
		return 255
	}
	return uint8(scaled + 0.5)
}

func buildWallTextureFromImage(page int, src image.Image) wl6.WallTexture {
	rgba := imageToRGBA(src)
	bounds := rgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return wl6.WallTexture{Page: page}
	}
	pixels := make([]uint32, width*height)
	for i := range pixels {
		x := i / height
		y := i % height
		pixels[i] = bits.ReverseBytes32(rgbaAt(rgba, x, y))
	}
	return wl6.WallTexture{
		Page:   page,
		Width:  width,
		Height: height,
		Pixels: pixels,
	}
}

func buildSpriteFromImage(src image.Image) wl6.Sprite {
	nrgba := imageToNRGBA(src)
	bounds := nrgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return wl6.Sprite{}
	}
	sprite := wl6.Sprite{
		Width:   width,
		Height:  height,
		Pixels:  make([]uint32, width*height),
		Columns: make([]wl6.SpriteColumn, width),
	}
	for x := 0; x < width; x++ {
		postStart := -1
		var postPixels []uint32
		for y := 0; y < height; y++ {
			rgbaValue := nrgbaAt(nrgba, x, y)
			if byte(rgbaValue) == 0 {
				if postStart >= 0 {
					sprite.Columns[x].Posts = append(sprite.Columns[x].Posts, wl6.SpritePost{
						StartY: postStart,
						EndY:   y,
						StartT: uint32(postStart<<16) / uint32(height),
						EndT:   uint32(y<<16) / uint32(height),
						Pixels: postPixels,
					})
					postStart = -1
					postPixels = nil
				}
				continue
			}
			sprite.Pixels[y*width+x] = rgbaValue
			if postStart < 0 {
				postStart = y
			}
			postPixels = append(postPixels, rgbaValue)
		}
		if postStart >= 0 {
			sprite.Columns[x].Posts = append(sprite.Columns[x].Posts, wl6.SpritePost{
				StartY: postStart,
				EndY:   height,
				StartT: uint32(postStart<<16) / uint32(height),
				EndT:   uint32(height<<16) / uint32(height),
				Pixels: postPixels,
			})
		}
	}
	return sprite
}

func dimWallTextureCopy(src wl6.WallTexture, brightness float64) wl6.WallTexture {
	width, height := src.Size()
	if (len(src.Pixels) == 0 && len(src.Indices) == 0) || width <= 0 || height <= 0 {
		return wl6.WallTexture{
			Page:          src.Page,
			Width:         src.Width,
			Height:        src.Height,
			UseDimPalette: true,
		}
	}
	if brightness < 0 {
		brightness = 0
	}
	indices := make([]byte, len(src.Indices))
	copy(indices, src.Indices)
	pixels := make([]uint32, len(src.Pixels))
	for i, stored := range src.Pixels {
		rgba := bits.ReverseBytes32(stored)
		pixels[i] = bits.ReverseBytes32(dimPackedRGBA(rgba, brightness))
	}
	return wl6.WallTexture{
		Page:          src.Page,
		Width:         src.Width,
		Height:        src.Height,
		Indices:       indices,
		UseDimPalette: true,
		Pixels:        pixels,
	}
}

func dimPackedRGBA(rgba uint32, brightness float64) uint32 {
	return uint32(scaleColorByte(uint8(rgba>>24), brightness))<<24 |
		uint32(scaleColorByte(uint8(rgba>>16), brightness))<<16 |
		uint32(scaleColorByte(uint8(rgba>>8), brightness))<<8 |
		uint32(uint8(rgba))
}

func rgbaAt(img *image.RGBA, x, y int) uint32 {
	i := img.PixOffset(x, y)
	return uint32(img.Pix[i])<<24 |
		uint32(img.Pix[i+1])<<16 |
		uint32(img.Pix[i+2])<<8 |
		uint32(img.Pix[i+3])
}

func nrgbaAt(img *image.NRGBA, x, y int) uint32 {
	i := img.PixOffset(x, y)
	return uint32(img.Pix[i])<<24 |
		uint32(img.Pix[i+1])<<16 |
		uint32(img.Pix[i+2])<<8 |
		uint32(img.Pix[i+3])
}

func pictureChunkFileName(chunk int) string {
	return fmt.Sprintf("chunk-%03d.png", chunk)
}

func wallPageFileName(page int) string {
	return fmt.Sprintf("page-%03d.png", page)
}

func wallOrientationPageCount() int {
	return (64 - 1) * 2
}

func wallOrientationPageSourceStart() int {
	return 0
}

func isWallOrientationPage(page int) bool {
	return page >= 0 && page < wallOrientationPageCount()
}

func wallTileFileName(tile int) string {
	return fmt.Sprintf("tile-%02d.png", tile)
}

func spriteShapeFileName(shape int) string {
	return fmt.Sprintf("shape-%03d.png", shape)
}
