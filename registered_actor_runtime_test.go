package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

func registeredActorRoom(info uint16) *wl6.MapData {
	data := &wl6.MapData{Header: wl6.MapHeader{Width: 64, Height: 64}, Planes: [2][]uint16{make([]uint16, 4096), make([]uint16, 4096)}}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			wall := uint16(107)
			if x == 0 || y == 0 || x == 63 || y == 63 {
				wall = 1
			}
			data.Planes[0][y*64+x] = wall
		}
	}
	data.Planes[1][registeredActorPlayerY(info)*64+32] = 20
	data.Planes[1][32*64+32] = info
	return data
}

func registeredActorPlayerY(info uint16) int {
	if info == 196 || info == 179 || info == 178 {
		return 40 // Face the southern ambush spawns.
	}
	return 24
}

func TestWolfRegisteredActorsRuntime(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("set GDWOLF_ENEMY_RUNTIME_REFERENCE to the compiled original runtime")
	}
	t.Setenv("GDWOLF_DEMO_REGISTERED", "1")
	t.Setenv("GDWOLF_DEMO_MAP_INDEX", "0")
	mode := os.Getenv("GDWOLF_ENEMY_SOUND_MODE")
	if mode == "" {
		mode = "off"
	}
	var files *wl6.Files
	if mode != "off" {
		var err error
		files, err = wl6.Open(os.Getenv("GDWOLF_ENEMY_RUNTIME_DATA"))
		if err != nil {
			t.Fatal(err)
		}
		configureWolfDemoReferenceSound(t, files, mode, 0, t.TempDir())
	} else {
		t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	}
	out := os.Getenv("GDWOLF_ENEMY_RUNTIME_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for _, enemy := range []struct {
		name string
		info uint16
	}{{"schabbs", 196}, {"gift", 215}, {"fat", 179}, {"fake", 160}, {"mecha", 178}, {"blinky", 224}, {"clyde", 225}, {"pinky", 226}, {"inky", 227}} {
		for _, kill := range []bool{false, true} {
			if kill && enemy.info >= 224 {
				continue // Original ghosts have no FL_SHOOTABLE flag.
			}
			name := fmt.Sprintf("%s/kill_%v", enemy.name, kill)
			t.Run(name, func(t *testing.T) {
				data := registeredActorRoom(enemy.info)
				playerY := registeredActorPlayerY(enemy.info)
				g := testGameWithLevel(data.Level())
				g.startNewGame()
				g.files = files
				g.difficulty = difficultyHard
				g.playerX, g.playerY = 32.5, float64(playerY)+0.5
				g.rng = newWolfRNG(0)
				g.actors = g.buildActors()
				sound, err := newWolfDemoSound(files, mode)
				if err != nil {
					t.Fatal(err)
				}
				g.demoPlayback = &wolfDemoPlayback{sound: sound}
				g.initializeDemoActorTimers()
				g.rebuildPlayerAreas()
				p := startWolfSourceBinary(t, path, "--enemy-damage-probe")
				if err := writeDemoStartMap(p.in, data); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				read := func() wolfDemoRuntimeState {
					if !p.out.Scan() {
						t.Fatalf("original %s stopped: %v", name, p.out.Err())
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
				trace, err := os.Create(filepath.Join(out, fmt.Sprintf("%s-kill-%v.jsonl", enemy.name, kill)))
				if err != nil {
					t.Fatal(err)
				}
				defer trace.Close()
				encode := json.NewEncoder(trace)
				moved, damaged := false, false
				projectileKinds := map[ActorKind]bool{}
				for step := 0; step < 240; step++ {
					target, damage := -1, 0
					if kill && step == 0 {
						target, damage = 0, 5000
					}
					if kill && enemy.info == 178 && step == 12 {
						target, damage = 1, 5000
					}
					if _, err := fmt.Fprintf(p.in, "%d %d 0 4 %d %d\n", 32*65536+32768, playerY*65536+32768, target, damage); err != nil {
						t.Fatal(err)
					}
					if err := p.in.Flush(); err != nil {
						t.Fatal(err)
					}
					g.demoPlayback.sound.advance(4)
					g.updateDoors(4)
					g.playerX, g.playerY = 32.5, float64(playerY)+0.5
					g.rebuildPlayerAreas()
					if target >= 0 {
						g.damageActor(&g.actors[target], damage)
					}
					beforeHealth := g.health
					g.updateDemoActors(4)
					want, got := read(), captureDemoRuntimeState(g)
					got.Damage = beforeHealth - g.health
					moved = moved || g.actors[0].x != 32.5 || g.actors[0].y != 32.5
					damaged = damaged || got.Damage > 0
					for _, a := range g.actors {
						if isTransientActorKind(a.kind) {
							projectileKinds[a.kind] = true
						}
					}
					if err := encode.Encode(enemyRuntimeStep{Step: step, Tics: 4, Original: want, Port: got}); err != nil {
						t.Fatal(err)
					}
					compareEnemyRuntimeStep(t, step, beforeHealth, want, got)
					if want.Sound != got.Sound {
						t.Fatalf("step%d sound: original=%+v port=%+v", step, want.Sound, got.Sound)
					}
					if want.Stats != got.Stats || want.Terminal != got.Terminal || want.Victory.Active != got.Victory.Active {
						t.Fatalf("step%d stats/terminal/victory: original=%+v/%d/%v port=%+v/%d/%v", step, want.Stats, want.Terminal, want.Victory.Active, got.Stats, got.Terminal, got.Victory.Active)
					}
					if g.playerDying || g.demoPlayback.levelExit != 0 {
						break
					}
				}
				if !kill && (!moved || !damaged) {
					t.Fatalf("encounter did not exercise movement and combat: moved=%v damaged=%v", moved, damaged)
				}
				if !kill {
					projectile := map[uint16]ActorKind{196: actorKindNeedle, 215: actorKindRocket, 179: actorKindRocket, 160: actorKindFire}
					if kind, ok := projectile[enemy.info]; ok && !projectileKinds[kind] {
						t.Fatalf("encounter did not exercise projectile kind%d", kind)
					}
				}
			})
		}
	}
}
