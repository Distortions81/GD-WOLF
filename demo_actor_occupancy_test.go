package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func actorGridTestGame() *game {
	g := testGameWithLevel(blankLevel(8, 8))
	for i := range g.level.Tiles {
		g.level.Tiles[i] = wl6.Tile{RawWall: 107, Area: 0}
	}
	g.demoPlayback = &wolfDemoPlayback{}
	g.playerX, g.playerY = 6.5, 6.5
	return g
}

func TestDemoActorGridDoorOpeningClearsReservation(t *testing.T) {
	g := actorGridTestGame()
	setLevelTile(g.level, 3, 2, wl6.Tile{RawWall: 90, Door: &wl6.Door{}, Area: 0})
	g.actors = []actorInstance{{kind: actorKindOfficer, x: 3.5, y: 3.5, tileX: 3, tileY: 3, shootable: true, alive: true}}
	g.initializeDemoActorPool()
	// DoActor clears the prior reservation before starting to wait for a door.
	g.beginDemoActor(&g.actors[0])
	g.actors[0].tileY = 2
	// It ends the command by overwriting the door's solid tag.
	g.finishDemoActor(&g.actors[0])
	if g.demoActorGridPlayerPositionClear(3.5, 4.25) {
		t.Fatal("reserved actor within MINDIST did not block player")
	}
	// DoorOpening clears actorat even when it contains an actor pointer.
	g.setDemoDoorActorMark(3, 2, false)
	if !g.demoActorGridPlayerPositionClear(3.5, 4.25) {
		t.Fatal("player was blocked by an actor whose reservation was cleared")
	}
}

func TestDemoActorGridChecksShootableFlag(t *testing.T) {
	g := actorGridTestGame()
	g.actors = []actorInstance{{kind: actorKindVictoryBJ, x: 3.5, y: 3.5, tileX: 3, tileY: 3, alive: true, blocking: true}}
	g.initializeDemoActorPool()
	walker := actorInstance{kind: actorKindGuard}
	for _, shootable := range []bool{false, true} {
		g.actors[0].shootable = shootable
		cardinal, wait := g.actorCardinalTilePassable(&walker, 3, 3)
		if cardinal == shootable || wait || g.actorDiagTilePassable(&walker, 3, 3) == shootable {
			t.Fatalf("shootable=%v: cardinal=%v waiting=%v", shootable, cardinal, wait)
		}
		if g.demoActorGridAt(3, 3) == nil {
			t.Fatal("nonshootable actors must remain visible to CloseDoor")
		}
	}
}

func TestDemoActorGridRetainsAndReusesStalePointers(t *testing.T) {
	g := actorGridTestGame()
	g.actors = []actorInstance{{kind: actorKindNeedle, x: 2.5, y: 2.5, tileX: 2, tileY: 2, alive: true}}
	g.initializeDemoActorPool()
	a := &g.actors[0]
	slot := a.poolSlot
	g.beginDemoActor(a) // FL_NONMARK leaves the old cell intact.
	a.x, a.tileX = 3.5, 3
	g.finishDemoActor(a)
	if g.demoActorGridAt(2, 2) != a || g.demoActorGridAt(3, 2) != a {
		t.Fatal("moving NONMARK actor lost one of its reservations")
	}
	a.removed = true
	g.finishDemoActor(a)
	g.pruneRemovedActors()
	if got := g.demoActorGridAt(2, 2); got == nil || got.x != 3.5 {
		t.Fatal("RemoveObj cleared retained object memory")
	}
	// The old grid pointers now alias the next allocation in the same slot.
	g.queueDemoActorSpawn(actorInstance{kind: actorKindGuard, x: 2.5, y: 2.5, tileX: 6, tileY: 6, shootable: true, alive: true}, false)
	g.drainActorSpawns()
	if g.actors[0].poolSlot != slot || g.demoActorGridAt(3, 2) != &g.actors[0] {
		t.Fatal("free-list reuse did not retain pointer aliasing")
	}
	if g.demoActorGridPlayerPositionClear(2.5, 3.5) {
		t.Fatal("recycled shootable actor was not reached through stale grid pointer")
	}
}

