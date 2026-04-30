package main

import "gd-wolf/internal/wl6"

type AnimSequenceID string

type AnimAction int

const (
	animActionNone AnimAction = iota
	animActionFireGun
	animActionFireKnife
	animActionFireActor
	animActionBiteActor
	animActionLoopIfPressed
	animActionLoopFireIfPressed
	animActionEnd
)

type AnimFrame struct {
	Shape  int
	Tics   int
	Action AnimAction
}

type AnimSequence struct {
	ID     AnimSequenceID
	Loop   bool
	Frames []AnimFrame
}

type StaticDef struct {
	Info     uint16
	Shape    int
	Blocking bool
	Pickup   pickupType
}

type ActorDef struct {
	Kind      ActorKind
	SpawnMode ActorSpawnMode

	InfoStart     uint16
	InfoEnd       uint16
	MinDifficulty gameDifficulty
	BaseShape     int
	Rotate        bool
	Blocking      bool
	Shootable     bool
	HitPoints     int
	PatrolSpeed   int
	ChaseSpeed    int

	StandSequence  AnimSequenceID
	PatrolSequence AnimSequenceID
	ChaseSequence  AnimSequenceID
	PainSequence   AnimSequenceID
	ShootSequence  AnimSequenceID
	JumpSequence   AnimSequenceID
	DeathSequence  AnimSequenceID

	DropPickup            pickupType
	DropScore             int
	ConditionalDropPickup pickupType
	ConditionalDropBelow  int
}

type WeaponAnimDef struct {
	ReadyShape     int
	AttackSequence AnimSequenceID
}

const (
	shapeSPR_STAT_0  = 2
	shapeSPR_STAT_1  = 3
	shapeSPR_STAT_2  = 4
	shapeSPR_STAT_3  = 5
	shapeSPR_STAT_4  = 6
	shapeSPR_STAT_5  = 7
	shapeSPR_STAT_6  = 8
	shapeSPR_STAT_7  = 9
	shapeSPR_STAT_8  = 10
	shapeSPR_STAT_9  = 11
	shapeSPR_STAT_10 = 12
	shapeSPR_STAT_11 = 13
	shapeSPR_STAT_12 = 14
	shapeSPR_STAT_13 = 15
	shapeSPR_STAT_14 = 16
	shapeSPR_STAT_15 = 17
	shapeSPR_STAT_16 = 18
	shapeSPR_STAT_17 = 19
	shapeSPR_STAT_18 = 20
	shapeSPR_STAT_19 = 21
	shapeSPR_STAT_20 = 22
	shapeSPR_STAT_21 = 23
	shapeSPR_STAT_22 = 24
	shapeSPR_STAT_23 = 25
	shapeSPR_STAT_24 = 26
	shapeSPR_STAT_25 = 27
	shapeSPR_STAT_26 = 28
	shapeSPR_STAT_27 = 29
	shapeSPR_STAT_28 = 30
	shapeSPR_STAT_29 = 31
	shapeSPR_STAT_30 = 32
	shapeSPR_STAT_31 = 33
	shapeSPR_STAT_32 = 34
	shapeSPR_STAT_33 = 35
	shapeSPR_STAT_34 = 36
	shapeSPR_STAT_35 = 37
	shapeSPR_STAT_36 = 38
	shapeSPR_STAT_37 = 39
	shapeSPR_STAT_38 = 40
	shapeSPR_STAT_39 = 41
	shapeSPR_STAT_40 = 42
	shapeSPR_STAT_41 = 43
	shapeSPR_STAT_42 = 44
	shapeSPR_STAT_43 = 45
	shapeSPR_STAT_44 = 46
	shapeSPR_STAT_45 = 47
	shapeSPR_STAT_46 = 48
	shapeSPR_STAT_47 = 49

	shapeGuardBase   = 50
	shapeDogBase     = 99
	shapeDogWalk2    = 107
	shapeDogWalk3    = 115
	shapeDogWalk4    = 123
	shapeDogDie1     = 131
	shapeDogDie2     = 132
	shapeDogDie3     = 133
	shapeDogDead     = 134
	shapeDogJump1    = 135
	shapeDogJump2    = 136
	shapeDogJump3    = 137
	shapeSSBase      = 138
	shapeMutantBase  = 187
	shapeOfficerBase = 238
	shapeBossBase    = 296
	shapeSchabbBase  = 307

	shapeGuardPain1 = 90
	shapeGuardDie1  = 91
	shapeGuardDie2  = 92
	shapeGuardDie3  = 93
	shapeGuardPain2 = 94
	shapeGuardDead  = 95

	shapeMutantPain1  = 227
	shapeMutantDie1   = 228
	shapeMutantDie2   = 229
	shapeMutantDie3   = 230
	shapeMutantPain2  = 231
	shapeMutantDie4   = 232
	shapeMutantDead   = 233
	shapeMutantShoot1 = 234
	shapeMutantShoot2 = 235
	shapeMutantShoot3 = 236
	shapeMutantShoot4 = 237

	shapeSSPain1  = 178
	shapeSSDie1   = 179
	shapeSSDie2   = 180
	shapeSSDie3   = 181
	shapeSSPain2  = 182
	shapeSSDead   = 183
	shapeSSShoot1 = 184
	shapeSSShoot2 = 185
	shapeSSShoot3 = 186

	shapeOfficerPain1  = 278
	shapeOfficerDie1   = 279
	shapeOfficerDie2   = 280
	shapeOfficerDie3   = 281
	shapeOfficerPain2  = 282
	shapeOfficerDie4   = 283
	shapeOfficerDead   = 284
	shapeOfficerShoot1 = 285
	shapeOfficerShoot2 = 286
	shapeOfficerShoot3 = 287

	shapeBossShoot1 = 300
	shapeBossShoot2 = 301
	shapeBossShoot3 = 302
	shapeBossDead   = 303
	shapeBossDie1   = 304
	shapeBossDie2   = 305
	shapeBossDie3   = 306
	shapeBJWalk1    = 395
	shapeBJWalk2    = 396
	shapeBJWalk3    = 397
	shapeBJWalk4    = 398
	shapeBJJump1    = 399
	shapeBJJump2    = 400
	shapeBJJump3    = 401
	shapeBJJump4    = 402

	shapeSPR_ROCKET_1 = 357
	shapeSPR_ROCKET_2 = 358
	shapeSPR_ROCKET_3 = 359
	shapeSPR_ROCKET_4 = 360
	shapeSPR_ROCKET_5 = 361
	shapeSPR_ROCKET_6 = 362
	shapeSPR_ROCKET_7 = 363
	shapeSPR_ROCKET_8 = 364
	shapeSPR_SMOKE_1  = 365
	shapeSPR_SMOKE_2  = 366
	shapeSPR_SMOKE_3  = 367
	shapeSPR_SMOKE_4  = 368
	shapeSPR_BOOM_1   = 369
	shapeSPR_BOOM_2   = 370
	shapeSPR_BOOM_3   = 371

	shapeSPR_KNIFEREADY      = 416
	shapeSPR_KNIFEATK1       = 417
	shapeSPR_KNIFEATK2       = 418
	shapeSPR_KNIFEATK3       = 419
	shapeSPR_KNIFEATK4       = 420
	shapeSPR_PISTOLREADY     = 421
	shapeSPR_PISTOLATK1      = 422
	shapeSPR_PISTOLATK2      = 423
	shapeSPR_PISTOLATK3      = 424
	shapeSPR_PISTOLATK4      = 425
	shapeSPR_MACHINEGUNREADY = 426
	shapeSPR_MACHINEGUNATK1  = 427
	shapeSPR_MACHINEGUNATK2  = 428
	shapeSPR_MACHINEGUNATK3  = 429
	shapeSPR_MACHINEGUNATK4  = 430
	shapeSPR_CHAINREADY      = 431
	shapeSPR_CHAINATK1       = 432
	shapeSPR_CHAINATK2       = 433
	shapeSPR_CHAINATK3       = 434
	shapeSPR_CHAINATK4       = 435
)

