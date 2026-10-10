package main

import (
	"fmt"
	"math"
	"testing"

	"gd-wolf/internal/wl6"

	"github.com/hajimehoshi/ebiten/v2"
)

func benchmarkGPUWallGame(width int, angle float64) *game {
	g := alphaRenderTestGame()
	g.layout.bufferWidth, g.layout.bufferHeight = width, width*3/4
	g.playerA = angle
	g.wallColumns = make([]wallColumn, width)
	g.cameraColumns = make([]float64, width)
	g.rayDirXColumns = make([]float64, width)
	g.rayDirYColumns = make([]float64, width)
	g.zbuffer = make([]float64, width)
	for x := range g.cameraColumns {
		g.cameraColumns[x] = 2*(float64(x)+0.5)/float64(width) - 1
	}
	wall := wl6.WallTexture{Width: 64, Height: 64, Pixels: make([]uint32, 64*64)}
	g.walls.Horizontal[1], g.walls.Vertical[1] = wall, wall
	g.textureCache = map[*wl6.WallTexture]*ebiten.Image{
		&g.walls.Horizontal[1]: new(ebiten.Image),
		&g.walls.Vertical[1]:   new(ebiten.Image),
	}
	forwardX, forwardY := math.Cos(angle), math.Sin(angle)
	planeScale := math.Tan(fov / 2)
	planeX, planeY := -forwardY*planeScale, forwardX*planeScale
	g.prepareRaycastDirections(0, width, forwardX, forwardY, planeX, planeY)
	g.renderRaycastColumns(true, 0, width, forwardX, forwardY, planeX, planeY, float64(width)/(2*planeScale))
	return g
}

func BenchmarkGPUWallCommands(b *testing.B) {
	for _, width := range []int{320, 1280, 1920} {
		for _, scene := range []struct {
			name  string
			angle float64
		}{{"front", 0}, {"angled", 0.37}} {
			b.Run(fmt.Sprintf("%s/%d", scene.name, width), func(b *testing.B) {
				g := benchmarkGPUWallGame(width, scene.angle)
				var renderer wolfGPURenderer
				g.drawWallColumnsGPU(&renderer)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					renderer.commands.reset()
					g.drawWallColumnsGPU(&renderer)
				}
			})
		}
	}
}

func BenchmarkGPUCommandRect(b *testing.B) {
	texture := new(ebiten.Image)
	var commands wolfGPUCommands
	for x := range 1920 {
		commands.rect(texture, x, 50, x, 800, uint32((x%64)<<16), 12345, 0, 4321)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		commands.reset()
		for x := range 1920 {
			commands.rect(texture, x, 50, x, 800, uint32((x%64)<<16), 12345, 0, 4321)
		}
	}
}
