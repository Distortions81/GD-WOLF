package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

type wolfDemoRuntimeActor struct {
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

type wolfDemoRuntimeState struct {
	RNGIndex int                    `json:"rng_index"`
	Damage   int                    `json:"damage"`
	Actors   []wolfDemoRuntimeActor `json:"actors"`
	Doors    []wolfDemoRuntimeDoor  `json:"doors"`
	Weapon   wolfDemoRuntimeWeapon  `json:"weapon"`
	Walls    []int                  `json:"walls"`
	Player   wolfDemoPlayerState    `json:"player"`
}

type wolfDemoRuntimeWeapon struct {
	Attacking bool `json:"attacking"`
	Frame     int  `json:"frame"`
	Timer     int  `json:"timer"`
	Ammo      int  `json:"ammo"`
	Shots     int  `json:"shots"`
}

type wolfDemoRuntimeDoor struct {
	Action   int `json:"action"`
	Position int `json:"position"`
	Timer    int `json:"timer"`
}

func captureDemoRuntimeState(g *game) wolfDemoRuntimeState {
	s := wolfDemoRuntimeState{RNGIndex: int(g.rng.index)}
	s.Player = captureDemoPlayer(g)
	s.Weapon = wolfDemoRuntimeWeapon{g.attacking, g.weaponFrameIdx, g.weaponFrameTics, g.ammo, g.demoPlayback.shots}
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
		s.Actors = append(s.Actors, wolfDemoRuntimeActor{int(a.kind), int64(math.Round(a.x * 65536)), int64(math.Round(a.y * 65536)), a.tileX, a.tileY, a.dir, a.area, distance, a.reactionTimer, a.health, flags, a.shapenum, count})
	}
	for i, tile := range g.level.Tiles {
		if tile.Door == nil {
			continue
		}
		action := []int{0, 2, 1, 3}[g.doorState[i]]
		timer := 0
		if action == 1 {
			timer = g.doorTimer[i]
		}
		s.Doors = append(s.Doors, wolfDemoRuntimeDoor{action, int(math.Round(g.doorOpen[i] * 65535)), timer})
	}
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			s.Walls = append(s.Walls, boolInt(g.isBlockingTile(x, y)))
		}
	}
	return s
}

func TestWolfDemoRuntimeCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_runtime_compare.sh")
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
	independentDoors, independentWorld := true, true
	p := startWolfSourceBinary(t, path)
	out := os.Getenv("GDWOLF_DEMO_RUNTIME_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		return f
	}
	p.in = bufio.NewWriter(io.MultiWriter(p.input, open("reference-input.txt")))
	referenceTrace, portTrace := json.NewEncoder(open("reference.jsonl")), json.NewEncoder(open("port.jsonl"))
	renderTrace := json.NewEncoder(open("reference-render.jsonl"))
	result := json.NewEncoder(open("result.json"))
	matched, status := 0, "mismatch"
	terminal := ""
	defer func() {
		if err := result.Encode(map[string]any{"demo_commands": len(demo.Commands), "matched_commands": matched, "matched_tics": matched * wl6.DemoTics, "remaining_commands": len(demo.Commands) - matched, "status": status, "terminal": terminal}); err != nil {
			t.Error(err)
		}
	}()
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	read := func() wolfDemoRuntimeState {
		if !p.out.Scan() {
			t.Fatal("original actor reference returned no state")
		}
		var s wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	compare := func(command int, want, got wolfDemoRuntimeState) {
		if err := referenceTrace.Encode(map[string]any{"command": command, "state": want}); err != nil {
			t.Fatal(err)
		}
		if err := portTrace.Encode(map[string]any{"command": command, "state": got}); err != nil {
			t.Fatal(err)
		}
		if want.Weapon != got.Weapon {
			t.Errorf("command %d weapon: original=%+v port=%+v", command, want.Weapon, got.Weapon)
		}
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
		if independentDoors {
			if len(want.Doors) != len(got.Doors) {
				t.Fatal("door count differs")
			}
			for i := range want.Doors {
				if want.Doors[i] != got.Doors[i] {
					t.Errorf("command %d door %d: original=%+v port=%+v", command, i, want.Doors[i], got.Doors[i])
				}
			}
		}
		if independentWorld {
			if want.Player != got.Player {
				t.Errorf("command %d player: original=%+v port=%+v", command, want.Player, got.Player)
			}
			if len(want.Walls) != len(got.Walls) {
				t.Fatal("wall count differs")
			}
			for i := range want.Walls {
				if want.Walls[i] != got.Walls[i] {
					t.Errorf("command %d wall (%d,%d): original=%d port=%d", command, i%64, i/64, want.Walls[i], got.Walls[i])
				}
			}
		}
		if t.Failed() {
			t.FailNow()
		}
	}
	compare(-1, read(), captureDemoRuntimeState(g))
	previousTiles := append([]wl6.Tile(nil), g.level.Tiles...)
	for i, command := range demo.Commands {
		if g.playerDying || g.victoryActive {
			status = "unsupported_terminal_state"
			t.Fatalf("port stopped before command %d", i)
		}
		useDoor := -1
		pushX, pushY, pushDir := 0, 0, -1
		if !g.attacking && command.Buttons&demoButtonUse != 0 && g.demoPlayback.buttons&demoButtonUse == 0 {
			x, y := g.cardinalUseTile()
			if g.level.Tile(x, y).RawInfo == pushableTile {
				pushX, pushY = x, y
				dx, dy := g.cardinalUseVector()
				switch {
				case dx > 0:
					pushDir = 1
				case dx < 0:
					pushDir = 3
				case dy < 0:
					pushDir = 0
				default:
					pushDir = 2
				}
			}
			index := 0
			for tileIndex, tile := range g.level.Tiles {
				if tile.Door == nil {
					continue
				}
				if tileIndex == y*g.levelWidth+x {
					useDoor = index
					break
				}
				index++
			}
		}
		wasAttacking := g.prepareDemoCommand(command)
		g.moveDemoPlayer(command)
		entryRNG, weapon, ammo, chosenWeapon := g.rng.index, g.weapon, g.ammo, g.chosenWeapon
		if wasAttacking {
			g.updateWeaponAttackWithInput(wl6.DemoTics, g.demoPlayback.inputButtons&demoButtonAttack != 0)
		}
		g.rebuildPlayerAreas()
		if _, err := fmt.Fprintf(p.in, "%d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d", int64(math.Round(g.playerX*65536)), int64(math.Round(g.playerY*65536)), entryRNG, boolInt(g.madeNoise), boolInt(g.playerMovingFast), g.bestWeapon, weapon, g.demoPlayback.shots, useDoor, command.Buttons, ammo, chosenWeapon, pushX, pushY, pushDir, command.ControlX, command.ControlY); err != nil {
			t.Fatal(err)
		}
		for area := 0; area < 64; area++ {
			connected := area < len(g.playerAreas) && g.playerAreas[area]
			if _, err := fmt.Fprintf(p.in, " %d", boolInt(connected)); err != nil {
				t.Fatal(err)
			}
		}
		// Changed wall tiles are diagnostic input; the original reference
		// evolves its own pushwalls and actor/static reservations.
		changed := []int{}
		for index, tile := range g.level.Tiles {
			if tile.Solid != previousTiles[index].Solid || tile.RawWall != previousTiles[index].RawWall || tile.Area != previousTiles[index].Area {
				changed = append(changed, index)
			}
		}
		if _, err := fmt.Fprintf(p.in, " %d", len(changed)); err != nil {
			t.Fatal(err)
		}
		for _, index := range changed {
			tile := g.level.Tiles[index]
			wall := 0
			if tile.Solid && tile.Door == nil {
				wall = int(tile.RawWall)
			}
			if _, err := fmt.Fprintf(p.in, " %d %d %d", index, wall, tile.Area); err != nil {
				t.Fatal(err)
			}
			previousTiles[index] = tile
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
			projection := demoActorProjection{}
			if index < len(g.demoPlayback.projections) {
				projection = g.demoPlayback.projections[index]
			}
			if _, err := fmt.Fprintf(p.in, " %d %d %d", boolInt(projection.visible), projection.viewX, projection.transX); err != nil {
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
		beforeWeapon := captureDemoRuntimeState(g).Weapon
		g.updateDemoActors(wl6.DemoTics)
		got := captureDemoRuntimeState(g)
		got.Weapon = beforeWeapon // Port death clears the weapon; compare before that terminal side effect.
		compare(i, want, got)
		if gotDamage := beforeHealth - g.health; gotDamage != minInt(beforeHealth, want.Damage) {
			t.Fatalf("command %d enemy damage: original=%d port=%d", i, want.Damage, gotDamage)
		}
		g.refreshDemoActorProjections()
		if _, err := fmt.Fprintf(p.in, "%d", g.demoPlayback.angle); err != nil {
			t.Fatal(err)
		}
		for _, visible := range g.demoPlayback.visibleTiles {
			if _, err := fmt.Fprintf(p.in, " %d", boolInt(visible)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := fmt.Fprintln(p.in); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		if !p.out.Scan() {
			t.Fatal("original renderer returned no projections")
		}
		var rendered struct {
			Health      int  `json:"health"`
			Ammo        int  `json:"ammo"`
			Died        bool `json:"died"`
			Projections []struct {
				Visible bool `json:"visible"`
				ViewX   int  `json:"view_x"`
				TransX  int  `json:"trans_x"`
			} `json:"projections"`
		}
		if err := json.Unmarshal(p.out.Bytes(), &rendered); err != nil {
			t.Fatal(err)
		}
		if err := renderTrace.Encode(map[string]any{"command": i, "render": rendered}); err != nil {
			t.Fatal(err)
		}
		if len(rendered.Projections) != len(g.actors) {
			t.Fatal("projection count differs")
		}
		for index, want := range rendered.Projections {
			got := g.demoPlayback.projections[index]
			if got != (demoActorProjection{want.Visible, want.ViewX, want.TransX}) {
				t.Fatalf("command %d actor %d renderer: original=%+v port=%+v", i, index, want, got)
			}
		}
		g.collectPickups()
		if rendered.Died != g.playerDying {
			t.Fatalf("command %d death: original=%v port=%v", i, rendered.Died, g.playerDying)
		}
		if rendered.Health != g.health || rendered.Ammo != g.ammo {
			t.Fatalf("command %d after pickups: original health/ammo=%d/%d port=%d/%d", i, rendered.Health, rendered.Ammo, g.health, g.ammo)
		}
		g.demoPlayback.buttons = g.demoPlayback.inputButtons
		g.demoPlayback.command++
		matched++
		if rendered.Died {
			status, terminal = "matched_terminal_state", "death"
			t.Logf("matched original death after command %d, tic %d; original playback leaves %d recorded commands unread", i, matched*wl6.DemoTics, len(demo.Commands)-matched)
			return
		}
	}
	status = "success"
	t.Log("matched every first-demo runtime update with original C, sharing visible-floor masks")
}
