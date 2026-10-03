package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gd-wolf/internal/wl6"
)

// These tests run an external binary compiled from original C, rather than the
// Go reference adapter in enemy_ai_harness_test.go. No cgo enters game builds.
type wolfSourceInput struct {
	ID          string      `json:"id"`
	Map         int         `json:"map"`
	Actor       int         `json:"actor"`
	Mode        int         `json:"mode"` // chase, dodge, run, TryWalk
	Dir         int         `json:"dir"`
	Seed        byte        `json:"seed"`
	Dog         bool        `json:"dog"`
	FirstAttack bool        `json:"first_attack"`
	X           int         `json:"x"`
	Y           int         `json:"y"`
	PlayerX     int         `json:"player_x"`
	PlayerY     int         `json:"player_y"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Cells       []int       `json:"cells"`                // row-major: empty=0, wall=1, door=128+n, actor=256
	DoorLocks   map[int]int `json:"door_locks,omitempty"` // key is row-major tile index
}

type wolfSourceState struct {
	Dir         int  `json:"dir"`
	X           int  `json:"x"`
	Y           int  `json:"y"`
	Move        bool `json:"move"`
	Wait        bool `json:"wait"`
	RNGIndex    byte `json:"rng_index"`
	FirstAttack bool `json:"first_attack"`
	OpenedDoor  int  `json:"opened_door"`
}

type wolfSourceProcess struct {
	in  *bufio.Writer
	out *bufio.Scanner
}

func startWolfSource(t *testing.T) *wolfSourceProcess {
	t.Helper()
	path := os.Getenv("GDWOLF_WOLFSRC_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_source_compare.sh to build the original C reference")
	}
	cmd := exec.Command(path)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("original source reference exited: %v", err)
		}
	})
	return &wolfSourceProcess{in: bufio.NewWriter(stdin), out: bufio.NewScanner(stdout)}
}

func (p *wolfSourceProcess) decision(t *testing.T, c wolfSourceInput) wolfSourceState {
	t.Helper()
	if len(c.Cells) != c.Width*c.Height {
		t.Fatal("invalid comparison map dimensions")
	}
	_, err := fmt.Fprintf(p.in, "%d %d %d %d %d %d %d %d %d %d %d", c.Mode, c.Dir, c.Seed,
		boolInt(c.Dog), boolInt(c.FirstAttack), c.X, c.Y, c.PlayerX, c.PlayerY, c.Width, c.Height)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range c.Cells {
		if _, err := fmt.Fprintf(p.in, " %d", cell); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.in.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	if !p.out.Scan() {
		t.Fatalf("reference returned no decision: %v", p.out.Err())
	}
	var state wolfSourceState
	if err := json.Unmarshal(p.out.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func wolfSourceOpenMap() wolfSourceInput {
	c := wolfSourceInput{Map: -1, Width: 7, Height: 7, X: 3, Y: 3, PlayerX: 5, PlayerY: 5}
	c.Cells = make([]int, c.Width*c.Height)
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			if x == 0 || y == 0 || x == c.Width-1 || y == c.Height-1 {
				c.Cells[y*c.Width+x] = 1
			}
		}
	}
	return c
}

func captureWolfPort(c wolfSourceInput) wolfSourceState {
	level := blankLevel(c.Width, c.Height)
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			cell := c.Cells[y*c.Width+x]
			tile := wl6.Tile{Area: 0}
			switch {
			case cell == 1 || cell == 256:
				tile.Solid = true
			case cell >= 128 && cell <= 191:
				tile.Door = &wl6.Door{Lock: c.DoorLocks[y*c.Width+x]}
			}
			setLevelTile(level, x, y, tile)
		}
	}
	g := testGameWithLevel(level)
	g.playerX, g.playerY = float64(c.PlayerX)+0.5, float64(c.PlayerY)+0.5
	g.rng = newWolfRNG(c.Seed)
	a := actorInstance{kind: actorKindGuard, tileX: c.X, tileY: c.Y,
		x: float64(c.X) + 0.5, y: float64(c.Y) + 0.5,
		dir: c.Dir, facingDir: c.Dir, firstAttack: c.FirstAttack,
		alive: true, blocking: true, shootable: true, area: 0}
	if c.Dog {
		a.kind = actorKindDog
	}
	if c.Mode == 3 {
		g.actorTryWalk(&a, c.Dir)
	} else if c.Mode == 0 {
		g.actorChooseDirectChaseGoal(&a)
	} else if c.Mode == 1 {
		g.actorChooseDodgeGoal(&a)
	} else {
		g.actorChooseRunGoal(&a)
	}
	opened := -1
	if a.hasGoal && a.moveDistance < 0 {
		opened = c.Cells[a.tileY*c.Width+a.tileX] & 63
	}
	return wolfSourceState{Dir: a.dir, X: a.tileX, Y: a.tileY, Move: a.hasGoal,
		Wait: a.moveDistance < 0, RNGIndex: g.rng.Index(), FirstAttack: a.firstAttack, OpenedDoor: opened}
}

func diffWolfSource(want, got wolfSourceState) string {
	wt, gt := reflect.TypeOf(want), reflect.ValueOf(got)
	wv := reflect.ValueOf(want)
	var fields []string
	for i := 0; i < wt.NumField(); i++ {
		if wv.Field(i).Interface() != gt.Field(i).Interface() {
			fields = append(fields, fmt.Sprintf("%s: original=%v port=%v", wt.Field(i).Tag.Get("json"), wv.Field(i).Interface(), gt.Field(i).Interface()))
		}
	}
	return strings.Join(fields, "; ")
}

func TestWolfSourceDiffDetectsDesync(t *testing.T) {
	baseline := wolfSourceState{Dir: 3, X: 4, Y: 5, Move: true, RNGIndex: 7, OpenedDoor: -1}
	if diff := diffWolfSource(baseline, baseline); diff != "" {
		t.Fatal(diff)
	}
	for i := 0; i < reflect.TypeOf(baseline).NumField(); i++ {
		changed := baseline
		field := reflect.ValueOf(&changed).Elem().Field(i)
		if field.Kind() == reflect.Bool {
			field.SetBool(!field.Bool())
		} else if field.Kind() == reflect.Uint8 {
			field.SetUint(field.Uint() + 1)
		} else {
			field.SetInt(field.Int() + 1)
		}
		if diffWolfSource(baseline, changed) == "" {
			t.Fatalf("missed difference in %s", reflect.TypeOf(baseline).Field(i).Name)
		}
	}
}

func TestWolfSourceReferenceProtocol(t *testing.T) {
	p := startWolfSource(t)
	c := wolfSourceOpenMap()
	// Dodge always consumes one RNG byte, even on an unobstructed map.
	for seed := 0; seed < 256; seed++ {
		c.Mode, c.Seed, c.FirstAttack = 1, byte(seed), true
		want := wolfSourceState{Dir: 7, X: 4, Y: 4, Move: true, RNGIndex: byte(seed + 1), OpenedDoor: -1}
		if got := p.decision(t, c); got != want {
			t.Fatalf("seed %d: %s", seed, diffWolfSource(want, got))
		}
		// Read the extracted original table independently of the port's RNG.
		// A blocked chase consumes exactly one byte and selects the search arc.
		blocked := wolfSourceOpenMap()
		blocked.Seed, blocked.Dir = byte(seed), 8
		blocked.Cells[3*7+4], blocked.Cells[4*7+3] = 1, 1
		got := p.decision(t, blocked)
		wantDir := 4
		if wolfRandomTable[byte(seed+1)] > 128 {
			wantDir = 2
		}
		if got.Dir != wantDir || got.RNGIndex != byte(seed+1) {
			t.Fatalf("original RNG/search arc seed=%d: %+v want dir=%d", seed, got, wantDir)
		}
	}
	for _, dog := range []bool{false, true} {
		c = wolfSourceOpenMap()
		c.Mode, c.Dir, c.Dog = 3, 0, dog
		c.Cells[3*7+4] = 131
		got := p.decision(t, c)
		want := wolfSourceState{Dir: 0, X: 4, Y: 3, Move: true, Wait: true, OpenedDoor: 3}
		if dog {
			want = wolfSourceState{Dir: 0, X: 3, Y: 3, OpenedDoor: -1}
		}
		if got != want {
			t.Fatalf("door dog=%t: %s", dog, diffWolfSource(want, got))
		}
	}
	// A regression that the rewritten Go reference missed: the original run
	// fallback scans west/northwest/north, never east or south.
	c = wolfSourceOpenMap()
	c.Mode, c.PlayerX, c.PlayerY = 2, 3, 2
	for _, i := range []int{2*7 + 3, 3*7 + 2, 4*7 + 3} {
		c.Cells[i] = 1
	}
	want := wolfSourceState{Dir: 8, X: 3, Y: 3, RNGIndex: 1, OpenedDoor: -1}
	if got := p.decision(t, c); got != want {
		t.Fatal(diffWolfSource(want, got))
	}
}

type wolfSourceTrace struct {
	input, reference, port, mismatch, replay *json.Encoder
}

func openWolfSourceTrace(t *testing.T) wolfSourceTrace {
	t.Helper()
	root := os.Getenv("GDWOLF_WOLFSRC_OUT")
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	open := func(name string) *json.Encoder {
		f, err := os.Create(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		var writer io.Writer = f
		if strings.HasSuffix(name, ".gz") {
			compressed, err := gzip.NewWriterLevel(f, gzip.BestSpeed)
			if err != nil {
				t.Fatal(err)
			}
			writer = compressed
			t.Cleanup(func() {
				if err := compressed.Close(); err != nil {
					t.Error(err)
				}
				if err := f.Close(); err != nil {
					t.Error(err)
				}
			})
		} else {
			t.Cleanup(func() {
				if err := f.Close(); err != nil {
					t.Error(err)
				}
			})
		}
		return json.NewEncoder(writer)
	}
	t.Logf("comparison artifacts: %s", root)
	return wolfSourceTrace{open("inputs.jsonl.gz"), open("reference.jsonl"), open("port.jsonl"), open("mismatch.jsonl"), open("mismatch-inputs.jsonl")}
}

func TestWolfSourceCompare(t *testing.T) {
	p := startWolfSource(t)
	if replay := os.Getenv("GDWOLF_WOLFSRC_INPUT"); replay != "" {
		input, err := filepath.Abs(replay)
		if err != nil {
			t.Fatal(err)
		}
		output, err := filepath.Abs(filepath.Join(os.Getenv("GDWOLF_WOLFSRC_OUT"), "inputs.jsonl.gz"))
		if err != nil {
			t.Fatal(err)
		}
		failed, err := filepath.Abs(filepath.Join(os.Getenv("GDWOLF_WOLFSRC_OUT"), "mismatch-inputs.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if input == output || input == failed {
			t.Fatal("replay input must differ from output inputs.jsonl")
		}
	}
	trace := openWolfSourceTrace(t)
	count := 0
	mismatches := 0
	limit := 1
	if raw := os.Getenv("GDWOLF_WOLFSRC_MAX_MISMATCHES"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 {
			t.Fatal("max mismatches must be a positive integer")
		}
	}
	write := func(enc *json.Encoder, v any) {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	compare := func(c wolfSourceInput) {
		want, got := p.decision(t, c), captureWolfPort(c)
		write(trace.input, c)
		write(trace.reference, struct {
			ID string `json:"id"`
			wolfSourceState
		}{c.ID, want})
		write(trace.port, struct {
			ID string `json:"id"`
			wolfSourceState
		}{c.ID, got})
		count++
		if diff := diffWolfSource(want, got); diff != "" {
			mismatches++
			write(trace.replay, c)
			write(trace.mismatch, struct {
				Input wolfSourceInput `json:"input"`
				Want  wolfSourceState `json:"original"`
				Got   wolfSourceState `json:"port"`
				Diff  string          `json:"diff"`
			}{c, want, got, diff})
			if mismatches <= 5 {
				t.Logf("desync at decision %d: %s: %s", count, c.ID, diff)
			}
			if mismatches >= limit {
				t.Fatalf("stopped after %d decisions and %d desyncs: %s: %s", count, mismatches, c.ID, diff)
			}
		}
	}
	if replay := os.Getenv("GDWOLF_WOLFSRC_INPUT"); replay != "" {
		f, err := os.Open(replay)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		var reader io.Reader = f
		if strings.HasSuffix(replay, ".gz") {
			compressed, err := gzip.NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			defer compressed.Close()
			reader = compressed
		}
		decoder := json.NewDecoder(reader)
		for {
			var c wolfSourceInput
			if err := decoder.Decode(&c); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			compare(c)
		}
		if count == 0 {
			t.Fatal("replay input contains no decisions")
		}
		if mismatches > 0 {
			t.Fatalf("%d desyncs in %d replayed decisions", mismatches, count)
		}
		t.Logf("matched %d replayed decisions", count)
		return
	}
	// Real shareware snapshots preserve all blocking actors and scenery. Each
	// decision is isolated; no Go reference routine computes the expected result.
	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		snapshot := enemyAIFuzzSnapshot(t, mapIndex)
		for _, actorIndex := range snapshot.liveEnemies {
			for posIndex, pos := range snapshot.placements[actorIndex] {
				for mode := 0; mode < 3; mode++ {
					for _, seed := range []byte{0, 9, 127, 255} {
						c := wolfSourceMapInput(snapshot.base, actorIndex, pos, mode, seed)
						c.ID = fmt.Sprintf("map=%d/actor=%d/pos=%d/mode=%d/seed=%d", mapIndex, actorIndex, posIndex, mode, seed)
						compare(c)
					}
				}
			}
		}
		t.Logf("map %d: compared %d cumulative decisions (%d desyncs)", mapIndex, count, mismatches)
	}
	// Stress all local blocking patterns, directions, modes and both door rules.
	for mask := 0; mask < 256; mask++ {
		for dir := 0; dir <= 8; dir++ {
			for mode := 0; mode < 4; mode++ {
				for _, dog := range []bool{false, true} {
					c := wolfSourceOpenMap()
					c.ID = fmt.Sprintf("mask=%d/dir=%d/mode=%d/dog=%t", mask, dir, mode, dog)
					c.Mode, c.Dir, c.Seed, c.Dog = mode, dir, byte(mask), dog
					c.FirstAttack = mask%2 == 0
					for d := 0; d < 8; d++ {
						if mask&(1<<d) != 0 {
							dx, dy := dirStep(d)
							c.Cells[(c.Y+dy)*c.Width+c.X+dx] = 1
						}
					}
					compare(c)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d desyncs in %d original C decisions", mismatches, count)
	}
	t.Logf("matched %d original C decisions", count)
}

func wolfSourceMapInput(g *game, actorIndex int, pos [2]float64, mode int, seed byte) wolfSourceInput {
	a := &g.actors[actorIndex]
	c := wolfSourceInput{Map: g.mapIndex, Actor: actorIndex, Mode: mode, Dir: a.dir, Seed: seed,
		Dog: a.kind == actorKindDog, FirstAttack: a.firstAttack, X: a.tileX, Y: a.tileY,
		PlayerX: int(pos[0]), PlayerY: int(pos[1]), Width: g.levelWidth, Height: g.levelHeight}
	c.Cells = make([]int, c.Width*c.Height)
	door := 0
	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			tile := g.level.Tile(x, y)
			i := y*c.Width + x
			switch {
			case x == 0 || y == 0 || x == c.Width-1 || y == c.Height-1:
				c.Cells[i] = 1
			case tile.Door != nil:
				if !g.isDoorOpen(x, y) {
					c.Cells[i] = 128 + door
				}
				door++
				if tile.Door.Lock != 0 {
					if c.DoorLocks == nil {
						c.DoorLocks = map[int]int{}
					}
					c.DoorLocks[i] = tile.Door.Lock
				}
			case tile.Solid:
				c.Cells[i] = 1
			}
		}
	}
	for _, spr := range g.staticSprites {
		if spr.alive && spr.blocking {
			c.Cells[int(spr.y)*c.Width+int(spr.x)] = 1
		}
	}
	for i, other := range g.actors {
		if i != actorIndex && other.alive && other.blocking {
			c.Cells[other.tileY*c.Width+other.tileX] = 256
		}
	}
	return c
}
