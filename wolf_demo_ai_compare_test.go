package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"gd-wolf/internal/wl6"
)

type wolfDemoAIActor struct {
	Kind     int   `json:"kind"`
	X        int64 `json:"x"`
	Y        int64 `json:"y"`
	TileX    int   `json:"tile_x"`
	TileY    int   `json:"tile_y"`
	Dir      int   `json:"dir"`
	Area     int   `json:"area"`
	Distance int64 `json:"distance"`
	Reaction int   `json:"reaction"`
	Health   int   `json:"health"`
	Flags    int   `json:"flags"`
	Shape    int   `json:"shape"`
	TicCount int   `json:"tic_count"`
}

type wolfDemoAIState struct {
	RNGIndex int               `json:"rng_index"`
	Damage   int               `json:"damage"`
	Actors   []wolfDemoAIActor `json:"actors"`
}

func captureDemoAI(g *game) wolfDemoAIState {
	s := wolfDemoAIState{RNGIndex: int(g.rng.index)}
	for _, a := range g.actors {
		flags := boolInt(a.shootable) | boolInt(a.alerted)*16 | boolInt(a.firstAttack)*32 | boolInt(a.ambush)*64
		seq, _ := LookupAnimSequence(a.sequenceID)
		count := seq.Frames[a.frameIndex].Tics - a.frameTimer
		if a.spawnAnimationFrozen {
			count = 0
		}
		distance := int64(math.Round(a.moveDistance * 65536))
		if a.moveDistance < 0 {
			index := 0
			for tileIndex, tile := range g.level.Tiles {
				if tile.Door == nil {
					continue
				}
				if tileIndex == a.tileY*g.levelWidth+a.tileX {
					distance = -int64(index + 1)
					break
				}
				index++
			}
		}
		s.Actors = append(s.Actors, wolfDemoAIActor{int(a.kind), int64(math.Round(a.x * 65536)), int64(math.Round(a.y * 65536)), a.tileX, a.tileY, a.dir, a.area, distance, a.reactionTimer, a.health, flags, a.shapenum, count})
	}
	return s
}

func TestWolfDemoAICompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_AI_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_ai_compare.sh")
	}
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	demo, err := files.LoadDemo(0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := files.LoadMap(demo.Map)
	if err != nil {
		t.Fatal(err)
	}
	g, err := buildEnemyAIFuzzBaseline(files, demo.Map)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.startDemo(demo); err != nil {
		t.Fatal(err)
	}
	p := startWolfSourceBinary(t, path)
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	read := func() wolfDemoAIState {
		if !p.out.Scan() {
			t.Fatal("original actor reference returned no state")
		}
		var s wolfDemoAIState
		if err := json.Unmarshal(p.out.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	compare := func(command int, want, got wolfDemoAIState) {
		if want.RNGIndex != got.RNGIndex {
			t.Errorf("command %d RNG: original=%d port=%d", command, want.RNGIndex, got.RNGIndex)
		}
		if len(want.Actors) != len(got.Actors) {
			t.Fatalf("actor count differs: original=%d port=%d", len(want.Actors), len(got.Actors))
		}
		for i := range want.Actors {
			if want.Actors[i] != got.Actors[i] {
				t.Errorf("command %d actor %d: original=%+v port=%+v", command, i, want.Actors[i], got.Actors[i])
			}
		}
		if t.Failed() {
			t.FailNow()
		}
	}
	compare(-1, read(), captureDemoAI(g))
	for i, command := range demo.Commands {
		if g.playerDying || g.victoryActive {
			t.Fatalf("port stopped before command %d", i)
		}
		wasAttacking := g.prepareDemoCommand(command)
		g.moveDemoPlayer(command)
		if wasAttacking {
			g.updateWeaponAttackWithInput(wl6.DemoTics, command.Buttons&demoButtonAttack != 0)
		}
		g.rebuildPlayerAreas()
		if _, err := fmt.Fprintf(p.in, "%d %d %d %d %d %d", int64(math.Round(g.playerX*65536)), int64(math.Round(g.playerY*65536)), g.rng.index, boolInt(g.madeNoise), boolInt(g.playerMovingFast), g.bestWeapon); err != nil {
			t.Fatal(err)
		}
		for area := 0; area < 64; area++ {
			connected := area < len(g.playerAreas) && g.playerAreas[area]
			if _, err := fmt.Fprintf(p.in, " %d", boolInt(connected)); err != nil {
				t.Fatal(err)
			}
		}
		for tileIndex, tile := range g.level.Tiles {
			if tile.Door == nil {
				continue
			}
			action := []int{0, 2, 1, 3}[g.doorState[tileIndex]]
			if _, err := fmt.Fprintf(p.in, " %d %d", action, int(math.Round(g.doorOpen[tileIndex]*65535))); err != nil {
				t.Fatal(err)
			}
		}
		for index := range g.actors {
			damage := -1
			for _, hit := range g.demoPlayback.hits {
				if hit.index == index {
					damage = hit.damage
				}
			}
			if _, err := fmt.Fprintf(p.in, " %d %d", damage, boolInt(g.actorVisibleToPlayer(&g.actors[index]))); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := fmt.Fprintln(p.in); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		want := read()
		beforeHealth := g.health
		g.updateDemoActors(wl6.DemoTics)
		got := captureDemoAI(g)
		compare(i, want, got)
		if gotDamage := beforeHealth - g.health; gotDamage != minInt(beforeHealth, want.Damage) {
			t.Fatalf("command %d enemy damage: original=%d port=%d", i, want.Damage, gotDamage)
		}
		g.collectPickups()
		g.refreshDemoActorProjections()
		g.demoPlayback.buttons = command.Buttons
		g.demoPlayback.command++
	}
	t.Log("matched every first-demo actor update with original C, sharing external player/door/visibility/damage inputs")
}