const (
	seqActorGuardStand    AnimSequenceID = "actor.guard.stand"
	seqActorGuardPatrol   AnimSequenceID = "actor.guard.patrol"
	seqActorGuardChase    AnimSequenceID = "actor.guard.chase"
	seqActorGuardDeath    AnimSequenceID = "actor.guard.death"
	seqActorGuardPain     AnimSequenceID = "actor.guard.pain"
	seqActorGuardPain2    AnimSequenceID = "actor.guard.pain2"
	seqActorGuardShoot    AnimSequenceID = "actor.guard.shoot"
	seqActorDogStand      AnimSequenceID = "actor.dog.stand"
	seqActorDogPatrol     AnimSequenceID = "actor.dog.patrol"
	seqActorDogChase      AnimSequenceID = "actor.dog.chase"
	seqActorDogJump       AnimSequenceID = "actor.dog.jump"
	seqActorOfficerStand  AnimSequenceID = "actor.officer.stand"
	seqActorOfficerPatrol AnimSequenceID = "actor.officer.patrol"
	seqActorOfficerChase  AnimSequenceID = "actor.officer.chase"
	seqActorOfficerPain   AnimSequenceID = "actor.officer.pain"
	seqActorOfficerPain2  AnimSequenceID = "actor.officer.pain2"
	seqActorOfficerShoot  AnimSequenceID = "actor.officer.shoot"
	seqActorOfficerDeath  AnimSequenceID = "actor.officer.death"
	seqActorMutantStand   AnimSequenceID = "actor.mutant.stand"
	seqActorMutantPatrol  AnimSequenceID = "actor.mutant.patrol"
	seqActorMutantChase   AnimSequenceID = "actor.mutant.chase"
	seqActorMutantPain    AnimSequenceID = "actor.mutant.pain"
	seqActorMutantPain2   AnimSequenceID = "actor.mutant.pain2"
	seqActorMutantShoot   AnimSequenceID = "actor.mutant.shoot"
	seqActorMutantDeath   AnimSequenceID = "actor.mutant.death"
	seqActorSSStand       AnimSequenceID = "actor.ss.stand"
	seqActorSSPatrol      AnimSequenceID = "actor.ss.patrol"
	seqActorSSChase       AnimSequenceID = "actor.ss.chase"
	seqActorSSPain        AnimSequenceID = "actor.ss.pain"
	seqActorSSPain2       AnimSequenceID = "actor.ss.pain2"
	seqActorSSShoot       AnimSequenceID = "actor.ss.shoot"
	seqActorSSDeath       AnimSequenceID = "actor.ss.death"
	seqActorBossStand     AnimSequenceID = "actor.boss.stand"
	seqActorBossChase     AnimSequenceID = "actor.boss.chase"
	seqActorBossShoot     AnimSequenceID = "actor.boss.shoot"
	seqActorDogDeath      AnimSequenceID = "actor.dog.death"
	seqActorBossDeath     AnimSequenceID = "actor.boss.death"
	seqVictoryBJRun       AnimSequenceID = "victory.bj.run"
	seqVictoryBJJump      AnimSequenceID = "victory.bj.jump"
	seqProjectileRocket   AnimSequenceID = "projectile.rocket"
	seqProjectileSmoke    AnimSequenceID = "projectile.smoke"
	seqProjectileBoom     AnimSequenceID = "projectile.boom"
	seqWeaponKnifeAttack  AnimSequenceID = "weapon.knife.attack"
	seqWeaponPistol       AnimSequenceID = "weapon.pistol.attack"
	seqWeaponMachineGun   AnimSequenceID = "weapon.machinegun.attack"
	seqWeaponChainGun     AnimSequenceID = "weapon.chaingun.attack"
)

