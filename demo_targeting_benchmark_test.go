package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func BenchmarkDemoActorProjections(b *testing.B) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		b.Fatal(err)
	}
	demo, err := files.LoadDemo(0)
	if err != nil {
		b.Fatal(err)
	}
	g, err := buildEnemyAIFuzzBaseline(files, demo.Map)
	if err != nil {
		b.Fatal(err)
	}
	if err := g.startDemo(demo); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		g.refreshDemoActorProjections()
		g.demoPlayback.angle = (g.demoPlayback.angle + 1) % 360
	}
}

func BenchmarkActorPathBlocked(b *testing.B) {
	g := testGameWithLevel(blankLevel(64, 64))
	a := actorInstance{}
	for _, tc := range []struct {
		name string
		x, y float64
	}{
		{"WithinTile", 2.75, 2.75},
		{"CrossTile", 3.25, 3.25},
		{"LongPath", 60.25, 55.25},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				g.actorPathBlocked(&a, 2.5, 2.5, tc.x, tc.y)
			}
		})
	}
}
