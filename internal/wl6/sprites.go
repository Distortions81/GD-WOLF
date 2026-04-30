package wl6

import (
	"encoding/binary"
	"fmt"
)

type Sprite struct {
	Width         int
	Height        int
	IndexedFormat bool
	Pixels        []uint32
	Columns       []SpriteColumn
}

type SpriteColumn struct {
	Posts []SpritePost
}

type SpritePost struct {
	StartY  int
	EndY    int
	StartT  uint32
	EndT    uint32
	Pixels  []uint32
	Indexed []byte
}

type SpriteSet struct {
	StartPage int
	Pages     []Sprite
}

func (s *SpriteSet) SpriteByShape(shape int) (*Sprite, bool) {
	if s == nil || shape < 0 || shape >= len(s.Pages) {
		return nil, false
	}
	sprite := &s.Pages[shape]
	if sprite.Width <= 0 || sprite.Height <= 0 || (!sprite.IndexedFormat && len(sprite.Pixels) == 0) {
		return nil, false
	}
	return sprite, true
}

func (f *Files) LoadSpriteSet() (*SpriteSet, error) {
	palette, err := loadGamePalette()
	if err != nil {
		return nil, err
	}

	chunkCount, spriteStart, soundStart, offsets, lengths, err := parseVSwapHeader(f.VSwap)
	if err != nil {
		return nil, err
	}
	if spriteStart > soundStart || soundStart > chunkCount {
		return nil, fmt.Errorf("invalid VSWAP sprite range %d..%d of %d", spriteStart, soundStart, chunkCount)
	}

	set := &SpriteSet{
		StartPage: spriteStart,
		Pages:     make([]Sprite, soundStart-spriteStart),
	}
	for page := spriteStart; page < soundStart; page++ {
		if lengths[page] == 0 {
			continue
		}
		sprite, err := decodeSpritePage(f.VSwap, offsets[page], lengths[page], palette)
		if err != nil {
			return nil, fmt.Errorf("decode sprite page %d: %w", page, err)
		}
		set.Pages[page-spriteStart] = sprite
	}
	return set, nil
}

func decodeSpritePage(vswap []byte, offset uint32, length uint16, palette [256]uint32) (Sprite, error) {
	const spriteSize = 64
	var sprite Sprite

	start := int(offset)
	end := start + int(length)
	if start < 0 || end > len(vswap) || end < start {
		return sprite, fmt.Errorf("page [%d:%d] out of bounds", start, end)
	}
	src := vswap[start:end]
	left := int(binary.LittleEndian.Uint16(src[0:2]))
	right := int(binary.LittleEndian.Uint16(src[2:4]))
	if left < 0 || right >= spriteSize || left > right {
		return sprite, fmt.Errorf("invalid sprite bounds %d..%d", left, right)
	}

	columnCount := right - left + 1
	if len(src) < 4+columnCount*2 {
		return sprite, fmt.Errorf("sprite page too small: %d", length)
	}
	columnOfs := make([]uint16, columnCount)
	for i := 0; i < columnCount; i++ {
		base := 4 + i*2
		columnOfs[i] = binary.LittleEndian.Uint16(src[base : base+2])
	}

	columns := make([]SpriteColumn, spriteSize)
	for x := left; x <= right; x++ {
		col := int(columnOfs[x-left])
		if col <= 0 || col+2 > len(src) {
			continue
		}
		for {
			if col+2 > len(src) {
				return sprite, fmt.Errorf("sprite column %d truncated", x)
			}
			endY2 := int(binary.LittleEndian.Uint16(src[col : col+2]))
			if endY2 == 0 {
				break
			}
			if col+6 > len(src) {
				return sprite, fmt.Errorf("sprite column %d truncated", x)
			}
			topAdjust := int(int16(binary.LittleEndian.Uint16(src[col+2 : col+4])))
			startY2 := int(binary.LittleEndian.Uint16(src[col+4 : col+6]))
			col += 6

			startY := startY2 / 2
			endY := endY2 / 2
			if startY < 0 || endY > spriteSize || startY >= endY {
				continue
			}

			// Wolf stores a signed top correction here, not a raw data offset.
			// The scaler jumps in at startY, so the actual post data begins at
			// topAdjust+startY inside the sprite chunk.
			pixelOfs := topAdjust + startY
			if pixelOfs < 0 || pixelOfs+(endY-startY) > len(src) {
				return sprite, fmt.Errorf("sprite post %d out of bounds", x)
			}

			postIndexed := make([]byte, endY-startY)
			for y := startY; y < endY; y++ {
				idx := src[pixelOfs+y-startY]
				postIndexed[y-startY] = idx
			}
			columns[x].Posts = append(columns[x].Posts, SpritePost{
				StartY:  startY,
				EndY:    endY,
				StartT:  uint32(startY<<16) / spriteSize,
				EndT:    uint32(endY<<16) / spriteSize,
				Indexed: postIndexed,
			})
		}
	}

	sprite.Width = spriteSize
	sprite.Height = spriteSize
	sprite.IndexedFormat = true
	sprite.Columns = columns
	return sprite, nil
}