var staticDefs = []StaticDef{
	{Info: 23, Shape: shapeSPR_STAT_0},
	{Info: 24, Shape: shapeSPR_STAT_1, Blocking: true},
	{Info: 25, Shape: shapeSPR_STAT_2, Blocking: true},
	{Info: 26, Shape: shapeSPR_STAT_3, Blocking: true},
	{Info: 27, Shape: shapeSPR_STAT_4},
	{Info: 28, Shape: shapeSPR_STAT_5, Blocking: true},
	{Info: 29, Shape: shapeSPR_STAT_6, Pickup: pickupAlpo},
	{Info: 30, Shape: shapeSPR_STAT_7, Blocking: true},
	{Info: 31, Shape: shapeSPR_STAT_8, Blocking: true},
	{Info: 32, Shape: shapeSPR_STAT_9},
	{Info: 33, Shape: shapeSPR_STAT_10, Blocking: true},
	{Info: 34, Shape: shapeSPR_STAT_11, Blocking: true},
	{Info: 35, Shape: shapeSPR_STAT_12, Blocking: true},
	{Info: 36, Shape: shapeSPR_STAT_13, Blocking: true},
	{Info: 37, Shape: shapeSPR_STAT_14},
	{Info: 38, Shape: shapeSPR_STAT_15},
	{Info: 39, Shape: shapeSPR_STAT_16, Blocking: true},
	{Info: 40, Shape: shapeSPR_STAT_17, Blocking: true},
	{Info: 41, Shape: shapeSPR_STAT_18, Blocking: true},
	{Info: 42, Shape: shapeSPR_STAT_19},
	{Info: 43, Shape: shapeSPR_STAT_20, Pickup: pickupKey1},
	{Info: 44, Shape: shapeSPR_STAT_21, Pickup: pickupKey2},
	{Info: 45, Shape: shapeSPR_STAT_22, Blocking: true},
	{Info: 46, Shape: shapeSPR_STAT_23},
	{Info: 47, Shape: shapeSPR_STAT_24, Pickup: pickupFood},
	{Info: 48, Shape: shapeSPR_STAT_25, Pickup: pickupFirstAid},
	{Info: 49, Shape: shapeSPR_STAT_26, Pickup: pickupClip},
	{Info: 50, Shape: shapeSPR_STAT_27, Pickup: pickupMachineGun},
	{Info: 51, Shape: shapeSPR_STAT_28, Pickup: pickupChaingun},
	{Info: 52, Shape: shapeSPR_STAT_29, Pickup: pickupCross},
	{Info: 53, Shape: shapeSPR_STAT_30, Pickup: pickupChalice},
	{Info: 54, Shape: shapeSPR_STAT_31, Pickup: pickupBible},
	{Info: 55, Shape: shapeSPR_STAT_32, Pickup: pickupCrown},
	{Info: 56, Shape: shapeSPR_STAT_33, Pickup: pickupFullHeal},
	{Info: 57, Shape: shapeSPR_STAT_34, Pickup: pickupGibs},
	{Info: 58, Shape: shapeSPR_STAT_35, Blocking: true},
	{Info: 59, Shape: shapeSPR_STAT_36, Blocking: true},
	{Info: 60, Shape: shapeSPR_STAT_37, Blocking: true},
	{Info: 61, Shape: shapeSPR_STAT_38, Pickup: pickupGibs},
	{Info: 62, Shape: shapeSPR_STAT_39, Blocking: true},
	{Info: 63, Shape: shapeSPR_STAT_40, Blocking: true},
	{Info: 64, Shape: shapeSPR_STAT_41},
	{Info: 65, Shape: shapeSPR_STAT_42},
	{Info: 66, Shape: shapeSPR_STAT_43},
	{Info: 67, Shape: shapeSPR_STAT_44},
	{Info: 68, Shape: shapeSPR_STAT_45, Blocking: true},
	{Info: 69, Shape: shapeSPR_STAT_46, Blocking: true},
	{Info: 70, Shape: shapeSPR_STAT_47},
	{Info: 74, Shape: shapeSPR_STAT_26, Pickup: pickupClip2},
	{Info: 124, Shape: shapeGuardDead},
}

