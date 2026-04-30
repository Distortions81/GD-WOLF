package wl6

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	numMaps     = 60
	headerCount = 100
	mapPlanes   = 3
)

var (
	errShortBuffer = errors.New("buffer too short")
)

// MapHeader is the on-disk header stored in GAMEMAPS.WL6 for one map.
type MapHeader struct {
	PlaneStart  [mapPlanes]uint32
	PlaneLength [mapPlanes]uint16
	Width       uint16
	Height      uint16
	Name        string
}

// MapSummary is the minimal decoded metadata for one map.
type MapSummary struct {
	Index  int
	Name   string
	Width  uint16
	Height uint16
}

// MapData contains decoded map planes.
type MapData struct {
	Header MapHeader
	Planes [2][]uint16
}

func (m *MapData) Width() int {
	return int(m.Header.Width)
}

func (m *MapData) Height() int {
	return int(m.Header.Height)
}

func (m *MapData) Name() string {
	return m.Header.Name
}

func (m *MapData) Cell(plane, x, y int) uint16 {
	if plane < 0 || plane >= len(m.Planes) {
		return 0
	}
	if x < 0 || y < 0 || x >= m.Width() || y >= m.Height() {
		return 0
	}

	return m.Planes[plane][y*m.Width()+x]
}

func (f *Files) mapSlotCount() int {
	count := f.Variant.EpisodeCount * 10
	if count <= 0 || count > numMaps {
		return numMaps
	}
	return count
}

// Maps parses available map headers.
func (f *Files) Maps() ([]MapSummary, error) {
	rlewTag, offsets, err := parseMapHead(f.MapHead)
	if err != nil {
		return nil, err
	}
	_ = rlewTag

	mapCount := f.mapSlotCount()
	summaries := make([]MapSummary, 0, mapCount)
	for i := 0; i < mapCount; i++ {
		if offsets[i] == 0xffffffff {
			continue
		}

		header, err := parseMapHeaderAt(f.GameMaps, offsets[i])
		if err != nil {
			return nil, fmt.Errorf("parse map %d header: %w", i, err)
		}

		summaries = append(summaries, MapSummary{
			Index:  i,
			Name:   header.Name,
			Width:  header.Width,
			Height: header.Height,
		})
	}

	return summaries, nil
}

// LoadMap decodes the two gameplay planes for one map.
func (f *Files) LoadMap(index int) (*MapData, error) {
	mapCount := f.mapSlotCount()
	if index < 0 || index >= mapCount {
		return nil, fmt.Errorf("map index %d out of range", index)
	}

	rlewTag, offsets, err := parseMapHead(f.MapHead)
	if err != nil {
		return nil, err
	}
	if offsets[index] == 0xffffffff {
		return nil, fmt.Errorf("map %d is sparse", index)
	}

	header, err := parseMapHeaderAt(f.GameMaps, offsets[index])
	if err != nil {
		return nil, fmt.Errorf("parse map %d header: %w", index, err)
	}

	data := &MapData{Header: header}
	expectedBytes := int(header.Width) * int(header.Height) * 2

	for plane := 0; plane < 2; plane++ {
		decoded, err := decodePlane(f.GameMaps, header.PlaneStart[plane], header.PlaneLength[plane], expectedBytes, rlewTag)
		if err != nil {
			return nil, fmt.Errorf("decode map %d plane %d: %w", index, plane, err)
		}
		data.Planes[plane] = decoded
	}

	return data, nil
}

func parseMapHead(data []byte) (uint16, [headerCount]uint32, error) {
	var offsets [headerCount]uint32
	needed := 2 + 4*headerCount
	if len(data) < needed {
		return 0, offsets, fmt.Errorf("%w: need at least %d bytes for MAPHEAD", errShortBuffer, needed)
	}

	rlewTag := binary.LittleEndian.Uint16(data[:2])
	pos := 2
	for i := 0; i < headerCount; i++ {
		offsets[i] = binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
	}

	return rlewTag, offsets, nil
}

func parseMapHeaderAt(gameMaps []byte, offset uint32) (MapHeader, error) {
	var header MapHeader
	const headerSize = 38

	start := int(offset)
	end := start + headerSize
	if start < 0 || end > len(gameMaps) {
		return header, fmt.Errorf("header offset %d out of bounds", offset)
	}

	buf := gameMaps[start:end]
	pos := 0
	for i := 0; i < mapPlanes; i++ {
		header.PlaneStart[i] = binary.LittleEndian.Uint32(buf[pos : pos+4])
		pos += 4
	}
	for i := 0; i < mapPlanes; i++ {
		header.PlaneLength[i] = binary.LittleEndian.Uint16(buf[pos : pos+2])
		pos += 2
	}
	header.Width = binary.LittleEndian.Uint16(buf[pos : pos+2])
	pos += 2
	header.Height = binary.LittleEndian.Uint16(buf[pos : pos+2])
	pos += 2
	header.Name = cString(buf[pos : pos+16])

	return header, nil
}

