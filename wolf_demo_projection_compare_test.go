package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestWolfDemoProjectionCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_runtime_compare.sh")
	}
	output, err := exec.Command(path, "--render-tables").Output()
	if err != nil {
		t.Fatal(err)
	}
	var tables struct {
		Tangents [900]int           `json:"tangents"`
		Pixels   [demoViewWidth]int `json:"pixel_angles"`
	}
	if err := json.Unmarshal(output, &tables); err != nil {
		t.Fatal(err)
	}
	if tables.Tangents != demoFineTangents || tables.Pixels != demoPixelAngles {
		t.Fatal("original BuildTables/CalcProjection render tables differ")
	}
	p := startWolfSourceBinary(t, path, "--projection")
	g := demoMovementTestGame()
	g.playerX, g.playerY = 3.25, 3.75
	positions := [][2]int{{3*65536 + 32768, 3*65536 + 32768}, {4*65536 + 32768, 3*65536 + 32768}, {2*65536 + 32768, 3*65536 + 32768}, {4*65536 + 32768, 4*65536 + 32768}, {1*65536 + 32768, 5*65536 + 32768}, {5*65536 + 32768, 2*65536 + 32768}}
	for angle := 0; angle < 360; angle++ {
		g.demoPlayback.angle = angle
		for _, xy := range positions {
			g.actors = []actorInstance{{x: float64(xy[0]) / 65536, y: float64(xy[1]) / 65536, tileX: xy[0] >> 16, tileY: xy[1] >> 16}}
			g.refreshDemoActorProjections()
			// Isolate TransformTile reach from the unverified ASM visibility.
			for i := range g.demoPlayback.visibleTiles {
				g.demoPlayback.visibleTiles[i] = true
			}
			if _, err := fmt.Fprintf(p.in, "%d %d %d %d %d\n", int(g.playerX*65536), int(g.playerY*65536), angle, xy[0], xy[1]); err != nil {
				t.Fatal(err)
			}
			if err := p.in.Flush(); err != nil {
				t.Fatal(err)
			}
			if !p.out.Scan() {
				t.Fatal("original projection returned no state")
			}
			var want struct {
				ViewX  int  `json:"view_x"`
				TransX int  `json:"trans_x"`
				Height int  `json:"height"`
				Pickup bool `json:"pickup"`
			}
			if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
				t.Fatal(err)
			}
			got := demoActorProjection{}
			cos, sin := wolfDemoTrigTable[angle+90], wolfDemoTrigTable[angle]
			viewX := int(g.playerX*65536) - wolfDemoFixedByFrac(0x5700, cos)
			viewY := int(g.playerY*65536) + wolfDemoFixedByFrac(0x5700, sin)
			transformDemoActor(&g.actors[0], &got, viewX, viewY, cos, sin)
			if got.viewX != want.ViewX || got.transX != want.TransX || g.demoPickupInReach(xy[0]>>16, xy[1]>>16) != want.Pickup {
				t.Fatalf("angle %d position %v: original=%+v port=%+v pickup=%v", angle, xy, want, got, g.demoPickupInReach(xy[0]>>16, xy[1]>>16))
			}
		}
	}
	t.Log("matched original render tables and 2,160 actor/pickup projections across all 360 angles")
}