var actorDefs = []ActorDef{
	{Kind: actorKindGuard, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 108, InfoEnd: 111, BaseShape: shapeGuardBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 25, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorGuardStand, PatrolSequence: seqActorGuardPatrol, ChaseSequence: seqActorGuardChase, PainSequence: seqActorGuardPain, ShootSequence: seqActorGuardShoot, DeathSequence: seqActorGuardDeath, DropPickup: pickupClip2, DropScore: 100},
	{Kind: actorKindGuard, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyEasy, InfoStart: 112, InfoEnd: 115, BaseShape: shapeGuardBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 25, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorGuardStand, PatrolSequence: seqActorGuardPatrol, ChaseSequence: seqActorGuardChase, PainSequence: seqActorGuardPain, ShootSequence: seqActorGuardShoot, DeathSequence: seqActorGuardDeath, DropPickup: pickupClip2, DropScore: 100},
	{Kind: actorKindOfficer, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 116, InfoEnd: 119, BaseShape: shapeOfficerBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 50, PatrolSpeed: 512, ChaseSpeed: 512 * 5, StandSequence: seqActorOfficerStand, PatrolSequence: seqActorOfficerPatrol, ChaseSequence: seqActorOfficerChase, PainSequence: seqActorOfficerPain, ShootSequence: seqActorOfficerShoot, DeathSequence: seqActorOfficerDeath, DropPickup: pickupClip2, DropScore: 400},
	{Kind: actorKindOfficer, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyEasy, InfoStart: 120, InfoEnd: 123, BaseShape: shapeOfficerBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 50, PatrolSpeed: 512, ChaseSpeed: 512 * 5, StandSequence: seqActorOfficerStand, PatrolSequence: seqActorOfficerPatrol, ChaseSequence: seqActorOfficerChase, PainSequence: seqActorOfficerPain, ShootSequence: seqActorOfficerShoot, DeathSequence: seqActorOfficerDeath, DropPickup: pickupClip2, DropScore: 400},
	{Kind: actorKindSS, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 126, InfoEnd: 129, BaseShape: shapeSSBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 100, PatrolSpeed: 512, ChaseSpeed: 512 * 4, StandSequence: seqActorSSStand, PatrolSequence: seqActorSSPatrol, ChaseSequence: seqActorSSChase, PainSequence: seqActorSSPain, ShootSequence: seqActorSSShoot, DeathSequence: seqActorSSDeath, DropPickup: pickupClip2, DropScore: 500, ConditionalDropPickup: pickupMachineGun, ConditionalDropBelow: 2},
	{Kind: actorKindSS, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyEasy, InfoStart: 130, InfoEnd: 133, BaseShape: shapeSSBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 100, PatrolSpeed: 512, ChaseSpeed: 512 * 4, StandSequence: seqActorSSStand, PatrolSequence: seqActorSSPatrol, ChaseSequence: seqActorSSChase, PainSequence: seqActorSSPain, ShootSequence: seqActorSSShoot, DeathSequence: seqActorSSDeath, DropPickup: pickupClip2, DropScore: 500, ConditionalDropPickup: pickupMachineGun, ConditionalDropBelow: 2},
	{Kind: actorKindDog, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 134, InfoEnd: 137, BaseShape: shapeDogBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 1, PatrolSpeed: 1500, ChaseSpeed: 1500 * 2, StandSequence: seqActorDogStand, PatrolSequence: seqActorDogPatrol, ChaseSequence: seqActorDogChase, JumpSequence: seqActorDogJump, DeathSequence: seqActorDogDeath, DropPickup: pickupNone, DropScore: 200},
	{Kind: actorKindDog, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyEasy, InfoStart: 138, InfoEnd: 141, BaseShape: shapeDogBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 1, PatrolSpeed: 1500, ChaseSpeed: 1500 * 2, StandSequence: seqActorDogStand, PatrolSequence: seqActorDogPatrol, ChaseSequence: seqActorDogChase, JumpSequence: seqActorDogJump, DeathSequence: seqActorDogDeath, DropPickup: pickupNone, DropScore: 200},
	{Kind: actorKindGuard, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 144, InfoEnd: 147, BaseShape: shapeGuardBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 25, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorGuardStand, PatrolSequence: seqActorGuardPatrol, ChaseSequence: seqActorGuardChase, PainSequence: seqActorGuardPain, ShootSequence: seqActorGuardShoot, DeathSequence: seqActorGuardDeath, DropPickup: pickupClip2, DropScore: 100},
	{Kind: actorKindGuard, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyMedium, InfoStart: 148, InfoEnd: 151, BaseShape: shapeGuardBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 25, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorGuardStand, PatrolSequence: seqActorGuardPatrol, ChaseSequence: seqActorGuardChase, PainSequence: seqActorGuardPain, ShootSequence: seqActorGuardShoot, DeathSequence: seqActorGuardDeath, DropPickup: pickupClip2, DropScore: 100},
	{Kind: actorKindOfficer, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 152, InfoEnd: 155, BaseShape: shapeOfficerBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 50, PatrolSpeed: 512, ChaseSpeed: 512 * 5, StandSequence: seqActorOfficerStand, PatrolSequence: seqActorOfficerPatrol, ChaseSequence: seqActorOfficerChase, PainSequence: seqActorOfficerPain, ShootSequence: seqActorOfficerShoot, DeathSequence: seqActorOfficerDeath, DropPickup: pickupClip2, DropScore: 400},
	{Kind: actorKindOfficer, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyMedium, InfoStart: 156, InfoEnd: 159, BaseShape: shapeOfficerBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 50, PatrolSpeed: 512, ChaseSpeed: 512 * 5, StandSequence: seqActorOfficerStand, PatrolSequence: seqActorOfficerPatrol, ChaseSequence: seqActorOfficerChase, PainSequence: seqActorOfficerPain, ShootSequence: seqActorOfficerShoot, DeathSequence: seqActorOfficerDeath, DropPickup: pickupClip2, DropScore: 400},
	{Kind: actorKindSS, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 162, InfoEnd: 165, BaseShape: shapeSSBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 100, PatrolSpeed: 512, ChaseSpeed: 512 * 4, StandSequence: seqActorSSStand, PatrolSequence: seqActorSSPatrol, ChaseSequence: seqActorSSChase, PainSequence: seqActorSSPain, ShootSequence: seqActorSSShoot, DeathSequence: seqActorSSDeath, DropPickup: pickupClip2, DropScore: 500, ConditionalDropPickup: pickupMachineGun, ConditionalDropBelow: 2},
	{Kind: actorKindSS, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyMedium, InfoStart: 166, InfoEnd: 169, BaseShape: shapeSSBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 100, PatrolSpeed: 512, ChaseSpeed: 512 * 4, StandSequence: seqActorSSStand, PatrolSequence: seqActorSSPatrol, ChaseSequence: seqActorSSChase, PainSequence: seqActorSSPain, ShootSequence: seqActorSSShoot, DeathSequence: seqActorSSDeath, DropPickup: pickupClip2, DropScore: 500, ConditionalDropPickup: pickupMachineGun, ConditionalDropBelow: 2},
	{Kind: actorKindDog, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 170, InfoEnd: 173, BaseShape: shapeDogBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 1, PatrolSpeed: 1500, ChaseSpeed: 1500 * 2, StandSequence: seqActorDogStand, PatrolSequence: seqActorDogPatrol, ChaseSequence: seqActorDogChase, JumpSequence: seqActorDogJump, DeathSequence: seqActorDogDeath, DropPickup: pickupNone, DropScore: 200},
	{Kind: actorKindDog, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyMedium, InfoStart: 174, InfoEnd: 177, BaseShape: shapeDogBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 1, PatrolSpeed: 1500, ChaseSpeed: 1500 * 2, StandSequence: seqActorDogStand, PatrolSequence: seqActorDogPatrol, ChaseSequence: seqActorDogChase, JumpSequence: seqActorDogJump, DeathSequence: seqActorDogDeath, DropPickup: pickupNone, DropScore: 200},
	{Kind: actorKindGuard, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 180, InfoEnd: 183, BaseShape: shapeGuardBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 25, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorGuardStand, PatrolSequence: seqActorGuardPatrol, ChaseSequence: seqActorGuardChase, PainSequence: seqActorGuardPain, ShootSequence: seqActorGuardShoot, DeathSequence: seqActorGuardDeath, DropPickup: pickupClip2, DropScore: 100},
	{Kind: actorKindGuard, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyHard, InfoStart: 184, InfoEnd: 187, BaseShape: shapeGuardBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 25, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorGuardStand, PatrolSequence: seqActorGuardPatrol, ChaseSequence: seqActorGuardChase, PainSequence: seqActorGuardPain, ShootSequence: seqActorGuardShoot, DeathSequence: seqActorGuardDeath, DropPickup: pickupClip2, DropScore: 100},
	{Kind: actorKindOfficer, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 188, InfoEnd: 191, BaseShape: shapeOfficerBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 50, PatrolSpeed: 512, ChaseSpeed: 512 * 5, StandSequence: seqActorOfficerStand, PatrolSequence: seqActorOfficerPatrol, ChaseSequence: seqActorOfficerChase, PainSequence: seqActorOfficerPain, ShootSequence: seqActorOfficerShoot, DeathSequence: seqActorOfficerDeath, DropPickup: pickupClip2, DropScore: 400},
	{Kind: actorKindOfficer, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyHard, InfoStart: 192, InfoEnd: 195, BaseShape: shapeOfficerBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 50, PatrolSpeed: 512, ChaseSpeed: 512 * 5, StandSequence: seqActorOfficerStand, PatrolSequence: seqActorOfficerPatrol, ChaseSequence: seqActorOfficerChase, PainSequence: seqActorOfficerPain, ShootSequence: seqActorOfficerShoot, DeathSequence: seqActorOfficerDeath, DropPickup: pickupClip2, DropScore: 400},
	{Kind: actorKindSS, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 198, InfoEnd: 201, BaseShape: shapeSSBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 100, PatrolSpeed: 512, ChaseSpeed: 512 * 4, StandSequence: seqActorSSStand, PatrolSequence: seqActorSSPatrol, ChaseSequence: seqActorSSChase, PainSequence: seqActorSSPain, ShootSequence: seqActorSSShoot, DeathSequence: seqActorSSDeath, DropPickup: pickupClip2, DropScore: 500, ConditionalDropPickup: pickupMachineGun, ConditionalDropBelow: 2},
	{Kind: actorKindSS, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyHard, InfoStart: 202, InfoEnd: 205, BaseShape: shapeSSBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 100, PatrolSpeed: 512, ChaseSpeed: 512 * 4, StandSequence: seqActorSSStand, PatrolSequence: seqActorSSPatrol, ChaseSequence: seqActorSSChase, PainSequence: seqActorSSPain, ShootSequence: seqActorSSShoot, DeathSequence: seqActorSSDeath, DropPickup: pickupClip2, DropScore: 500, ConditionalDropPickup: pickupMachineGun, ConditionalDropBelow: 2},
	{Kind: actorKindDog, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 206, InfoEnd: 209, BaseShape: shapeDogBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 1, PatrolSpeed: 1500, ChaseSpeed: 1500 * 2, StandSequence: seqActorDogStand, PatrolSequence: seqActorDogPatrol, ChaseSequence: seqActorDogChase, JumpSequence: seqActorDogJump, DeathSequence: seqActorDogDeath, DropPickup: pickupNone, DropScore: 200},
	{Kind: actorKindDog, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyHard, InfoStart: 210, InfoEnd: 213, BaseShape: shapeDogBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 1, PatrolSpeed: 1500, ChaseSpeed: 1500 * 2, StandSequence: seqActorDogStand, PatrolSequence: seqActorDogPatrol, ChaseSequence: seqActorDogChase, JumpSequence: seqActorDogJump, DeathSequence: seqActorDogDeath, DropPickup: pickupNone, DropScore: 200},
	{Kind: actorKindBoss, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 214, InfoEnd: 214, BaseShape: shapeBossBase, Rotate: false, Blocking: true, Shootable: true, HitPoints: 1050, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorBossStand, ChaseSequence: seqActorBossChase, ShootSequence: seqActorBossShoot, DeathSequence: seqActorBossDeath, DropPickup: pickupKey1, DropScore: 5000},
	{Kind: actorKindBoss, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 214, InfoEnd: 214, BaseShape: shapeBossBase, Rotate: false, Blocking: true, Shootable: true, HitPoints: 950, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorBossStand, ChaseSequence: seqActorBossChase, ShootSequence: seqActorBossShoot, DeathSequence: seqActorBossDeath, DropPickup: pickupKey1, DropScore: 5000},
	{Kind: actorKindBoss, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 214, InfoEnd: 214, BaseShape: shapeBossBase, Rotate: false, Blocking: true, Shootable: true, HitPoints: 850, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorBossStand, ChaseSequence: seqActorBossChase, ShootSequence: seqActorBossShoot, DeathSequence: seqActorBossDeath, DropPickup: pickupKey1, DropScore: 5000},
	{Kind: actorKindMutant, SpawnMode: actorSpawnStand, MinDifficulty: difficultyEasy, InfoStart: 216, InfoEnd: 219, BaseShape: shapeMutantBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 45, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorMutantStand, PatrolSequence: seqActorMutantPatrol, ChaseSequence: seqActorMutantChase, PainSequence: seqActorMutantPain, ShootSequence: seqActorMutantShoot, DeathSequence: seqActorMutantDeath, DropPickup: pickupClip2, DropScore: 700},
	{Kind: actorKindMutant, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyEasy, InfoStart: 220, InfoEnd: 223, BaseShape: shapeMutantBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 45, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorMutantStand, PatrolSequence: seqActorMutantPatrol, ChaseSequence: seqActorMutantChase, PainSequence: seqActorMutantPain, ShootSequence: seqActorMutantShoot, DeathSequence: seqActorMutantDeath, DropPickup: pickupClip2, DropScore: 700},
	{Kind: actorKindMutant, SpawnMode: actorSpawnStand, MinDifficulty: difficultyMedium, InfoStart: 234, InfoEnd: 237, BaseShape: shapeMutantBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 55, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorMutantStand, PatrolSequence: seqActorMutantPatrol, ChaseSequence: seqActorMutantChase, PainSequence: seqActorMutantPain, ShootSequence: seqActorMutantShoot, DeathSequence: seqActorMutantDeath, DropPickup: pickupClip2, DropScore: 700},
	{Kind: actorKindMutant, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyMedium, InfoStart: 238, InfoEnd: 241, BaseShape: shapeMutantBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 55, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorMutantStand, PatrolSequence: seqActorMutantPatrol, ChaseSequence: seqActorMutantChase, PainSequence: seqActorMutantPain, ShootSequence: seqActorMutantShoot, DeathSequence: seqActorMutantDeath, DropPickup: pickupClip2, DropScore: 700},
	{Kind: actorKindMutant, SpawnMode: actorSpawnStand, MinDifficulty: difficultyHard, InfoStart: 252, InfoEnd: 255, BaseShape: shapeMutantBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 55, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorMutantStand, PatrolSequence: seqActorMutantPatrol, ChaseSequence: seqActorMutantChase, PainSequence: seqActorMutantPain, ShootSequence: seqActorMutantShoot, DeathSequence: seqActorMutantDeath, DropPickup: pickupClip2, DropScore: 700},
	{Kind: actorKindMutant, SpawnMode: actorSpawnPatrol, MinDifficulty: difficultyHard, InfoStart: 256, InfoEnd: 259, BaseShape: shapeMutantBase, Rotate: true, Blocking: true, Shootable: true, HitPoints: 55, PatrolSpeed: 512, ChaseSpeed: 512 * 3, StandSequence: seqActorMutantStand, PatrolSequence: seqActorMutantPatrol, ChaseSequence: seqActorMutantChase, PainSequence: seqActorMutantPain, ShootSequence: seqActorMutantShoot, DeathSequence: seqActorMutantDeath, DropPickup: pickupClip2, DropScore: 700},
}

