package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestDemoUseDirectionMatchesOriginalAngleBoundaries(t *testing.T) {
	for _, test := range []struct {
		angle, dx, dy int
	}{
		{0, 1, 0}, {44, 1, 0}, {45, 0, -1}, {134, 0, -1},
		{135, -1, 0}, {224, -1, 0}, {225, 0, 1}, {315, 0, 1},
		{316, 1, 0}, {359, 1, 0},
	} {
		g := &game{demoPlayback: &wolfDemoPlayback{angle: test.angle}, playerA: -float64(test.angle) * math.Pi / 180}
		dx, dy := g.cardinalUseVector()
		if dx != test.dx || dy != test.dy || g.canUseElevatorSwitch() != (test.dx != 0) {
			t.Fatalf("angle %d: direction (%d,%d) elevator=%v, want (%d,%d)", test.angle, dx, dy, g.canUseElevatorSwitch(), test.dx, test.dy)
		}
	}
}

func TestDemoMovementChecksVictoryTileAfterEachThrust(t *testing.T) {
	g := demoMovementTestGame()
	g.playerX, g.playerY = 3.7, 3.7
	setLevelTile(g.level, 3, 4, wl6.Tile{RawInfo: wolfExitTile})
	g.moveDemoPlayer(wl6.DemoCommand{Buttons: demoButtonStrafe, ControlX: 63, ControlY: -63})
	if int(g.playerX) != 4 || int(g.playerY) != 4 {
		t.Fatalf("combined movement ended at (%f,%f), want tile (4,4)", g.playerX, g.playerY)
	}
	if !g.victoryActive {
		t.Fatal("strafe crossed exit tile without starting victory")
	}
}

func TestDemoElevatorEndsPlaybackWithoutLevelTransition(t *testing.T) {
	for _, test := range []struct {
		name  string
		floor uint16
		state int
	}{{"normal", 108, 1}, {"secret", wolfAltElevatorTile, 9}} {
		t.Run(test.name, func(t *testing.T) {
			tmp := withTempWorkingDir(t)
			g := demoMovementTestGame()
			g.mapData = &wl6.MapData{
				Header: wl6.MapHeader{Width: 7, Height: 7},
				Planes: [2][]uint16{make([]uint16, 49), make([]uint16, 49)},
			}
			g.playerX, g.playerY = 3.5, 3.5
			setLevelTile(g.level, 3, 3, wl6.Tile{RawWall: test.floor})
			setLevelTile(g.level, 4, 3, wl6.Tile{RawWall: wolfElevatorTile, Solid: true, RenderWall: true})
			g.demoPlayback.demo = &wl6.Demo{Commands: []wl6.DemoCommand{
				{Buttons: demoButtonUse, ControlY: 20}, {ControlY: 20},
			}}
			g.updateDemoPlayback(8)
			if g.demoPlayback.levelExit != test.state || g.demoPlayback.command != 1 {
				t.Fatalf("elevator terminal=%d commands=%d, want %d/1", g.demoPlayback.levelExit, g.demoPlayback.command, test.state)
			}
			if g.level.Tile(4, 3).RawWall != wolfElevatorUsedTile {
				t.Fatal("demo did not flip the elevator switch")
			}
			if g.playerX >= 3.5 {
				t.Fatal("elevator skipped movement remaining in the terminal command")
			}
			x := g.playerX
			g.updateDemoPlayback(4)
			if g.playerX != x || g.demoPlayback.command != 1 {
				t.Fatal("elevator consumed an additional recorded command")
			}
			if g.fadePhase != 0 || g.fadeAction != nil || g.mapIndex != 0 {
				t.Fatal("demo elevator started an interactive level transition")
			}
			if _, err := os.Stat(filepath.Join(tmp, autosaveSlotPath(0))); !os.IsNotExist(err) {
				t.Fatalf("demo elevator wrote an autosave: %v", err)
			}
		})
	}
}

func TestDemoVictoryPreservesAttackStateAndContinuesCommands(t *testing.T) {
	g := demoMovementTestGame()
	g.health, g.ammo, g.weapon = 100, 8, 1
	g.attacking = true
	g.weaponSequence = seqWeaponPistol
	g.weaponFrameIdx, g.weaponFrameTics = 1, 1
	setLevelTile(g.level, 4, 3, wl6.Tile{RawInfo: wolfExitTile})
	g.demoPlayback.demo = &wl6.Demo{Commands: []wl6.DemoCommand{{ControlY: -63}, {ControlY: -63}}}
	g.updateDemoPlayback(8)
	if !g.victoryActive || g.demoPlayback.command != 2 || g.ammo != 8 || g.demoPlayback.shots != 0 || !g.attacking || g.weaponFrameIdx != 1 || g.weaponFrameTics != 1 {
		t.Fatalf("victory: active=%v commands=%d ammo=%d shots=%d", g.victoryActive, g.demoPlayback.command, g.ammo, g.demoPlayback.shots)
	}
	g.takePlayerDamage(200)
	if g.health != 100 || g.playerDying {
		t.Fatal("victory player took damage")
	}
}
