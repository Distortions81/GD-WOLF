package main

import (
	"fmt"

	"gd-wolf/internal/wl6"
)

// The original map-word plane changes independently of both tilemap and
// actorat. SpawnDoor changes its area first, SpawnStand resolves ambush cells
// in scan order, and SetupGameLevel resolves the remaining ambush cells last.
func (g *game) initializeDemoActorAreas() {
	d := g.demoPlayback
	if d == nil || g.level == nil || len(d.areaPlane) == len(g.level.Tiles) {
		return
	}
	g.initializeDemoDoors()
	plane := make([]uint16, len(g.level.Tiles))
	for i, tile := range g.level.Tiles {
		plane[i] = tile.RawWall
		// Small isolated fixtures construct floors with Area and no raw word.
		if tile.RawWall == 0 && !tile.Solid && tile.Area >= 0 {
			plane[i] = uint16(107 + tile.Area)
		}
	}
	for i, tile := range g.level.Tiles {
		if tile.Door == nil {
			continue
		}
		neighbor := i - g.levelWidth
		if tile.Door.Vertical {
			neighbor = i - 1
		}
		if neighbor >= 0 {
			plane[i] = plane[neighbor]
		}
	}
	resolveAmbush := func(i int, standing bool) {
		found := false
		// Each matching neighbor overwrites tile; west is checked last.
		for _, neighbor := range []int{i + 1, i - g.levelWidth, i + g.levelWidth, i - 1} {
			if neighbor >= 0 && neighbor < len(plane) && plane[neighbor] >= 107 {
				plane[i] = plane[neighbor]
				found = true
			}
		}
		if !found && standing {
			panic(fmt.Sprintf("original ambush area is uninitialized at (%d,%d)", i%g.levelWidth, i/g.levelWidth))
		}
	}
	spawnAreas := make([]byte, len(plane))
	for i, tile := range g.level.Tiles {
		if def, ok := LookupActorSpawn(tile.RawInfo, g.difficulty); ok && def.SpawnMode == actorSpawnStand {
			switch def.Kind {
			case actorKindGuard, actorKindOfficer, actorKindSS, actorKindMutant:
				if plane[i] == 106 {
					resolveAmbush(i, true)
				}
			}
		}
		spawnAreas[i] = byte(plane[i] - 107)
	}
	for i := range plane {
		if plane[i] == 106 {
			resolveAmbush(i, false)
		}
	}
	d.areaPlane, d.spawnAreas = plane, spawnAreas
	for i := range g.actors {
		a := &g.actors[i]
		x, y := int(a.x), int(a.y)
		if !isEnemyActorKind(a.kind) || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
			continue
		}
		a.area = int(spawnAreas[y*g.levelWidth+x])
		if a.spawnMode == actorSpawnPatrol {
			// SpawnPatrol neither resolves AMBUSHTILE nor sets FL_AMBUSH.
			a.ambush = false
		}
	}
}

func (g *game) updateDemoAreaPlaneTile(x, y int, tile wl6.Tile) {
	if g.demoPlayback == nil || len(g.demoPlayback.areaPlane) != len(g.level.Tiles) {
		return
	}
	// MovePWalls writes the player's area only when clearing the trailing
	// cell. Leading wall reservations and the elevator switch alter neither
	// the original floor-area word nor an actor's existing area number.
	if old := g.level.Tile(x, y); old.Solid && !tile.Solid && tile.Door == nil {
		g.demoPlayback.areaPlane[y*g.levelWidth+x] = uint16(107 + tile.Area)
	}
}
