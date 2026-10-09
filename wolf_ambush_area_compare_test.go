package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWolfAmbushAreaCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	t.Setenv("GDWOLF_DEMO_REGISTERED", "1")
	t.Setenv("GDWOLF_DEMO_MAP_INDEX", "0")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	t.Setenv("GDWOLF_DEMO_AREA_PLANE", "1")
	for _, info := range []uint16{108, 112, 196, 224} {
		t.Run(fmt.Sprintf("spawn_%d", info), func(t *testing.T) {
			data := registeredActorRoom(info)
			data.Planes[0][32*64+32] = 106
			for i, xy := range [][2]int{{31, 32}, {33, 32}, {32, 31}, {32, 33}} {
				data.Planes[0][xy[1]*64+xy[0]] = uint16(110 + i)
			}
			g := testGameWithLevel(data.Level())
			g.startNewGame()
			g.difficulty = difficultyHard
			g.playerX, g.playerY = 32.5, float64(registeredActorPlayerY(info))+0.5
			g.rng = newWolfRNG(0)
			g.actors = g.buildActors()
			g.demoPlayback = &wolfDemoPlayback{}
			g.initializeDemoActorTimers()
			g.rebuildPlayerAreas()
			p := startWolfSourceBinary(t, path, "--enemy-damage-probe")
			if err := writeDemoStartMap(p.in, data); err != nil {
				t.Fatal(err)
			}
			if _, err := fmt.Fprintln(p.in, 0); err != nil {
				t.Fatal(err)
			}
			if err := p.in.Flush(); err != nil {
				t.Fatal(err)
			}
			if !p.out.Scan() {
				t.Fatalf("original ambush fixture returned no state: %v", p.out.Err())
			}
			var want wolfDemoRuntimeState
			if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
				t.Fatal(err)
			}
			compareEnemyRuntimeStep(t, -1, 100, want, captureDemoRuntimeState(g))
			if area := g.actorAreaAt(32, 32); area != 3 {
				t.Fatalf("final ambush floor area=%d, want3", area)
			}
		})
	}
}
