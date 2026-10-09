package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func gretelTestGame(difficulty gameDifficulty) *game {
	level := blankLevel(5, 5)
	setLevelTile(level, 2, 2, wl6.Tile{RawInfo: 197, Area: 0})
	g := testGameWithLevel(level)
	g.difficulty = difficulty
	g.actors = g.buildActors()
	g.demoPlayback = &wolfDemoPlayback{}
	g.rng = newWolfRNG(0)
	g.initializeDemoActorTimers()
	return g
}

func TestGretelSpawnMatchesOriginal(t *testing.T) {
	for _, test := range []struct {
		difficulty gameDifficulty
		health     int
	}{{difficultyEasy, 950}, {difficultyMedium, 1050}, {difficultyHard, 1200}} {
		g := gretelTestGame(test.difficulty)
		if len(g.actors) != 1 {
			t.Fatalf("difficulty %d spawned %d actors", test.difficulty, len(g.actors))
		}
		a := &g.actors[0]
		if a.kind != actorKindGretel || a.health != test.health || a.shapenum != 385 || a.dir != 2 || !a.ambush || a.rotate {
			t.Fatalf("Gretel spawn differs from SpawnGretel: %+v", a)
		}
		if a.patrolSpeed != 512.0/65536 || a.chaseSpeed != 1536.0/65536 || g.actorReactionTics(a) != 1 || g.rng.index != 0 {
			t.Fatalf("Gretel speed/reaction consumed unexpected RNG: patrol=%f chase=%f rng=%d", a.patrolSpeed, a.chaseSpeed, g.rng.index)
		}
	}
}

func TestGretelBurstUsesOriginalExitActions(t *testing.T) {
	g := gretelTestGame(difficultyHard)
	a := &g.actors[0]
	a.aiState = actorStateShoot
	g.startActorSequence(a, a.shootSequence(), false)
	shots := 0
	action := func(action AnimAction) {
		if action == animActionFireActor {
			shots++
		}
	}
	g.advanceDemoActorSequence(a, 39, action)
	if shots != 0 || a.shapenum != 390 {
		t.Fatalf("Gretel fired before shoot2 expired: shots=%d shape=%d", shots, a.shapenum)
	}
	g.advanceDemoActorSequence(a, 1, action)
	if shots != 1 {
		t.Fatalf("Gretel first shot count=%d, want1", shots)
	}
	g.advanceDemoActorSequence(a, 50, action)
	if shots != 6 || a.shapenum != 389 {
		t.Fatalf("Gretel six-shot burst: shots=%d shape=%d", shots, a.shapenum)
	}
	g.advanceDemoActorSequence(a, 10, action)
	if a.aiState != actorStateChase || a.sequenceID != seqActorGretelChase || a.rotate || a.shapenum != 385 {
		t.Fatalf("Gretel did not resume nonrotating chase: %+v", a)
	}
}

func TestGretelUsesGuardAccuracyAndFireSound(t *testing.T) {
	g := testGameWithLevel(blankLevel(10, 3))
	g.playerX, g.playerY = 7.5, 1.5
	g.health = 100
	g.playerAreas = []bool{true}
	g.playerMovingFast = true
	g.demoPlayback = &wolfDemoPlayback{}
	g.rng = testRNGForPredicates(rngValueRange(112, 127), rngValueNonZeroDamage)
	seed := g.rng.index
	a := actorInstance{kind: actorKindGretel, x: 1.5, y: 1.5, tileX: 1, tileY: 1, area: 0}
	g.guardTryShoot(&a)
	if g.health != 100 || g.rng.index != seed+1 || g.lastPlayedSound != soundEnemyAttackGuard {
		t.Fatalf("Gretel incorrectly gained Hans's accuracy or sound: health=%d rng=%d sound=%d", g.health, g.rng.index, g.lastPlayedSound)
	}
	a.kind = actorKindBoss
	g.rng = newWolfRNG(seed)
	g.guardTryShoot(&a)
	if g.health >= 100 || g.rng.index != seed+2 {
		t.Fatal("accuracy setup did not distinguish Hans's shorter effective range")
	}
}

func TestGretelDamageHasNoPainAndDropsGoldKey(t *testing.T) {
	g := gretelTestGame(difficultyHard)
	a := &g.actors[0]
	g.damageActor(a, 1)
	if a.aiState != actorStateChase || a.sequenceID != seqActorGretelChase || a.health != 1198 || a.rotate || g.lastPlayedSound != soundEnemyAlertGretel {
		t.Fatalf("Gretel hit response: %+v sound=%d", a, g.lastPlayedSound)
	}
	g.damageActor(a, 1200)
	if a.alive || a.shootable || a.blocking || a.sequenceID != seqActorGretelDeath || g.score != 5000 {
		t.Fatalf("Gretel death state: %+v score=%d", a, g.score)
	}
	if len(g.staticSprites) != 1 || g.staticSprites[0].pickup != pickupKey1 || g.staticSprites[0].x != 2.5 || g.staticSprites[0].y != 2.5 {
		t.Fatalf("Gretel gold-key drop: %+v", g.staticSprites)
	}
	g.advanceDemoActorSequence(a, 15, func(action AnimAction) { g.actorSequenceActionFrame(a, action) })
	if g.lastPlayedSound != soundEnemyDeathGretel || a.shapenum != 394 || g.rng.index != 0 {
		t.Fatalf("Gretel death action: shape=%d sound=%d rng=%d", a.shapenum, g.lastPlayedSound, g.rng.index)
	}
	g.advanceDemoActorSequence(a, 30, func(AnimAction) {})
	if a.shapenum != 392 || a.frameTimer != 0 {
		t.Fatalf("Gretel terminal corpse: shape=%d timer=%d", a.shapenum, a.frameTimer)
	}
}

func TestSharewareDemoDeathSoundsUseOriginalRandomDraws(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	for seed := 0; seed < 256; seed++ {
		g := &game{files: files, demoPlayback: &wolfDemoPlayback{}, rng: newWolfRNG(byte(seed)), mapIndex: 9}
		want := []soundID{soundEnemyDeathGuard, soundEnemyDeathGuard2}[int(wolfRandomTable[byte(seed+1)])%2]
		if got := g.enemyDeathSound(&actorInstance{kind: actorKindGuard}); got != want || g.rng.index != byte(seed+1) {
			t.Fatalf("shareware guard seed%d: sound=%d rng=%d, want %d/%d", seed, got, g.rng.index, want, byte(seed+1))
		}
	}
	g := &game{files: files, demoPlayback: &wolfDemoPlayback{}, rng: newWolfRNG(0), mapIndex: 9}
	if got := g.enemyDeathSound(&actorInstance{kind: actorKindMutant}); got != soundEnemyDeathMutant || g.rng.index != 0 {
		t.Fatalf("mutant sound/RNG=%d/%d, want AHHHGSND/no draw", got, g.rng.index)
	}
}

func TestDemoDamageDoesNotPlaySyntheticHurtSound(t *testing.T) {
	g := &game{health: 100, demoPlayback: &wolfDemoPlayback{}, lastPlayedSound: soundPickupChaingun}
	g.takePlayerDamage(5)
	if g.health != 95 || g.lastPlayedSound != soundPickupChaingun {
		t.Fatalf("demo damage health=%d sound=%d, want95/unchanged", g.health, g.lastPlayedSound)
	}
}
