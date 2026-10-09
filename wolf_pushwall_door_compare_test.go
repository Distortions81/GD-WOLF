package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestWolfPushWallIntoOpenDoorCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	t.Setenv("GDWOLF_DEMO_REGISTERED", "1")
	t.Setenv("GDWOLF_DEMO_MAP_INDEX", "0")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	t.Setenv("GDWOLF_DEMO_OCCUPANCY", "1")
	t.Setenv("GDWOLF_DEMO_AREA_PLANE", "1")
	data := victoryProbeMap()
	data.Planes[1][32*64+32] = pushableTile
	data.Planes[0][32*64+32] = 1
	data.Planes[0][32*64+33] = 91
	data.Planes[1][32*64+34] = 22
	data.Planes[0][40*64+40] = 91 // A later door must keep ordinal1.
	p := startWolfSourceBinary(t, path, "--victory-probe")
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(p.in, "180 0 0") // Ordinary player, with victory untriggered.
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	g := testGameWithLevel(data.Level())
	g.startNewGame()
	g.secretTotal = g.countSecretWalls()
	g.rng = newWolfRNG(0)
	g.playerX, g.playerY, g.playerA = 34.5, 32.5, -3.141592653589793
	g.demoPlayback = &wolfDemoPlayback{angle: 180}
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
			t.Fatalf("original door fixture stopped at%d: %v", step, p.out.Err())
		}
		var want wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		got := captureDemoRuntimeState(g)
		compareEnemyRuntimeStep(t, step, 100, want, got)
		if want.Player != got.Player || want.PushWall != got.PushWall || want.Stats != got.Stats || !reflect.DeepEqual(want.Victory, got.Victory) {
			t.Fatalf("step%d player/pushwall/stats: original=%+v/%+v/%+v port=%+v/%+v/%+v", step, want.Player, want.PushWall, want.Stats, got.Player, got.PushWall, got.Stats)
		}
	}
	check(-1)
	commands := []wl6.DemoCommand{{Buttons: demoButtonUse}}
	repeat := func(count int, command wl6.DemoCommand) {
		for i := 0; i < count; i++ {
			commands = append(commands, command)
		}
	}
	repeat(20, wl6.DemoCommand{})
	repeat(2, wl6.DemoCommand{Buttons: demoButtonStrafe, ControlX: 127})
	repeat(5, wl6.DemoCommand{ControlY: -127})
	repeat(2, wl6.DemoCommand{Buttons: demoButtonStrafe, ControlX: -127})
	repeat(10, wl6.DemoCommand{ControlX: 90})
	repeat(1, wl6.DemoCommand{Buttons: demoButtonUse})
	repeat(120, wl6.DemoCommand{})
	pushed := false
	for step, command := range commands {
		fmt.Fprintf(p.in, "%d %d %d\n", command.Buttons, command.ControlX, command.ControlY)
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
		pushed = pushed || g.pushWall.active
		check(step)
	}
	if !pushed || g.pushWall.active || g.doorState[32*64+33] != 0 {
		t.Fatalf("fixture incomplete: pushed=%v active=%v door=%d player=(%g,%g) angle=%d", pushed, g.pushWall.active, g.doorState[32*64+33], g.playerX, g.playerY, g.demoPlayback.angle)
	}
}
