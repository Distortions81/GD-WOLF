package main

import "testing"

func TestDemoPainSpriteUsesOriginalTwoViewRotation(t *testing.T) {
	g := &game{demoPlayback: &wolfDemoPlayback{angle: 183}}
	a := actorInstance{kind: actorKindOfficer, aiState: actorStatePain, dir: 1, shapenum: 282}
	// Captured from registered demo0 command81: pain states use rotate=2
	// even though the interactive actor's rotate flag is cleared on damage.
	if got := g.demoActorRenderShape(&a, 118); got != 286 {
		t.Fatalf("pain view = %d, want original shape286", got)
	}
	g.demoPlayback.angle = 3
	if got := g.demoActorRenderShape(&a, 118); got != 282 {
		t.Fatalf("opposite pain view = %d, want original shape282", got)
	}
}

func TestDemoDeathKeepsOriginalWeaponState(t *testing.T) {
	g := &game{health: 10, weapon: 2, attacking: true, weaponSequence: seqWeaponMachineGun,
		weaponFrameIdx: 2, weaponFrameTics: 3, demoPlayback: &wolfDemoPlayback{}}
	g.beginPlayerDeath(2, 2, true)
	if !g.playerDying || g.health != 0 || !g.attacking || g.weaponSequence != seqWeaponMachineGun || g.weaponFrameIdx != 2 || g.weaponFrameTics != 3 || g.deathPhase != deathPhaseNone {
		t.Fatal("demo death entered interactive Died() or reset its weapon state")
	}
}