func decodePlane(gameMaps []byte, offset uint32, compressedLen uint16, expectedBytes int, rlewTag uint16) ([]uint16, error) {
	start := int(offset)
	end := start + int(compressedLen)
	if start < 0 || end > len(gameMaps) {
		return nil, fmt.Errorf("plane chunk [%d:%d] out of bounds", start, end)
	}
	if compressedLen < 2 {
		return nil, fmt.Errorf("compressed plane too small: %d", compressedLen)
	}

	chunk := gameMaps[start:end]
	carmackExpandedSize := int(binary.LittleEndian.Uint16(chunk[:2]))
	carmackExpanded, err := carmackExpand(chunk[2:], carmackExpandedSize)
	if err != nil {
		return nil, err
	}
	if len(carmackExpanded) < 2 {
		return nil, fmt.Errorf("carmack output too small: %d", len(carmackExpanded))
	}

	rlewWords, err := bytesToWords(carmackExpanded[2:])
	if err != nil {
		return nil, fmt.Errorf("decode RLEW source words: %w", err)
	}

	words, err := rlewExpand(rlewWords, expectedBytes, rlewTag)
	if err != nil {
		return nil, err
	}
	return words, nil
}

func carmackExpand(src []byte, expandedBytes int) ([]byte, error) {
	if expandedBytes%2 != 0 {
		return nil, fmt.Errorf("invalid Carmack expanded size %d", expandedBytes)
	}

	const (
		nearTag = 0xa7
		farTag  = 0xa8
	)

	outWords := make([]uint16, 0, expandedBytes/2)
	pos := 0

	for len(outWords) < expandedBytes/2 {
		if pos+2 > len(src) {
			return nil, errors.New("unexpected end of Carmack stream")
		}

		ch := binary.LittleEndian.Uint16(src[pos : pos+2])
		pos += 2
		high := byte(ch >> 8)
		count := int(ch & 0x00ff)

		switch high {
		case nearTag:
			if count == 0 {
				if pos >= len(src) {
					return nil, errors.New("near tag escape missing raw byte")
				}
				raw := uint16(src[pos])
				pos++
				outWords = append(outWords, ch|raw)
				continue
			}

			if pos >= len(src) {
				return nil, errors.New("near tag missing offset byte")
			}
			offset := int(src[pos])
			pos++
			if offset <= 0 || offset > len(outWords) {
				return nil, fmt.Errorf("near copy offset %d out of range", offset)
			}
			if len(outWords)+count > expandedBytes/2 {
				return nil, errors.New("near copy overruns output")
			}
			copyPos := len(outWords) - offset
			for i := 0; i < count; i++ {
				outWords = append(outWords, outWords[copyPos+i])
			}

		case farTag:
			if count == 0 {
				if pos >= len(src) {
					return nil, errors.New("far tag escape missing raw byte")
				}
				raw := uint16(src[pos])
				pos++
				outWords = append(outWords, ch|raw)
				continue
			}

			if pos+2 > len(src) {
				return nil, errors.New("far tag missing offset word")
			}
			offset := int(binary.LittleEndian.Uint16(src[pos : pos+2]))
			pos += 2
			if offset < 0 || offset >= len(outWords) {
				return nil, fmt.Errorf("far copy offset %d out of range", offset)
			}
			if len(outWords)+count > expandedBytes/2 {
				return nil, errors.New("far copy overruns output")
			}
			for i := 0; i < count; i++ {
				idx := offset + i
				if idx >= len(outWords) {
					return nil, fmt.Errorf("far copy source %d out of range", idx)
				}
				outWords = append(outWords, outWords[idx])
			}

		default:
			outWords = append(outWords, ch)
		}
	}

	if len(outWords) != expandedBytes/2 {
		return nil, fmt.Errorf("unexpected Carmack output size %d", len(outWords)*2)
	}

	out := make([]byte, len(outWords)*2)
	for i, w := range outWords {
		binary.LittleEndian.PutUint16(out[i*2:], w)
	}
	return out, nil
}

func rlewExpand(src []uint16, expandedBytes int, tag uint16) ([]uint16, error) {
	if expandedBytes%2 != 0 {
		return nil, fmt.Errorf("invalid RLEW expanded size %d", expandedBytes)
	}

	wantWords := expandedBytes / 2
	out := make([]uint16, 0, wantWords)

	for i := 0; i < len(src) && len(out) < wantWords; i++ {
		value := src[i]
		if value != tag {
			out = append(out, value)
			continue
		}

		if i+2 >= len(src) {
			return nil, errors.New("truncated RLEW run")
		}
		count := int(src[i+1])
		runValue := src[i+2]
		i += 2
		if len(out)+count > wantWords {
			return nil, errors.New("RLEW run overruns output")
		}
		for j := 0; j < count; j++ {
			out = append(out, runValue)
		}
	}

	if len(out) != wantWords {
		return nil, fmt.Errorf("unexpected RLEW output size %d, want %d", len(out), wantWords)
	}

	return out, nil
}

func bytesToWords(data []byte) ([]uint16, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("odd byte count %d", len(data))
	}

	words := make([]uint16, len(data)/2)
	for i := range words {
		words[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	return words, nil
}

func cString(data []byte) string {
	n := 0
	for n < len(data) && data[n] != 0 {
		n++
	}
	return strings.TrimSpace(string(data[:n]))
}