func TestDemoActorPoolPreservesInertSlotsAndOrder(t *testing.T) {
	g := actorGridTestGame()
	setLevelTile(g.level, 1, 1, wl6.Tile{RawWall: 107, RawInfo: 124, Area: 0})
	g.actors = []actorInstance{{kind: actorKindGuard, x: 3.5, y: 3.5, tileX: 3, tileY: 3, alive: true, shootable: true}}
	g.initializeDemoActorPool()
	if g.actors[0].poolSlot != 2 || g.demoActorGridAt(1, 1).kind != -1 {
		t.Fatal("SpawnDeadGuard did not consume the first actor slot")
	}
	g.queueDemoActorSpawn(actorInstance{kind: actorKindSmoke, alive: true}, false)
	g.drainActorSpawns()
	g.actors[0].removed = true
	g.finishDemoActor(&g.actors[0])
	g.queueDemoActorSpawn(actorInstance{kind: actorKindNeedle, alive: true}, false)
	g.drainActorSpawns()
	g.pruneRemovedActors()
	want := []int{1, 3, 2}
	for i, slot := range g.demoActorPool.order {
		if slot != want[i] {
			t.Fatalf("linked allocation order=%v, want%v", g.demoActorPool.order, want)
		}
	}
	if g.demoActorPool.slots[2].index != 1 || g.demoActorPool.slots[3].index != 0 {
		t.Fatal("slice compaction changed stable pool pointers")
	}
}

func TestDemoActorPoolExhaustionStopsPlayback(t *testing.T) {
	g := actorGridTestGame()
	g.initializeDemoActorPool()
	for i := 1; i < demoMaxActors; i++ {
		g.queueDemoActorSpawn(actorInstance{kind: actorKindSmoke}, false)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("allocation beyond the original MAXACTORS did not stop playback")
		}
	}()
	g.queueDemoActorSpawn(actorInstance{kind: actorKindSmoke}, false)
}

func TestDemoActorGridPushWallAndElevatorWrites(t *testing.T) {
	g := actorGridTestGame()
	setLevelTile(g.level, 2, 2, wl6.Tile{RawWall: 90, Solid: true, Door: &wl6.Door{Vertical: true}})
	setLevelTile(g.level, 2, 3, wl6.Tile{RawWall: 1, RawInfo: pushableTile, Solid: true})
	setLevelTile(g.level, 5, 2, wl6.Tile{RawWall: wolfElevatorTile, Solid: true})
	g.initializeDemoTileMap()
	g.initializeDemoActorPool()
	if !g.startPushWall(2, 3, 0, 1) {
		t.Fatal("could not start pushwall")
	}
	if origin, leading := g.demoActorTagAt(2, 3), g.demoActorTagAt(2, 4); origin != 1 || leading != 0x41 {
		t.Fatalf("initial pushwall actorat origin/leading=%d/%d, want1/65", origin, leading)
	}
	g.updateDemoPushWall(128)
	if g.demoActorTagAt(2, 3) != 0 || g.demoActorTagAt(2, 4) != 0x41 || g.demoActorTagAt(2, 5) != 1 {
		t.Fatal("pushwall crossing did not preserve current tag and mark next tile")
	}
	g.updateDemoPushWall(128)
	if g.demoActorTagAt(2, 4) != 0 || g.demoActorTagAt(2, 5) != 1 {
		t.Fatal("finished pushwall did not clear its trailing tile")
	}
	g.flipElevatorSwitch(5, 2)
	if g.demoTileMapAt(5, 2) != wolfElevatorUsedTile || g.demoActorTagAt(5, 2) != wolfElevatorTile {
		t.Fatal("elevator must flip tilemap without changing actorat")
	}
}

func TestDemoActorGridClosingDoorIgnoresNeighborJambBit(t *testing.T) {
	g := actorGridTestGame()
	setLevelTile(g.level, 2, 2, wl6.Tile{RawWall: 90, Solid: true, Door: &wl6.Door{Vertical: true}})
	setLevelTile(g.level, 3, 2, wl6.Tile{RawWall: 91, Solid: true, Door: &wl6.Door{}})
	g.initializeDemoTileMap()
	g.initializeDemoActorPool()
	i := 2*g.levelWidth + 2
	g.doorState[i], g.doorOpen[i] = 3, 1
	g.setDemoDoorActorMark(2, 2, true)
	if g.demoTileMapAt(2, 2) != 0xc0 || g.demoActorTagAt(2, 2) != 0x80 {
		t.Fatal("adjacent SpawnDoor did not leave separate tilemap and actorat bytes")
	}
	g.updateDoors(4)
	if g.doorState[i] != 3 || g.doorOpen[i] >= 1 {
		t.Fatal("DoorClosing reopened after comparing a jamb-marked tilemap byte")
	}
}