var animSequences = map[AnimSequenceID]AnimSequence{
	seqActorGuardPain: {
		ID: seqActorGuardPain,
		Frames: []AnimFrame{
			{Shape: shapeGuardPain1, Tics: 10},
		},
	},
	seqActorGuardPain2: {
		ID: seqActorGuardPain2,
		Frames: []AnimFrame{
			{Shape: shapeGuardPain2, Tics: 10},
		},
	},
	seqActorGuardStand: {
		ID:   seqActorGuardStand,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeGuardBase, Tics: 0},
		},
	},
	seqActorGuardPatrol: {
		ID:   seqActorGuardPatrol,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: 58, Tics: 20},
			{Shape: 58, Tics: 5},
			{Shape: 66, Tics: 15},
			{Shape: 74, Tics: 20},
			{Shape: 74, Tics: 5},
			{Shape: 82, Tics: 15},
		},
	},
	seqActorGuardChase: {
		ID:   seqActorGuardChase,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: 58, Tics: 10},
			{Shape: 58, Tics: 3},
			{Shape: 66, Tics: 8},
			{Shape: 74, Tics: 10},
			{Shape: 74, Tics: 3},
			{Shape: 82, Tics: 8},
		},
	},
	seqActorGuardShoot: {
		ID: seqActorGuardShoot,
		Frames: []AnimFrame{
			{Shape: 96, Tics: 20},
			{Shape: 97, Tics: 20, Action: animActionFireActor},
			{Shape: 98, Tics: 20},
		},
	},
	seqActorDogStand: {
		ID:   seqActorDogStand,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeDogBase, Tics: 0},
		},
	},
	seqActorDogPatrol: {
		ID:   seqActorDogPatrol,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeDogBase, Tics: 20},
			{Shape: shapeDogBase, Tics: 5},
			{Shape: shapeDogWalk2, Tics: 15},
			{Shape: shapeDogWalk3, Tics: 20},
			{Shape: shapeDogWalk3, Tics: 5},
			{Shape: shapeDogWalk4, Tics: 15},
		},
	},
	seqActorDogChase: {
		ID:   seqActorDogChase,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeDogBase, Tics: 10},
			{Shape: shapeDogBase, Tics: 3},
			{Shape: shapeDogWalk2, Tics: 8},
			{Shape: shapeDogWalk3, Tics: 10},
			{Shape: shapeDogWalk3, Tics: 3},
			{Shape: shapeDogWalk4, Tics: 8},
		},
	},
	seqActorDogJump: {
		ID: seqActorDogJump,
		Frames: []AnimFrame{
			{Shape: shapeDogJump1, Tics: 10},
			{Shape: shapeDogJump2, Tics: 10, Action: animActionBiteActor},
			{Shape: shapeDogJump3, Tics: 10},
			{Shape: shapeDogJump1, Tics: 10},
			{Shape: shapeDogBase, Tics: 10},
		},
	},
	seqActorOfficerPain: {
		ID: seqActorOfficerPain,
		Frames: []AnimFrame{
			{Shape: shapeOfficerPain1, Tics: 10},
		},
	},
	seqActorOfficerPain2: {
		ID: seqActorOfficerPain2,
		Frames: []AnimFrame{
			{Shape: shapeOfficerPain2, Tics: 10},
		},
	},
	seqActorOfficerStand: {
		ID:   seqActorOfficerStand,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeOfficerBase, Tics: 0},
		},
	},
	seqActorOfficerPatrol: {
		ID:   seqActorOfficerPatrol,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeOfficerBase + 8, Tics: 20},
			{Shape: shapeOfficerBase + 8, Tics: 5},
			{Shape: shapeOfficerBase + 16, Tics: 15},
			{Shape: shapeOfficerBase + 24, Tics: 20},
			{Shape: shapeOfficerBase + 24, Tics: 5},
			{Shape: shapeOfficerBase + 32, Tics: 15},
		},
	},
	seqActorOfficerChase: {
		ID:   seqActorOfficerChase,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeOfficerBase + 8, Tics: 10},
			{Shape: shapeOfficerBase + 8, Tics: 3},
			{Shape: shapeOfficerBase + 16, Tics: 8},
			{Shape: shapeOfficerBase + 24, Tics: 10},
			{Shape: shapeOfficerBase + 24, Tics: 3},
			{Shape: shapeOfficerBase + 32, Tics: 8},
		},
	},
	seqActorOfficerShoot: {
		ID: seqActorOfficerShoot,
		Frames: []AnimFrame{
			{Shape: shapeOfficerShoot1, Tics: 6},
			{Shape: shapeOfficerShoot2, Tics: 20, Action: animActionFireActor},
			{Shape: shapeOfficerShoot3, Tics: 10},
		},
	},
	seqActorMutantPain: {
		ID: seqActorMutantPain,
		Frames: []AnimFrame{
			{Shape: shapeMutantPain1, Tics: 10},
		},
	},
	seqActorMutantPain2: {
		ID: seqActorMutantPain2,
		Frames: []AnimFrame{
			{Shape: shapeMutantPain2, Tics: 10},
		},
	},
	seqActorMutantStand: {
		ID:   seqActorMutantStand,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeMutantBase, Tics: 0},
		},
	},
	seqActorMutantPatrol: {
		ID:   seqActorMutantPatrol,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeMutantBase + 8, Tics: 20},
			{Shape: shapeMutantBase + 8, Tics: 5},
			{Shape: shapeMutantBase + 16, Tics: 15},
			{Shape: shapeMutantBase + 24, Tics: 20},
			{Shape: shapeMutantBase + 24, Tics: 5},
			{Shape: shapeMutantBase + 32, Tics: 15},
		},
	},
	seqActorMutantChase: {
		ID:   seqActorMutantChase,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeMutantBase + 8, Tics: 10},
			{Shape: shapeMutantBase + 8, Tics: 3},
			{Shape: shapeMutantBase + 16, Tics: 8},
			{Shape: shapeMutantBase + 24, Tics: 10},
			{Shape: shapeMutantBase + 24, Tics: 3},
			{Shape: shapeMutantBase + 32, Tics: 8},
		},
	},
	seqActorMutantShoot: {
		ID: seqActorMutantShoot,
		Frames: []AnimFrame{
			{Shape: shapeMutantShoot1, Tics: 6, Action: animActionFireActor},
			{Shape: shapeMutantShoot2, Tics: 20},
			{Shape: shapeMutantShoot3, Tics: 10, Action: animActionFireActor},
			{Shape: shapeMutantShoot4, Tics: 20},
		},
	},
	seqActorSSPain: {
		ID: seqActorSSPain,
		Frames: []AnimFrame{
			{Shape: shapeSSPain1, Tics: 10},
		},
	},
	seqActorSSPain2: {
		ID: seqActorSSPain2,
		Frames: []AnimFrame{
			{Shape: shapeSSPain2, Tics: 10},
		},
	},
	seqActorSSStand: {
		ID:   seqActorSSStand,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeSSBase, Tics: 0},
		},
	},
	seqActorSSPatrol: {
		ID:   seqActorSSPatrol,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeSSBase + 8, Tics: 20},
			{Shape: shapeSSBase + 8, Tics: 5},
			{Shape: shapeSSBase + 16, Tics: 15},
			{Shape: shapeSSBase + 24, Tics: 20},
			{Shape: shapeSSBase + 24, Tics: 5},
			{Shape: shapeSSBase + 32, Tics: 15},
		},
	},
	seqActorSSChase: {
		ID:   seqActorSSChase,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeSSBase + 8, Tics: 10},
			{Shape: shapeSSBase + 8, Tics: 3},
			{Shape: shapeSSBase + 16, Tics: 8},
			{Shape: shapeSSBase + 24, Tics: 10},
			{Shape: shapeSSBase + 24, Tics: 3},
			{Shape: shapeSSBase + 32, Tics: 8},
		},
	},
	seqActorSSShoot: {
		ID: seqActorSSShoot,
		Frames: []AnimFrame{
			{Shape: shapeSSShoot1, Tics: 20},
			{Shape: shapeSSShoot2, Tics: 20, Action: animActionFireActor},
			{Shape: shapeSSShoot3, Tics: 10},
			{Shape: shapeSSShoot2, Tics: 10, Action: animActionFireActor},
			{Shape: shapeSSShoot3, Tics: 10},
			{Shape: shapeSSShoot2, Tics: 10, Action: animActionFireActor},
			{Shape: shapeSSShoot3, Tics: 10},
			{Shape: shapeSSShoot2, Tics: 10, Action: animActionFireActor},
			{Shape: shapeSSShoot3, Tics: 10},
		},
	},
	seqActorBossStand: {
		ID:   seqActorBossStand,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeBossBase, Tics: 0},
		},
	},
	seqActorBossChase: {
		ID:   seqActorBossChase,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeBossBase, Tics: 10},
			{Shape: shapeBossBase, Tics: 3},
			{Shape: shapeBossBase + 1, Tics: 8},
			{Shape: shapeBossBase + 2, Tics: 10},
			{Shape: shapeBossBase + 2, Tics: 3},
			{Shape: shapeBossBase + 3, Tics: 8},
		},
	},
	seqActorBossShoot: {
		ID: seqActorBossShoot,
		Frames: []AnimFrame{
			{Shape: shapeBossShoot1, Tics: 30},
			{Shape: shapeBossShoot2, Tics: 10, Action: animActionFireActor},
			{Shape: shapeBossShoot3, Tics: 10, Action: animActionFireActor},
			{Shape: shapeBossShoot2, Tics: 10, Action: animActionFireActor},
			{Shape: shapeBossShoot3, Tics: 10, Action: animActionFireActor},
			{Shape: shapeBossShoot2, Tics: 10, Action: animActionFireActor},
			{Shape: shapeBossShoot3, Tics: 10, Action: animActionFireActor},
			{Shape: shapeBossShoot1, Tics: 10},
		},
	},
	seqActorGuardDeath: {
		ID: seqActorGuardDeath,
		Frames: []AnimFrame{
			{Shape: shapeGuardDie1, Tics: 15},
			{Shape: shapeGuardDie2, Tics: 15},
			{Shape: shapeGuardDie3, Tics: 15},
			{Shape: shapeGuardDead, Tics: 0},
		},
	},
	seqActorOfficerDeath: {
		ID: seqActorOfficerDeath,
		Frames: []AnimFrame{
			{Shape: shapeOfficerDie1, Tics: 11},
			{Shape: shapeOfficerDie2, Tics: 11},
			{Shape: shapeOfficerDie3, Tics: 11},
			{Shape: shapeOfficerDie4, Tics: 11},
			{Shape: shapeOfficerDead, Tics: 0},
		},
	},
	seqActorMutantDeath: {
		ID: seqActorMutantDeath,
		Frames: []AnimFrame{
			{Shape: shapeMutantDie1, Tics: 7},
			{Shape: shapeMutantDie2, Tics: 7},
			{Shape: shapeMutantDie3, Tics: 7},
			{Shape: shapeMutantDie4, Tics: 7},
			{Shape: shapeMutantDead, Tics: 0},
		},
	},
	seqActorSSDeath: {
		ID: seqActorSSDeath,
		Frames: []AnimFrame{
			{Shape: shapeSSDie1, Tics: 15},
			{Shape: shapeSSDie2, Tics: 15},
			{Shape: shapeSSDie3, Tics: 15},
			{Shape: shapeSSDead, Tics: 0},
		},
	},
	seqActorDogDeath: {
		ID: seqActorDogDeath,
		Frames: []AnimFrame{
			{Shape: shapeDogDie1, Tics: 15},
			{Shape: shapeDogDie2, Tics: 15},
			{Shape: shapeDogDie3, Tics: 15},
			{Shape: shapeDogDead, Tics: 15},
		},
	},
	seqActorBossDeath: {
		ID: seqActorBossDeath,
		Frames: []AnimFrame{
			{Shape: shapeBossDie1, Tics: 15},
			{Shape: shapeBossDie2, Tics: 15},
			{Shape: shapeBossDie3, Tics: 15},
			{Shape: shapeBossDead, Tics: 0},
		},
	},
	seqVictoryBJRun: {
		ID:   seqVictoryBJRun,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: shapeBJWalk1, Tics: 12},
			{Shape: shapeBJWalk1, Tics: 3},
			{Shape: shapeBJWalk2, Tics: 8},
			{Shape: shapeBJWalk3, Tics: 12},
			{Shape: shapeBJWalk3, Tics: 3},
			{Shape: shapeBJWalk4, Tics: 8},
		},
	},
	seqVictoryBJJump: {
		ID: seqVictoryBJJump,
		Frames: []AnimFrame{
			{Shape: shapeBJJump1, Tics: 14},
			{Shape: shapeBJJump2, Tics: 14},
			{Shape: shapeBJJump3, Tics: 14},
			{Shape: shapeBJJump4, Tics: 300},
		},
	},
	seqProjectileRocket: {
		ID: seqProjectileRocket,
		Frames: []AnimFrame{
			{Shape: shapeSPR_ROCKET_1, Tics: 3},
			{Shape: shapeSPR_ROCKET_2, Tics: 3},
			{Shape: shapeSPR_ROCKET_3, Tics: 3},
			{Shape: shapeSPR_ROCKET_4, Tics: 3},
			{Shape: shapeSPR_ROCKET_5, Tics: 3},
			{Shape: shapeSPR_ROCKET_6, Tics: 3},
			{Shape: shapeSPR_ROCKET_7, Tics: 3},
			{Shape: shapeSPR_ROCKET_8, Tics: 3},
		},
	},
	seqProjectileSmoke: {
		ID: seqProjectileSmoke,
		Frames: []AnimFrame{
			{Shape: shapeSPR_SMOKE_1, Tics: 3},
			{Shape: shapeSPR_SMOKE_2, Tics: 3},
			{Shape: shapeSPR_SMOKE_3, Tics: 3},
			{Shape: shapeSPR_SMOKE_4, Tics: 3},
		},
	},
	seqProjectileBoom: {
		ID: seqProjectileBoom,
		Frames: []AnimFrame{
			{Shape: shapeSPR_BOOM_1, Tics: 6},
			{Shape: shapeSPR_BOOM_2, Tics: 6},
			{Shape: shapeSPR_BOOM_3, Tics: 0},
		},
	},
	seqWeaponKnifeAttack: {
		ID: seqWeaponKnifeAttack,
		Frames: []AnimFrame{
			{Shape: shapeSPR_KNIFEATK1, Tics: 6, Action: animActionNone},
			{Shape: shapeSPR_KNIFEATK2, Tics: 6, Action: animActionFireKnife},
			{Shape: shapeSPR_KNIFEATK3, Tics: 6, Action: animActionNone},
			{Shape: shapeSPR_KNIFEATK4, Tics: 6, Action: animActionEnd},
		},
	},
	seqWeaponPistol: {
		ID: seqWeaponPistol,
		Frames: []AnimFrame{
			{Shape: shapeSPR_PISTOLATK1, Tics: 6, Action: animActionNone},
			{Shape: shapeSPR_PISTOLATK2, Tics: 6, Action: animActionFireGun},
			{Shape: shapeSPR_PISTOLATK3, Tics: 6, Action: animActionNone},
			{Shape: shapeSPR_PISTOLATK4, Tics: 6, Action: animActionEnd},
		},
	},
	seqWeaponMachineGun: {
		ID: seqWeaponMachineGun,
		Frames: []AnimFrame{
			{Shape: shapeSPR_MACHINEGUNATK1, Tics: 6, Action: animActionNone},
			{Shape: shapeSPR_MACHINEGUNATK2, Tics: 6, Action: animActionFireGun},
			{Shape: shapeSPR_MACHINEGUNATK3, Tics: 6, Action: animActionLoopIfPressed},
			{Shape: shapeSPR_MACHINEGUNATK4, Tics: 6, Action: animActionEnd},
		},
	},
	seqWeaponChainGun: {
		ID: seqWeaponChainGun,
		Frames: []AnimFrame{
			{Shape: shapeSPR_CHAINATK1, Tics: 6, Action: animActionNone},
			{Shape: shapeSPR_CHAINATK2, Tics: 6, Action: animActionFireGun},
			{Shape: shapeSPR_CHAINATK3, Tics: 6, Action: animActionLoopFireIfPressed},
			{Shape: shapeSPR_CHAINATK4, Tics: 6, Action: animActionEnd},
		},
	},
}

