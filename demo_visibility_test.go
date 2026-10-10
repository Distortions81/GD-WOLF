package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func TestDemoVisibilityRefreshClearsOldViewAndPreservesProbeMask(t *testing.T) {
	g := demoTileMapTestGame()
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			if x == 0 || y == 0 || x == g.levelWidth-1 || y == g.levelHeight-1 {
				setLevelTile(g.level, x, y, wl6.Tile{RawWall: 1, Solid: true})
			}
		}
	}
	g.playerX, g.playerY = 3.5, 3.5
	g.refreshDemoActorProjections()
	east, west := 3*g.levelWidth+5, 3*g.levelWidth+1
	if !g.demoPlayback.visibleTiles[east] || g.demoPlayback.visibleTiles[west] {
		t.Fatal("initial view did not face east")
	}

	probeMask := make([]bool, len(g.level.Tiles))
	for i := range probeMask {
		probeMask[i] = true
	}
	g.refreshDemoActorProjectionsWithVisibility(probeMask)
	g.demoPlayback.angle = 180
	g.refreshDemoActorProjections()
	if g.demoPlayback.visibleTiles[east] || !g.demoPlayback.visibleTiles[west] {
		t.Fatal("turning west retained visibility from the east-facing refresh")
	}
	for i, visible := range probeMask {
		if !visible {
			t.Fatalf("normal refresh modified supplied visibility at tile %d", i)
		}
	}
}
