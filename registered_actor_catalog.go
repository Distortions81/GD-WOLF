package main

// Registered Wolf3D states from WL_ACT2.C; numeric shapes follow WL_DEF.H.
const (
	seqActorSchabbsStand AnimSequenceID = "actor.schabb.stand"
	seqActorSchabbsChase AnimSequenceID = "actor.schabb.chase"
	seqActorSchabbsShoot AnimSequenceID = "actor.schabb.shoot"
	seqActorSchabbsDeath AnimSequenceID = "actor.schabb.death"
	seqActorGiftStand    AnimSequenceID = "actor.gift.stand"
	seqActorGiftChase    AnimSequenceID = "actor.gift.chase"
	seqActorGiftShoot    AnimSequenceID = "actor.gift.shoot"
	seqActorGiftDeath    AnimSequenceID = "actor.gift.death"
	seqActorFatStand     AnimSequenceID = "actor.fat.stand"
	seqActorFatChase     AnimSequenceID = "actor.fat.chase"
	seqActorFatShoot     AnimSequenceID = "actor.fat.shoot"
	seqActorFatDeath     AnimSequenceID = "actor.fat.death"
	seqActorFakeStand    AnimSequenceID = "actor.fake.stand"
	seqActorFakeChase    AnimSequenceID = "actor.fake.chase"
	seqActorFakeShoot    AnimSequenceID = "actor.fake.shoot"
	seqActorFakeDeath    AnimSequenceID = "actor.fake.death"
	seqActorMechaStand   AnimSequenceID = "actor.mecha.stand"
	seqActorMechaChase   AnimSequenceID = "actor.mecha.chase"
	seqActorMechaShoot   AnimSequenceID = "actor.mecha.shoot"
	seqActorMechaDeath   AnimSequenceID = "actor.mecha.death"
	seqActorHitlerChase  AnimSequenceID = "actor.hitler.chase"
	seqActorHitlerShoot  AnimSequenceID = "actor.hitler.shoot"
	seqActorHitlerDeath  AnimSequenceID = "actor.hitler.death"
	seqActorBlinkyChase  AnimSequenceID = "actor.blinky.chase"
	seqActorClydeChase   AnimSequenceID = "actor.clyde.chase"
	seqActorPinkyChase   AnimSequenceID = "actor.pinky.chase"
	seqActorInkyChase    AnimSequenceID = "actor.inky.chase"
	seqActorNeedle       AnimSequenceID = "actor.projectile.needle"
	seqActorRocket       AnimSequenceID = "actor.projectile.rocket"
	seqActorFire         AnimSequenceID = "actor.projectile.fire"
	seqActorSmoke        AnimSequenceID = "actor.projectile.smoke"
	seqActorBoom         AnimSequenceID = "actor.projectile.boom"
)

