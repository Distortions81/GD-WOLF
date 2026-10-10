package main

import (
	"fmt"
	"math"
	"testing"

	"gd-wolf/internal/wl6"
)

func performanceMapGame(tb testing.TB, mapIndex int) *game {
	tb.Helper()
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		tb.Fatal(err)
	}
	data, err := files.LoadMap(mapIndex)
	if err != nil {
		tb.Fatal(err)
	}
	g := testGameWithLevel(data.Level())
	g.files, g.mapData = files, data
	g.resetPlayer()
	g.playerAreas = g.computePlayerAreas()
	return g
}

func BenchmarkPlayerAreaConnectivity(b *testing.B) {
	for _, mapIndex := range []int{0, 6, 9} {
		for _, demo := range []bool{false, true} {
			for _, open := range []bool{false, true} {
				b.Run(fmt.Sprintf("map%d/demo%v/open%v", mapIndex, demo, open), func(b *testing.B) {
					g := performanceMapGame(b, mapIndex)
					if demo {
						g.demoPlayback = &wolfDemoPlayback{}
						g.initializeDemoActorAreas()
					}
					if open {
						for i, tile := range g.level.Tiles {
							if tile.Door != nil {
								g.doorOpen[i], g.doorState[i] = 1, 2
							}
						}
					}
					g.rebuildPlayerAreas()
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						g.rebuildPlayerAreas()
					}
				})
			}
		}
	}
}

func BenchmarkWallRaycast(b *testing.B) {
	for _, width := range []int{320, 1280, 1920} {
		for _, workers := range []int{1, 2, 4, 0} {
			b.Run(fmt.Sprintf("width%d/workers%d", width, workers), func(b *testing.B) {
				g := performanceMapGame(b, 0)
				g.renderThreads = workers
				g.layout.bufferWidth, g.layout.bufferHeight = width, width*3/4
				g.cameraColumns = make([]float64, width)
				g.rayDirXColumns = make([]float64, width)
				g.rayDirYColumns = make([]float64, width)
				g.zbuffer = make([]float64, width)
				g.wallColumns = make([]wallColumn, width)
				for x := range g.cameraColumns {
					g.cameraColumns[x] = 2*(float64(x)+0.5)/float64(width) - 1
				}
				fx, fy := math.Cos(g.playerA), math.Sin(g.playerA)
				px, py := -fy*math.Tan(fov/2), fx*math.Tan(fov/2)
				projection := float64(width) / (2 * math.Tan(fov/2))
				g.prepareRaycastDirections(0, width, fx, fy, px, py)
				g.renderRaycastColumns(true, 0, width, fx, fy, px, py, projection)
				b.Cleanup(func() {
					if g.raycastJobs != nil {
						close(g.raycastJobs)
					}
				})
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					g.renderRaycastColumns(true, 0, width, fx, fy, px, py, projection)
				}
			})
		}
	}
}
