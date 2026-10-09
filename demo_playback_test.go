package main

import (
	"gd-wolf/internal/wl6"
	"testing"
)

func TestDemoActorSpawnTimersMatchOriginalFirstMap(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	demo, err := files.LoadDemo(0)
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
	// Golden states from compiled original ScanInfoPlane/SpawnNewObj.
	indices := []int{17, 18, 19, 20, 25, 26, 27, 28, 31}
	remaining := []int{8, 9, 0, 2, 1, 9, 7, 15, 8}
	if g.rng.index != 9 {
		t.Fatalf("initial RNG index = %d, want 9", g.rng.index)
	}
	for i, index := range indices {
		a := &g.actors[index]
		if a.frameTimer != 20-remaining[i] {
			t.Fatalf("actor %d frame timer = %d, want %d", index, a.frameTimer, 20-remaining[i])
		}
	}
	frozen := &g.actors[19]
	g.advanceActorSequence(frozen, 40)
	if frozen.frameIndex != 0 || !frozen.spawnAnimationFrozen {
		t.Fatal("original zero ticcount must hold the initial state")
	}
	g.startActorSequence(frozen, frozen.chaseSequence(), true)
	g.advanceActorSequence(frozen, 4)
	if frozen.spawnAnimationFrozen || frozen.frameTimer != 4 {
		t.Fatal("entering a new state must clear the spawn freeze")
	}
	g.prepareDemoCommand(demo.Commands[0])
	if g.rng.index != 10 || g.demoPlayback.faceCount != 4 {
		t.Fatalf("first player command skipped face RNG: index=%d face count=%d", g.rng.index, g.demoPlayback.faceCount)
	}
}

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
	g.doorState[3*7+4] = 1
	g.updateDoors(64)
	if !g.demoPlayerPositionClear(3.7, 3.5) {
		t.Fatal("open door blocked demo movement")
	}
	g.staticSprites = []staticSprite{{x: 4.5, y: 3.5, alive: true, blocking: true}}
	g.initializeDemoActorPool()
	if g.demoPlayerPositionClear(3.7, 3.5) {
		t.Fatal("blocking static tile did not block demo player")
	}
	g.staticSprites = nil
	g.actors = []actorInstance{{x: 4.5, y: 3.5, tileX: 4, tileY: 3, alive: true, blocking: true, shootable: true}}
	g.initializeDemoActorPool()
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

func TestDemoFinalCommandStillMovesAndCompletes(t *testing.T) {
	g := demoMovementTestGame()
	g.demoPlayback.demo = &wl6.Demo{Commands: []wl6.DemoCommand{{ControlY: -10}}}
	g.updateDemoPlayback(wl6.DemoTics)
	if g.demoPlayback.command != 1 || g.demoPlayback.levelExit != 1 || g.playerX <= 3.5 || g.demoPlayback.timeCount != wl6.DemoTics {
		t.Fatalf("final command was skipped or failed to complete: command=%d exit=%d x=%f time=%d", g.demoPlayback.command, g.demoPlayback.levelExit, g.playerX, g.demoPlayback.timeCount)
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

func TestDemoDoorConnectsAreasOnFirstMovement(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{Vertical: true}, Area: -1})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 1})
	g := testGameWithLevel(level)
	g.playerX, g.playerY = 1.5, 1.5
	g.playerAreas = make([]bool, 2)
	g.demoPlayback = &wolfDemoPlayback{}
	g.openDoorAt(2, 1)
	g.rebuildPlayerAreas()
	if g.doorOpen[7] != 0 || g.playerAreas[1] {
		t.Fatal("door opened or connected its areas before the first movement")
	}
	g.updateDoors(4)
	g.rebuildPlayerAreas()
	if g.doorOpen[7] <= 0 || !g.playerAreas[1] {
		t.Fatal("moving door did not connect its areas")
	}
}

func TestDemoActorShootActionRunsOnExit(t *testing.T) {
	g := &game{}
	a := actorInstance{kind: actorKindGuard, aiState: actorStateShoot, chaseSeq: seqActorGuardChase}
	g.startActorSequence(&a, seqActorGuardShoot, false)
	shots := 0
	action := func(action AnimAction) {
		if action == animActionFireActor {
			shots++
		}
	}
	// Original s_grdshoot1 (20) enters s_grdshoot2 (20) without firing.
	if g.advanceDemoActorSequence(&a, 20, action) || shots != 0 {
		t.Fatal("entering the shoot action frame fired early")
	}
	if g.advanceDemoActorSequence(&a, 19, action) || shots != 0 {
		t.Fatal("shoot action fired before state exit")
	}
	if g.advanceDemoActorSequence(&a, 1, action) || shots != 1 {
		t.Fatal("shoot action did not fire exactly once on state exit")
	}
}

