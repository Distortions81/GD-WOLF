package wl6

import (
	"encoding/binary"
	"fmt"
	"image"
)

const (
	structPicChunk = 0
)

type huffNode struct {
	bit0 uint16
	bit1 uint16
}

type Picture struct {
	Width  int
	Height int
	Data   []byte
}

func (f *Files) PictureChunkRange() (start, end int, err error) {
	offsets, err := parseVGAHead(f.VGAHead)
	if err != nil {
		return 0, 0, err
	}
	nodes, err := parseVGADict(f.VGADict)
	if err != nil {
		return 0, 0, err
	}
	pics, err := loadPicTable(f.VGAGraph, offsets, nodes)
	if err != nil {
		return 0, 0, err
	}
	start = f.Variant.StartPicsChunk
	end = start + len(pics)
	return start, end, nil
}

func (f *Files) LoadPicture(chunk int) (*Picture, error) {
	if chunk < f.Variant.StartPicsChunk {
		return nil, fmt.Errorf("picture chunk %d out of range", chunk)
	}

	offsets, err := parseVGAHead(f.VGAHead)
	if err != nil {
		return nil, err
	}
	nodes, err := parseVGADict(f.VGADict)
	if err != nil {
		return nil, err
	}
	pics, err := loadPicTable(f.VGAGraph, offsets, nodes)
	if err != nil {
		return nil, err
	}

	picIndex := chunk - f.Variant.StartPicsChunk
	if picIndex < 0 || picIndex >= len(pics) {
		return nil, fmt.Errorf("picture index %d out of range", picIndex)
	}
	width := pics[picIndex][0]
	height := pics[picIndex][1]
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid picture size %dx%d", width, height)
	}

	raw, err := loadGraphicChunk(f.VGAGraph, offsets, nodes, chunk)
	if err != nil {
		return nil, err
	}
	data, err := decodePlanarPicture(raw, width, height)
	if err != nil {
		return nil, err
	}
	return &Picture{Width: width, Height: height, Data: data}, nil
}

func (p *Picture) RGBA(palette [256]uint32) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	for i, idx := range p.Data {
		rgba := palette[idx]
		img.Pix[i*4] = byte(rgba >> 24)
		img.Pix[i*4+1] = byte(rgba >> 16)
		img.Pix[i*4+2] = byte(rgba >> 8)
		img.Pix[i*4+3] = byte(rgba)
	}
	return img
}

func parseVGAHead(data []byte) ([]uint32, error) {
	if len(data) < 6 || len(data)%3 != 0 {
		return nil, fmt.Errorf("VGAHEAD invalid size: %d", len(data))
	}

	offsets := make([]uint32, len(data)/3)
	for i := range offsets {
		pos := i * 3
		offsets[i] = uint32(data[pos]) | uint32(data[pos+1])<<8 | uint32(data[pos+2])<<16
	}
	return offsets, nil
}

func parseVGADict(data []byte) ([]huffNode, error) {
	const nodeCount = 255
	need := nodeCount * 4
	if len(data) < need {
		return nil, fmt.Errorf("VGADICT too small: %d", len(data))
	}

	nodes := make([]huffNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		pos := i * 4
		nodes[i] = huffNode{
			bit0: binary.LittleEndian.Uint16(data[pos : pos+2]),
			bit1: binary.LittleEndian.Uint16(data[pos+2 : pos+4]),
		}
	}
	return nodes, nil
}

func loadPicTable(graph []byte, offsets []uint32, nodes []huffNode) ([][2]int, error) {
	raw, err := loadGraphicChunk(graph, offsets, nodes, structPicChunk)
	if err != nil {
		return nil, err
	}
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("pictable has invalid size: %d", len(raw))
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("pictable too small: %d", len(raw))
	}

	pics := make([][2]int, len(raw)/4)
	for i := range pics {
		pos := i * 4
		pics[i] = [2]int{
			int(binary.LittleEndian.Uint16(raw[pos : pos+2])),
			int(binary.LittleEndian.Uint16(raw[pos+2 : pos+4])),
		}
	}
	return pics, nil
}

func loadGraphicChunk(graph []byte, offsets []uint32, nodes []huffNode, chunk int) ([]byte, error) {
	if chunk < 0 || chunk >= len(offsets)-1 {
		return nil, fmt.Errorf("graphic chunk %d out of range", chunk)
	}
	start := offsets[chunk]
	if start == 0xFFFFFF {
		return nil, fmt.Errorf("graphic chunk %d is sparse", chunk)
	}

	next := chunk + 1
	for next < len(offsets) && offsets[next] == 0xFFFFFF {
		next++
	}
	if next >= len(offsets) {
		return nil, fmt.Errorf("graphic chunk %d has no following offset", chunk)
	}
	end := offsets[next]
	if start+4 > uint32(len(graph)) || end > uint32(len(graph)) || end < start+4 {
		return nil, fmt.Errorf("graphic chunk %d out of bounds", chunk)
	}

	expanded := int(binary.LittleEndian.Uint32(graph[start : start+4]))
	compressed := graph[start+4 : end]
	return huffExpand(compressed, expanded, nodes)
}

func huffExpand(src []byte, expanded int, nodes []huffNode) ([]byte, error) {
	if len(nodes) != 255 {
		return nil, fmt.Errorf("unexpected huffman node count %d", len(nodes))
	}
	out := make([]byte, 0, expanded)
	if expanded == 0 {
		return out, nil
	}

	head := 254
	node := head
	bytePos := 0
	bit := byte(1)

	for len(out) < expanded {
		if bytePos >= len(src) {
			return nil, fmt.Errorf("huffman source exhausted at %d/%d bytes", len(out), expanded)
		}

		cur := src[bytePos]
		var code uint16
		if cur&bit != 0 {
			code = nodes[node].bit1
		} else {
			code = nodes[node].bit0
		}

		bit <<= 1
		if bit == 0 {
			bytePos++
			bit = 1
		}

		if code < 256 {
			out = append(out, byte(code))
			node = head
			continue
		}

		idx := int(code - 256)
		if idx < 0 || idx >= len(nodes) {
			return nil, fmt.Errorf("invalid huffman node pointer %d", code)
		}
		node = idx
	}

	return out, nil
}

func decodePlanarPicture(src []byte, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 || width%4 != 0 {
		return nil, fmt.Errorf("unsupported picture size %dx%d", width, height)
	}

	planeStride := width / 4
	planeSize := planeStride * height
	if len(src) < planeSize*4 {
		return nil, fmt.Errorf("picture data too small: %d", len(src))
	}

	out := make([]byte, width*height)
	for plane := 0; plane < 4; plane++ {
		base := plane * planeSize
		for y := 0; y < height; y++ {
			row := base + y*planeStride
			for xByte := 0; xByte < planeStride; xByte++ {
				x := xByte*4 + plane
				out[y*width+x] = src[row+xByte]
			}
		}
	}
	return out, nil
}
