package wl6

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
)

const (
	wallTextureSize = 64
	wallTextureArea = wallTextureSize * wallTextureSize
	maxWallTiles    = 64
	WallShadeScale  = 0.66
)

const (
	sharewareElevatorWallTile     = 21
	sharewareElevatorWallUsedTile = 22
)

type WallTexture struct {
	Page          int
	Width         int
	Height        int
	Indices       []byte   // palette indices in x-major order
	UseDimPalette bool     // selects the pre-dimmed render palette for indexed draws
	Pixels        []uint32 // unpacked 0xRRGGBBAA pixels in x-major order
}

type WallSet struct {
	Palette    [256]uint32
	RenderLUT  [2][256]uint32
	AllPages   []WallTexture
	Horizontal [maxWallTiles]WallTexture
	Vertical   [maxWallTiles]WallTexture
}

func WallTextureSize() int {
	return wallTextureSize
}

func WallTileUsesExplicitVertical(tile int) bool {
	switch tile {
	case sharewareElevatorWallTile, sharewareElevatorWallUsedTile:
		return true
	default:
		return false
	}
}

func (w WallTexture) Pixel(x, y int) uint32 {
	width, height := w.Size()
	if x < 0 || y < 0 || x >= width || y >= height {
		return 0
	}
	idx := x*height + y
	if idx >= 0 && idx < len(w.Indices) {
		colorIndex := w.Indices[idx]
		if w.UseDimPalette {
			return dimPackedRGBA(defaultGamePalette[colorIndex], WallShadeScale)
		}
		return defaultGamePalette[colorIndex]
	}
	if idx < 0 || idx >= len(w.Pixels) {
		return 0
	}
	return bits.ReverseBytes32(w.Pixels[idx])
}

func (w WallTexture) Empty() bool {
	width, height := w.Size()
	return (len(w.Pixels) == 0 && len(w.Indices) == 0) || width <= 0 || height <= 0
}

func (w WallTexture) Size() (width, height int) {
	width = w.Width
	height = w.Height
	if width <= 0 {
		width = wallTextureSize
	}
	if height <= 0 {
		height = wallTextureSize
	}
	return width, height
}

func (f *Files) LoadWallSet() (*WallSet, error) {
	palette, err := loadGamePalette()
	if err != nil {
		return nil, err
	}

	chunkCount, spriteStart, _, offsets, lengths, err := parseVSwapHeader(f.VSwap)
	if err != nil {
		return nil, err
	}
	if spriteStart > chunkCount {
		return nil, fmt.Errorf("invalid VSWAP sprite start %d > chunk count %d", spriteStart, chunkCount)
	}

	set := &WallSet{Palette: palette}
	for i, rgba := range palette {
		set.RenderLUT[0][i] = bits.ReverseBytes32(rgba)
		set.RenderLUT[1][i] = bits.ReverseBytes32(dimPackedRGBA(rgba, WallShadeScale))
	}
	set.AllPages = make([]WallTexture, spriteStart)
	for page := 0; page < spriteStart; page++ {
		set.AllPages[page].Page = page
		if lengths[page] == 0 {
			continue
		}
		indices, pixels, err := decodeWallPage(f.VSwap, offsets[page], lengths[page], palette)
		if err != nil {
			return nil, fmt.Errorf("decode wall page %d: %w", page, err)
		}
		set.AllPages[page].Width = wallTextureSize
		set.AllPages[page].Height = wallTextureSize
		set.AllPages[page].Indices = indices
		set.AllPages[page].Pixels = pixels
	}
	for tile := 1; tile < maxWallTiles; tile++ {
		hPage := (tile - 1) * 2
		vPage := hPage + 1
		if hPage < len(set.AllPages) {
			set.Horizontal[tile] = set.AllPages[hPage]
			if vPage < len(set.AllPages) {
				if WallTileUsesExplicitVertical(tile) {
					set.Vertical[tile] = set.AllPages[vPage]
				} else {
					set.Vertical[tile] = dimWallTexture(set.AllPages[hPage], WallShadeScale)
				}
			}
			continue
		}
		if vPage < len(set.AllPages) {
			set.Vertical[tile] = set.AllPages[vPage]
		}
	}

	return set, nil
}

