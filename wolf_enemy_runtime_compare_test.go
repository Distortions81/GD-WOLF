package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

type enemyRuntimeStep struct {
	Map      int                  `json:"map"`
	Actor    int                  `json:"actor"`
	Step     int                  `json:"step"`
	X        int64                `json:"x"`
	Y        int64                `json:"y"`
	Noise    int                  `json:"noise"`
	Tics     int                  `json:"tics"`
	Original wolfDemoRuntimeState `json:"original"`
	Port     wolfDemoRuntimeState `json:"port"`
}

func TestWolfEnemyRuntimeCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	dataDir := os.Getenv("GDWOLF_ENEMY_RUNTIME_DATA")
	var files *wl6.Files
	var err error
	if dataDir == "" {
		files, err = wl6.OpenEmbeddedShareware()
	} else {
		files, err = wl6.Open(dataDir)
	}
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("GDWOLF_ENEMY_RUNTIME_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	seen := map[ActorKind]bool{}
	count := 0
	for mapIndex := 0; mapIndex < files.Variant.EpisodeCount*10 && len(seen) < 13; mapIndex++ {
		data, err := files.LoadMap(mapIndex)
		if err != nil {
			continue
		}
		g, err := buildEnemyAIFuzzBaseline(files, mapIndex)
		if err != nil {
			t.Fatal(err)
		}
		if err := g.startDemo(&wl6.Demo{Map: mapIndex}); err != nil {
			t.Fatal(err)
		}
		for i := range g.actors {
			a := &g.actors[i]
			if seen[a.kind] || !a.alive || !isEnemyActorKind(a.kind) {
				continue
			}
			positions := fuzzNearbyPlayerPositions(g, a)
			if len(positions) == 0 {
				continue
			}
			seen[a.kind] = true
			count++
			t.Run(fmt.Sprintf("map_%02d_actor_%03d_kind_%d", mapIndex, i, a.kind), func(t *testing.T) {
				runEnemyRuntimeEncounter(t, path, out, data, files, mapIndex, i, positions)
			})
		}
	}
	if count == 0 {
		t.Fatal("no supported encounters found")
	}
	if files.Variant.EpisodeCount > 1 && count != 13 {
		t.Fatalf("registered data covered %d map-spawned enemy kinds, want 13", count)
	}
	if seen[actorKindGretel] {
		t.Run("gretel_open_room", func(t *testing.T) {
			runGretelOpenRoomRuntime(t, path, out)
		})
	}
	t.Logf("compared %d enemy kinds", count)
}

func runEnemyRuntimeEncounter(t *testing.T, path, out string, data *wl6.MapData, files *wl6.Files, mapIndex, actorIndex int, positions [][2]float64) {
	t.Helper()
	g, err := buildEnemyAIFuzzBaseline(files, mapIndex)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.startDemo(&wl6.Demo{Map: mapIndex}); err != nil {
		t.Fatal(err)
	}
	p := startWolfSourceBinary(t, path, "--enemy-probe")
	tracePath := filepath.Join(out, fmt.Sprintf("map-%02d-actor-%03d.jsonl", mapIndex, actorIndex))
	trace, err := os.Create(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := trace.Close(); err != nil {
			t.Error(err)
		}
	})
	encode := json.NewEncoder(trace)
	read := func() wolfDemoRuntimeState {
		if !p.out.Scan() {
			t.Fatalf("original source returned no state: %v", p.out.Err())
		}
		var s wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	want := read()
	got := captureDemoRuntimeState(g)
	if len(want.Actors) != len(got.Actors) {
		t.Fatalf("actor count: original=%d port=%d", len(want.Actors), len(got.Actors))
	}
	if actorIndex >= len(want.Actors) {
		t.Fatal("target index exceeds original actor count")
	}
	if err := encode.Encode(enemyRuntimeStep{Map: mapIndex, Actor: actorIndex, Step: -1, Original: want, Port: got}); err != nil {
		t.Fatal(err)
	}
	compareEnemyRuntimeStep(t, -1, 100, want, got)
	if _, err := fmt.Fprintln(p.in, actorIndex); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	g.actors[actorIndex].demoActive = true
	// Keep the player near the selected enemy, first stationary and then at
	// alternating nearby positions. This drives sight, chase and attacks.
	for step := 0; step < 80; step++ {
		position := positions[len(positions)/2]
		if step >= 24 {
			position = positions[(step/8)%len(positions)]
		}
		x, y := int64(math.Round(position[0]*65536)), int64(math.Round(position[1]*65536))
		noise, tics := 0, 4
		if step%13 == 0 {
			noise = 1
		}
		if step%7 == 0 {
			tics = 7
		}
		if _, err := fmt.Fprintf(p.in, "%d %d %d %d\n", x, y, noise, tics); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		g.updateDoors(tics)
		g.playerX, g.playerY = position[0], position[1]
		g.madeNoise = noise != 0
		beforeHealth := g.health
		g.updateDemoActors(tics)
		want, got = read(), captureDemoRuntimeState(g)
		got.Damage = beforeHealth - g.health
		if err := encode.Encode(enemyRuntimeStep{mapIndex, actorIndex, step, x, y, noise, tics, want, got}); err != nil {
			t.Fatal(err)
		}
		compareEnemyRuntimeStep(t, step, beforeHealth, want, got)
		if g.playerDying {
			break
		}
	}
}

func compareEnemyRuntimeStep(t *testing.T, step, beforeHealth int, want, got wolfDemoRuntimeState) {
	t.Helper()
	compareDemoActorGrid(t, step, want.ActorAt, got.ActorAt)
	compareDemoAreaPlane(t, step, want.AreaPlane, got.AreaPlane)
	// Original TakeDamage records the full hit; the port's health stops at zero.
	if want.RNGIndex != got.RNGIndex || minInt(want.Damage, beforeHealth) != got.Damage {
		t.Fatalf("step %d RNG/damage: original=%d/%d port=%d/%d", step, want.RNGIndex, want.Damage, got.RNGIndex, got.Damage)
	}
	if len(want.Actors) != len(got.Actors) {
		t.Fatalf("step %d actor count: original=%d port=%d", step, len(want.Actors), len(got.Actors))
	}
	for i := range want.Actors {
		if want.Actors[i] != got.Actors[i] {
			t.Fatalf("step %d actor %d: original=%+v port=%+v", step, i, want.Actors[i], got.Actors[i])
		}
	}
	if len(want.Doors) != len(got.Doors) {
		t.Fatalf("step %d door count: original=%d port=%d", step, len(want.Doors), len(got.Doors))
	}
	for i := range want.Doors {
		if want.Doors[i] != got.Doors[i] {
			t.Fatalf("step %d door %d: original=%+v port=%+v", step, i, want.Doors[i], got.Doors[i])
		}
	}
}
