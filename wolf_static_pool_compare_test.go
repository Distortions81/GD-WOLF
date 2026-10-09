package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gd-wolf/internal/wl6"
)

type staticProbeAction struct {
	action  string
	a, b, c int
}

func TestWolfStaticPoolCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	t.Setenv("GDWOLF_DEMO_STATICS", "1")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	collect := staticProbeAction{"collect", 32 * 65536, 32*65536 + 32768, 0}
	scenarios := []struct {
		name    string
		actions []staticProbeAction
	}{
		{"recycled_clip_precedes_machinegun", []staticProbeAction{
			{"spawn", 26, 20, 20}, {"spawn", 27, 32, 32}, {"bonus", 0, 0, 0},
			{"inventory", 100, 95, 0}, {"drop", 14, 32, 32}, collect,
		}},
		{"appended_clip_follows_machinegun", []staticProbeAction{
			{"spawn", 27, 32, 32}, {"inventory", 100, 95, 0}, {"drop", 14, 32, 32}, collect,
		}},
		{"recycled_gibs_precede_food", []staticProbeAction{
			{"spawn", 26, 20, 20}, {"spawn", 24, 32, 32}, {"bonus", 0, 0, 0},
			{"inventory", 10, 8, 0}, {"drop", 2, 32, 32}, collect,
		}},
		{"appended_gibs_follow_food", []staticProbeAction{
			{"spawn", 24, 32, 32}, {"inventory", 10, 8, 0}, {"drop", 2, 32, 32}, collect,
		}},
		{"map_terminator_slot_reused", []staticProbeAction{
			{"spawn", 49, 20, 20}, {"spawn", 27, 32, 32},
			{"inventory", 100, 95, 0}, {"drop", 14, 32, 32}, collect,
		}},
	}
	var full []staticProbeAction
	for i := 0; i < 399; i++ {
		full = append(full, staticProbeAction{"spawn", 0, 1 + i%62, 1 + i/62})
	}
	full = append(full, staticProbeAction{"drop", 5, 32, 32}, staticProbeAction{"drop", 14, 32, 32},
		staticProbeAction{"bonus", 399, 0, 0}, staticProbeAction{"drop", 14, 32, 32}, collect)
	scenarios = append(scenarios, struct {
		name    string
		actions []staticProbeAction
	}{"runtime_limit_and_reuse", full})
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			p := startWolfSourceBinary(t, path, "--static-probe")
			g := testGameWithLevel(victoryProbeMap().Level())
			g.startNewGame()
			g.demoPlayback = &wolfDemoPlayback{}
			for step, action := range scenario.actions {
				line := fmt.Sprintf("%s %d", action.action, action.a)
				if action.action != "bonus" {
					line += fmt.Sprintf(" %d", action.b)
				}
				if action.action != "bonus" && action.action != "inventory" {
					line += fmt.Sprintf(" %d", action.c)
				}
				fmt.Fprintln(p.in, line)
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				applyStaticProbeAction(t, g, action)
				if !p.out.Scan() {
					t.Fatalf("original static probe stopped at %s: %v", line, p.out.Err())
				}
				var want wolfDemoRuntimeState
				if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
					t.Fatal(err)
				}
				compareDemoStatics(t, step, want.Statics, captureDemoRuntimeStatics(g))
				if got := captureDemoRuntimeStats(g); got != want.Stats {
					t.Fatalf("step %d %s inventory: original=%+v port=%+v", step, line, want.Stats, got)
				}
			}
		})
	}
	t.Run("initial_limit", func(t *testing.T) {
		var input, stderr bytes.Buffer
		for i := 0; i < 400; i++ {
			fmt.Fprintf(&input, "spawn 0 %d %d\n", 1+i%62, 1+i/62)
		}
		cmd := exec.Command(path, "--static-probe")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = &input, io.Discard, &stderr
		if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), "Too many static objects!") {
			t.Fatalf("original 400th SpawnStatic should quit: err=%v stderr=%q", err, stderr.String())
		}
		g := testGameWithLevel(victoryProbeMap().Level())
		g.demoPlayback = &wolfDemoPlayback{}
		g.staticSprites = make([]staticSprite, 400)
		for i := range g.staticSprites {
			g.staticSprites[i] = staticSprite{x: float64(1+i%62) + .5, y: float64(1+i/62) + .5, alive: true}
		}
		defer func() {
			if recovered := recover(); recovered == nil || !strings.Contains(fmt.Sprint(recovered), "MAXSTATS=400") {
				t.Fatalf("port initial static pool limit: %v", recovered)
			}
		}()
		g.initializeDemoStaticSlots()
	})
}

