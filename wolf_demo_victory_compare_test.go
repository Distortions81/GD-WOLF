package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"gd-wolf/internal/wl6"
)

func victoryProbeMap() *wl6.MapData {
	m := &wl6.MapData{Header: wl6.MapHeader{Width: 64, Height: 64}, Planes: [2][]uint16{make([]uint16, 4096), make([]uint16, 4096)}}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			wall := uint16(107)
			if x == 0 || y == 0 || x == 63 || y == 63 {
				wall = 1
			}
			m.Planes[0][y*64+x] = wall
		}
	}
	m.Planes[1][32*64+32] = 20
	return m
}

func TestWolfDemoVictoryCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run with the original demo runtime reference")
	}
	files, err := wl6.OpenEmbeddedShareware()
	if dir := os.Getenv("GDWOLF_DEMO_RUNTIME_DATA"); dir != "" {
		files, err = wl6.Open(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	setSpriteCatalogVariant(files.Variant)
	baselineFiles, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	defer setSpriteCatalogVariant(baselineFiles.Variant)
	data := victoryProbeMap()
	checks := 0
	for _, mode := range []string{"off", "adlib", "adlib-digi"} {
		for angle := 0; angle < 360; angle++ {
			for _, attacking := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s-angle%d-attack%v", mode, angle, attacking), func(t *testing.T) {
					checks += compareVictoryProbe(t, path, files, data, mode, angle, attacking, true, nil)
				})
				if t.Failed() {
					t.FailNow()
				}
			}
		}
	}
	t.Logf("matched %d original-C victory states across every angle, both player states and all sound modes", checks)
}

func compareVictoryProbe(t *testing.T, path string, files *wl6.Files, data *wl6.MapData, mode string, angle int, attacking, trigger bool, first *wl6.DemoCommand) int {
	t.Helper()
	var err error
	checks := 0
	visibility := make([]bool, 4096)
	for i := range visibility {
		visibility[i] = true
	}
	configureWolfDemoReferenceSound(t, files, mode, 0, t.TempDir())
	p := startWolfSourceBinary(t, path, "--victory-probe")
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(p.in, "%d %d %d\n", angle, boolInt(attacking), boolInt(trigger))
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	g := testGameWithLevel(data.Level())
	g.startNewGame()
	g.files = files
	g.playerX, g.playerY, g.playerA = 32.5, 32.5, -float64(angle)*math.Pi/180
	g.rng = newWolfRNG(0)
	g.demoPlayback = &wolfDemoPlayback{angle: angle, victoryTileY: 32}
	g.demoPlayback.sound, err = newWolfDemoSound(files, mode)
	if err != nil {
		t.Fatal(err)
	}
	g.playerAreas = g.computePlayerAreas()
	g.rebuildPlayerAreas()
	if attacking {
		g.updateWeaponAttackWithInput(0, true)
	}
	if trigger {
		g.spawnDemoVictoryBJ()
	}
	check := func(step int) {
		t.Helper()
		if !p.out.Scan() {
			t.Fatalf("victory reference ended at %d: %v", step, p.out.Err())
		}
		var want wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		got := captureDemoRuntimeState(g)
		// This isolated probe uses a shared fully visible floor mask;
		// the corpus performs the separate original-x86 visibility audit.
		want.Walls, got.Walls = nil, nil
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("victory step %d:\noriginal=%+v\nport=%+v", step, want, got)
		}
		checks++
	}
	check(-1)
	for step := 0; g.demoPlayback.levelExit == 0 && step < 200; step++ {
		// Inputs keep changing throughout victory; original player
		// states consume commands but ignore their movement/use/fire.
		command := wl6.DemoCommand{Buttons: byte(step % 256), ControlX: 63, ControlY: -63}
		if step == 0 && first != nil {
			command = *first
		}
		fmt.Fprintf(p.in, "%d %d %d\n", command.Buttons, command.ControlX, command.ControlY)
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		g.prepareDemoCommand(command)
		g.moveDemoPlayer(command)
		g.updateDemoActors(wl6.DemoTics)
		g.refreshDemoActorProjectionsWithVisibility(visibility)
		g.collectPickups()
		g.demoPlayback.timeCount += wl6.DemoTics
		g.demoPlayback.buttons = g.demoPlayback.inputButtons
		check(step)
	}
	if g.demoPlayback.levelExit != 6 {
		t.Fatal("victory did not reach the original terminal state")
	}
	return checks
}

func TestWolfDemoVictoryEntryCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run with the original demo runtime reference")
	}
	files, err := wl6.OpenEmbeddedShareware()
	if dir := os.Getenv("GDWOLF_DEMO_RUNTIME_DATA"); dir != "" {
		files, err = wl6.Open(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	setSpriteCatalogVariant(files.Variant)
	defer setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"})
	for _, mode := range []string{"off", "adlib", "adlib-digi"} {
		for angle := 0; angle < 360; angle += 90 {
			for _, attacking := range []bool{false, true} {
				for _, double := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s-angle%d-attack%v-double%v", mode, angle, attacking, double), func(t *testing.T) {
						data := victoryProbeMap()
						dx, dy := dirStep(angle / 45)
						x, y := 32, 32
						command := wl6.DemoCommand{ControlY: -63}
						if double {
							sx, sy := dirStep((angle/45 + 6) % 8)
							x, y = x+sx, y+sy
							data.Planes[1][y*64+x] = wolfExitTile
							command.Buttons, command.ControlX = demoButtonStrafe, 63
						}
						data.Planes[1][(y+dy)*64+x+dx] = wolfExitTile
						compareVictoryProbe(t, path, files, data, mode, angle, attacking, false, &command)
					})
					if t.Failed() {
						t.FailNow()
					}
				}
			}
		}
	}
}
