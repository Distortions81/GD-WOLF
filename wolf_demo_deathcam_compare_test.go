package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"gd-wolf/internal/wl6"
)

// Exercise camera geometry independently of the player's recorded route.
// A nearby wall forces CheckPosition to reject the first camera candidate.
func TestWolfDemoDeathCamCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run with the compiled original enemy runtime reference")
	}
	t.Setenv("GDWOLF_DEMO_REGISTERED", "1")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"})
	defer setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"})
	for angle := 0; angle < 360; angle++ {
		for _, obstructed := range []bool{false, true} {
			t.Run(fmt.Sprintf("angle%d-wall%v", angle, obstructed), func(t *testing.T) {
				data := registeredActorRoom(196)
				cos, sin := math.Cos(float64(angle)*math.Pi/180), math.Sin(float64(angle)*math.Pi/180)
				px := 32*65536 + 32768 - int(math.Round(cos*6*65536))
				py := 32*65536 + 32768 + int(math.Round(sin*6*65536))
				if obstructed {
					x, y := int(32.5-cos*1.5), int(32.5+sin*1.5)
					data.Planes[0][y*64+x] = 1
				}
				g := testGameWithLevel(data.Level())
				g.startNewGame()
				g.difficulty = difficultyHard
				g.playerX, g.playerY = 32.5, 40.5
				g.rng = newWolfRNG(0)
				g.actors = g.buildActors()
				g.demoPlayback = &wolfDemoPlayback{}
				g.initializeDemoActorTimers()
				g.initializeDemoActorPool()
				g.rebuildPlayerAreas()
				p := startWolfSourceBinary(t, path, "--enemy-damage-probe")
				if err := writeDemoStartMap(p.in, data); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				read := func() wolfDemoRuntimeState {
					t.Helper()
					if !p.out.Scan() {
						t.Fatalf("death camera reference ended: %v", p.out.Err())
					}
					var s wolfDemoRuntimeState
					if err := json.Unmarshal(p.out.Bytes(), &s); err != nil {
						t.Fatal(err)
					}
					return s
				}
				compareEnemyRuntimeStep(t, -1, 100, read(), captureDemoRuntimeState(g))
				fmt.Fprintln(p.in, 0)
				g.actors[0].demoActive = true
				for step := 0; step < 8; step++ {
					target, damage := -1, 0
					if step == 0 {
						target, damage = 0, 5000
					}
					fmt.Fprintf(p.in, "%d %d 0 70 %d %d\n", px, py, target, damage)
					if err := p.in.Flush(); err != nil {
						t.Fatal(err)
					}
					g.playerX, g.playerY = float64(px)/65536, float64(py)/65536
					g.rebuildPlayerAreas()
					if damage > 0 {
						g.damageActor(&g.actors[0], damage)
					}
					g.updateDemoActors(70)
					want, got := read(), captureDemoRuntimeState(g)
					compareEnemyRuntimeStep(t, step, 100, want, got)
					if want.Player != got.Player || want.PlayerState != got.PlayerState || want.Victory.Active != got.Victory.Active || want.Terminal != got.Terminal {
						t.Fatalf("step%d camera: original=%+v/%d/%v/%d port=%+v/%d/%v/%d", step, want.Player, want.PlayerState, want.Victory.Active, want.Terminal, got.Player, got.PlayerState, got.Victory.Active, got.Terminal)
					}
					if g.demoPlayback.deathCam {
						if obstructed && math.Hypot(g.playerX-32.5, g.playerY-32.5) <= 1.26 {
							t.Fatal("wall did not exercise camera repositioning")
						}
						return
					}
				}
				t.Fatal("boss death never entered its camera state")
			})
			if t.Failed() {
				t.FailNow()
			}
		}
	}
}