// Spawn operations only occur during setup. Rebuilding from the actual map
// exercises the catalog and map scan; runtime drop operations use the pool.
func applyStaticProbeAction(t *testing.T, g *game, action staticProbeAction) {
	t.Helper()
	switch action.action {
	case "spawn":
		tile := g.level.Tile(action.b, action.c)
		tile.RawInfo = uint16(action.a + 23)
		setLevelTile(g.level, action.b, action.c, tile)
		g.staticSprites, g.treasureTotal = g.buildStaticSprites()
		g.demoPlayback.staticSlots, g.demoPlayback.staticInfo = nil, nil
		g.initializeDemoStaticSlots()
	case "drop":
		pickup := wolfPickupForItem(byte(action.a))
		def, ok := StaticDefForPickup(pickup)
		if !ok {
			for _, candidate := range activeStaticDefs {
				if candidate.Pickup == pickup {
					def, ok = candidate, true
					break
				}
			}
		}
		if !ok {
			t.Fatalf("unknown fixture item %d", action.a)
		}
		g.placeDemoStaticItem(staticSprite{x: float64(action.b) + .5, y: float64(action.c) + .5, shapenum: def.Shape, alive: true, pickup: pickup})
	case "bonus":
		index := g.demoPlayback.staticSlots[action.a]
		if g.applyPickup(wolfPickupForItem(g.demoPlayback.staticInfo[action.a].item)) {
			g.staticSprites[index].alive = false
		}
	case "inventory":
		g.health, g.ammo = action.a, action.b
	case "collect":
		g.playerX, g.playerY = float64(action.a)/65536, float64(action.b)/65536
		g.demoPlayback.angle = action.c
		g.demoPlayback.visibleTiles = make([]bool, 4096)
		for i := range g.demoPlayback.visibleTiles {
			g.demoPlayback.visibleTiles[i] = true
		}
		g.collectDemoPickups()
	}
}

func TestWolfStaticCatalogCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_enemy_runtime_compare.sh")
	}
	t.Setenv("GDWOLF_DEMO_STATICS", "1")
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", "off")
	data := victoryProbeMap()
	for info := 23; info <= 72; info++ {
		data.Planes[1][2*64+info-22] = uint16(info)
	}
	p := startWolfSourceBinary(t, path, "--victory-probe")
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(p.in, "0 0 0")
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	if !p.out.Scan() {
		t.Fatalf("original static catalog returned no state: %v", p.out.Err())
	}
	var want wolfDemoRuntimeState
	if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	g := testGameWithLevel(data.Level())
	g.startNewGame()
	g.staticSprites, g.treasureTotal = g.buildStaticSprites()
	g.demoPlayback = &wolfDemoPlayback{}
	compareDemoStatics(t, -1, want.Statics, captureDemoRuntimeStatics(g))
	if got := captureDemoRuntimeStats(g); got != want.Stats {
		t.Fatalf("catalog inventory: original=%+v port=%+v", want.Stats, got)
	}
}

func TestDemoStaticCatalogRejectsOriginalOutOfBounds(t *testing.T) {
	for _, info := range []uint16{73, 74} {
		t.Run(fmt.Sprint(info), func(t *testing.T) {
			g := testGameWithLevel(blankLevel(8, 8))
			setLevelTile(g.level, 2, 2, wl6.Tile{RawInfo: info})
			g.demoPlayback = &wolfDemoPlayback{}
			defer func() {
				if recovered := recover(); recovered == nil || !strings.Contains(fmt.Sprint(recovered), "statinfo") {
					t.Fatalf("out-of-bounds static should fail explicitly: %v", recovered)
				}
			}()
			g.initializeDemoStaticSlots()
		})
	}
}
