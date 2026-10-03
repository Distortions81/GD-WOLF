package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gd-wolf/internal/wl6"
)

type wolfDemoPlayerState struct {
	X         int64 `json:"x"`
	Y         int64 `json:"y"`
	Angle     int   `json:"angle"`
	AngleFrac int   `json:"angle_frac"`
}

func TestWolfDemoPlayerReferenceProtocol(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_PLAYER_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_player_compare.sh --self-test")
	}
	p := startWolfSourceBinary(t, path)
	g := demoMovementTestGame()
	if _, err := fmt.Fprintf(p.in, "229376 229376 0 7 7\n"); err != nil {
		t.Fatal(err)
	}
	commands := make([]wl6.DemoCommand, 360)
	for i := range commands {
		commands[i] = wl6.DemoCommand{ControlX: -5, ControlY: -1}
	}
	commands = append(commands,
		wl6.DemoCommand{Buttons: demoButtonStrafe, ControlX: 127, ControlY: -128},
		wl6.DemoCommand{Buttons: demoButtonStrafe, ControlX: -128, ControlY: 127},
		wl6.DemoCommand{ControlX: 1, ControlY: 70},
		wl6.DemoCommand{ControlX: -1, ControlY: -70})
	for i, command := range commands {
		if err := writeDemoWorld(p.in, g, command); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		if !p.out.Scan() {
			t.Fatalf("reference returned no state at protocol case %d", i)
		}
		var want wolfDemoPlayerState
		if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		g.moveDemoPlayer(command)
		if got := captureDemoPlayer(g); got != want {
			t.Fatalf("protocol case %d: original=%+v port=%+v", i, want, got)
		}
	}
	for _, input := range []string{"0 0 0 64 64\n", "229376 229376 0 7 7\n0 0 0 151\n"} {
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(input)
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("reference accepted malformed input: %s", output)
		}
	}
	t.Log("matched all 360 angles plus strafe, reverse, speed clamp and fractional-turn cases")
}

func captureDemoRuntime(g *game) map[string]any {
	actors := make([]map[string]any, len(g.actors))
	for i := range g.actors {
		a := &g.actors[i]
		actors[i] = map[string]any{"index": i, "kind": a.kind, "state": captureHarnessRuntimeState(a), "health": a.health, "alive": a.alive, "blocking": a.blocking, "shootable": a.shootable, "frame": a.frameIndex, "shape": a.shapenum, "frame_timer": a.frameTimer, "frame_action_done": a.frameActionDone, "sequence_loop": a.sequenceLoop, "spawn_animation_frozen": a.spawnAnimationFrozen}
	}
	doors := []map[string]any{}
	for i, tile := range g.level.Tiles {
		if tile.Door != nil {
			doors = append(doors, map[string]any{"x": i % g.levelWidth, "y": i / g.levelWidth, "lock": tile.Door.Lock, "state": g.doorState[i], "open": g.doorOpen[i], "timer": g.doorTimer[i]})
		}
	}
	return map[string]any{"health": g.health, "ammo": g.ammo, "weapon": g.weapon, "attacking": g.attacking, "weapon_frame": g.weaponFrameIdx, "weapon_timer": g.weaponFrameTics, "rng_index": g.rng.index, "face_count": g.demoPlayback.faceCount, "face_frame": g.demoPlayback.faceFrame, "actors": actors, "doors": doors, "player_dying": g.playerDying, "victory": g.victoryActive}
}

func captureDemoPlayer(g *game) wolfDemoPlayerState {
	return wolfDemoPlayerState{int64(math.Round(g.playerX * 65536)), int64(math.Round(g.playerY * 65536)), g.demoPlayback.angle, g.demoPlayback.angleFrac}
}

