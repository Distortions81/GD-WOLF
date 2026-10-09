package main

import (
	"encoding/binary"
	"fmt"
	"math"
)

const wolfDemoRegisteredMemoryProfile = "registered-apogee-v1.4-2a969a97"
const wolfDemoRegisteredExecutableSHA256 = "2a969a97bc644d04be030d0d08bfda5477179d60f5dbfcc04616a18fa083ce77"

type wolfDemoMemory struct {
	profile string
	update  [20 * 13]byte
}

func (g *game) configureDemoMemory(profile string) error {
	if profile == "" {
		profile = "strict-source"
	}
	if profile != "strict-source" && profile != wolfDemoRegisteredMemoryProfile {
		return fmt.Errorf("unknown demo memory profile %q", profile)
	}
	m := &wolfDemoMemory{profile: profile}
	// DrawPlayScreen marks the play border and status bar before PlayLoop.
	m.markPlayBorder()
	m.markUpdateBlock(0, 160, 319, 199)
	g.demoPlayback.memory = m
	return nil
}

func (g *game) demoMemory() *wolfDemoMemory {
	if g.demoPlayback.memory == nil {
		_ = g.configureDemoMemory("strict-source")
	}
	return g.demoPlayback.memory
}

func (m *wolfDemoMemory) markUpdateBlock(x1, y1, x2, y2 int) {
	for y := maxInt(0, y1>>4); y <= minInt(12, y2>>4); y++ {
		for x := maxInt(0, x1>>4); x <= minInt(19, x2>>4); x++ {
			m.update[y*20+x] = 1
		}
	}
}

func (m *wolfDemoMemory) markPlayBorder() {
	// DrawPlayBorder's first VWB_Bar covers the entire playfield. Its other
	// bars and lines are inside that rectangle at the original 240x120 view.
	m.markUpdateBlock(0, 0, 320, 159)
}

func (m *wolfDemoMemory) clearDirtyBlocks() {
	// Original VH_UpdateScreen clears bytes with the dirty low bit set.
	for i, dirty := range m.update {
		if dirty&1 != 0 {
			m.update[i] = 0
		}
	}
}

func (m *wolfDemoMemory) area255Word() int {
	return int(binary.LittleEndian.Uint16(m.update[36:38]))
}

// These byte offsets are verified against the named DOS executable's memory,
// not the host C compiler's structure sizes or undefined out-of-bounds reads.
func (g *game) demoDOSStaticByte(offset int) byte {
	g.initializeDemoStaticSlots()
	if offset == -2 || offset == -1 {
		pointer := 0x803a + len(g.demoPlayback.staticSlots)*8
		return byte(pointer >> ((offset + 2) * 8))
	}
	if offset < 0 || offset >= demoMaxStatics*8 {
		panic("DOS static alias outside verified region")
	}
	slot := offset / 8
	if slot >= len(g.demoPlayback.staticSlots) {
		panic("unsupported original memory access: DOS static alias reaches unallocated slot with unverified prior-level bytes")
	}
	sprite := &g.staticSprites[g.demoPlayback.staticSlots[slot]]
	info := g.demoPlayback.staticInfo[slot]
	x, y := int(sprite.x), int(sprite.y)
	shape := sprite.shapenum
	if !sprite.alive {
		shape = -1
	}
	switch offset % 8 {
	case 0:
		return byte(x)
	case 1:
		return byte(y)
	case 2, 3:
		return byte(info.visibility >> ((offset%8 - 2) * 8))
	case 4, 5:
		return byte(uint16(shape) >> ((offset%8 - 4) * 8))
	case 6:
		return info.flags
	default:
		return info.item
	}
}