var weaponAnimDefs = [...]WeaponAnimDef{
	{ReadyShape: shapeSPR_KNIFEREADY, AttackSequence: seqWeaponKnifeAttack},
	{ReadyShape: shapeSPR_PISTOLREADY, AttackSequence: seqWeaponPistol},
	{ReadyShape: shapeSPR_MACHINEGUNREADY, AttackSequence: seqWeaponMachineGun},
	{ReadyShape: shapeSPR_CHAINREADY, AttackSequence: seqWeaponChainGun},
}

var (
	activeStaticDefs     = staticDefs
	activeActorDefs      = actorDefs
	activeAnimSequences  = animSequences
	activeWeaponAnimDefs = weaponAnimDefs
	activeVictoryBJWalk1 = shapeBJWalk1
)

func init() {
	setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"})
}

func setSpriteCatalogVariant(spec wl6.VariantSpec) {
	activeStaticDefs = staticDefs
	activeActorDefs = actorDefs
	activeAnimSequences = animSequences
	activeWeaponAnimDefs = weaponAnimDefs
	activeVictoryBJWalk1 = shapeBJWalk1

	if spec.Ext != "WL1" {
		return
	}

	activeVictoryBJWalk1 = 408
	activeAnimSequences = cloneAnimSequences(animSequences)
	activeAnimSequences[seqProjectileRocket] = AnimSequence{
		ID: seqProjectileRocket,
		Frames: []AnimFrame{
			{Shape: 370, Tics: 3},
			{Shape: 371, Tics: 3},
			{Shape: 372, Tics: 3},
			{Shape: 373, Tics: 3},
			{Shape: 374, Tics: 3},
			{Shape: 375, Tics: 3},
			{Shape: 376, Tics: 3},
			{Shape: 377, Tics: 3},
		},
	}
	activeAnimSequences[seqProjectileSmoke] = AnimSequence{
		ID: seqProjectileSmoke,
		Frames: []AnimFrame{
			{Shape: 378, Tics: 3},
			{Shape: 379, Tics: 3},
			{Shape: 380, Tics: 3},
			{Shape: 381, Tics: 3},
		},
	}
	activeAnimSequences[seqProjectileBoom] = AnimSequence{
		ID: seqProjectileBoom,
		Frames: []AnimFrame{
			{Shape: 382, Tics: 6},
			{Shape: 383, Tics: 6},
			{Shape: 384, Tics: 0},
		},
	}
	activeAnimSequences[seqVictoryBJRun] = AnimSequence{
		ID:   seqVictoryBJRun,
		Loop: true,
		Frames: []AnimFrame{
			{Shape: 408, Tics: 12},
			{Shape: 408, Tics: 3},
			{Shape: 409, Tics: 8},
			{Shape: 410, Tics: 12},
			{Shape: 410, Tics: 3},
			{Shape: 411, Tics: 8},
		},
	}
	activeAnimSequences[seqVictoryBJJump] = AnimSequence{
		ID: seqVictoryBJJump,
		Frames: []AnimFrame{
			{Shape: 412, Tics: 14},
			{Shape: 413, Tics: 14},
			{Shape: 414, Tics: 14},
			{Shape: 415, Tics: 300},
		},
	}
}

