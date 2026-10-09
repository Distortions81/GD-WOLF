package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

// The registered-map encounter can kill the player before Gretel moves. This
// open room starts farther away, exercising chase motion before the burst.
func runGretelOpenRoomRuntime(t *testing.T, reference, out string) {
	t.Helper()
	data := &wl6.MapData{
		Header: wl6.MapHeader{Width: 64, Height: 64},
		Planes: [2][]uint16{make([]uint16, 4096), make([]uint16, 4096)},
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			wall := uint16(107)
			if x == 0 || y == 0 || x == 63 || y == 63 {
				wall = 1
			}
			data.Planes[0][y*64+x] = wall
		}
	}
	data.Planes[1][24*64+32] = 20 // SpawnPlayer facing east.
	data.Planes[1][32*64+32] = 197
	g := testGameWithLevel(data.Level())
	g.startNewGame()
	g.difficulty = difficultyHard
	g.playerX, g.playerY = 32.5, 24.5
	g.rng = newWolfRNG(0)
	g.actors = g.buildActors()
	g.demoPlayback = &wolfDemoPlayback{}
	g.initializeDemoActorTimers()
	g.rebuildPlayerAreas()
	p := startWolfSourceBinary(t, reference, "--enemy-probe")
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	read := func() wolfDemoRuntimeState {
		if !p.out.Scan() {
			t.Fatalf("original Gretel encounter returned no state: %v", p.out.Err())
		}
		var state wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	compareEnemyRuntimeStep(t, -1, 100, read(), captureDemoRuntimeState(g))
	if _, err := fmt.Fprintln(p.in, 0); err != nil {
		t.Fatal(err)
	}
	g.actors[0].demoActive = true
	trace, err := os.Create(filepath.Join(out, "gretel-open-room.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer trace.Close()
	encode := json.NewEncoder(trace)
	moved, shot := false, false
	for step := 0; step < 100; step++ {
		if _, err := fmt.Fprintf(p.in, "%d %d 0 4\n", 32*65536+32768, 24*65536+32768); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		beforeHealth := g.health
		g.updateDemoActors(4)
		want, got := read(), captureDemoRuntimeState(g)
		got.Damage = beforeHealth - g.health
		compareEnemyRuntimeStep(t, step, beforeHealth, want, got)
		if err := encode.Encode(enemyRuntimeStep{Step: step, Tics: 4, Original: want, Port: got}); err != nil {
			t.Fatal(err)
		}
		moved = moved || g.actors[0].x != 32.5 || g.actors[0].y != 32.5
		shot = shot || got.Damage > 0
		if g.playerDying {
			break
		}
	}
	if !moved || !shot {
		t.Fatalf("Gretel encounter coverage: moved=%v shot=%v", moved, shot)
	}
}
