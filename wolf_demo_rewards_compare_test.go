package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Built-in demo routes do not exercise every pickup at inventory limits or
// score thresholds. Compare those boundaries directly with original GetBonus
// and GivePoints, without sharing the port's resulting state with the oracle.
func TestWolfDemoRewardsCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_runtime_compare.sh")
	}
	p := startWolfSourceBinary(t, path, "--bonus-probe")
	type inventory struct {
		name                                    string
		health, ammo, score, nextExtra, lives   int
		best, chosen, weapon, keys, attackFrame int
		attacking                               bool
	}
	states := []inventory{
		{"new game", 100, 8, 0, 40000, 3, 1, 1, 1, 0, 0, false},
		{"empty", 0, 0, 39999, 40000, 3, 1, 1, 0, 0, 0, false},
		{"gibs boundary", 10, 98, 39900, 40000, 8, 2, 2, 2, 1, 0, false},
		{"gibs rejected", 11, 1, 79500, 80000, 9, 3, 1, 1, 15, 0, false},
		{"capped", 100, 99, 39999, 40000, 9, 3, 3, 3, 15, 0, false},
		{"knife attack first frame", 99, 0, 39000, 40000, 3, 3, 3, 0, 2, 0, true},
		{"knife attack later frame", 1, 0, 35000, 40000, 3, 2, 2, 0, 4, 1, true},
		{"gun attack", 75, 94, 79900, 80000, 8, 3, 2, 2, 8, 2, true},
	}
	actions := []struct {
		name   string
		pickup pickupType
		points int
	}{
		{"cross", pickupCross, 0}, {"chalice", pickupChalice, 0},
		{"bible", pickupBible, 0}, {"crown", pickupCrown, 0},
		{"fullheal", pickupFullHeal, 0}, {"food", pickupFood, 0},
		{"medkit", pickupFirstAid, 0}, {"alpo", pickupAlpo, 0},
		{"gibs", pickupGibs, 0}, {"ammo", pickupClip, 0},
		{"ammo2", pickupClip2, 0}, {"key1", pickupKey1, 0},
		{"key2", pickupKey2, 0}, {"key3", pickupKey3, 0},
		{"key4", pickupKey4, 0}, {"machinegun", pickupMachineGun, 0},
		{"chaingun", pickupChaingun, 0},
		{"points", pickupNone, 1}, {"points", pickupNone, 120000},
	}
	for _, state := range states {
		for _, action := range actions {
			t.Run(fmt.Sprintf("%s/%s_%d", state.name, action.name, action.points), func(t *testing.T) {
				g := &game{
					health: state.health, ammo: state.ammo, score: state.score,
					nextExtra: state.nextExtra, lives: state.lives, bestWeapon: state.best,
					chosenWeapon: state.chosen, weapon: state.weapon, keys: byte(state.keys),
					attacking: state.attacking, weaponFrameIdx: state.attackFrame,
					demoPlayback: &wolfDemoPlayback{faceCount: 23},
				}
				if _, err := fmt.Fprintf(p.in, "%s %d %d %d %d %d %d %d %d %d %d %d\n",
					action.name, state.health, state.ammo, state.score, state.nextExtra,
					state.lives, state.best, state.chosen, state.weapon, state.keys,
					state.attackFrame, action.points); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				if !p.out.Scan() {
					t.Fatalf("original pickup reference returned no state: %v", p.out.Err())
				}
				var want struct {
					Stats     wolfDemoRuntimeStats `json:"stats"`
					Weapon    int                  `json:"weapon"`
					FaceTimer int                  `json:"face_timer"`
					Collected bool                 `json:"collected"`
				}
				if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
					t.Fatal(err)
				}
				collected := false
				if action.name == "points" {
					g.givePoints(action.points)
				} else {
					collected = g.applyPickup(action.pickup)
				}
				if got := captureDemoRuntimeStats(g); got != want.Stats || g.weapon != want.Weapon || g.demoPlayback.faceCount != want.FaceTimer || collected != want.Collected {
					t.Fatalf("original=%+v port stats=%+v weapon=%d face_timer=%d collected=%v", want, got, g.weapon, g.demoPlayback.faceCount, collected)
				}
			})
		}
	}
	t.Logf("matched %d original-C pickup and score boundary cases", len(states)*len(actions))
}
