package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func demoTileMapTestGame() *game {
	level := blankLevel(7, 7)
	for i := range level.Tiles {
		level.Tiles[i] = wl6.Tile{RawWall: 107, Area: 0}
	}
	g := testGameWithLevel(level)
	g.demoPlayback = &wolfDemoPlayback{}
	g.playerX, g.playerY = 5.5, 5.5
	return g
}

func TestDemoTileMapDoorJambDoesNotChangeCollision(t *testing.T) {
	g := demoTileMapTestGame()
	setLevelTile(g.level, 2, 2, wl6.Tile{RawWall: 90, Solid: true, Door: &wl6.Door{Vertical: true}})
	g.initializeDemoTileMap()
	if g.demoTileMapAt(2, 2) != 0x80 || g.demoTileMapAt(2, 3) != 0x40 {
		t.Fatalf("door/jamb bytes = %x/%x", g.demoTileMapAt(2, 2), g.demoTileMapAt(2, 3))
	}
	if !g.demoPlayerPositionClear(2.5, 3.5) || g.level.Tile(2, 3).Solid {
		t.Fatal("raycaster jamb changed physical floor collision")
	}
	if g.demoRayPassesTile(2, 3, 3*65536+32768, 0) || !g.demoRayPassesTile(3, 3, 3*65536+32768, 0) {
		t.Fatal("raycaster did not distinguish the marked jamb from plain floor")
	}
	visible := make([]bool, len(g.level.Tiles))
	visible[3*g.levelWidth+2] = true
	actor := &actorInstance{tileX: 3, tileY: 3}
	if g.demoActorTileVisible(actor, visible) {
		t.Fatal("actor became visible through a nonzero neighboring tilemap byte")
	}
	visible[3*g.levelWidth+3] = true
	if !g.demoActorTileVisible(actor, visible) {
		t.Fatal("actor's own visible tile was ignored")
	}
}

func TestDemoTileMapDoorOrderAndAmbushClear(t *testing.T) {
	g := demoTileMapTestGame()
	setLevelTile(g.level, 2, 2, wl6.Tile{RawWall: 90, Solid: true, Door: &wl6.Door{Vertical: true}})
	setLevelTile(g.level, 3, 2, wl6.Tile{RawWall: 91, Solid: true, Door: &wl6.Door{}})
	setLevelTile(g.level, 2, 3, wl6.Tile{RawWall: 106, Ambush: true})
	g.initializeDemoTileMap()
	// The later horizontal door marks its already-spawned neighbor. Ambush
	// removal happens after every SpawnDoor, including its jamb marking.
	if g.demoTileMapAt(2, 2) != 0xc0 || g.demoTileMapAt(3, 2) != 0x81 || g.demoTileMapAt(2, 3) != 0 {
		t.Fatalf("row-major doors/ambush bytes = %x/%x/%x", g.demoTileMapAt(2, 2), g.demoTileMapAt(3, 2), g.demoTileMapAt(2, 3))
	}
}

func TestDemoTileMapTracksPushWallAndElevatorMutations(t *testing.T) {
	g := demoTileMapTestGame()
	setLevelTile(g.level, 2, 2, wl6.Tile{RawWall: 90, Solid: true, Door: &wl6.Door{Vertical: true}})
	setLevelTile(g.level, 2, 3, wl6.Tile{RawWall: 1, RawInfo: pushableTile, Solid: true})
	setLevelTile(g.level, 5, 2, wl6.Tile{RawWall: 21, Solid: true})
	g.initializeDemoTileMap()
	if !g.startPushWall(2, 3, 0, 1) {
		t.Fatal("could not start test pushwall")
	}
	if g.demoTileMapAt(2, 3) != 0xc1 || g.demoTileMapAt(2, 4) != 0x41 {
		t.Fatalf("initial moving/leading bytes = %x/%x", g.demoTileMapAt(2, 3), g.demoTileMapAt(2, 4))
	}
	g.updateDemoPushWall(128)
	if g.demoTileMapAt(2, 3) != 0 || g.demoTileMapAt(2, 4) != 0xc1 || g.demoTileMapAt(2, 5) != 1 {
		t.Fatalf("crossing bytes = %x/%x/%x", g.demoTileMapAt(2, 3), g.demoTileMapAt(2, 4), g.demoTileMapAt(2, 5))
	}
	g.updateDemoPushWall(128)
	if g.demoTileMapAt(2, 4) != 0 || g.demoTileMapAt(2, 5) != 1 || g.pushWall.active {
		t.Fatal("finished wall retained a moving marker or blocked its old tile")
	}
	tile := g.level.Tile(5, 2)
	tile.RawWall++
	g.setLevelTile(5, 2, tile)
	if g.demoTileMapAt(5, 2) != 22 {
		t.Fatal("elevator switch did not update its original tilemap byte")
	}
}
