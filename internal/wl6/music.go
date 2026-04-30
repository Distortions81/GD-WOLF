package wl6

import (
	"encoding/binary"
	"fmt"
)

const musicChunkTrailerSize = 88

type MusicEvent struct {
	Reg   uint16
	Value uint8
	Delay uint16
}

type MusicChunk struct {
	Name   string
	Events []MusicEvent
}

func (f *Files) LoadMusicChunk(song int) (*MusicChunk, error) {
	if f == nil {
		return nil, fmt.Errorf("nil files")
	}
	if song < 0 || song >= f.Variant.MusicCount {
		return nil, fmt.Errorf("music song %d out of range", song)
	}
	raw, err := f.loadAudioChunk(f.Variant.StartMusicChunk + song)
	if err != nil {
		return nil, err
	}
	if len(raw) < musicChunkTrailerSize+6 {
		return nil, fmt.Errorf("music song %d unavailable", song)
	}

	dataLen := int(binary.LittleEndian.Uint16(raw[:2]))
	if dataLen < 4 {
		return nil, fmt.Errorf("music song %d has invalid data length %d", song, dataLen)
	}
	if 2+dataLen+musicChunkTrailerSize != len(raw) {
		return nil, fmt.Errorf("music song %d has unexpected chunk size %d for data length %d", song, len(raw), dataLen)
	}

	eventBytes := raw[6 : 2+dataLen]
	if len(eventBytes)%4 != 0 {
		return nil, fmt.Errorf("music song %d has invalid event byte count %d", song, len(eventBytes))
	}

	events := make([]MusicEvent, 0, len(eventBytes)/4)
	for i := 0; i < len(eventBytes); i += 4 {
		regValue := binary.LittleEndian.Uint16(eventBytes[i : i+2])
		delay := binary.LittleEndian.Uint16(eventBytes[i+2 : i+4])
		events = append(events, MusicEvent{
			Reg:   uint16(byte(regValue)),
			Value: uint8(regValue >> 8),
			Delay: delay,
		})
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("music song %d has no events", song)
	}

	return &MusicChunk{
		Name:   parseMusicChunkName(raw[len(raw)-musicChunkTrailerSize:]),
		Events: events,
	}, nil
}

func (f *Files) loadAudioChunk(chunk int) ([]byte, error) {
	offsets, err := parseAudioHead(f.AudioHead)
	if err != nil {
		return nil, err
	}
	if chunk < 0 || chunk >= len(offsets)-1 {
		return nil, fmt.Errorf("audio chunk %d out of range", chunk)
	}
	start := offsets[chunk]
	end := offsets[chunk+1]
	if end < start || end > uint32(len(f.AudioT)) {
		return nil, fmt.Errorf("audio chunk %d out of bounds", chunk)
	}
	buf := make([]byte, int(end-start))
	copy(buf, f.AudioT[start:end])
	return buf, nil
}

func parseAudioHead(data []byte) ([]uint32, error) {
	if len(data) < 8 || len(data)%4 != 0 {
		return nil, fmt.Errorf("AUDIOHED invalid size: %d", len(data))
	}
	offsets := make([]uint32, len(data)/4)
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return offsets, nil
}

func parseMusicChunkName(trailer []byte) string {
	if len(trailer) == 0 {
		return ""
	}
	for i := 0; i < len(trailer)-4; i++ {
		if trailer[i] == 0 {
			continue
		}
		j := i
		for j < len(trailer) && trailer[j] != 0 {
			j++
		}
		if j+4 <= len(trailer) && string(trailer[j:j+4]) == "\x00IMF" {
			return string(trailer[i:j])
		}
	}
	return ""
}