func init() {
	animSequences[seqActorSchabbsStand] = AnimSequence{ID: seqActorSchabbsStand, Loop: true, Frames: []AnimFrame{
		{Shape: 307, Tics: 0}, // s_schabbstand
	}}
	animSequences[seqActorSchabbsChase] = AnimSequence{ID: seqActorSchabbsChase, Loop: true, Frames: []AnimFrame{
		{Shape: 307, Tics: 10}, // s_schabbchase1
		{Shape: 307, Tics: 3},  // s_schabbchase1s
		{Shape: 308, Tics: 8},  // s_schabbchase2
		{Shape: 309, Tics: 10}, // s_schabbchase3
		{Shape: 309, Tics: 3},  // s_schabbchase3s
		{Shape: 310, Tics: 8},  // s_schabbchase4
	}}
	animSequences[seqActorSchabbsShoot] = AnimSequence{ID: seqActorSchabbsShoot, Loop: false, Frames: []AnimFrame{
		{Shape: 311, Tics: 30},                                // s_schabbshoot1
		{Shape: 312, Tics: 10, Action: animActionThrowNeedle}, // s_schabbshoot2
	}}
	animSequences[seqActorSchabbsDeath] = AnimSequence{ID: seqActorSchabbsDeath, Loop: false, Frames: []AnimFrame{
		{Shape: 307, Tics: 10, Action: animActionDeathScream},  // s_schabbdie1
		{Shape: 307, Tics: 10},                                 // s_schabbdie2
		{Shape: 313, Tics: 10},                                 // s_schabbdie3
		{Shape: 314, Tics: 10},                                 // s_schabbdie4
		{Shape: 315, Tics: 10},                                 // s_schabbdie5
		{Shape: 316, Tics: 20, Action: animActionBossDeathCam}, // s_schabbdie6
	}}
	animSequences[seqActorGiftStand] = AnimSequence{ID: seqActorGiftStand, Loop: true, Frames: []AnimFrame{
		{Shape: 360, Tics: 0}, // s_giftstand
	}}
	animSequences[seqActorGiftChase] = AnimSequence{ID: seqActorGiftChase, Loop: true, Frames: []AnimFrame{
		{Shape: 360, Tics: 10}, // s_giftchase1
		{Shape: 360, Tics: 3},  // s_giftchase1s
		{Shape: 361, Tics: 8},  // s_giftchase2
		{Shape: 362, Tics: 10}, // s_giftchase3
		{Shape: 362, Tics: 3},  // s_giftchase3s
		{Shape: 363, Tics: 8},  // s_giftchase4
	}}
	animSequences[seqActorGiftShoot] = AnimSequence{ID: seqActorGiftShoot, Loop: false, Frames: []AnimFrame{
		{Shape: 364, Tics: 30},                                // s_giftshoot1
		{Shape: 365, Tics: 10, Action: animActionThrowRocket}, // s_giftshoot2
	}}
	animSequences[seqActorGiftDeath] = AnimSequence{ID: seqActorGiftDeath, Loop: false, Frames: []AnimFrame{
		{Shape: 360, Tics: 1, Action: animActionDeathScream},   // s_giftdie1
		{Shape: 360, Tics: 10},                                 // s_giftdie2
		{Shape: 366, Tics: 10},                                 // s_giftdie3
		{Shape: 367, Tics: 10},                                 // s_giftdie4
		{Shape: 368, Tics: 10},                                 // s_giftdie5
		{Shape: 369, Tics: 20, Action: animActionBossDeathCam}, // s_giftdie6
	}}
	animSequences[seqActorFatStand] = AnimSequence{ID: seqActorFatStand, Loop: true, Frames: []AnimFrame{
		{Shape: 396, Tics: 0}, // s_fatstand
	}}
	animSequences[seqActorFatChase] = AnimSequence{ID: seqActorFatChase, Loop: true, Frames: []AnimFrame{
		{Shape: 396, Tics: 10}, // s_fatchase1
		{Shape: 396, Tics: 3},  // s_fatchase1s
		{Shape: 397, Tics: 8},  // s_fatchase2
		{Shape: 398, Tics: 10}, // s_fatchase3
		{Shape: 398, Tics: 3},  // s_fatchase3s
		{Shape: 399, Tics: 8},  // s_fatchase4
	}}
	animSequences[seqActorFatShoot] = AnimSequence{ID: seqActorFatShoot, Loop: false, Frames: []AnimFrame{
		{Shape: 400, Tics: 30},                                // s_fatshoot1
		{Shape: 401, Tics: 10, Action: animActionThrowRocket}, // s_fatshoot2
		{Shape: 402, Tics: 10, Action: animActionFireActor},   // s_fatshoot3
		{Shape: 403, Tics: 10, Action: animActionFireActor},   // s_fatshoot4
		{Shape: 402, Tics: 10, Action: animActionFireActor},   // s_fatshoot5
		{Shape: 403, Tics: 10, Action: animActionFireActor},   // s_fatshoot6
	}}
	animSequences[seqActorFatDeath] = AnimSequence{ID: seqActorFatDeath, Loop: false, Frames: []AnimFrame{
		{Shape: 396, Tics: 1, Action: animActionDeathScream},   // s_fatdie1
		{Shape: 396, Tics: 10},                                 // s_fatdie2
		{Shape: 404, Tics: 10},                                 // s_fatdie3
		{Shape: 405, Tics: 10},                                 // s_fatdie4
		{Shape: 406, Tics: 10},                                 // s_fatdie5
		{Shape: 407, Tics: 20, Action: animActionBossDeathCam}, // s_fatdie6
	}}
	animSequences[seqActorFakeStand] = AnimSequence{ID: seqActorFakeStand, Loop: true, Frames: []AnimFrame{
		{Shape: 321, Tics: 0}, // s_fakestand
	}}
	animSequences[seqActorFakeChase] = AnimSequence{ID: seqActorFakeChase, Loop: true, Frames: []AnimFrame{
		{Shape: 321, Tics: 10}, // s_fakechase1
		{Shape: 321, Tics: 3},  // s_fakechase1s
		{Shape: 322, Tics: 8},  // s_fakechase2
		{Shape: 323, Tics: 10}, // s_fakechase3
		{Shape: 323, Tics: 3},  // s_fakechase3s
		{Shape: 324, Tics: 8},  // s_fakechase4
	}}
	animSequences[seqActorFakeShoot] = AnimSequence{ID: seqActorFakeShoot, Loop: false, Frames: []AnimFrame{
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot1
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot2
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot3
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot4
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot5
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot6
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot7
		{Shape: 325, Tics: 8, Action: animActionThrowFire}, // s_fakeshoot8
		{Shape: 325, Tics: 8},                              // s_fakeshoot9
	}}
	animSequences[seqActorFakeDeath] = AnimSequence{ID: seqActorFakeDeath, Loop: false, Frames: []AnimFrame{
		{Shape: 328, Tics: 10, Action: animActionDeathScream}, // s_fakedie1
		{Shape: 329, Tics: 10},                                // s_fakedie2
		{Shape: 330, Tics: 10},                                // s_fakedie3
		{Shape: 331, Tics: 10},                                // s_fakedie4
		{Shape: 332, Tics: 10},                                // s_fakedie5
		{Shape: 333, Tics: 0},                                 // s_fakedie6
	}}
	animSequences[seqActorMechaStand] = AnimSequence{ID: seqActorMechaStand, Loop: true, Frames: []AnimFrame{
		{Shape: 334, Tics: 0}, // s_mechastand
	}}
	animSequences[seqActorMechaChase] = AnimSequence{ID: seqActorMechaChase, Loop: true, Frames: []AnimFrame{
		{Shape: 334, Tics: 10, Action: animActionMechaStep}, // s_mechachase1
		{Shape: 334, Tics: 6},                               // s_mechachase1s
		{Shape: 335, Tics: 8},                               // s_mechachase2
		{Shape: 336, Tics: 10, Action: animActionMechaStep}, // s_mechachase3
		{Shape: 336, Tics: 6},                               // s_mechachase3s
		{Shape: 337, Tics: 8},                               // s_mechachase4
	}}
	animSequences[seqActorMechaShoot] = AnimSequence{ID: seqActorMechaShoot, Loop: false, Frames: []AnimFrame{
		{Shape: 338, Tics: 30},                              // s_mechashoot1
		{Shape: 339, Tics: 10, Action: animActionFireActor}, // s_mechashoot2
		{Shape: 340, Tics: 10, Action: animActionFireActor}, // s_mechashoot3
		{Shape: 339, Tics: 10, Action: animActionFireActor}, // s_mechashoot4
		{Shape: 340, Tics: 10, Action: animActionFireActor}, // s_mechashoot5
		{Shape: 339, Tics: 10, Action: animActionFireActor}, // s_mechashoot6
	}}
	animSequences[seqActorMechaDeath] = AnimSequence{ID: seqActorMechaDeath, Loop: false, Frames: []AnimFrame{
		{Shape: 342, Tics: 10, Action: animActionDeathScream}, // s_mechadie1
		{Shape: 343, Tics: 10},                                // s_mechadie2
		{Shape: 344, Tics: 10, Action: animActionHitlerMorph}, // s_mechadie3
		{Shape: 341, Tics: 0},                                 // s_mechadie4
	}}
	animSequences[seqActorHitlerChase] = AnimSequence{ID: seqActorHitlerChase, Loop: true, Frames: []AnimFrame{
		{Shape: 345, Tics: 6}, // s_hitlerchase1
		{Shape: 345, Tics: 4}, // s_hitlerchase1s
		{Shape: 346, Tics: 2}, // s_hitlerchase2
		{Shape: 347, Tics: 6}, // s_hitlerchase3
		{Shape: 347, Tics: 4}, // s_hitlerchase3s
		{Shape: 348, Tics: 2}, // s_hitlerchase4
	}}
	animSequences[seqActorHitlerShoot] = AnimSequence{ID: seqActorHitlerShoot, Loop: false, Frames: []AnimFrame{
		{Shape: 349, Tics: 30},                              // s_hitlershoot1
		{Shape: 350, Tics: 10, Action: animActionFireActor}, // s_hitlershoot2
		{Shape: 351, Tics: 10, Action: animActionFireActor}, // s_hitlershoot3
		{Shape: 350, Tics: 10, Action: animActionFireActor}, // s_hitlershoot4
		{Shape: 351, Tics: 10, Action: animActionFireActor}, // s_hitlershoot5
		{Shape: 350, Tics: 10, Action: animActionFireActor}, // s_hitlershoot6
	}}
	animSequences[seqActorHitlerDeath] = AnimSequence{ID: seqActorHitlerDeath, Loop: false, Frames: []AnimFrame{
		{Shape: 345, Tics: 1, Action: animActionDeathScream},   // s_hitlerdie1
		{Shape: 345, Tics: 10},                                 // s_hitlerdie2
		{Shape: 353, Tics: 10, Action: animActionSlurpie},      // s_hitlerdie3
		{Shape: 354, Tics: 10},                                 // s_hitlerdie4
		{Shape: 355, Tics: 10},                                 // s_hitlerdie5
		{Shape: 356, Tics: 10},                                 // s_hitlerdie6
		{Shape: 357, Tics: 10},                                 // s_hitlerdie7
		{Shape: 358, Tics: 10},                                 // s_hitlerdie8
		{Shape: 359, Tics: 10},                                 // s_hitlerdie9
		{Shape: 352, Tics: 20, Action: animActionBossDeathCam}, // s_hitlerdie10
	}}
	animSequences[seqActorBlinkyChase] = AnimSequence{ID: seqActorBlinkyChase, Loop: true, Frames: []AnimFrame{
		{Shape: 288, Tics: 10}, // s_blinkychase1
		{Shape: 289, Tics: 10}, // s_blinkychase2
	}}
	animSequences[seqActorClydeChase] = AnimSequence{ID: seqActorClydeChase, Loop: true, Frames: []AnimFrame{
		{Shape: 292, Tics: 10}, // s_clydechase1
		{Shape: 293, Tics: 10}, // s_clydechase2
	}}
	animSequences[seqActorPinkyChase] = AnimSequence{ID: seqActorPinkyChase, Loop: true, Frames: []AnimFrame{
		{Shape: 290, Tics: 10}, // s_pinkychase1
		{Shape: 291, Tics: 10}, // s_pinkychase2
	}}
	animSequences[seqActorInkyChase] = AnimSequence{ID: seqActorInkyChase, Loop: true, Frames: []AnimFrame{
		{Shape: 294, Tics: 10}, // s_inkychase1
		{Shape: 295, Tics: 10}, // s_inkychase2
	}}
	animSequences[seqActorNeedle] = AnimSequence{ID: seqActorNeedle, Loop: true, Frames: []AnimFrame{
		{Shape: 317, Tics: 6}, // s_needle1
		{Shape: 318, Tics: 6}, // s_needle2
		{Shape: 319, Tics: 6}, // s_needle3
		{Shape: 320, Tics: 6}, // s_needle4
	}}
	animSequences[seqActorRocket] = AnimSequence{ID: seqActorRocket, Loop: true, Frames: []AnimFrame{
		{Shape: 370, Tics: 3, Action: animActionSmoke}, // s_rocket
	}}
	animSequences[seqActorFire] = AnimSequence{ID: seqActorFire, Loop: true, Frames: []AnimFrame{
		{Shape: 326, Tics: 6, Action: animActionMoveProjectile}, // s_fire1
		{Shape: 327, Tics: 6, Action: animActionMoveProjectile}, // s_fire2
	}}
	animSequences[seqActorSmoke] = AnimSequence{ID: seqActorSmoke, Loop: false, Frames: []AnimFrame{
		{Shape: 378, Tics: 3}, // s_smoke1
		{Shape: 379, Tics: 3}, // s_smoke2
		{Shape: 380, Tics: 3}, // s_smoke3
		{Shape: 381, Tics: 3}, // s_smoke4
	}}
	animSequences[seqActorBoom] = AnimSequence{ID: seqActorBoom, Loop: false, Frames: []AnimFrame{
		{Shape: 382, Tics: 6}, // s_boom1
		{Shape: 383, Tics: 6}, // s_boom2
		{Shape: 384, Tics: 6}, // s_boom3
	}}
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindSchabbs, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 196, InfoEnd: 196, BaseShape: 307, Blocking: true, Shootable: true, HitPoints: 2400, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorSchabbsStand, ChaseSequence: seqActorSchabbsChase, ShootSequence: seqActorSchabbsShoot, DeathSequence: seqActorSchabbsDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindSchabbs, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 196, InfoEnd: 196, BaseShape: 307, Blocking: true, Shootable: true, HitPoints: 1550, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorSchabbsStand, ChaseSequence: seqActorSchabbsChase, ShootSequence: seqActorSchabbsShoot, DeathSequence: seqActorSchabbsDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindSchabbs, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 196, InfoEnd: 196, BaseShape: 307, Blocking: true, Shootable: true, HitPoints: 950, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorSchabbsStand, ChaseSequence: seqActorSchabbsChase, ShootSequence: seqActorSchabbsShoot, DeathSequence: seqActorSchabbsDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGift, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 215, InfoEnd: 215, BaseShape: 360, Blocking: true, Shootable: true, HitPoints: 1200, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorGiftStand, ChaseSequence: seqActorGiftChase, ShootSequence: seqActorGiftShoot, DeathSequence: seqActorGiftDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGift, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 215, InfoEnd: 215, BaseShape: 360, Blocking: true, Shootable: true, HitPoints: 1050, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorGiftStand, ChaseSequence: seqActorGiftChase, ShootSequence: seqActorGiftShoot, DeathSequence: seqActorGiftDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGift, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 215, InfoEnd: 215, BaseShape: 360, Blocking: true, Shootable: true, HitPoints: 950, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorGiftStand, ChaseSequence: seqActorGiftChase, ShootSequence: seqActorGiftShoot, DeathSequence: seqActorGiftDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindFat, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 179, InfoEnd: 179, BaseShape: 396, Blocking: true, Shootable: true, HitPoints: 1200, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorFatStand, ChaseSequence: seqActorFatChase, ShootSequence: seqActorFatShoot, DeathSequence: seqActorFatDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindFat, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 179, InfoEnd: 179, BaseShape: 396, Blocking: true, Shootable: true, HitPoints: 1050, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorFatStand, ChaseSequence: seqActorFatChase, ShootSequence: seqActorFatShoot, DeathSequence: seqActorFatDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindFat, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 179, InfoEnd: 179, BaseShape: 396, Blocking: true, Shootable: true, HitPoints: 950, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorFatStand, ChaseSequence: seqActorFatChase, ShootSequence: seqActorFatShoot, DeathSequence: seqActorFatDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindFake, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 160, InfoEnd: 160, BaseShape: 321, Blocking: true, Shootable: true, HitPoints: 500, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorFakeStand, ChaseSequence: seqActorFakeChase, ShootSequence: seqActorFakeShoot, DeathSequence: seqActorFakeDeath, DropScore: 2000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindFake, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 160, InfoEnd: 160, BaseShape: 321, Blocking: true, Shootable: true, HitPoints: 400, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorFakeStand, ChaseSequence: seqActorFakeChase, ShootSequence: seqActorFakeShoot, DeathSequence: seqActorFakeDeath, DropScore: 2000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindFake, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 160, InfoEnd: 160, BaseShape: 321, Blocking: true, Shootable: true, HitPoints: 300, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorFakeStand, ChaseSequence: seqActorFakeChase, ShootSequence: seqActorFakeShoot, DeathSequence: seqActorFakeDeath, DropScore: 2000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindMecha, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 178, InfoEnd: 178, BaseShape: 334, Blocking: true, Shootable: true, HitPoints: 1200, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorMechaStand, ChaseSequence: seqActorMechaChase, ShootSequence: seqActorMechaShoot, DeathSequence: seqActorMechaDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindMecha, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 178, InfoEnd: 178, BaseShape: 334, Blocking: true, Shootable: true, HitPoints: 1050, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorMechaStand, ChaseSequence: seqActorMechaChase, ShootSequence: seqActorMechaShoot, DeathSequence: seqActorMechaDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindMecha, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 178, InfoEnd: 178, BaseShape: 334, Blocking: true, Shootable: true, HitPoints: 950, PatrolSpeed: 512, ChaseSpeed: 1536, StandSequence: seqActorMechaStand, ChaseSequence: seqActorMechaChase, ShootSequence: seqActorMechaShoot, DeathSequence: seqActorMechaDeath, DropScore: 5000})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGhost, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 224, InfoEnd: 224, BaseShape: 288, PatrolSpeed: 1500, ChaseSpeed: 1500, StandSequence: seqActorBlinkyChase, ChaseSequence: seqActorBlinkyChase})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGhost, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 225, InfoEnd: 225, BaseShape: 292, PatrolSpeed: 1500, ChaseSpeed: 1500, StandSequence: seqActorClydeChase, ChaseSequence: seqActorClydeChase})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGhost, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 226, InfoEnd: 226, BaseShape: 290, PatrolSpeed: 1500, ChaseSpeed: 1500, StandSequence: seqActorPinkyChase, ChaseSequence: seqActorPinkyChase})
	actorDefs = append(actorDefs, ActorDef{Kind: actorKindGhost, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 227, InfoEnd: 227, BaseShape: 294, PatrolSpeed: 1500, ChaseSpeed: 1500, StandSequence: seqActorInkyChase, ChaseSequence: seqActorInkyChase})
}
