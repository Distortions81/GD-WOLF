package main

import (
	"testing"

	"gd-wolf/internal/wl6"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGPUWallCommandsTextureRuns(t *testing.T) {
	g := benchmarkGPUWallGame(8, 0)
	g.walls.AllPages = []wl6.WallTexture{{Width: 1, Height: 1, Pixels: []uint32{0xffffffff}}}
	override := new(ebiten.Image)
	g.textureCache[&g.walls.AllPages[0]] = override
	for i := range g.wallColumns {
		g.wallColumns[i] = wallColumn{hit: true, wallID: 1, texOverride: -1, origTop: 0, origBottom: 8, drawTop: 0, drawBottom: 8}
	}
	g.wallColumns[2].side = 1
	g.wallColumns[3].texOverride = 0
	g.wallColumns[4].wallID = 2 // A missing asset must not reuse the preceding image.
	g.wallColumns[5].wallID = 2
	var renderer wolfGPURenderer
	g.drawWallColumnsGPU(&renderer)
	wantImages := []*ebiten.Image{
		g.textureCache[&g.walls.Horizontal[1]],
		g.textureCache[&g.walls.Vertical[1]],
		override,
		g.textureCache[&g.walls.Horizontal[1]],
	}
	wantStarts := []float32{0, 2, 3, 6}
	wantVertices := []int{8, 4, 4, 8}
	if renderer.commands.used != len(wantImages) {
		t.Fatalf("texture batches = %d, want %d", renderer.commands.used, len(wantImages))
	}
	for i, want := range wantImages {
		batch := &renderer.commands.batches[i]
		if batch.image != want || len(batch.vertices) != wantVertices[i] || batch.vertices[0].DstX != wantStarts[i] {
			t.Fatalf("batch %d did not preserve texture selection or skipped-column geometry", i)
		}
	}
}

func TestGPUCommandRectBatchBoundary(t *testing.T) {
	texture := new(ebiten.Image)
	var commands wolfGPUCommands
	const rectCount = 65532/4 + 1
	for x := range rectCount {
		commands.rect(texture, x, 7, x+2, 11, 0x12345678, 0x87654321, 0x00014321, 0x00029876)
	}
	if commands.used != 2 {
		t.Fatalf("batches = %d, want 2", commands.used)
	}
	for i := 0; i < commands.used; i++ {
		batch := &commands.batches[i]
		for _, index := range batch.indices {
			if int(index) >= len(batch.vertices) {
				t.Fatalf("batch %d has out-of-range vertex index %d", i, index)
			}
		}
		for j := 0; j < len(batch.vertices); j += 4 {
			quad := batch.vertices[j : j+4]
			if quad[1].DstX != quad[0].DstX+3 || quad[2].DstY != quad[0].DstY+5 || quad[3].DstX != quad[1].DstX || quad[3].DstY != quad[2].DstY {
				t.Fatalf("batch %d quad %d has incorrect corners", i, j/4)
			}
			for _, vertex := range quad {
				if vertex.SrcX != quad[0].DstX || vertex.SrcY != 7 || vertex.ColorR != 0x1234 || vertex.ColorG != 0x5678 || vertex.Custom3 != 0x9876 {
					t.Fatalf("batch %d quad %d lost texture sampling attributes", i, j/4)
				}
			}
		}
	}
}
