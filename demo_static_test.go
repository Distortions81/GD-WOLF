package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func TestDemoStaticDropReusesFirstCollectedSlot(t *testing.T) {
	g := testGameWithLevel(blankLevel(8, 8))
	g.demoPlayback = &wolfDemoPlayback{}
	setLevelTile(g.level, 1, 1, wl6.Tile{RawInfo: 124})
	g.staticSprites = []staticSprite{
		{x: 1.5, y: 1.5, alive: true, shapenum: shapeGuardDead},
		{x: 2.5, y: 2.5, pickup: pickupFood, alive: false},
		{x: 3.5, y: 3.5, pickup: pickupClip, alive: false},
	}
	g.spawnDroppedPickup(4.25, 5.25, pickupClip2)
	if len(g.staticSprites) != 3 || len(g.demoPlayback.staticSlots) != 2 || g.demoPlayback.staticSlots[0] != 1 {
		t.Fatal("drop appended or corpse consumed an original static slot")
	}
	if drop := g.staticSprites[1]; !drop.alive || drop.x != 4.5 || drop.y != 5.5 || drop.pickup != pickupClip2 {
		t.Fatalf("first free slot was not replaced: %+v", drop)
	}
	if g.staticSprites[2].alive {
		t.Fatal("later free slot was used before the first")
	}
}

func TestDemoStaticPoolDropsNothingWhenFull(t *testing.T) {
	g := testGameWithLevel(blankLevel(8, 8))
	g.demoPlayback = &wolfDemoPlayback{staticSlots: make([]int, demoMaxStatics)}
	g.staticSprites = make([]staticSprite, demoMaxStatics)
	for i := range g.staticSprites {
		g.staticSprites[i] = staticSprite{alive: true, pickup: pickupClip}
		g.demoPlayback.staticSlots[i] = i
	}
	g.spawnDroppedPickup(2.5, 2.5, pickupKey1)
	if len(g.staticSprites) != demoMaxStatics {
		t.Fatal("full original static pool grew")
	}
	g.staticSprites[27].alive = false
	g.spawnDroppedPickup(2.5, 2.5, pickupKey1)
	if len(g.staticSprites) != demoMaxStatics || g.staticSprites[27].pickup != pickupKey1 || !g.staticSprites[27].alive {
		t.Fatal("full pool did not reuse its free slot")
	}
}
