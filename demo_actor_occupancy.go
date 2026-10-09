package main

import (
	"math"

	"gd-wolf/internal/wl6"
)

const demoMaxActors = 150

type demoActorPoolSlot struct {
	used       bool
	index      int
	orderIndex int
	value      actorInstance // Retained after removal, just like the original object memory.
}

type demoActorPool struct {
	tags  []int // 0 empty; 1..255 solid/door tags; 256+slot actor pointers.
	free  []int
	order []int
	slots [demoMaxActors]demoActorPoolSlot
}

func (g *game) initializeDemoActorPool() {
	if g.demoPlayback == nil || g.level == nil {
		return
	}
	g.initializeDemoActorAreas()
	p := &demoActorPool{tags: make([]int, len(g.level.Tiles))}
	g.demoActorPool = p
	for slot := demoMaxActors - 1; slot > 0; slot-- {
		p.free = append(p.free, slot)
		p.slots[slot].index = -1
	}
	p.slots[0].used = true // InitActorList allocates the player first.
	door := 0
	for i, tile := range g.level.Tiles {
		if tile.RawWall < 107 {
			p.tags[i] = int(tile.RawWall)
		}
		if tile.Door != nil {
			if g.doorState[i] != 2 {
				p.tags[i] = 128 + door
			} else {
				p.tags[i] = 0
			}
			door++
		}
	}
	// ScanInfoPlane allocates objects and places static blockers in map order.
	// Corpses consume pool slots even though the renderer keeps them as statics.
	assigned := make([]bool, len(g.actors))
	for i, tile := range g.level.Tiles {
		x, y := i%g.levelWidth, i/g.levelWidth
		if g.blockingStaticAt(x, y) {
			p.tags[i] = 1
		}
		if tile.RawInfo == 124 {
			corpse := actorInstance{kind: -1, x: float64(x) + 0.5, y: float64(y) + 0.5, tileX: x, tileY: y, dir: 8, area: int(g.demoPlayback.spawnAreas[i]), aiState: actorStateDead}
			slot := g.allocateDemoActorSlot(&corpse)
			p.tags[i] = 256 + slot
		}
		for ai := range g.actors {
			a := &g.actors[ai]
			if assigned[ai] || int(a.x) != x || int(a.y) != y {
				continue
			}
			assigned[ai] = true
			slot := g.allocateDemoActorSlot(a)
			p.slots[slot].index = ai
			p.tags[i] = 256 + slot
			if a.spawnMode == actorSpawnPatrol {
				p.tags[i] = 0
				g.setDemoActorTag(a.tileX, a.tileY, 256+slot)
			}
		}
	}
	// Small synthetic tests may construct actors outside their raw spawn tiles.
	for ai := range g.actors {
		if !assigned[ai] {
			a := &g.actors[ai]
			slot := g.allocateDemoActorSlot(a)
			p.slots[slot].index = ai
			g.setDemoActorTag(a.tileX, a.tileY, 256+slot)
		}
	}
	for i, tile := range g.level.Tiles {
		if tile.RawWall == 106 && p.tags[i] == 106 {
			p.tags[i] = 0
		}
	}
}

func (g *game) allocateDemoActorSlot(a *actorInstance) int {
	p := g.demoActorPool
	if len(p.free) == 0 {
		panic("original actor pool exhausted: MAXACTORS=150")
	}
	slot := p.free[len(p.free)-1]
	p.free = p.free[:len(p.free)-1]
	a.poolSlot = slot
	p.slots[slot] = demoActorPoolSlot{used: true, index: -1, orderIndex: len(p.order), value: *a}
	p.order = append(p.order, slot)
	return slot
}

// SpawnNewObj marks its tile immediately; GetNewActor-only projectile and
// smoke spawns do not. A stale grid tag intentionally follows a reused slot.
func (g *game) queueDemoActorSpawn(a actorInstance, mark bool) {
	if g.demoPlayback != nil {
		g.ensureDemoActorPool()
		slot := g.allocateDemoActorSlot(&a)
		if mark {
			g.setDemoActorTag(a.tileX, a.tileY, 256+slot)
		}
	}
	g.pendingActors = append(g.pendingActors, a)
}

func (g *game) ensureDemoActorPool() {
	if g.demoPlayback == nil || g.level == nil {
		return
	}
	if g.demoActorPool == nil || len(g.demoActorPool.tags) != len(g.level.Tiles) {
		g.initializeDemoActorPool()
	}
}