func (g *game) operateDemoAliasedDoor(index int) bool {
	if g.demoMemory().profile != wolfDemoRegisteredMemoryProfile || index < 64 || index >= 128 {
		return false
	}
	var raw [10]byte
	for i := range raw {
		raw[i] = g.demoDOSStaticByte(index*10 - 642 + i)
	}
	lock := raw[4]
	if lock >= 1 && lock <= 4 && g.keys&(1<<(lock-1)) == 0 {
		g.playSound(soundNoWay)
		return true
	}
	action := int16(binary.LittleEndian.Uint16(raw[6:8]))
	switch action {
	case 1, 3: // dr_closed, dr_closing: OpenDoor starts opening.
		g.setDemoDOSStaticWord(index*10-642+6, 2)
	case 0, 2: // dr_open, dr_opening: CloseDoor can reject an obstruction.
		if g.closeDemoAliasedDoor(index, int(raw[0]), int(raw[1]), binary.LittleEndian.Uint16(raw[2:4]) != 0) {
			g.setDemoDOSStaticWord(index*10-642+6, 3)
		}
	default:
		// OperateDoor has no default branch; invalid actions change nothing
		// and emit no sound (also captured on original maps 4 and 27).
	}
	return true
}

// DOS door writes can overlap any pair of static fields. Keep visibility and
// the actor grid independent of the sprite's tile, just like statobjlist.
func (g *game) setDemoDOSStaticWord(offset int, value uint16) {
	g.initializeDemoStaticSlots()
	d := g.demoPlayback
	if offset < 0 || offset&1 != 0 || offset+1 >= len(d.staticSlots)*8 {
		panic("unsupported original memory access: DOS static alias write outside allocated slots")
	}
	slot := offset / 8
	sprite, info := &g.staticSprites[d.staticSlots[slot]], &d.staticInfo[slot]
	switch offset % 8 {
	case 0:
		sprite.x, sprite.y = float64(value&255)+0.5, float64(value>>8)+0.5
	case 2:
		if value < demoDOSVisibilityBase || value >= demoDOSVisibilityBase+4096 {
			panic("unsupported original memory access: static visibility pointer outside verified spotvis")
		}
		info.visibility = value
	case 4:
		sprite.shapenum = int(int16(value))
		sprite.alive = sprite.shapenum != -1
	case 6:
		info.flags, info.item = byte(value), byte(value>>8)
		sprite.pickup = wolfPickupForItem(info.item)
	}
}

func (g *game) closeDemoAliasedDoor(index, x, y int, vertical bool) bool {
	actorTag := func(x, y int) int {
		if x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
			panic(fmt.Sprintf("unsupported original memory access: CloseDoor actorat[%d][%d] outside map", x, y))
		}
		return g.demoActorTagAt(x, y)
	}
	if actorTag(x, y) != 0 || (int(g.playerX) == x && int(g.playerY) == y) {
		return false
	}
	coordinate := func(x, y int, vertical bool) (float64, bool) {
		tag := actorTag(x, y)
		if tag == 0 {
			return 0, false
		}
		if tag < 256 || tag >= 256+demoMaxActors {
			panic("unsupported original memory access: CloseDoor dereferenced non-object near pointer")
		}
		if tag == 256 {
			if vertical {
				return g.playerX, true
			}
			return g.playerY, true
		}
		a := g.demoActorForSlot(tag - 256)
		if vertical {
			return a.x, true
		}
		return a.y, true
	}
	px, py, tx, ty := g.playerX, g.playerY, x, y
	dx, dy := 1, 0
	if !vertical {
		px, py, tx, ty, dx, dy = py, px, ty, tx, 0, 1
	}
	if int(py) == ty && (int(math.Floor(px+playerRadius)) == tx || int(math.Floor(px-playerRadius)) == tx) {
		return false
	}
	if c, ok := coordinate(x-dx, y-dy, vertical); ok && int(math.Floor(c+playerRadius)) == tx {
		return false
	}
	if c, ok := coordinate(x+dx, y+dy, vertical); ok && int(math.Floor(c-playerRadius)) == tx {
		return false
	}
	g.initializeDemoActorAreas()
	area := int(g.demoPlayback.areaPlane[y*g.levelWidth+x]) - 107
	if (area < 0 || area >= 37) && area != 255 {
		panic(fmt.Sprintf("unsupported original memory access: CloseDoor area=%d outside original areas[37]", area))
	}
	if g.isAreaConnectedToPlayer(area) {
		g.playWorldSound(soundDoorClose, float64(x)+0.5, float64(y)+0.5)
	}
	g.setDemoActorTag(x, y, index|128)
	return true
}
