package wl6

import (
	"encoding/binary"
	"fmt"
)

// DemoTics is DEMOTICS from the original ID_HEADS.H. Each recorded command
// advances four 70 Hz simulation tics, rather than one rendering frame.
const DemoTics = 4

type DemoCommand struct {
	Buttons  byte
	ControlX int8
	ControlY int8
}

type Demo struct {
	Map      int
	Commands []DemoCommand
}

// ParseDemo reads the four-byte header and signed three-byte input records
// written by RecordDemo. The fourth header byte is unused (often nonzero).
func ParseDemo(data []byte) (*Demo, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("demo is too short: %d bytes", len(data))
	}
	length := int(binary.LittleEndian.Uint16(data[1:3]))
	if length != len(data) || (length-4)%3 != 0 {
		return nil, fmt.Errorf("invalid demo length: header=%d actual=%d", length, len(data))
	}
	if data[0] >= 60 {
		return nil, fmt.Errorf("demo map %d out of range", data[0])
	}
	demo := &Demo{Map: int(data[0]), Commands: make([]DemoCommand, (length-4)/3)}
	for i := range demo.Commands {
		p := 4 + i*3
		demo.Commands[i] = DemoCommand{data[p], int8(data[p+1]), int8(data[p+2])}
	}
	return demo, nil
}

func (f *Files) LoadDemo(index int) (*Demo, error) {
	if index < 0 || index >= 4 || f.Variant.DemoStartChunk == 0 {
		return nil, fmt.Errorf("built-in demo index %d out of range", index)
	}
	offsets, err := parseVGAHead(f.VGAHead)
	if err != nil {
		return nil, err
	}
	nodes, err := parseVGADict(f.VGADict)
	if err != nil {
		return nil, err
	}
	data, err := loadGraphicChunk(f.VGAGraph, offsets, nodes, f.Variant.DemoStartChunk+index)
	if err != nil {
		return nil, err
	}
	demo, err := ParseDemo(data)
	if err != nil {
		return nil, err
	}
	if demo.Map >= f.mapSlotCount() {
		return nil, fmt.Errorf("demo map %d unavailable in %s", demo.Map, f.Variant.Name)
	}
	return demo, nil
}
