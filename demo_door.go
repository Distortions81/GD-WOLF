package main

import "gd-wolf/internal/wl6"

type demoDoor struct {
	x, y       int
	definition wl6.Door
}

// Original doorobjlist entries survive tilemap/actorat changes. In particular,
// a pushwall may cross an open door without removing or renumbering it.
func (g *game) initializeDemoDoors() {
	d := g.demoPlayback
	if d == nil || g.level == nil || len(d.doorIndices) == len(g.level.Tiles) {
		return
	}
	d.doorIndices = make([]int, len(g.level.Tiles))
	for i := range d.doorIndices {
		d.doorIndices[i] = -1
	}
	for i, tile := range g.level.Tiles {
		if tile.Door != nil {
			d.doorIndices[i] = len(d.doors)
			d.doors = append(d.doors, demoDoor{i % g.levelWidth, i / g.levelWidth, *tile.Door})
		}
	}
}

func (g *game) doorDefinitionAt(x, y int) *wl6.Door {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return nil
	}
	if d := g.demoPlayback; d != nil {
		g.initializeDemoDoors()
		if index := d.doorIndices[y*g.levelWidth+x]; index >= 0 {
			return &d.doors[index].definition
		}
		return nil
	}
	return g.level.Tile(x, y).Door
}

func (g *game) demoDoorActorTag(x, y int) int {
	g.initializeDemoDoors()
	index := g.demoPlayback.doorIndices[y*g.levelWidth+x]
	if index < 0 {
		panic("original door tag requested at a tile without a door")
	}
	return 128 + index
}

func (g *game) demoDoorByIndex(index int) demoDoor {
	g.initializeDemoDoors()
	if index < 0 || index >= len(g.demoPlayback.doors) {
		panic("original actor referenced an unallocated door")
	}
	return g.demoPlayback.doors[index]
}
