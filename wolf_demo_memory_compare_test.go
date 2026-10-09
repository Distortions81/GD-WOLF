package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"gd-wolf/internal/wl6"
)

// A moving texture8 wall addresses false door72. Its action word aliases
// static10's shape; shape3 is dr_closing, so Use writes dr_opening (shape2).
// The next Use attempts CloseDoor and returns at the occupied (0,0) tile.
// The same write and early-return paths have separate executable captures.
func TestWolfDemoDOSAliasedDoorCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	t.Setenv("GDWOLF_DEMO_REGISTERED", "1")
	t.Setenv("GDWOLF_DEMO_MAP_INDEX", "0")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	t.Setenv("GDWOLF_DEMO_MEMORY_PROFILE", wolfDemoRegisteredMemoryProfile)
	t.Setenv("GDWOLF_DEMO_OCCUPANCY", "1")
	t.Setenv("GDWOLF_DEMO_AREA_PLANE", "1")
	t.Setenv("GDWOLF_DEMO_STATICS", "1")
	data := victoryProbeMap()
	for x := 10; x < 22; x++ {
		data.Planes[1][10*64+x] = 24 // Blocking barrel, shape3.
	}
	data.Planes[0][32*64+33] = 8
	data.Planes[1][32*64+33] = pushableTile
	p := startWolfSourceBinary(t, path, "--victory-probe")
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(p.in, "0 0 0")
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	g := testGameWithLevel(data.Level())
	g.startNewGame()
	g.staticSprites, g.treasureTotal = g.buildStaticSprites()
	g.secretTotal = g.countSecretWalls()
	g.rng = newWolfRNG(0)
	g.playerX, g.playerY, g.playerA = 32.5, 32.5, 0
	g.demoPlayback = &wolfDemoPlayback{}
	if err := g.configureDemoMemory(wolfDemoRegisteredMemoryProfile); err != nil {
		t.Fatal(err)
	}
	g.initializeDemoActorTimers()
	g.initializeDemoActorPool()
	g.playerAreas = g.computePlayerAreas()
	g.rebuildPlayerAreas()
	visible := make([]bool, 4096)
	for i := range visible {
		visible[i] = true
	}
	check := func(step int) {
		t.Helper()
		if !p.out.Scan() {
			t.Fatalf("original DOS alias fixture stopped at %d: %v", step, p.out.Err())
		}
		var want wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		got := captureDemoRuntimeState(g)
		compareEnemyRuntimeStep(t, step, 100, want, got)
		compareDemoStatics(t, step, want.Statics, got.Statics)
		if want.Player != got.Player || want.PushWall != got.PushWall || want.Stats != got.Stats || !reflect.DeepEqual(want.Victory, got.Victory) {
			t.Fatalf("step %d player/pushwall/stats mismatch", step)
		}
	}
	check(-1)
	for step := 0; step < 7; step++ {
		command := wl6.DemoCommand{}
		if step%2 == 0 {
			command.Buttons = demoButtonUse
		}
		fmt.Fprintf(p.in, "%d 0 0\n", command.Buttons)
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		g.prepareDemoCommand(command)
		g.moveDemoPlayer(command)
		g.updateDemoActors(wl6.DemoTics)
		g.refreshDemoActorProjectionsWithVisibility(visible)
		g.collectPickups()
		g.demoPlayback.timeCount += wl6.DemoTics
		g.demoPlayback.buttons = g.demoPlayback.inputButtons
		check(step)
	}
	if g.staticSprites[10].shapenum != 2 || !g.pushWall.active {
		t.Fatal("fixture did not exercise the aliased shape write")
	}
}