// writeDemoWorld sends the actual port occupancy after doors/use and before
// player movement. The reference carries its own player state, but shares this
// world: this is a conditional player comparison, not full-engine parity.
func writeDemoWorld(w io.Writer, g *game, command wl6.DemoCommand) error {
	actors := make([]actorInstance, 0, len(g.actors))
	for _, a := range g.actors {
		if a.alive && a.blocking && a.shootable {
			actors = append(actors, a)
		}
	}
	if _, err := fmt.Fprintf(w, "%d %d %d %d", command.Buttons, command.ControlX, command.ControlY, len(actors)); err != nil {
		return err
	}
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			blocked := g.demoSolidTile(x, y)
			if _, err := fmt.Fprintf(w, " %d", boolInt(blocked)); err != nil {
				return err
			}
		}
	}
	for _, a := range actors {
		if _, err := fmt.Fprintf(w, " %d %d %d %d", a.tileX, a.tileY, int64(math.Round(a.x*65536)), int64(math.Round(a.y*65536))); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func TestWolfDemoPlayerCompare(t *testing.T) {
	binary := os.Getenv("GDWOLF_DEMO_PLAYER_REFERENCE")
	if binary == "" {
		t.Skip("run scripts/wolf_demo_player_compare.sh")
	}
	index := 0
	if s := os.Getenv("GDWOLF_DEMO_INDEX"); s != "" {
		var err error
		index, err = strconv.Atoi(s)
		if err != nil {
			t.Fatal(err)
		}
	}
	limit := 0
	if s := os.Getenv("GDWOLF_DEMO_COMMAND_LIMIT"); s != "" {
		var err error
		limit, err = strconv.Atoi(s)
		if err != nil || limit < 0 {
			t.Fatal("invalid command limit")
		}
	}
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	demo, err := files.LoadDemo(index)
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("GDWOLF_DEMO_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		demo, err = wl6.ParseDemo(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := buildEnemyAIFuzzBaseline(files, demo.Map)
	if err != nil {
		t.Fatal(err)
	}
	if err = g.startDemo(demo); err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("GDWOLF_DEMO_OUT")
	if out == "" {
		t.Fatal("comparison requires an output directory")
	}
	if err = os.MkdirAll(out, 0755); err != nil {
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
	refTrace, portTrace := json.NewEncoder(open("reference.jsonl")), json.NewEncoder(open("port.jsonl"))
	commands := json.NewEncoder(open("commands.jsonl"))
	worldInput := open("reference-input.txt")
	mismatch := json.NewEncoder(open("mismatch.jsonl"))
	runtime := json.NewEncoder(open("port-runtime.jsonl"))
	result := json.NewEncoder(open("result.json"))
	cmd := exec.Command(binary)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = in.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("reference exited: %v", err)
		}
	})
	w := bufio.NewWriter(io.MultiWriter(in, worldInput))
	scanner := bufio.NewScanner(stdout)
	initial := captureDemoPlayer(g)
	if _, err = fmt.Fprintf(w, "%d %d %d %d %d\n", initial.X, initial.Y, initial.Angle, g.levelWidth, g.levelHeight); err != nil {
		t.Fatal(err)
	}
	encode := func(e *json.Encoder, v any) {
		if err := e.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	meta := map[string]any{"kind": "initial", "demo": index, "map": demo.Map, "player": initial, "scope": "player movement with shared port world snapshots"}
	if file := os.Getenv("GDWOLF_DEMO_FILE"); file != "" {
		meta["demo_file"] = file
	}
	encode(refTrace, meta)
	encode(portTrace, meta)
	encode(runtime, map[string]any{"kind": "initial", "runtime": captureDemoRuntime(g)})
	count := len(demo.Commands)
	if limit > 0 && limit < count {
		count = limit
	}
	for i, command := range demo.Commands[:count] {
		if g.playerDying || g.victoryActive {
			encode(result, map[string]any{"status": "unsupported_terminal_state", "matched_commands": i, "matched_tics": i * wl6.DemoTics, "demo_commands": len(demo.Commands), "runtime": captureDemoRuntime(g)})
			t.Fatalf("port reached death/victory before command %d; terminal-state comparison is not implemented", i)
		}
		wasAttacking := g.prepareDemoCommand(command)
		encode(commands, map[string]any{"command": i, "end_tic": (i + 1) * wl6.DemoTics, "input": command})
		if err = writeDemoWorld(w, g, command); err != nil {
			t.Fatal(err)
		}
		if err = w.Flush(); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() {
			t.Fatalf("reference returned no state at command %d: %v", i, scanner.Err())
		}
		var want wolfDemoPlayerState
		if err = json.Unmarshal(scanner.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		g.moveDemoPlayer(command)
		got := captureDemoPlayer(g)
		encode(refTrace, map[string]any{"command": i, "end_tic": (i + 1) * wl6.DemoTics, "player": want})
		encode(portTrace, map[string]any{"command": i, "end_tic": (i + 1) * wl6.DemoTics, "player": got})
		if want != got {
			for y := int(g.playerY) - 1; y <= int(g.playerY)+1; y++ {
				for x := int(g.playerX) - 1; x <= int(g.playerX)+1; x++ {
					tile := g.level.Tile(x, y)
					if tile.Door != nil {
						t.Logf("nearby door (%d,%d): state=%d openness=%f lock=%d", x, y, g.doorState[y*g.levelWidth+x], g.doorOpen[y*g.levelWidth+x], tile.Door.Lock)
					}
				}
			}
			encode(mismatch, map[string]any{"command": i, "end_tic": (i + 1) * wl6.DemoTics, "input": command, "reference": want, "port": got})
			encode(result, map[string]any{"status": "player_mismatch", "matched_commands": i, "command": i})
			t.Fatalf("demo %d command %d (end tic %d): original=%+v port=%+v; artifacts: %s", index, i, (i+1)*wl6.DemoTics, want, got, out)
		}
		g.finishDemoCommand(command, wasAttacking)
		g.demoPlayback.command++
		encode(runtime, map[string]any{"command": i, "end_tic": (i + 1) * wl6.DemoTics, "runtime": captureDemoRuntime(g)})
	}
	encode(result, map[string]any{"status": "matched_player_movement", "matched_commands": count, "matched_tics": count * wl6.DemoTics, "demo_commands": len(demo.Commands), "scope": "player movement with shared port world snapshots"})
	t.Logf("matched %d demo commands (%d tics) for player movement with shared port world snapshots", count, count*wl6.DemoTics)
}
