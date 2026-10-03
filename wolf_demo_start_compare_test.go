package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gd-wolf/internal/wl6"
)

type wolfDemoActorStart struct {
	Class    int   `json:"class"`
	X        int64 `json:"x"`
	Y        int64 `json:"y"`
	TileX    int   `json:"tile_x"`
	TileY    int   `json:"tile_y"`
	Dir      int   `json:"dir"`
	Area     int   `json:"area"`
	Ambush   bool  `json:"ambush"`
	Health   int   `json:"health"`
	Speed    int64 `json:"speed"`
	Distance int64 `json:"distance"`
	Shape    int   `json:"shape"`
	TicCount int   `json:"tic_count"`
}

type wolfDemoStartState struct {
	RNGIndex int                  `json:"rng_index"`
	Actors   []wolfDemoActorStart `json:"actors"`
}

type wolfDemoFaceState struct {
	RNGIndex int `json:"rng_index"`
	Count    int `json:"face_count"`
	Frame    int `json:"face_frame"`
}

func writeDemoStartMap(w io.Writer, data *wl6.MapData) error {
	if _, err := fmt.Fprintf(w, "%d %d 3", data.Width(), data.Height()); err != nil {
		return err
	}
	for _, plane := range data.Planes {
		for _, cell := range plane {
			if _, err := fmt.Fprintf(w, " %d", cell); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func captureDemoStart(g *game) wolfDemoStartState {
	s := wolfDemoStartState{RNGIndex: int(g.rng.index), Actors: []wolfDemoActorStart{}}
	for _, a := range g.actors {
		// WOLFSRC guardobj+enemy_t: mutant follows the three unsupported bosses.
		class := map[ActorKind]int{actorKindGuard: 1, actorKindOfficer: 2, actorKindSS: 3, actorKindDog: 4, actorKindBoss: 5, actorKindMutant: 9}[a.kind]
		seq, _ := LookupAnimSequence(a.sequenceID)
		ticCount := 0
		if len(seq.Frames) > 0 && seq.Frames[0].Tics > 0 {
			ticCount = seq.Frames[0].Tics - a.frameTimer
		}
		s.Actors = append(s.Actors, wolfDemoActorStart{class,
			int64(math.Round(a.x * 65536)), int64(math.Round(a.y * 65536)),
			a.tileX, a.tileY, a.dir, a.area, a.ambush, a.health,
			int64(math.Round(a.patrolSpeed * 65536 * 60 / 70)),
			int64(math.Round(a.moveDistance * 65536)), a.shapenum, ticCount})
	}
	return s
}

func TestWolfDemoStartCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_START_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_start_compare.sh")
	}
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 4; index++ {
		t.Run(fmt.Sprintf("demo_%d", index), func(t *testing.T) {
			demo, err := files.LoadDemo(index)
			if err != nil {
				t.Fatal(err)
			}
			data, err := files.LoadMap(demo.Map)
			if err != nil {
				t.Fatal(err)
			}
			p := startWolfSourceBinary(t, path)
			var input bytes.Buffer
			if err := writeDemoStartMap(&input, data); err != nil {
				t.Fatal(err)
			}
			if _, err := p.in.Write(input.Bytes()); err != nil {
				t.Fatal(err)
			}
			if err := p.in.Flush(); err != nil {
				t.Fatal(err)
			}
			if !p.out.Scan() {
				t.Fatal("reference returned no initial state")
			}
			var want wolfDemoStartState
			if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
				t.Fatal(err)
			}
			g, err := buildEnemyAIFuzzBaseline(files, demo.Map)
			if err != nil {
				t.Fatal(err)
			}
			if err := g.startDemo(demo); err != nil {
				t.Fatal(err)
			}
			got := captureDemoStart(g)
			if out := os.Getenv("GDWOLF_DEMO_START_OUT"); out != "" {
				for name, state := range map[string]wolfDemoStartState{"reference": want, "port": got} {
					raw, err := json.MarshalIndent(state, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("demo-%d-%s.json", index, name)), append(raw, '\n'), 0644); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("demo-%d-input.txt", index)), input.Bytes(), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if got.RNGIndex != want.RNGIndex {
				t.Errorf("initial RNG: original=%d port=%d", want.RNGIndex, got.RNGIndex)
			}
			if len(got.Actors) != len(want.Actors) {
				t.Fatalf("actor count: original=%d port=%d", len(want.Actors), len(got.Actors))
			}
			for i := range want.Actors {
				if !reflect.DeepEqual(got.Actors[i], want.Actors[i]) {
					t.Errorf("actor %d: original=%+v port=%+v", i, want.Actors[i], got.Actors[i])
				}
			}
			t.Logf("matched %d initial actors and RNG index %d", len(want.Actors), want.RNGIndex)
			// Explicitly share the entry RNG index to isolate UpdateFace from
			// actor/combat draws, which are not yet independently reproduced.
			for i := 0; i < 1024; i++ {
				seed := (i * 37) & 255
				gatling := i%17 == 0
				g.rng.index = byte(seed)
				if _, err := fmt.Fprintf(p.in, "4 %d %d\n", seed, boolInt(gatling)); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				if !p.out.Scan() {
					t.Fatal("reference returned no face state")
				}
				var wantFace wolfDemoFaceState
				if err := json.Unmarshal(p.out.Bytes(), &wantFace); err != nil {
					t.Fatal(err)
				}
				g.updateDemoFace(4, gatling)
				gotFace := wolfDemoFaceState{int(g.rng.index), g.demoPlayback.faceCount, g.demoPlayback.faceFrame}
				if gotFace != wantFace {
					t.Fatalf("face call %d: original=%+v port=%+v", i, wantFace, gotFace)
				}
			}
			t.Log("matched 1,024 conditional face updates, including sound suppression")
		})
	}
}