func cloneAnimSequences(src map[AnimSequenceID]AnimSequence) map[AnimSequenceID]AnimSequence {
	cloned := make(map[AnimSequenceID]AnimSequence, len(src))
	for id, seq := range src {
		frames := make([]AnimFrame, len(seq.Frames))
		copy(frames, seq.Frames)
		cloned[id] = AnimSequence{
			ID:     seq.ID,
			Loop:   seq.Loop,
			Frames: frames,
		}
	}
	return cloned
}

func LookupStatic(info uint16) (StaticDef, bool) {
	for _, def := range activeStaticDefs {
		if def.Info == info {
			return def, true
		}
	}
	return StaticDef{}, false
}

func LookupActor(info uint16) (ActorDef, bool) {
	for _, def := range activeActorDefs {
		if info >= def.InfoStart && info <= def.InfoEnd {
			return def, true
		}
	}
	return ActorDef{}, false
}

func LookupActorSpawn(info uint16, difficulty gameDifficulty) (ActorDef, bool) {
	best := -1
	var match ActorDef
	for _, def := range activeActorDefs {
		if difficulty < def.MinDifficulty {
			continue
		}
		if info >= def.InfoStart && info <= def.InfoEnd {
			if int(def.MinDifficulty) >= best {
				best = int(def.MinDifficulty)
				match = def
			}
		}
	}
	if best >= 0 {
		return match, true
	}
	return ActorDef{}, false
}

