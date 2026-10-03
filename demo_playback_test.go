package main

import (
	"gd-wolf/internal/wl6"
	"testing"
)

func demoMovementTestGame() *game {
	level := blankLevel(7, 7)
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			if x == 0 || y == 0 || x == 6 || y == 6 {
				setLevelTile(level, x, y, wl6.Tile{Solid: true})
			}
		}
	}
	g := testGameWithLevel(level)
	g.playerX, g.playerY = 3.5, 3.5
	g.demoPlayback = &wolfDemoPlayback{}
	return g
}

func TestDemoFirstMovingCommandMatchesOriginalFixedPoint(t *testing.T) {
	g := demoMovementTestGame()
	g.moveDemoPlayer(wl6.DemoCommand{ControlX: 1, ControlY: -63})
	if got := captureDemoPlayer(g); got != (wolfDemoPlayerState{X: 267175, Y: 229376, Angle: 0, AngleFrac: 4}) {
		t.Fatalf("first moving command: %+v", got)
	}
	// Large products must also work on 32-bit wasm hosts.
	if got := wolfDemoFixedByFrac(0xafff, 0xffff); got != 0xaffe {
		t.Fatalf("fixed multiply overflow: %d", got)
	}
}

func TestDemoFractionalTurnsAndStrafing(t *testing.T) {
	g := demoMovementTestGame()
	for i := 0; i < 4; i++ {
		g.moveDemoPlayer(wl6.DemoCommand{ControlX: -1})
	}
	if g.demoPlayback.angle != 0 || g.demoPlayback.angleFrac != -16 {
		t.Fatal("fractional turns lost")
	}
	g.moveDemoPlayer(wl6.DemoCommand{ControlX: -1})
	if g.demoPlayback.angle != 1 || g.demoPlayback.angleFrac != 0 {
		t.Fatal("negative turn did not carry into degrees")
	}
	g.moveDemoPlayer(wl6.DemoCommand{Buttons: demoButtonStrafe, ControlX: 10})
	if g.demoPlayback.angle != 1 || g.demoPlayback.angleFrac != 0 || g.playerY <= 3.5 {
		t.Fatal("strafe turned the player or moved in the wrong direction")
	}
}

func TestDemoCollisionUsesOriginalDoorsAndStaticTiles(t *testing.T) {
	g := demoMovementTestGame()
	setLevelTile(g.level, 4, 3, wl6.Tile{Door: &wl6.Door{}})
	if g.demoPlayerPositionClear(3.7, 3.5) {
		t.Fatal("demo crossed closed door tile")
	}
	g.doorState[3*7+4] = 2
	g.doorOpen[3*7+4] = 1
	if !g.demoPlayerPositionClear(3.7, 3.5) {
		t.Fatal("open door blocked demo movement")
	}
	g.staticSprites = []staticSprite{{x: 4.5, y: 3.5, alive: true, blocking: true}}
	if g.demoPlayerPositionClear(3.7, 3.5) {
		t.Fatal("blocking static tile did not block demo player")
	}
	g.staticSprites = nil
	g.actors = []actorInstance{{x: 4.5, y: 3.5, tileX: 4, tileY: 3, alive: true, blocking: true, shootable: true}}
	if g.demoPlayerPositionClear(3.5, 3.5) {
		t.Fatal("original inclusive actor distance did not block player")
	}
}

func TestDemoConsumesFourTicsPerCommand(t *testing.T) {
	g := demoMovementTestGame()
	g.demoPlayback.demo = &wl6.Demo{Commands: []wl6.DemoCommand{{ControlY: -10}, {ControlY: -10}}}
	g.updateDemoPlayback(3)
	if g.demoPlayback.command != 0 || g.playerX != 3.5 {
		t.Fatal("demo advanced before four tics")
	}
	g.updateDemoPlayback(1)
	if g.demoPlayback.command != 1 || g.playerX <= 3.5 {
		t.Fatal("demo did not advance after four tics")
	}
	x := g.playerX
	g.updateDemoPlayback(0)
	if g.playerX != x {
		t.Fatal("demo advanced without tics")
	}
}

func TestModernDoorsOptionAndDemoOverride(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{Vertical: true}})
	g := testGameWithLevel(level)
	if !g.modernDoorCollisionEnabled() {
		t.Fatal("interactive door assist should default on")
	}
	if g.collides(1.7, 1.5) {
		t.Fatal("modern door used solid tile outside slab")
	}
	g.modernDoors = false
	if !g.collides(1.7, 1.5) {
		t.Fatal("disabling modern doors did not restore solid tile")
	}
	g.playerX, g.playerY = 1.5, 1.5
	g.tryMove(0.3, 0.3)
	if g.playerX != 1.5 || g.playerY != 1.8 {
		t.Fatalf("classic door did not allow sliding: (%f,%f)", g.playerX, g.playerY)
	}
	g.modernDoors = true
	g.demoPlayback = &wolfDemoPlayback{}
	if g.modernDoorCollisionEnabled() || !g.collides(1.7, 1.5) {
		t.Fatal("demo did not override modern door preference")
	}
	if !g.modernDoors {
		t.Fatal("demo overwrote saved door preference")
	}
	g.demoPlayback = nil
	if !g.modernDoorCollisionEnabled() {
		t.Fatal("modern door preference was not restored after demo")
	}
}
