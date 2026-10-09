package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Move to a new corner every forty commands so launched projectiles
// travel past the player into a wall. This exercises transient state removal,
// smoke, rocket explosions and free-list recycling without feeding Go state
// back to the original implementation.
func TestWolfProjectilesRuntimeCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	t.Setenv("GDWOLF_DEMO_REGISTERED", "1")
	t.Setenv("GDWOLF_DEMO_MAP_INDEX", "0")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	t.Setenv("GDWOLF_DEMO_OCCUPANCY", "1")
	for _, enemy := range []struct {
		name string
		info uint16
		kind ActorKind
	}{{"needle", 196, actorKindNeedle}, {"gift_rocket", 215, actorKindRocket}, {"fat_rocket", 179, actorKindRocket}, {"fire", 160, actorKindFire}} {
		t.Run(enemy.name, func(t *testing.T) {
			data := registeredActorRoom(enemy.info)
			g := testGameWithLevel(data.Level())
			g.startNewGame()
			g.difficulty = difficultyHard
			g.playerX, g.playerY = 32.5, float64(registeredActorPlayerY(enemy.info))+0.5
			g.rng = newWolfRNG(0)
			g.actors = g.buildActors()
			g.demoPlayback = &wolfDemoPlayback{}
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
					t.Fatalf("original projectile encounter stopped: %v", p.out.Err())
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
			seen, prior := map[int]ActorKind{}, map[int]ActorKind{}
			projectile, smoke, explosion, removed, reused := false, false, false, false, false
			for step := 0; step < 384; step++ {
				corner := [4][2]int{{10, 10}, {10, 53}, {53, 53}, {53, 10}}[(step/40)%4]
				fixedX, fixedY := corner[0]*65536+32768, corner[1]*65536+32768
				if _, err := fmt.Fprintf(p.in, "%d %d 0 4 -1 0\n", fixedX, fixedY); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				g.updateDoors(4)
				g.playerX, g.playerY = float64(corner[0])+0.5, float64(corner[1])+0.5
				g.madeNoise = false
				before := g.health
				g.updateDemoActors(4)
				want, got := read(), captureDemoRuntimeState(g)
				got.Damage = before - g.health
				compareEnemyRuntimeStep(t, step, before, want, got)
				current := map[int]ActorKind{}
				for _, a := range g.actors {
					if !isTransientActorKind(a.kind) {
						continue
					}
					projectile = projectile || a.kind == enemy.kind
					smoke = smoke || a.kind == actorKindSmoke
					explosion = explosion || (a.kind == actorKindRocket && a.aiState == actorStateEffect)
					if old, used := seen[a.poolSlot]; used {
						_, previous := prior[a.poolSlot]
						reused = reused || !previous || old != a.kind
					}
					seen[a.poolSlot], current[a.poolSlot] = a.kind, a.kind
				}
				for slot := range prior {
					if _, remains := current[slot]; !remains {
						removed = true
					}
				}
				prior = current
				if projectile && removed && reused && (enemy.kind != actorKindRocket || smoke && explosion) {
					t.Logf("matched %d commands including transient removal and slot reuse", step+1)
					break
				}
				if g.playerDying {
					t.Fatalf("player died at step%d: projectile=%v removed=%v reused=%v smoke=%v explosion=%v", step, projectile, removed, reused, smoke, explosion)
				}
			}
			if !projectile || !removed || !reused || (enemy.kind == actorKindRocket && (!smoke || !explosion)) {
				t.Fatalf("incomplete projectile coverage: projectile=%v removed=%v reused=%v smoke=%v explosion=%v", projectile, removed, reused, smoke, explosion)
			}
		})
	}
}