func TestDemoDogCorpseKeepsOriginalTimedSelfLoop(t *testing.T) {
	g := &game{}
	a := actorInstance{kind: actorKindDog, aiState: actorStateDead}
	g.startActorSequence(&a, seqActorDogDeath, false)
	g.advanceDemoActorSequence(&a, 46, func(AnimAction) {})
	if a.frameIndex != 3 || a.frameTimer != 1 {
		t.Fatalf("dog corpse after 46 tics: frame=%d elapsed=%d", a.frameIndex, a.frameTimer)
	}
	g.advanceDemoActorSequence(&a, 14, func(AnimAction) {})
	if a.frameIndex != 3 || a.frameTimer != 0 {
		t.Fatalf("dog corpse self-loop: frame=%d elapsed=%d", a.frameIndex, a.frameTimer)
	}
}

func TestDemoSuppressedAttackPressCanStartNextCommand(t *testing.T) {
	g := demoMovementTestGame()
	g.weapon, g.chosenWeapon, g.ammo = 1, 1, 8
	g.startWeaponSequence()
	g.weaponFrameIdx, g.weaponFrameTics = 3, 4
	command := wl6.DemoCommand{Buttons: demoButtonAttack | demoButtonUse}
	g.stepDemoCommand(command)
	if g.attacking || g.demoPlayback.buttons&(demoButtonAttack|demoButtonUse) != 0 {
		t.Fatal("T_Attack did not suppress the new presses before returning to player state")
	}
	g.stepDemoCommand(command)
	if !g.attacking {
		t.Fatal("suppressed attack press prevented the next command from starting an attack")
	}
}

func TestDemoUseDoesNotSkipDoorOpening(t *testing.T) {
	g := testGameWithLevel(blankLevel(5, 3))
	setLevelTile(g.level, 2, 1, wl6.Tile{Door: &wl6.Door{Vertical: true}})
	g.playerX, g.playerY = 1.5, 1.5
	g.demoPlayback = &wolfDemoPlayback{}
	g.doorState[7], g.doorOpen[7] = 1, 0.25
	g.useDoorAhead()
	if g.doorState[7] != 1 || g.doorOpen[7] != 0.25 {
		t.Fatal("using an opening door skipped its remaining motion")
	}
	g.updateDoors(64)
	g.useDoorAhead()
	if g.doorState[7] != 3 {
		t.Fatal("using an unoccupied open door did not start closing it")
	}
}

func TestDemoPushwallReservesLeadingTile(t *testing.T) {
	g := testGameWithLevel(blankLevel(7, 5))
	g.playerX, g.playerY = 2.5, 2.5
	g.demoPlayback = &wolfDemoPlayback{}
	setLevelTile(g.level, 3, 2, wl6.Tile{RawWall: 10, RawInfo: pushableTile, Solid: true})
	if !g.startPushWall(3, 2, 1, 0) {
		t.Fatal("pushwall did not start")
	}
	if !g.isBlockingTile(3, 2) || !g.isBlockingTile(4, 2) {
		t.Fatal("moving pushwall did not block its current and leading tiles")
	}
	g.updatePushWall(128)
	if g.isBlockingTile(3, 2) || !g.isBlockingTile(4, 2) || !g.isBlockingTile(5, 2) {
		t.Fatal("pushwall reservations did not advance after the first tile crossing")
	}
	g.updatePushWall(128)
	if g.pushWall.active || g.isBlockingTile(4, 2) || !g.isBlockingTile(5, 2) {
		t.Fatal("pushwall did not leave the final wall after two tile crossings")
	}
}

func TestDemoRangeMissDiffersFromZeroDamageHit(t *testing.T) {
	for _, tile := range []int{5, 2} {
		g := demoMovementTestGame()
		g.playerX, g.playerY = 1.5, 3.5
		g.weapon = 1
		g.rng = testRNGForPredicates(rngValueEquals(0))
		g.actors = []actorInstance{{kind: actorKindGuard, x: float64(tile) + 0.5, y: 3.5, tileX: tile, tileY: 3, health: 25, alive: true, shootable: true}}
		g.demoPlayback.projections = []demoActorProjection{{visible: true, viewX: demoViewWidth/2 - 1, transX: (tile - 1) * 65536}}
		g.shootDemoAhead()
		if g.actors[0].health != 25 {
			t.Fatal("zero RNG roll changed target health")
		}
		if tile == 5 && (g.actors[0].alerted || len(g.demoPlayback.hits) != 0) {
			t.Fatal("range miss alerted or damaged the target")
		}
		if tile == 2 && (!g.actors[0].alerted || len(g.demoPlayback.hits) != 1) {
			t.Fatal("real zero-damage hit did not alert the target")
		}
	}
}
