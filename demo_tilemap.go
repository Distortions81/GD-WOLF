package main

import "gd-wolf/internal/wl6"

// Original tilemap bytes drive ray traversal independently of actorat, which
// drives physical collision. A door jamb can mark an otherwise walkable floor.
func (g *game) initializeDemoTileMap() {
	if g.demoPlayback == nil || g.level == nil {
		return
	}
	tiles := make([]byte, len(g.level.Tiles))
	for i, tile := range g.level.Tiles {
		if tile.RawWall < 107 {
			tiles[i] = byte(tile.RawWall)
		}
	}
	markJamb := func(x, y int) {
		if x >= 0 && y >= 0 && x < g.levelWidth && y < g.levelHeight {
			tiles[y*g.levelWidth+x] |= 0x40
		}
	}
	door := 0
	for i, tile := range g.level.Tiles {
		if tile.Door == nil {
			continue
		}
		x, y := i%g.levelWidth, i/g.levelWidth
		tiles[i] = byte(door) | 0x80
		if tile.Door.Vertical {
			markJamb(x, y-1)
			markJamb(x, y+1)
		} else {
			markJamb(x-1, y)
			markJamb(x+1, y)
		}
		door++
	}
	for i, tile := range g.level.Tiles {
		if tile.RawWall == 106 {
			tiles[i] = 0
		}
	}
	g.demoPlayback.tileMap = tiles
}

func (g *game) demoTileMapAt(x, y int) byte {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return 1
	}
	if g.demoPlayback == nil {
		if g.level.Tile(x, y).Solid {
			return 1
		}
		return 0
	}
	if len(g.demoPlayback.tileMap) != len(g.level.Tiles) {
		g.initializeDemoTileMap()
	}
	tiles := g.demoPlayback.tileMap
	if g.pushWall.active {
		wall := tiles[g.pushWall.y*g.levelWidth+g.pushWall.x]
		if x == g.pushWall.x && y == g.pushWall.y {
			return wall | 0xc0
		}
		// PushWall copies the complete original byte into its leading tile.
		// At the first crossing MovePWalls keeps only its low six bits.
		if g.pushWall.steps == 0 && x == g.pushWall.x+g.pushWall.dx && y == g.pushWall.y+g.pushWall.dy {
			return wall
		}
	}
	return tiles[y*g.levelWidth+x]
}

func (g *game) demoHasRayWall(x, y int) bool {
	tile := g.demoTileMapAt(x, y)
	return tile != 0 && (tile&0x80 == 0 || tile&0x40 != 0)
}

// setLevelTile changes only the selected runtime tile. Preserve SpawnDoor's
// jamb bits when merely removing a secret marker from an unchanged wall.
func (g *game) updateDemoTileMapTile(x, y int, tile wl6.Tile) {
	if g.demoPlayback == nil {
		return
	}
	if len(g.demoPlayback.tileMap) != len(g.level.Tiles) {
		g.initializeDemoTileMap()
	}
	old := g.level.Tile(x, y)
	if old.RawWall == tile.RawWall && old.Solid == tile.Solid && old.Door == tile.Door {
		return
	}
	value := byte(0)
	if tile.Door != nil {
		door := 0
		for i := 0; i < y*g.levelWidth+x; i++ {
			if g.level.Tiles[i].Door != nil {
				door++
			}
		}
		value = byte(door) | 0x80
	} else if tile.Solid {
		value = byte(tile.RawWall)
	}
	g.demoPlayback.tileMap[y*g.levelWidth+x] = value
}