func LookupAnimSequence(id AnimSequenceID) (AnimSequence, bool) {
	seq, ok := activeAnimSequences[id]
	return seq, ok
}

func WeaponAnim(weapon int) (WeaponAnimDef, bool) {
	if weapon < 0 || weapon >= len(activeWeaponAnimDefs) {
		return WeaponAnimDef{}, false
	}
	return activeWeaponAnimDefs[weapon], true
}

func StaticDefForPickup(pickup pickupType) (StaticDef, bool) {
	switch pickup {
	case pickupClip:
		return StaticDef{Shape: shapeSPR_STAT_26, Pickup: pickupClip}, true
	case pickupClip2:
		return StaticDef{Shape: shapeSPR_STAT_26, Pickup: pickupClip2}, true
	case pickupMachineGun:
		return StaticDef{Shape: shapeSPR_STAT_27, Pickup: pickupMachineGun}, true
	case pickupKey1:
		return StaticDef{Shape: shapeSPR_STAT_20, Pickup: pickupKey1}, true
	default:
		return StaticDef{}, false
	}
}

func (d ActorDef) SpawnShape(info uint16) int {
	if d.Rotate {
		return d.BaseShape
	}
	return d.BaseShape + int(info-d.InfoStart)
}

func (d ActorDef) FacingDir(info uint16) int {
	if !d.Rotate {
		return 0
	}
	return mapInfoFacing(info - d.InfoStart)
}

func (d ActorDef) ResolveDrop(bestWeapon int) (pickup pickupType, score int) {
	if d.ConditionalDropPickup != pickupNone && bestWeapon < d.ConditionalDropBelow {
		return d.ConditionalDropPickup, d.DropScore
	}
	return d.DropPickup, d.DropScore
}
