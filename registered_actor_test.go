package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func TestRegisteredBossSpawnPreservesRawAmbushArea(t *testing.T) {
	for _, info := range []uint16{214, 197, 196, 215, 179, 160, 178, 224, 225, 226, 227} {
		level := blankLevel(5, 5)
		setLevelTile(level, 2, 2, wl6.Tile{RawWall: 106, RawInfo: info, Area: -1, Ambush: true})
		g := testGameWithLevel(level)
		g.difficulty = difficultyHard
		actors := g.buildActors()
		if len(actors) != 1 || actors[0].area != 255 {
			t.Fatalf("spawn info%d: actors=%+v, want original unsigned-byte area255", info, actors)
		}
	}
}

func TestDemoKnifeHitMakesNoiseEvenWithZeroDamage(t *testing.T) {
	g := gretelTestGame(difficultyHard)
	g.weapon = 0
	g.demoPlayback.projections = []demoActorProjection{{visible: true, viewX: demoViewWidth/2 - 1, transX: 65536}}
	g.rng = testRNGForPredicates(rngValueRange(0, 15))
	g.shootDemoAhead()
	if !g.madeNoise || len(g.demoPlayback.hits) != 1 || g.demoPlayback.hits[0].damage != 0 {
		t.Fatalf("zero-damage knife hit: noise=%v hits=%+v", g.madeNoise, g.demoPlayback.hits)
	}
	g.madeNoise = false
	g.demoPlayback.projections[0].visible = false
	g.shootDemoAhead()
	if g.madeNoise {
		t.Fatal("knife miss made noise")
	}
}

func TestProjectileBlockingPrecedesPlayerHit(t *testing.T) {
	g := testGameWithLevel(blankLevel(5, 5))
	g.demoPlayback = &wolfDemoPlayback{}
	g.playerX, g.playerY = 2.5, 2.5
	g.health, g.rng = 100, newWolfRNG(0)
	setLevelTile(g.level, 2, 2, wl6.Tile{RawWall: 1, Solid: true})
	a := actorInstance{kind: actorKindNeedle, x: 2, y: 2.5, angle: 0, projectileSpeed: 0x2000, aiState: actorStateProjectile}
	g.updateActorProjectile(&a, 4)
	if !a.removed || g.health != 100 || g.rng.index != 0 {
		t.Fatalf("projectile wall priority: removed=%v health=%d rng=%d", a.removed, g.health, g.rng.index)
	}
}

func TestDemoActorPartialStepUsesInclusivePlayerBoundary(t *testing.T) {
	for _, kind := range []ActorKind{actorKindGuard, actorKindGhost} {
		g := testGameWithLevel(blankLevel(8, 5))
		g.demoPlayback = &wolfDemoPlayback{}
		g.playerX, g.playerY, g.health = 4.5, 2.5, 100
		g.playerAreas = []bool{true}
		a := actorInstance{kind: kind, x: 3.25, y: 2.5, dir: 0, area: 0, moveDistance: 0.75, hasGoal: true, tileX: 4, tileY: 2, actionTics: 4}
		consumed, reached, blocked := g.moveDemoActorTowardGoalStep(&a, 0.25)
		wantHealth := 100
		if kind == actorKindGhost {
			wantHealth = 92
		}
		if consumed != 0 || reached || !blocked || a.x != 3.25 || a.moveDistance != 0.75 || g.health != wantHealth {
			t.Fatalf("boundary kind%d: actor=%+v consumed=%g reached=%v blocked=%v health=%d", kind, a, consumed, reached, blocked, g.health)
		}
	}
}

func TestDemoActorFullStepSnapsWithoutPlayerCollision(t *testing.T) {
	g := testGameWithLevel(blankLevel(8, 5))
	g.demoPlayback = &wolfDemoPlayback{}
	g.playerX, g.playerY, g.health = 4.5, 2.5, 100
	g.playerAreas = []bool{true}
	a := actorInstance{kind: actorKindGhost, x: 3.25, y: 2.5, dir: 0, area: 0, moveDistance: 0.25, hasGoal: true, tileX: 3, tileY: 2, actionTics: 4}
	consumed, reached, blocked := g.moveDemoActorTowardGoalStep(&a, 0.25)
	if consumed != 0.25 || !reached || blocked || a.x != 3.5 || g.health != 100 {
		t.Fatalf("tile snap: actor=%+v consumed=%g reached=%v blocked=%v health=%d", a, consumed, reached, blocked, g.health)
	}
}
