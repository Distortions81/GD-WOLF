package main

import "testing"

func TestGivePointsMatchesOriginalExtraLifeThresholds(t *testing.T) {
	g := &game{}
	g.startNewGame()
	if g.nextExtra != 40000 || g.lives != 3 {
		t.Fatalf("new game threshold/lives = %d/%d, want 40000/3", g.nextExtra, g.lives)
	}
	g.givePoints(39999)
	if g.lives != 3 || g.nextExtra != 40000 {
		t.Fatalf("awarded an early life: lives=%d threshold=%d", g.lives, g.nextExtra)
	}
	g.givePoints(1)
	if g.lives != 4 || g.nextExtra != 80000 {
		t.Fatalf("exact threshold: lives=%d threshold=%d, want 4/80000", g.lives, g.nextExtra)
	}
	g.givePoints(120000)
	if g.score != 160000 || g.lives != 7 || g.nextExtra != 200000 {
		t.Fatalf("multiple thresholds: score=%d lives=%d threshold=%d, want 160000/7/200000", g.score, g.lives, g.nextExtra)
	}
	g.givePoints(200000)
	if g.lives != 9 || g.nextExtra != 400000 {
		t.Fatalf("capped lives lost threshold progress: lives=%d threshold=%d, want 9/400000", g.lives, g.nextExtra)
	}
	g.lives--
	g.givePoints(100)
	if g.lives != 8 {
		t.Fatal("life cap deferred an already-consumed score reward")
	}
	g.resetLevelState()
	if g.nextExtra != 400000 {
		t.Fatal("level reset lost the next extra-life threshold")
	}
}

func TestTreasureAndActorKillsAwardThresholdLives(t *testing.T) {
	g := &game{score: 39900, lives: 3, nextExtra: 40000}
	if !g.applyPickup(pickupCross) {
		t.Fatal("cross pickup rejected")
	}
	if g.score != 40000 || g.treasureCount != 1 || g.lives != 4 || g.lastPlayedSound != soundPickupOneUp {
		t.Fatalf("cross reward: score=%d treasure=%d lives=%d sound=%d", g.score, g.treasureCount, g.lives, g.lastPlayedSound)
	}
	g.score = 79900
	a := actorInstance{kind: actorKindGuard, alive: true, shootable: true, health: 1, scoreValue: 100}
	if !g.damageActor(&a, 1) {
		t.Fatal("actor damage rejected")
	}
	if g.score != 80000 || g.lives != 5 || g.nextExtra != 120000 {
		t.Fatalf("kill reward: score=%d lives=%d threshold=%d, want 80000/5/120000", g.score, g.lives, g.nextExtra)
	}
	g.damageActor(&a, 1)
	if g.score != 80000 || g.lives != 5 {
		t.Fatal("dead actor awarded its reward again")
	}
}

func TestFullHealMatchesOriginalLifeAndTreasureRewards(t *testing.T) {
	for _, startLives := range []int{3, 8, 9} {
		g := &game{health: 5, ammo: 90, lives: startLives, score: 1000, nextExtra: 40000}
		if !g.applyPickup(pickupFullHeal) {
			t.Fatal("full heal pickup rejected")
		}
		if g.health != 100 || g.ammo != 99 || g.lives != minInt(9, startLives+1) || g.treasureCount != 1 {
			t.Fatalf("full heal from %d lives: health=%d ammo=%d lives=%d treasure=%d", startLives, g.health, g.ammo, g.lives, g.treasureCount)
		}
		if g.score != 1000 || g.nextExtra != 40000 {
			t.Fatal("full heal changed score-based rewards")
		}
	}
	// DrawScaleds can collect a bonus after lethal damage in the same command.
	// GetBonus calls HealSelf(99), preserving the recorded dead playstate.
	g := &game{playerDying: true, health: 0}
	g.applyPickup(pickupFullHeal)
	if g.health != 99 || !g.playerDying {
		t.Fatalf("lethal-frame full heal: health=%d dying=%v, want 99/true", g.health, g.playerDying)
	}
}

func TestAmmoPickupDuringKnifeWindupRestoresChosenWeapon(t *testing.T) {
	g := &game{weapon: 0, chosenWeapon: 2, bestWeapon: 2}
	g.updateWeaponAttackWithInput(0, true)
	g.giveAmmo(8)
	if g.weapon != 2 || g.weaponSequence != seqWeaponMachineGun || g.weaponFrameIdx != 0 || g.weaponFrameTics != 6 {
		t.Fatalf("initial knife pickup: weapon=%d sequence=%s frame=%d timer=%d", g.weapon, g.weaponSequence, g.weaponFrameIdx, g.weaponFrameTics)
	}
	g.updateWeaponAttackWithInput(12, false)
	if g.ammo != 7 || g.lastPlayedSound != soundForWeapon(2) {
		t.Fatalf("resumed attack did not fire the selected gun: ammo=%d sound=%d", g.ammo, g.lastPlayedSound)
	}

	g = &game{weapon: 0, chosenWeapon: 2, bestWeapon: 2}
	g.updateWeaponAttackWithInput(6, true)
	g.giveAmmo(8)
	if g.weapon != 0 || g.weaponSequence != seqWeaponKnifeAttack {
		t.Fatal("ammo pickup interrupted a knife attack past its initial frame")
	}
	g.updateWeaponAttackWithInput(18, false)
	if g.attacking || g.weapon != 2 || g.ammo != 8 {
		t.Fatalf("knife recovery: attacking=%v weapon=%d ammo=%d", g.attacking, g.weapon, g.ammo)
	}
}

func TestWeaponPickupSwitchesActiveAttackWithoutRestarting(t *testing.T) {
	g := &game{weapon: 1, chosenWeapon: 1, bestWeapon: 1, ammo: 8}
	g.updateWeaponAttackWithInput(16, true)
	if g.weaponFrameIdx != 2 || g.weaponFrameTics != 2 || g.ammo != 7 {
		t.Fatalf("unexpected pistol setup: frame=%d timer=%d ammo=%d", g.weaponFrameIdx, g.weaponFrameTics, g.ammo)
	}
	g.giveWeapon(3)
	if g.weaponSequence != seqWeaponChainGun || g.weaponFrameIdx != 2 || g.weaponFrameTics != 2 {
		t.Fatalf("chaingun pickup restarted attack: sequence=%s frame=%d timer=%d", g.weaponSequence, g.weaponFrameIdx, g.weaponFrameTics)
	}
	g.updateWeaponAttackWithInput(2, false)
	if g.ammo != 12 || g.lastPlayedSound != soundForWeapon(3) {
		t.Fatalf("chaingun third frame did not fire: ammo=%d sound=%d", g.ammo, g.lastPlayedSound)
	}
}