func dimWallTexture(src WallTexture, brightness float64) WallTexture {
	width, height := src.Size()
	if (len(src.Pixels) == 0 && len(src.Indices) == 0) || width <= 0 || height <= 0 {
		return WallTexture{
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
	return WallTexture{
		Page:          src.Page,
		Width:         src.Width,
		Height:        src.Height,
		Indices:       indices,
		UseDimPalette: true,
		Pixels:        pixels,
	}
}

func dimPackedRGBA(rgba uint32, brightness float64) uint32 {
	return uint32(scaleWallColorByte(byte(rgba>>24), brightness))<<24 |
		uint32(scaleWallColorByte(byte(rgba>>16), brightness))<<16 |
		uint32(scaleWallColorByte(byte(rgba>>8), brightness))<<8 |
		uint32(byte(rgba))
}

func scaleWallColorByte(value uint8, scale float64) uint8 {
	scaled := float64(value) * scale
	if scaled <= 0 {
		return 0
	}
	if scaled >= 255 {
		return 255
	}
	return uint8(scaled + 0.5)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseVSwapHeader(data []byte) (chunkCount int, spriteStart int, soundStart int, offsets []uint32, lengths []uint16, err error) {
	if len(data) < 6 {
		return 0, 0, 0, nil, nil, fmt.Errorf("VSWAP too small")
	}

	chunkCount = int(binary.LittleEndian.Uint16(data[0:2]))
	spriteStart = int(binary.LittleEndian.Uint16(data[2:4]))
	soundStart = int(binary.LittleEndian.Uint16(data[4:6]))

	offsetTableEnd := 6 + chunkCount*4
	lengthTableEnd := offsetTableEnd + chunkCount*2
	if len(data) < lengthTableEnd {
		return 0, 0, 0, nil, nil, fmt.Errorf("VSWAP truncated header")
	}

	offsets = make([]uint32, chunkCount)
	lengths = make([]uint16, chunkCount)
	pos := 6
	for i := 0; i < chunkCount; i++ {
		offsets[i] = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}
	for i := 0; i < chunkCount; i++ {
		lengths[i] = binary.LittleEndian.Uint16(data[pos : pos+2])
		pos += 2
	}

	return chunkCount, spriteStart, soundStart, offsets, lengths, nil
}

func decodeWallPage(vswap []byte, offset uint32, length uint16, palette [256]uint32) ([]byte, []uint32, error) {
	start := int(offset)
	end := start + int(length)
	if start < 0 || end > len(vswap) {
		return nil, nil, fmt.Errorf("page [%d:%d] out of bounds", start, end)
	}
	if int(length) < wallTextureArea {
		return nil, nil, fmt.Errorf("wall page too small: %d", length)
	}

	src := vswap[start:end]
	indices := make([]byte, wallTextureArea)
	copy(indices, src[:wallTextureArea])
	pixels := make([]uint32, wallTextureArea)
	for i := 0; i < wallTextureArea; i++ {
		pixels[i] = bits.ReverseBytes32(palette[src[i]])
	}
	return indices, pixels, nil
}

func loadGamePalette() ([256]uint32, error) {
	var palette [256]uint32

	data := embeddedGamePalOBJ
	if path, err := findRepoFile(filepath.Join("WOLFSRC", "OBJ", "GAMEPAL.OBJ")); err == nil {
		if repoData, err := os.ReadFile(path); err == nil {
			data = repoData
		}
	}

	raw, err := extractGamePalBytes(data)
	if err != nil {
		return palette, err
	}

	for i := 0; i < 256; i++ {
		r := scaleVGA(raw[i*3])
		g := scaleVGA(raw[i*3+1])
		b := scaleVGA(raw[i*3+2])
		palette[i] = packRGBA(r, g, b, 0xff)
	}
	defaultGamePalette = palette

	return palette, nil
}

func extractGamePalBytes(obj []byte) ([]byte, error) {
	start := bytes.Index(obj, gamePalPrefix)
	if start < 0 {
		return nil, fmt.Errorf("GAMEPAL data not found")
	}
	if start+768 > len(obj) {
		return nil, fmt.Errorf("GAMEPAL data truncated")
	}

	buf := make([]byte, 768)
	copy(buf, obj[start:start+768])
	return buf, nil
}

var gamePalPrefix = []byte{
	0x00, 0x00, 0x00,
	0x00, 0x00, 0x2a,
	0x00, 0x2a, 0x00,
	0x00, 0x2a, 0x2a,
	0x2a, 0x00, 0x00,
	0x2a, 0x00, 0x2a,
	0x2a, 0x15, 0x00,
	0x2a, 0x2a, 0x2a,
	0x15, 0x15, 0x15,
	0x15, 0x15, 0x3f,
	0x15, 0x3f, 0x15,
	0x15, 0x3f, 0x3f,
	0x3f, 0x15, 0x15,
	0x3f, 0x15, 0x3f,
	0x3f, 0x3f, 0x15,
	0x3f, 0x3f, 0x3f,
}

var defaultGamePalette [256]uint32

func findRepoFile(rel string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	current := cwd
	for {
		candidate := filepath.Join(current, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("could not find %s", rel)
}

func scaleVGA(v byte) uint32 {
	return uint32(v) * 255 / 63
}

func DimPackedRGBAForRender(rgba uint32, brightness float64) uint32 {
	return dimPackedRGBA(rgba, brightness)
}

func packRGBA(r, g, b, a uint32) uint32 {
	return (r << 24) | (g << 16) | (b << 8) | a
}
