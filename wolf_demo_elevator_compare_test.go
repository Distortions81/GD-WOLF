package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestWolfDemoElevatorCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_runtime_compare.sh")
	}
	p := startWolfSourceBinary(t, path, "--elevator-probe")
	for angle := 0; angle < 360; angle++ {
		for _, floor := range []uint16{107, 108} {
			for _, held := range []bool{false, true} {
				g := testGameWithLevel(blankLevel(9, 9))
				g.playerX, g.playerY = 4.5, 4.5
				g.playerA = -float64(angle) * math.Pi / 180
				g.demoPlayback = &wolfDemoPlayback{angle: angle}
				if held {
					g.demoPlayback.buttons = demoButtonUse
				}
				setLevelTile(g.level, 4, 4, wl6.Tile{RawWall: floor, Area: int(floor) - 107})
				for _, xy := range [][2]int{{4, 3}, {5, 4}, {4, 5}, {3, 4}} {
					setLevelTile(g.level, xy[0], xy[1], wl6.Tile{RawWall: wolfElevatorTile, Solid: true})
				}
				if _, err := fmt.Fprintf(p.in, "%d %d %d\n", angle, floor, boolInt(held)); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				if !p.out.Scan() {
					t.Fatalf("original elevator reference returned no state: %v", p.out.Err())
				}
				var want struct {
					Terminal int `json:"terminal"`
					North    int `json:"north_wall"`
					East     int `json:"east_wall"`
					South    int `json:"south_wall"`
					West     int `json:"west_wall"`
				}
				if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
					t.Fatal(err)
				}
				g.prepareDemoCommand(wl6.DemoCommand{Buttons: demoButtonUse})
				walls := [4]int{int(g.level.Tile(4, 3).RawWall), int(g.level.Tile(5, 4).RawWall), int(g.level.Tile(4, 5).RawWall), int(g.level.Tile(3, 4).RawWall)}
				if g.demoPlayback.levelExit != want.Terminal || walls != [4]int{want.North, want.East, want.South, want.West} {
					t.Fatalf("angle=%d floor=%d held=%v: original=%+v port terminal=%d walls=%v", angle, floor, held, want, g.demoPlayback.levelExit, walls)
				}
			}
		}
	}
	t.Log("matched 1,440 original-C elevator uses across every angle, normal/secret floors, and held/fresh use")
}
