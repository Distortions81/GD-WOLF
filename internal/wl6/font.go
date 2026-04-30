package wl6

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

const startFontChunk = 1

type Font struct {
	Height   int
	Location [256]uint16
	Width    [256]byte
}

const fontHeaderSize = 2 + 256*2 + 256

func (f *Files) LoadFont(index int) (*Font, error) {
	if index < 0 || index >= 2 {
		return nil, fmt.Errorf("font index %d out of range", index)
	}
	offsets, err := parseVGAHead(f.VGAHead)
	if err != nil {
		return nil, err
	}
	nodes, err := parseVGADict(f.VGADict)
	if err != nil {
		return nil, err
	}
	raw, err := loadGraphicChunk(f.VGAGraph, offsets, nodes, startFontChunk+index)
	if err != nil {
		return nil, err
	}
	if len(raw) < 2+256*2+256 {
		return nil, fmt.Errorf("font chunk too small: %d", len(raw))
	}
	font := &Font{
		Height: int(binary.LittleEndian.Uint16(raw[:2])),
	}
	pos := 2
	for i := range font.Location {
		font.Location[i] = binary.LittleEndian.Uint16(raw[pos : pos+2])
		pos += 2
	}
	copy(font.Width[:], raw[pos:pos+256])
	if font.Height <= 0 {
		return nil, fmt.Errorf("invalid font height %d", font.Height)
	}
	return font, nil
}

func (f *Font) Measure(text string) (width, height int) {
	height = f.Height
	lineWidth := 0
	lines := 1
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\n' {
			if lineWidth > width {
				width = lineWidth
			}
			lineWidth = 0
			lines++
			continue
		}
		lineWidth += int(f.Width[ch])
	}
	if lineWidth > width {
		width = lineWidth
	}
	return width, lines * f.Height
}

func (f *Font) GlyphImage(raw []byte, ch byte, fg color.Color) (*image.RGBA, error) {
	width := int(f.Width[ch])
	if width <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, f.Height)), nil
	}
	start := int(f.Location[ch])
	size := width * f.Height
	if start < 0 || start+size > len(raw) {
		return nil, fmt.Errorf("glyph %d out of range", ch)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, f.Height))
	r, g, b, a := fg.RGBA()
	for y := 0; y < f.Height; y++ {
		for x := 0; x < width; x++ {
			if raw[start+y*width+x] == 0 {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}
	return img, nil
}

func (f *Files) LoadRawFont(index int) (*Font, []byte, error) {
	if index < 0 || index >= 2 {
		return nil, nil, fmt.Errorf("font index %d out of range", index)
	}
	offsets, err := parseVGAHead(f.VGAHead)
	if err != nil {
		return nil, nil, err
	}
	nodes, err := parseVGADict(f.VGADict)
	if err != nil {
		return nil, nil, err
	}
	raw, err := loadGraphicChunk(f.VGAGraph, offsets, nodes, startFontChunk+index)
	if err != nil {
		return nil, nil, err
	}
	font, err := f.LoadFont(index)
	if err != nil {
		return nil, nil, err
	}
	return font, raw, nil
}
