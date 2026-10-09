package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestWolfActorMoveContactCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ENEMY_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("set GDWOLF_ENEMY_RUNTIME_REFERENCE to the original runtime")
	}
	p := startWolfSourceBinary(t, path, "--moveobj-probe")
	checked := 0
	for _, kind := range []ActorKind{actorKindGuard, actorKindGhost} {
		for dir := 0; dir <= 8; dir++ {
			for _, connected := range []bool{false, true} {
				for _, speed := range []int{512, 1500} {
					for _, edgeX := range []int{-65537, -65536, -65535, 0, 65535, 65536, 65537} {
						for _, edgeY := range []int{-65537, -65536, -65535, 0, 65535, 65536, 65537} {
							ax, ay, tics := 32*65536+32768, 32*65536+32768, 4
							dx, dy := dirStep(dir)
							px, py := ax+dx*speed*tics+edgeX, ay+dy*speed*tics+edgeY
							if _, err := fmt.Fprintf(p.in, "%d %d %d %d %d %d %d %d %d\n", px, py, ax, ay, dir, boolInt(connected), speed, tics, kind); err != nil {
								t.Fatal(err)
							}
							if err := p.in.Flush(); err != nil {
								t.Fatal(err)
							}
							if !p.out.Scan() {
								t.Fatalf("original MoveObj stopped: %v", p.out.Err())
							}
							var want struct {
								X, Y, Distance, Health, Damage int
							}
							if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
								t.Fatal(err)
							}
							g := &game{demoPlayback: &wolfDemoPlayback{}, health: 100, playerX: float64(px) / 65536, playerY: float64(py) / 65536, playerAreas: []bool{false, connected}}
							a := actorInstance{kind: kind, x: float64(ax) / 65536, y: float64(ay) / 65536, dir: dir, area: 1, hasGoal: true, moveDistance: 1, actionTics: tics}
							g.moveDemoActorTowardGoalStep(&a, float64(speed*tics)/65536)
							if int(math.Round(a.x*65536)) != want.X || int(math.Round(a.y*65536)) != want.Y || int(math.Round(a.moveDistance*65536)) != want.Distance || g.health != want.Health {
								t.Fatalf("kind%d dir%d connected%v speed%d edge%d,%d: original=%+v port=(%g,%g) distance%g health%d", kind, dir, connected, speed, edgeX, edgeY, want, a.x, a.y, a.moveDistance, g.health)
							}
							checked++
						}
					}
				}
			}
		}
	}
	t.Logf("matched %d original MoveObj contact states", checked)
}