func (g *game) demoActorForSlot(slot int) *actorInstance {
	p := g.demoActorPool
	if slot <= 0 || slot >= demoMaxActors {
		return nil
	}
	s := &p.slots[slot]
	if s.index >= 0 && s.index < len(g.actors) && g.actors[s.index].poolSlot == slot && !g.actors[s.index].removed {
		return &g.actors[s.index]
	}
	return &s.value
}

func (g *game) demoActorTagAt(x, y int) int {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return 1
	}
	g.ensureDemoActorPool()
	return g.demoActorPool.tags[y*g.levelWidth+x]
}

func (g *game) demoActorGridAt(x, y int) *actorInstance {
	tag := g.demoActorTagAt(x, y)
	if tag < 256 {
		return nil
	}
	return g.demoActorForSlot(tag - 256)
}

func (g *game) setDemoActorTag(x, y, tag int) {
	if g.demoActorPool != nil && x >= 0 && y >= 0 && x < g.levelWidth && y < g.levelHeight {
		g.demoActorPool.tags[y*g.levelWidth+x] = tag
	}
}

func (g *game) beginDemoActor(a *actorInstance) {
	if wolfActorMarkFlags(a)&(128|4) == 0 {
		g.setDemoActorTag(a.tileX, a.tileY, 0)
	}
}

func (g *game) finishDemoActor(a *actorInstance) {
	p := g.demoActorPool
	if a.removed {
		p.slots[a.poolSlot].value = *a
		p.slots[a.poolSlot].index = -1
		p.slots[a.poolSlot].used = false
		p.order[p.slots[a.poolSlot].orderIndex] = 0
		p.free = append(p.free, a.poolSlot)
		return
	}
	flags := wolfActorMarkFlags(a)
	if flags&4 != 0 || (flags&128 != 0 && g.demoActorTagAt(a.tileX, a.tileY) != 0) {
		return
	}
	g.setDemoActorTag(a.tileX, a.tileY, 256+a.poolSlot)
}

func (g *game) demoActorGridPlayerPositionClear(x, y float64) bool {
	xl, xh, yl, yh := playerTileSpan(x, y)
	for ty := yl; ty <= yh; ty++ {
		for tx := xl; tx <= xh; tx++ {
			if tag := g.demoActorTagAt(tx, ty); tag != 0 && tag < 256 {
				return false
			}
		}
	}
	for ty := yl - 1; ty <= yh+1; ty++ {
		for tx := xl - 1; tx <= xh+1; tx++ {
			if a := g.demoActorGridAt(tx, ty); a != nil && a.shootable && math.Abs(x-a.x) <= 1 && math.Abs(y-a.y) <= 1 {
				return false
			}
		}
	}
	return true
}

func (g *game) setDemoDoorActorMark(x, y int, occupied bool) {
	if g.demoPlayback == nil {
		return
	}
	g.ensureDemoActorPool()
	tag := 0
	if occupied {
		tag = g.demoDoorActorTag(x, y)
	}
	g.setDemoActorTag(x, y, tag)
}

func (g *game) updateDemoActorGridTile(x, y int, tile wl6.Tile) {
	if g.demoPlayback == nil || g.demoActorPool == nil {
		return
	}
	old := g.level.Tile(x, y)
	if old.RawWall == tile.RawWall && old.Solid == tile.Solid && old.Door == tile.Door {
		return
	}
	// Cmd_Use flips the elevator's tilemap byte without writing actorat.
	// Likewise, stopping a pushwall leaves its existing actorat tag intact.
	if old.Solid && tile.Solid {
		return
	}
	tag := 0
	if tile.Solid {
		tag = int(tile.RawWall)
	}
	g.setDemoActorTag(x, y, tag)
}

// Inert corpses participate in DrawScaleds activation even though this port
// renders their images using the static sprite collection.
func (g *game) activateVisibleDemoInertActors(visible []bool) {
	g.ensureDemoActorPool()
	for _, slot := range g.demoActorPool.order {
		if slot == 0 {
			continue
		}
		a := g.demoActorForSlot(slot)
		if a.kind < 0 && g.demoActorTileVisible(a, visible) {
			a.demoActive = true
		}
	}
}
