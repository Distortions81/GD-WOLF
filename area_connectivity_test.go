package main

import (
	"reflect"
	"testing"
)

// Keep the pre-optimization map-scan traversal as an independent ordering and
// connectivity reference. It deliberately does not use the new edge list.
func referencePlayerAreasByMapScan(g *game) []bool {
	areas := make([]bool, len(g.playerAreas))
	start := g.playerArea()
	if start < 0 || start >= len(areas) {
		return areas
	}
	areas[start] = true
	queue := []int{start}
	for len(queue) != 0 {
		area := queue[0]
		queue = queue[1:]
		for y := 0; y < g.levelHeight; y++ {
			for x := 0; x < g.levelWidth; x++ {
				door := g.doorDefinitionAt(x, y)
				if door == nil {
					continue
				}
				i := y*g.levelWidth + x
				if g.doorState[i] == 0 || (g.demoPlayback != nil && g.doorOpen[i] == 0) {
					continue
				}
				a, b, ok := g.doorAreas(x, y, door.Vertical)
				if !ok {
					continue
				}
				if a == area && b >= 0 && b < len(areas) && !areas[b] {
					areas[b] = true
					queue = append(queue, b)
				}
				if b == area && a >= 0 && a < len(areas) && !areas[a] {
					areas[a] = true
					queue = append(queue, a)
				}
			}
		}
	}
	return areas
}

func TestPlayerAreaConnectionsMatchMapScan(t *testing.T) {
	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		for _, demo := range []bool{false, true} {
			g := performanceMapGame(t, mapIndex)
			if demo {
				g.demoPlayback = &wolfDemoPlayback{}
				g.initializeDemoActorAreas()
			}
			// Reuse the same scratch storage while doors open and close, and
			// start from every floor area so disconnected components are tested.
			for pattern := 0; pattern < 7; pattern++ {
				for i, tile := range g.level.Tiles {
					if tile.Door == nil {
						continue
					}
					state := pattern
					if state >= 5 {
						state = (i*17 + pattern) % 5
					}
					g.doorState[i], g.doorOpen[i] = byte(state), 0
					switch state {
					case 2:
						g.doorOpen[i] = 1
					case 3:
						g.doorOpen[i] = 0.5
					case 4:
						g.doorState[i], g.doorOpen[i] = 1, 0.01
					}
				}
				seen := make(map[int]bool)
				for i, tile := range g.level.Tiles {
					if tile.Solid || tile.Door != nil {
						continue
					}
					g.playerX, g.playerY = float64(i%g.levelWidth)+0.5, float64(i/g.levelWidth)+0.5
					area := g.playerArea()
					if area < 0 || seen[area] {
						continue
					}
					seen[area] = true
					want := referencePlayerAreasByMapScan(g)
					g.rebuildPlayerAreas()
					if !reflect.DeepEqual(g.playerAreas, want) {
						t.Fatalf("map=%d demo=%v pattern=%d area=%d: got %v, want %v", mapIndex, demo, pattern, area, g.playerAreas, want)
					}
				}
			}
		}
	}
}
