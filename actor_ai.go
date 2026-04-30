package main

import (
	"fmt"
	"math"
	"sort"
)

type gameDifficulty int

const (
	difficultyEasy gameDifficulty = iota
	difficultyMedium
	difficultyHard
)

type ActorKind int

const (
	actorKindGuard ActorKind = iota
	actorKindOfficer
	actorKindSS
	actorKindDog
	actorKindBoss
	actorKindMutant
)

type ActorSpawnMode int

const (
	actorSpawnStand ActorSpawnMode = iota
	actorSpawnPatrol
)

type ActorAIState int

const (
	actorStateStand ActorAIState = iota
	actorStatePatrol
	actorStateChase
	actorStateShoot
	actorStateJump
	actorStatePain
	actorStateDead
)

type actorInstance struct {
	kind      ActorKind
	shapenum  int
	x         float64
	y         float64
	tileX     int
	tileY     int
	goalX     int
	goalY     int
	hasGoal   bool
	dir       int
	facingDir int
	rotate    bool

	blocking    bool
	shootable   bool
	alive       bool
	alerted     bool
	ambush      bool
	firstAttack bool
	area        int

	health      int
	patrolSpeed float64
	chaseSpeed  float64
	scoreValue  int
	dropPickup  pickupType
	standSeq    AnimSequenceID
	patrolSeq   AnimSequenceID
	chaseSeq    AnimSequenceID
	painSeq     AnimSequenceID
	shootSeq    AnimSequenceID
	jumpSeq     AnimSequenceID
	deathSeq    AnimSequenceID

	aiState         ActorAIState
	spawnMode       ActorSpawnMode
	reactionTimer   int
	sequenceID      AnimSequenceID
	sequenceLoop    bool
	frameIndex      int
	frameTimer      int
	frameActionDone bool
	moveDistance    float64
}

const actorDoorWaitDistance = -1.0

func (a *actorInstance) reserveTileGoal(x, y int) {
	if a == nil {
		return
	}
	a.tileX = x
	a.tileY = y
	a.goalX = x
	a.goalY = y
	a.hasGoal = true
}

func (a *actorInstance) clearTileGoal() {
	if a == nil {
		return
	}
	a.goalX = a.tileX
	a.goalY = a.tileY
	a.hasGoal = false
}

func (a *actorInstance) standSequence() AnimSequenceID {
	if a.standSeq != "" {
		return a.standSeq
	}
	switch a.kind {
	case actorKindDog:
		return seqActorDogStand
	case actorKindMutant:
		return seqActorMutantStand
	default:
		return seqActorGuardStand
	}
}

func (a *actorInstance) patrolSequence() AnimSequenceID {
	if a.patrolSeq != "" {
		return a.patrolSeq
	}
	switch a.kind {
	case actorKindDog:
		return seqActorDogPatrol
	case actorKindMutant:
		return seqActorMutantPatrol
	default:
		return seqActorGuardPatrol
	}
}

func (a *actorInstance) chaseSequence() AnimSequenceID {
	if a.chaseSeq != "" {
		return a.chaseSeq
	}
	switch a.kind {
	case actorKindDog:
		return seqActorDogChase
	case actorKindMutant:
		return seqActorMutantChase
	default:
		return seqActorGuardChase
	}
}

func (a *actorInstance) painSequence() AnimSequenceID {
	if a.painSeq != "" {
		return a.painSeq
	}
	if a.kind == actorKindGuard {
		return seqActorGuardPain
	}
	return ""
}

func (g *game) actorPainSequence(a *actorInstance) AnimSequenceID {
	if a == nil {
		return ""
	}
	switch a.kind {
	case actorKindGuard:
		if a.health&1 != 0 {
			return seqActorGuardPain
		}
		return seqActorGuardPain2
	case actorKindMutant:
		if a.health&1 != 0 {
			return seqActorMutantPain
		}
		return seqActorMutantPain2
	case actorKindOfficer:
		if a.health&1 != 0 {
			return seqActorOfficerPain
		}
		return seqActorOfficerPain2
	case actorKindSS:
		if a.health&1 != 0 {
			return seqActorSSPain
		}
		return seqActorSSPain2
	default:
		return a.painSequence()
	}
}

func (a *actorInstance) shootSequence() AnimSequenceID {
	if a.shootSeq != "" {
		return a.shootSeq
	}
	if a.kind == actorKindGuard {
		return seqActorGuardShoot
	}
	return ""
}

func (a *actorInstance) jumpSequence() AnimSequenceID {
	if a.jumpSeq != "" {
		return a.jumpSeq
	}
	if a.kind == actorKindDog {
		return seqActorDogJump
	}
	return ""
}

func (a *actorInstance) deathSequence() AnimSequenceID {
	if a.deathSeq != "" {
		return a.deathSeq
	}
	switch a.kind {
	case actorKindDog:
		return seqActorDogDeath
	case actorKindMutant:
		return seqActorMutantDeath
	default:
		return seqActorGuardDeath
	}
}

func defaultRNG() *wolfRNG {
	return newWolfRNG(0)
}

func wolfSpeedToUnits(speed int) float64 {
	return (float64(speed) / 65536.0) * (70.0 / 60.0)
}

func isActiveGameplayActorKind(kind ActorKind) bool {
	switch kind {
	case actorKindGuard, actorKindOfficer, actorKindSS, actorKindDog, actorKindBoss, actorKindMutant:
		return true
	default:
		return false
	}
}

func actorUsesDirectionalRotation(kind ActorKind) bool {
	return kind != actorKindBoss
}

func (g *game) buildActors() []actorInstance {
	if g.level == nil {
		return nil
	}

	actors := make([]actorInstance, 0, 16)
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			info := g.level.Tile(x, y).RawInfo
			def, ok := LookupActorSpawn(info, g.difficulty)
			if !ok || !isActiveGameplayActorKind(def.Kind) {
				continue
			}

			dropPickup, scoreValue := def.ResolveDrop(g.bestWeapon)
			dir := def.FacingDir(info)
			ambush := g.level.Tile(x, y).Ambush
			if def.Kind == actorKindBoss {
				// WOLFSRC SpawnBoss always starts Hans facing south and in ambush.
				dir = 6
				ambush = true
			}

			actor := actorInstance{
				kind:        def.Kind,
				shapenum:    def.SpawnShape(info),
				x:           float64(x) + 0.5,
				y:           float64(y) + 0.5,
				tileX:       x,
				tileY:       y,
				goalX:       x,
				goalY:       y,
				dir:         dir,
				facingDir:   dir,
				rotate:      def.Rotate,
				blocking:    def.Blocking,
				shootable:   def.Shootable,
				alive:       true,
				ambush:      ambush,
				area:        g.actorAreaAt(x, y),
				health:      def.HitPoints,
				patrolSpeed: wolfSpeedToUnits(def.PatrolSpeed),
				chaseSpeed:  wolfSpeedToUnits(def.ChaseSpeed),
				scoreValue:  scoreValue,
				dropPickup:  dropPickup,
				standSeq:    def.StandSequence,
				patrolSeq:   def.PatrolSequence,
				chaseSeq:    def.ChaseSequence,
				painSeq:     def.PainSequence,
				shootSeq:    def.ShootSequence,
				jumpSeq:     def.JumpSequence,
				deathSeq:    def.DeathSequence,
				spawnMode:   def.SpawnMode,
			}
			switch def.SpawnMode {
			case actorSpawnPatrol:
				actor.aiState = actorStatePatrol
				g.startActorSequence(&actor, actor.patrolSequence(), true)
				g.initPatrolSpawn(&actor)
			default:
				actor.aiState = actorStateStand
				g.startActorSequence(&actor, actor.standSequence(), true)
			}
			actors = append(actors, actor)
		}
	}

	return actors
}

func (g *game) updateActors(tics int) {
	g.rebuildPlayerAreas()
	for i := range g.actors {
		a := &g.actors[i]
		if !a.alive && a.aiState != actorStateDead {
			continue
		}

		prevX, prevY := a.x, a.y
		if a.aiState == actorStatePatrol {
			prevState := a.aiState

			switch a.kind {
			case actorKindGuard:
				g.updateGuardActor(a, tics)
			case actorKindOfficer:
				g.updateGuardActor(a, tics)
			case actorKindSS:
				g.updateGuardActor(a, tics)
			case actorKindBoss:
				g.updateGuardActor(a, tics)
			case actorKindMutant:
				g.updateGuardActor(a, tics)
			case actorKindDog:
				g.updateDogActor(a, tics)
			}
			g.fatalIfActorExceededSpeedBudget(a, prevX, prevY, tics)

			if a.aiState != prevState || a.x != prevX || a.y != prevY {
				g.advanceActorSequence(a, tics)
			}
			continue
		}

		g.advanceActorSequence(a, tics)

		switch a.kind {
		case actorKindGuard:
			g.updateGuardActor(a, tics)
		case actorKindOfficer:
			g.updateGuardActor(a, tics)
		case actorKindSS:
			g.updateGuardActor(a, tics)
		case actorKindBoss:
			g.updateGuardActor(a, tics)
		case actorKindMutant:
			g.updateGuardActor(a, tics)
		case actorKindDog:
			g.updateDogActor(a, tics)
		}
		g.fatalIfActorExceededSpeedBudget(a, prevX, prevY, tics)
	}
}

func (g *game) fatalIfActorExceededSpeedBudget(a *actorInstance, prevX, prevY float64, tics int) {
	if a == nil || tics <= 0 {
		return
	}
	maxSpeed := math.Max(a.patrolSpeed, a.chaseSpeed)
	if maxSpeed <= 0 {
		return
	}
	limit := maxSpeed * float64(tics)
	dx := math.Abs(a.x - prevX)
	dy := math.Abs(a.y - prevY)
	// Actor movement is axis-aligned per dir step, so the per-tick world budget
	// is the larger axis delta rather than Euclidean distance.
	if math.Max(dx, dy) <= limit+1e-9 {
		return
	}
	panic(fmt.Sprintf("actor exceeded speed budget: kind=%v state=%v moved=(%.6f,%.6f) limit=%.6f from=(%.6f,%.6f) to=(%.6f,%.6f)", a.kind, a.aiState, dx, dy, limit, prevX, prevY, a.x, a.y))
}

func (g *game) initPatrolSpawn(a *actorInstance) {
	if a == nil {
		return
	}
	dx, dy := dirStep(a.dir)
	targetX := a.tileX + dx
	targetY := a.tileY + dy
	a.reserveTileGoal(targetX, targetY)
	a.area = g.actorAreaAt(targetX, targetY)
	a.moveDistance = 1
}

func (g *game) updateGuardActor(a *actorInstance, tics int) {
	switch a.aiState {
	case actorStateStand:
		g.updateGuardStand(a, tics)
	case actorStatePatrol:
		g.updateGuardPatrol(a, tics)
	case actorStateChase:
		g.updateGuardChase(a, tics)
	case actorStateShoot:
		if !a.sequenceLoop && g.actorSequenceDone(a) {
			g.guardResumeChase(a)
		}
	case actorStatePain:
		if !a.sequenceLoop && g.actorSequenceDone(a) {
			g.guardResumeChase(a)
		}
	case actorStateDead:
	}
}

func (g *game) updateDogActor(a *actorInstance, tics int) {
	switch a.aiState {
	case actorStateStand:
		g.updateDogStand(a, tics)
	case actorStatePatrol:
		g.updateDogPatrol(a, tics)
	case actorStateChase:
		g.updateDogChase(a, tics)
	case actorStateJump:
		if !a.sequenceLoop && g.actorSequenceDone(a) {
			g.dogResumeChase(a)
		}
	case actorStateDead:
	}
}

func (g *game) updateGuardStand(a *actorInstance, tics int) {
	if !g.actorAcquireTarget(a) {
		return
	}
	if a.reactionTimer > 0 {
		a.reactionTimer -= tics
		if a.reactionTimer > 0 {
			return
		}
		a.reactionTimer = 0
	}
	g.guardFirstSighting(a)
}

func (g *game) updateGuardPatrol(a *actorInstance, tics int) {
	if g.actorAcquireTarget(a) {
		if a.reactionTimer > 0 {
			a.reactionTimer -= tics
			if a.reactionTimer <= 0 {
				a.reactionTimer = 0
				g.guardFirstSighting(a)
				return
			}
		}
	}

	if a.aiState != actorStatePatrol {
		return
	}
	remaining := a.patrolSpeed * float64(tics)
	for remaining > 0 {
		if !a.hasGoal {
			dir := a.dir
			if override, ok := patrolDirFromInfo(g.level.Tile(a.tileX, a.tileY).RawInfo); ok {
				dir = override
			}
			if !g.actorSetGoal(a, dir) {
				return
			}
		}
		consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(a, remaining)
		if blocked || consumed <= 0 {
			return
		}
		remaining -= consumed
		if !reachedGoal {
			return
		}
	}
}

func (g *game) updateGuardChase(a *actorInstance, tics int) {
	dodge := g.actorCanSeePlayer(a)
	if g.guardTryStartShoot(a, tics) {
		return
	}
	remaining := a.chaseSpeed * float64(tics)
	for remaining > 0 {
		if !a.hasGoal {
			g.actorChooseChaseGoal(a, dodge)
			if !a.hasGoal {
				return
			}
		}
		consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(a, remaining)
		if blocked || consumed <= 0 {
			return
		}
		remaining -= consumed
		if !reachedGoal {
			return
		}
	}
}

func (g *game) updateDogStand(a *actorInstance, tics int) {
	if !g.actorAcquireTarget(a) {
		return
	}
	if a.reactionTimer > 0 {
		a.reactionTimer -= tics
		if a.reactionTimer > 0 {
			return
		}
		a.reactionTimer = 0
	}
	g.dogFirstSighting(a)
}

func (g *game) updateDogPatrol(a *actorInstance, tics int) {
	if g.actorAcquireTarget(a) {
		if a.reactionTimer > 0 {
			a.reactionTimer -= tics
			if a.reactionTimer <= 0 {
				a.reactionTimer = 0
				g.dogFirstSighting(a)
				return
			}
		}
	}

	if a.aiState != actorStatePatrol {
		return
	}
	remaining := a.patrolSpeed * float64(tics)
	for remaining > 0 {
		if !a.hasGoal {
			dir := a.dir
			if override, ok := patrolDirFromInfo(g.level.Tile(a.tileX, a.tileY).RawInfo); ok {
				dir = override
			}
			if !g.actorSetGoal(a, dir) {
				return
			}
		}
		consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(a, remaining)
		if blocked || consumed <= 0 {
			return
		}
		remaining -= consumed
		if !reachedGoal {
			return
		}
	}
}

func (g *game) updateDogChase(a *actorInstance, tics int) {
	if g.dogCanStartJump(a, tics) {
		g.startDogJump(a)
		return
	}
	remaining := a.chaseSpeed * float64(tics)
	for remaining > 0 {
		if !a.hasGoal {
			g.actorChooseChaseGoal(a, true)
			if !a.hasGoal {
				return
			}
		}
		consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(a, remaining)
		if blocked || consumed <= 0 {
			return
		}
		remaining -= consumed
		if !reachedGoal {
			return
		}
	}
}

func (g *game) actorAcquireTarget(a *actorInstance) bool {
	if a.alerted {
		return true
	}
	if a.reactionTimer > 0 {
		return true
	}
	if !g.actorCanNoticePlayer(a) {
		return false
	}
	a.reactionTimer = g.actorReactionTics(a)
	if a.reactionTimer == 0 {
		a.reactionTimer = 1
	}
	// WOLFSRC SightPlayer arms the reaction timer and returns. The actor does
	// not burn reaction time until a later tick.
	return false
}

func (g *game) actorCanNoticePlayer(a *actorInstance) bool {
	if g.level == nil {
		return false
	}
	if !g.isAreaConnectedToPlayer(a.area) {
		return false
	}
	hasSight := g.actorHasInitialSight(a)
	if a.ambush {
		if !hasSight {
			return false
		}
		a.ambush = false
		return true
	}
	if g.madeNoise {
		return true
	}
	return hasSight
}

func (g *game) actorHasInitialSight(a *actorInstance) bool {
	if a == nil {
		return false
	}
	dx := g.playerX - a.x
	dy := g.playerY - a.y

	// WOLFSRC CheckSight: very close range ignores facing and line tracing.
	const minSight = 1.5
	if dx > -minSight && dx < minSight && dy > -minSight && dy < minSight {
		return true
	}

	switch a.dir & 7 {
	case 2: // north
		if dy > 0 {
			return false
		}
	case 0: // east
		if dx < 0 {
			return false
		}
	case 6: // south
		if dy < 0 {
			return false
		}
	case 4: // west
		if dx > 0 {
			return false
		}
	}

	return g.actorCanSeePlayer(a)
}

func (g *game) actorReactionTics(a *actorInstance) int {
	switch a.kind {
	case actorKindOfficer:
		return 2
	case actorKindSS:
		return g.ssReactionTics()
	case actorKindMutant:
		return g.ssReactionTics()
	case actorKindDog:
		return g.dogReactionTics()
	case actorKindBoss:
		return 1
	default:
		return g.guardReactionTics()
	}
}

func (g *game) guardReactionTics() int {
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	return maxInt(1, 1+g.rng.Intn(256)/4)
}

func (g *game) dogReactionTics() int {
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	return maxInt(1, 1+g.rng.Intn(256)/8)
}

func (g *game) ssReactionTics() int {
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	return maxInt(1, 1+g.rng.Intn(256)/6)
}

func enemyAlertSound(kind ActorKind) soundID {
	switch kind {
	case actorKindMutant:
		return 0
	case actorKindOfficer:
		return soundEnemyAlertOfficer
	case actorKindSS:
		return soundEnemyAlertSS
	case actorKindBoss:
		return soundEnemyAlertBoss
	case actorKindDog:
		return soundEnemyAlertDog
	default:
		return soundEnemyAlertGuard
	}
}

func enemyAttackSound(kind ActorKind) soundID {
	switch kind {
	case actorKindBoss:
		return soundEnemyAttackBoss
	case actorKindSS:
		return soundEnemyAttackSS
	case actorKindDog:
		return soundEnemyAttackDog
	default:
		return soundEnemyAttackGuard
	}
}

func (g *game) enemyDeathSound(a *actorInstance) soundID {
	if a == nil {
		return soundEnemyDeathGuard
	}
	if g.mapIndex == 9 {
		if g.rng == nil {
			g.rng = defaultRNG()
		}
		if g.rng.Intn(256) == 0 {
			switch a.kind {
			case actorKindGuard, actorKindOfficer, actorKindSS, actorKindDog, actorKindMutant:
				return soundEnemyDeathGuard6
			}
		}
	}
	switch a.kind {
	case actorKindGuard:
		if g.rng == nil {
			g.rng = defaultRNG()
		}
		switch g.rng.Intn(8) {
		case 0:
			return soundEnemyDeathGuard
		case 1:
			return soundEnemyDeathGuard2
		case 2:
			return soundEnemyDeathGuard3
		case 3:
			return soundEnemyDeathGuard4
		case 4:
			return soundEnemyDeathGuard5
		case 5:
			return soundEnemyDeathGuard7
		case 6:
			return soundEnemyDeathGuard8
		default:
			return soundEnemyDeathGuard9
		}
	case actorKindBoss:
		return soundEnemyDeathBoss
	case actorKindMutant:
		return soundEnemyDeathGuard8
	case actorKindOfficer:
		return soundEnemyDeathOfficer
	case actorKindSS:
		return soundEnemyDeathSS
	case actorKindDog:
		return soundEnemyDeathDog
	default:
		return soundEnemyDeathGuard
	}
}

func (g *game) guardFirstSighting(a *actorInstance) {
	if a == nil || !a.alive || a.aiState == actorStateDead {
		return
	}
	a.alerted = true
	a.firstAttack = true
	a.reactionTimer = 0
	if a.kind != actorKindMutant {
		g.playWorldSound(enemyAlertSound(a.kind), a.x, a.y)
	}
	a.aiState = actorStateChase
	a.rotate = actorUsesDirectionalRotation(a.kind)
	if a.moveDistance < 0 {
		a.moveDistance = 0
	}
	g.startActorSequence(a, a.chaseSequence(), true)
}

func (g *game) dogFirstSighting(a *actorInstance) {
	if a == nil || !a.alive || a.aiState == actorStateDead {
		return
	}
	a.alerted = true
	a.firstAttack = true
	a.reactionTimer = 0
	g.playWorldSound(enemyAlertSound(a.kind), a.x, a.y)
	a.aiState = actorStateChase
	a.rotate = true
	if a.moveDistance < 0 {
		a.moveDistance = 0
	}
	g.startActorSequence(a, a.chaseSequence(), true)
}

func (g *game) guardResumeChase(a *actorInstance) {
	a.aiState = actorStateChase
	a.rotate = actorUsesDirectionalRotation(a.kind)
	g.startActorSequence(a, a.chaseSequence(), true)
}

func (g *game) dogResumeChase(a *actorInstance) {
	a.aiState = actorStateChase
	a.rotate = true
	g.startActorSequence(a, a.chaseSequence(), true)
}

func actorStartShootChance(dist, tics int, pointBlank bool) int {
	if pointBlank {
		return 300
	}
	if tics < 1 {
		tics = 1
	}
	return (tics << 4) / maxInt(1, dist)
}

func (g *game) guardTryStartShoot(a *actorInstance, tics int) bool {
	if !g.actorCanSeePlayer(a) {
		return false
	}
	dx := absInt(a.tileX - int(g.playerX))
	dy := absInt(a.tileY - int(g.playerY))
	dist := maxInt(dx, dy)

	pointBlank := dist == 0 || (dist == 1 && (a.moveDistance == 0 || (a.moveDistance > 0 && a.moveDistance < 0.25)))
	chance := actorStartShootChance(dist, tics, pointBlank)
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	if g.rng.Intn(256) >= chance {
		return false
	}
	a.aiState = actorStateShoot
	a.rotate = false
	g.startActorSequence(a, a.shootSequence(), false)
	return true
}

func (g *game) actorCanSeePlayer(a *actorInstance) bool {
	return !g.lineBlocked(a.x, a.y, g.playerX, g.playerY)
}

func (g *game) actorVisibleToPlayer(a *actorInstance) bool {
	if a == nil {
		return false
	}
	dx := a.x - g.playerX
	dy := a.y - g.playerY
	if dx == 0 && dy == 0 {
		return true
	}
	target := math.Atan2(dy, dx)
	return math.Abs(shortestAngleDelta(g.playerA, target)) <= fov/2
}

func (g *game) guardTryShoot(a *actorInstance) {
	if !g.isAreaConnectedToPlayer(a.area) {
		return
	}
	if !g.actorCanSeePlayer(a) {
		return
	}
	distX := absInt(a.tileX - int(g.playerX))
	distY := absInt(a.tileY - int(g.playerY))
	dist := maxInt(distX, distY)
	if a.kind == actorKindSS || a.kind == actorKindBoss {
		dist = (dist * 2) / 3
	}

	hitchance := 128
	if g.playerMovingFast {
		if g.actorVisibleToPlayer(a) {
			hitchance = 160 - dist*16
		} else {
			hitchance = 160 - dist*8
		}
	} else {
		if g.actorVisibleToPlayer(a) {
			hitchance = 256 - dist*16
		} else {
			hitchance = 256 - dist*8
		}
	}
	if hitchance < 0 {
		hitchance = 0
	}
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	hit := g.rng.Intn(256) < hitchance
	if hit {
		damage := 0
		switch {
		case dist < 2:
			damage = g.rng.Intn(256) >> 2
		case dist < 4:
			damage = g.rng.Intn(256) >> 3
		default:
			damage = g.rng.Intn(256) >> 4
		}
		g.takePlayerDamageAt(damage, a.x, a.y, true)
	}
	g.playWorldSound(enemyAttackSound(a.kind), a.x, a.y)
}

func (g *game) startDogJump(a *actorInstance) {
	if a.jumpSequence() == "" {
		return
	}
	a.aiState = actorStateJump
	a.rotate = false
	g.startActorSequence(a, a.jumpSequence(), false)
}

func (g *game) dogCanStartJump(a *actorInstance, tics int) bool {
	if tics < 1 {
		tics = 1
	}
	// T_DogChase compares against MINACTORDIST after subtracting this tick's
	// pending movement from the dog's current chase position. It does not
	// re-check sight or area connectivity once the dog is already in chase.
	reach := math.Max(playerBlockDist, enemyRadius+playerRadius) + a.chaseSpeed*float64(tics)
	dx := math.Abs(g.playerX - a.x)
	dy := math.Abs(g.playerY - a.y)
	return dx <= reach && dy <= reach
}

func (g *game) dogCanBitePlayer(a *actorInstance) bool {
	// T_Bite resolves purely from current proximity. It does not re-check line
	// of sight or area connectivity after the jump has already started.
	reach := playerBlockDist * 2
	dx := math.Abs(g.playerX - a.x)
	dy := math.Abs(g.playerY - a.y)
	return dx <= reach && dy <= reach
}

func (g *game) dogTryBite(a *actorInstance) {
	g.playWorldSound(enemyAttackSound(a.kind), a.x, a.y)
	if !g.dogCanBitePlayer(a) {
		return
	}
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	if g.rng.Intn(256) >= 180 {
		return
	}
	g.takePlayerDamageAt(g.rng.Intn(256)>>4, a.x, a.y, true)
}

func (g *game) takePlayerDamage(damage int) {
	g.takePlayerDamageAt(damage, 0, 0, false)
}

func (g *game) takePlayerDamageAt(damage int, attackerX, attackerY float64, hasAttacker bool) {
	if damage <= 0 || g.health <= 0 {
		return
	}
	if g.godMode {
		g.setNotice("God mode")
		return
	}
	g.playSound(soundPlayerHurt)
	g.startDamageFlash(damage)
	g.health -= damage
	if g.health <= 0 {
		g.beginPlayerDeath(attackerX, attackerY, hasAttacker)
		return
	}
	g.setNotice("Guard hit")
}

func (g *game) actorChooseChaseGoal(a *actorInstance, dodge bool) {
	if dodge {
		g.actorChooseDodgeGoal(a)
		return
	}
	g.actorChooseDirectChaseGoal(a)
}

func vectorToDir(xDir, yDir int) int {
	switch {
	case xDir > 0 && yDir == 0:
		return 0
	case xDir > 0 && yDir < 0:
		return 1
	case xDir == 0 && yDir < 0:
		return 2
	case xDir < 0 && yDir < 0:
		return 3
	case xDir < 0 && yDir == 0:
		return 4
	case xDir < 0 && yDir > 0:
		return 5
	case xDir == 0 && yDir > 0:
		return 6
	case xDir > 0 && yDir > 0:
		return 7
	default:
		return 0
	}
}

func signInt(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

func oppositeDir(dir int) int {
	if dir < 0 || dir > 7 {
		return 8
	}
	return (dir + 4) & 7
}

func diagonalDir(dir1, dir2 int) int {
	dx1, dy1 := dirStep(dir1)
	dx2, dy2 := dirStep(dir2)
	xDir := signInt(dx1 + dx2)
	yDir := signInt(dy1 + dy2)
	if xDir == 0 || yDir == 0 {
		return 8
	}
	return vectorToDir(xDir, yDir)
}

func (g *game) actorChooseDodgeGoal(a *actorInstance) {
	turnaround := oppositeDir(a.dir)
	if a.firstAttack {
		turnaround = 8
		a.firstAttack = false
	}
	deltaX := int(g.playerX) - a.tileX
	deltaY := int(g.playerY) - a.tileY

	dirTry := [5]int{}
	if deltaX > 0 {
		dirTry[1] = 0
		dirTry[3] = 4
	} else {
		dirTry[1] = 4
		dirTry[3] = 0
	}
	if deltaY > 0 {
		dirTry[2] = 6
		dirTry[4] = 2
	} else {
		dirTry[2] = 2
		dirTry[4] = 6
	}

	if absInt(deltaX) > absInt(deltaY) {
		dirTry[1], dirTry[2] = dirTry[2], dirTry[1]
		dirTry[3], dirTry[4] = dirTry[4], dirTry[3]
	}
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	if g.rng.Intn(256) < 128 {
		dirTry[1], dirTry[2] = dirTry[2], dirTry[1]
		dirTry[3], dirTry[4] = dirTry[4], dirTry[3]
	}
	dirTry[0] = diagonalDir(dirTry[1], dirTry[2])

	for _, dir := range dirTry {
		if dir == 8 || dir == turnaround {
			continue
		}
		if g.actorSetGoal(a, dir) {
			return
		}
	}
	if turnaround != 8 && g.actorSetGoal(a, turnaround) {
		return
	}
	a.clearTileGoal()
}

func (g *game) actorChooseDirectChaseGoal(a *actorInstance) {
	oldDir := a.dir
	turnaround := oppositeDir(oldDir)
	deltaX := int(g.playerX) - a.tileX
	deltaY := int(g.playerY) - a.tileY

	firstDir, secondDir := 8, 8
	if deltaX > 0 {
		firstDir = 0
	} else if deltaX < 0 {
		firstDir = 4
	}
	if deltaY > 0 {
		secondDir = 6
	} else if deltaY < 0 {
		secondDir = 2
	}
	if absInt(deltaY) > absInt(deltaX) {
		firstDir, secondDir = secondDir, firstDir
	}
	if firstDir == turnaround {
		firstDir = 8
	}
	if secondDir == turnaround {
		secondDir = 8
	}

	for _, dir := range []int{firstDir, secondDir} {
		if dir != 8 && g.actorSetGoal(a, dir) {
			return
		}
	}
	if oldDir != 8 && g.actorSetGoal(a, oldDir) {
		return
	}

	// WOLFSRC SelectChaseDir only scans the cardinal directions here.
	searchOrder := []int{2, 0, 6, 4}
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	if g.rng.Intn(256) <= 128 {
		searchOrder = []int{4, 6, 0, 2}
	}
	for _, dir := range searchOrder {
		if dir == turnaround {
			continue
		}
		if g.actorSetGoal(a, dir) {
			return
		}
	}
	if turnaround != 8 && g.actorSetGoal(a, turnaround) {
		return
	}
	a.clearTileGoal()
}

func (g *game) actorShouldRunFromPlayer(a *actorInstance) bool {
	if a == nil || a.kind == actorKindDog {
		return false
	}
	deltaX := absInt(int(g.playerX) - a.tileX)
	deltaY := absInt(int(g.playerY) - a.tileY)
	return maxInt(deltaX, deltaY) < 4
}

func (g *game) actorChooseRunGoal(a *actorInstance) {
	deltaX := int(g.playerX) - a.tileX
	deltaY := int(g.playerY) - a.tileY

	firstDir, secondDir := 4, 2
	if deltaX < 0 {
		firstDir = 0
	}
	if deltaY < 0 {
		secondDir = 6
	}
	if absInt(deltaY) > absInt(deltaX) {
		firstDir, secondDir = secondDir, firstDir
	}

	for _, dir := range []int{firstDir, secondDir} {
		if g.actorSetGoal(a, dir) {
			return
		}
	}

	searchOrder := []int{4, 6, 0, 2}
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	if g.rng.Intn(256) > 128 {
		searchOrder = []int{2, 0, 6, 4}
	}
	for _, dir := range searchOrder {
		if g.actorSetGoal(a, dir) {
			return
		}
	}
	a.clearTileGoal()
}

func patrolDirFromInfo(info uint16) (int, bool) {
	if info < 90 || info > 97 {
		return 0, false
	}
	return int(info - 90), true
}

func dirStep(dir int) (int, int) {
	switch dir & 7 {
	case 0:
		return 1, 0
	case 1:
		return 1, -1
	case 2:
		return 0, -1
	case 3:
		return -1, -1
	case 4:
		return -1, 0
	case 5:
		return -1, 1
	case 6:
		return 0, 1
	case 7:
		return 1, 1
	default:
		return 0, 0
	}
}

func (g *game) actorSetGoal(a *actorInstance, dir int) bool {
	return g.actorTryWalk(a, dir)
}

func (g *game) actorTryWalk(a *actorInstance, dir int) bool {
	if a == nil {
		return false
	}
	dx, dy := dirStep(dir)
	if dx == 0 && dy == 0 {
		return false
	}
	targetX := a.tileX + dx
	targetY := a.tileY + dy
	if dx != 0 && dy != 0 {
		if !g.actorDiagTilePassable(a, a.tileX+dx, a.tileY+dy) ||
			!g.actorDiagTilePassable(a, a.tileX+dx, a.tileY) ||
			!g.actorDiagTilePassable(a, a.tileX, a.tileY+dy) {
			return false
		}
		return g.actorReserveGoal(a, dir, targetX, targetY, false)
	}
	passable, waitDoor := g.actorCardinalTilePassable(a, targetX, targetY)
	if !passable {
		return false
	}
	return g.actorReserveGoal(a, dir, targetX, targetY, waitDoor)
}

func (g *game) actorReserveGoal(a *actorInstance, dir, targetX, targetY int, waitDoor bool) bool {
	a.dir = dir & 7
	a.facingDir = a.dir
	a.reserveTileGoal(targetX, targetY)
	a.moveDistance = 1
	if waitDoor {
		a.moveDistance = actorDoorWaitDistance
	}
	if g.level == nil || g.level.Tile(targetX, targetY).Door == nil {
		a.area = g.actorAreaAt(targetX, targetY)
	}
	return true
}

func (g *game) actorCardinalTilePassable(a *actorInstance, x, y int) (passable bool, waitDoor bool) {
	if a == nil || g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false, false
	}
	if !g.actorGoalTileClear(a, x, y) {
		return false, false
	}
	tile := g.level.Tile(x, y)
	if tile.Door != nil {
		if tile.Door.Lock != 0 {
			return false, false
		}
		if g.isDoorOpen(x, y) {
			return true, false
		}
		if a.kind == actorKindDog {
			return false, false
		}
		g.openDoorAt(x, y)
		return true, true
	}
	if tile.Solid {
		return false, false
	}
	return true, false
}

func (g *game) actorDiagTilePassable(a *actorInstance, x, y int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false
	}
	tile := g.level.Tile(x, y)
	if tile.Door != nil {
		return g.isDoorOpen(x, y) && g.actorGoalTileClear(a, x, y)
	}
	if tile.Solid {
		return false
	}
	return g.actorGoalTileClear(a, x, y)
}

func (g *game) blockingActorAt(exclude *actorInstance, x, y int) *actorInstance {
	for i := range g.actors {
		other := &g.actors[i]
		if other == exclude || !other.alive || !other.blocking {
			continue
		}
		if other.tileX == x && other.tileY == y {
			return other
		}
	}
	return nil
}

func (g *game) blockingStaticAt(x, y int) bool {
	for _, spr := range g.staticSprites {
		if !spr.alive || !spr.blocking {
			continue
		}
		if int(spr.x) == x && int(spr.y) == y {
			return true
		}
	}
	return false
}

func (g *game) actorGoalTileClear(a *actorInstance, x, y int) bool {
	return g.blockingActorAt(a, x, y) == nil && !g.blockingStaticAt(x, y)
}

func (g *game) openDoorAt(x, y int) {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return
	}
	tile := g.level.Tile(x, y)
	if tile.Door == nil || tile.Door.Lock != 0 {
		return
	}
	i := y*g.levelWidth + x
	if i < 0 || i >= len(g.doorState) {
		return
	}
	if g.doorState[i] == 0 || g.doorState[i] == 3 {
		g.doorState[i] = 1
		g.doorOpen[i] = max(g.doorOpen[i], 0.01)
		g.playWorldSound(soundDoorOpen, float64(x)+0.5, float64(y)+0.5)
	}
	g.doorTimer[i] = 0
}

func (g *game) moveActorTowardGoal(a *actorInstance, speed float64) {
	g.moveActorTowardGoalStep(a, speed)
}

func (g *game) actorWorldBlockedAt(a *actorInstance, x, y float64) bool {
	if g == nil {
		return true
	}
	tx := int(math.Floor(x))
	ty := int(math.Floor(y))
	if g.isBlockingTile(tx, ty) {
		return true
	}
	if minX, maxX, minY, maxY, ok := g.pushWallBounds(); ok {
		if x >= minX && x < maxX && y >= minY && y < maxY {
			return true
		}
	}
	return false
}

func (g *game) actorPathBlocked(a *actorInstance, fromX, fromY, toX, toY float64) bool {
	dx := toX - fromX
	dy := toY - fromY
	if dx == 0 && dy == 0 {
		return g.actorWorldBlockedAt(a, toX, toY)
	}

	ts := []float64{0, 1}
	collectCrossings := func(start, delta float64) {
		if delta == 0 {
			return
		}
		minPos := start
		maxPos := start + delta
		if minPos > maxPos {
			minPos, maxPos = maxPos, minPos
		}
		first := int(math.Floor(minPos)) - 1
		last := int(math.Floor(maxPos)) + 1
		for grid := first; grid <= last; grid++ {
			t := (float64(grid) - start) / delta
			if t > 0 && t < 1 {
				ts = append(ts, t)
			}
		}
	}
	collectCrossings(fromX, dx)
	collectCrossings(fromY, dy)
	sort.Float64s(ts)
	for i := 0; i < len(ts)-1; i++ {
		t := (ts[i] + ts[i+1]) * 0.5
		x := fromX + dx*t
		y := fromY + dy*t
		if g.actorWorldBlockedAt(a, x, y) {
			return true
		}
	}
	return g.actorWorldBlockedAt(a, toX, toY)
}

func (g *game) moveActorTowardGoalStep(a *actorInstance, speed float64) (consumed float64, reachedGoal bool, blocked bool) {
	if !a.hasGoal {
		return 0, false, true
	}
	targetX := float64(a.tileX) + 0.5
	targetY := float64(a.tileY) + 0.5
	if a.moveDistance < 0 {
		if g.level != nil {
			tile := g.level.Tile(a.tileX, a.tileY)
			if tile.Door != nil && !g.isDoorOpen(a.tileX, a.tileY) {
				g.openDoorAt(a.tileX, a.tileY)
				return 0, false, true
			}
		}
		a.moveDistance = 1
	}
	// Goal state can survive save/load or AI state changes. Only repair
	// obviously-corrupt cached distance here; otherwise preserve the Wolf-style
	// remaining-distance counter instead of recomputing from world position.
	actualRemaining := math.Max(math.Abs(targetX-a.x), math.Abs(targetY-a.y))
	if actualRemaining == 0 {
		a.moveDistance = 0
	} else if a.moveDistance >= 0 && (a.moveDistance == 0 || actualRemaining-a.moveDistance > 0.5) {
		a.moveDistance = actualRemaining
	}
	if g.level != nil {
		tile := g.level.Tile(a.tileX, a.tileY)
		if tile.Door != nil && !g.isDoorOpen(a.tileX, a.tileY) {
			g.openDoorAt(a.tileX, a.tileY)
			return 0, false, true
		}
	}
	if a.moveDistance <= speed || a.moveDistance == 0 {
		if g.isAreaConnectedToPlayer(a.area) && math.Abs(targetX-g.playerX) < playerBlockDist && math.Abs(targetY-g.playerY) < playerBlockDist {
			return 0, false, true
		}
		if g.actorPathBlocked(a, a.x, a.y, targetX, targetY) {
			return 0, false, true
		}
		consumed = a.moveDistance
		a.x = targetX
		a.y = targetY
		if g.level == nil || g.level.Tile(a.tileX, a.tileY).Door == nil {
			a.area = g.actorAreaAt(a.tileX, a.tileY)
		}
		a.clearTileGoal()
		a.moveDistance = 0
		return consumed, true, false
	}
	stepX, stepY := dirStep(a.dir)
	nextX := a.x + float64(stepX)*speed
	nextY := a.y + float64(stepY)*speed
	if g.isAreaConnectedToPlayer(a.area) && math.Abs(nextX-g.playerX) < playerBlockDist && math.Abs(nextY-g.playerY) < playerBlockDist {
		return 0, false, true
	}
	if g.actorPathBlocked(a, a.x, a.y, nextX, nextY) {
		return 0, false, true
	}
	a.x = nextX
	a.y = nextY
	a.moveDistance -= speed
	return speed, false, false
}

func (g *game) startActorSequence(a *actorInstance, id AnimSequenceID, loop bool) bool {
	seq, ok := LookupAnimSequence(id)
	if !ok || len(seq.Frames) == 0 {
		return false
	}
	a.sequenceID = id
	a.sequenceLoop = loop
	a.frameIndex = 0
	a.frameTimer = 0
	a.frameActionDone = false
	a.shapenum = seq.Frames[0].Shape
	return true
}

func (g *game) advanceActorSequence(a *actorInstance, tics int) {
	if a.sequenceID == "" {
		return
	}
	seq, ok := LookupAnimSequence(a.sequenceID)
	if !ok || len(seq.Frames) == 0 {
		return
	}
	if a.frameIndex < 0 || a.frameIndex >= len(seq.Frames) {
		a.frameIndex = 0
	}
	cur := seq.Frames[a.frameIndex]
	if !a.frameActionDone {
		g.actorSequenceActionFrame(a, cur.Action)
		a.frameActionDone = true
	}

	frameTics := cur.Tics
	if frameTics <= 0 {
		return
	}
	a.frameTimer += tics
	for a.frameTimer >= frameTics {
		a.frameTimer -= frameTics
		a.frameIndex++
		if a.frameIndex >= len(seq.Frames) {
			if !a.sequenceLoop {
				a.frameIndex = len(seq.Frames) - 1
				a.frameTimer = 0
				return
			}
			a.frameIndex = 0
		}
		a.frameActionDone = false
		a.shapenum = seq.Frames[a.frameIndex].Shape
		cur = seq.Frames[a.frameIndex]
		frameTics = cur.Tics
		if frameTics <= 0 {
			return
		}
	}
}

func (g *game) actorSequenceActionFrame(a *actorInstance, action AnimAction) {
	switch action {
	case animActionFireActor:
		g.guardTryShoot(a)
	case animActionBiteActor:
		g.dogTryBite(a)
	}
}

func (g *game) actorSequenceDone(a *actorInstance) bool {
	seq, ok := LookupAnimSequence(a.sequenceID)
	if !ok || len(seq.Frames) == 0 {
		return true
	}
	return !a.sequenceLoop && a.frameIndex >= len(seq.Frames)-1 && (seq.Frames[a.frameIndex].Tics <= 0 || a.frameTimer == 0)
}

func (g *game) damageActor(a *actorInstance, damage int) bool {
	if !a.alive || !a.shootable || damage <= 0 {
		return false
	}
	if !a.alerted {
		damage <<= 1
	}
	a.health -= damage
	if a.health <= 0 {
		a.health = 0
		a.alive = false
		a.blocking = false
		a.shootable = false
		a.rotate = false
		a.aiState = actorStateDead
		g.playWorldSound(g.enemyDeathSound(a), a.x, a.y)
		g.startActorSequence(a, a.deathSequence(), false)
		g.score += a.scoreValue
		if a.dropPickup != pickupNone {
			g.spawnDroppedPickup(a.x, a.y, a.dropPickup)
			a.dropPickup = pickupNone
		}
		return true
	}
	if !a.alerted {
		switch a.kind {
		case actorKindDog:
			g.dogFirstSighting(a)
		default:
			g.guardFirstSighting(a)
		}
	}
	painSeq := g.actorPainSequence(a)
	if painSeq == "" {
		return true
	}
	a.aiState = actorStatePain
	a.rotate = false
	a.painSeq = painSeq
	g.startActorSequence(a, painSeq, false)
	return true
}

func (g *game) playerArea() int {
	if g.level == nil {
		return -1
	}
	return g.actorAreaAt(int(g.playerX), int(g.playerY))
}

func (g *game) isAreaConnectedToPlayer(area int) bool {
	if area < 0 {
		return false
	}
	if len(g.playerAreas) == 0 {
		return area == g.playerArea()
	}
	return area < len(g.playerAreas) && g.playerAreas[area]
}

func (g *game) lineBlocked(x1, y1, x2, y2 float64) bool {
	if g.level == nil {
		return true
	}
	x1f := x1 * 256
	y1f := y1 * 256
	x2f := x2 * 256
	y2f := y2 * 256
	xt1 := int(x1f) >> 8
	yt1 := int(y1f) >> 8
	xt2 := int(x2f) >> 8
	yt2 := int(y2f) >> 8

	xdist := absInt(xt2 - xt1)
	if xdist > 0 {
		partial := int(x1f) & 0xff
		xstep := -1
		if xt2 > xt1 {
			partial = 256 - partial
			xstep = 1
		}

		deltafrac := absInt(int(x2f) - int(x1f))
		delta := int(y2f) - int(y1f)
		ystep := 0
		if deltafrac != 0 {
			ystep = (delta << 8) / deltafrac
		}
		yfrac := int(y1f) + (ystep*partial)>>8

		x := xt1 + xstep
		xtEnd := xt2 + xstep
		for x != xtEnd {
			y := yfrac >> 8
			yfrac += ystep
			if g.sightTileBlocked(x, y, true, yfrac-ystep/2) {
				return true
			}
			x += xstep
		}
	}

	ydist := absInt(yt2 - yt1)
	if ydist > 0 {
		partial := int(y1f) & 0xff
		ystep := -1
		if yt2 > yt1 {
			partial = 256 - partial
			ystep = 1
		}

		deltafrac := absInt(int(y2f) - int(y1f))
		delta := int(x2f) - int(x1f)
		xstep := 0
		if deltafrac != 0 {
			xstep = (delta << 8) / deltafrac
		}
		xfrac := int(x1f) + (xstep*partial)>>8

		y := yt1 + ystep
		ytEnd := yt2 + ystep
		for y != ytEnd {
			x := xfrac >> 8
			xfrac += xstep
			if g.sightTileBlocked(x, y, false, xfrac-xstep/2) {
				return true
			}
			y += ystep
		}
	}

	return false
}

func (g *game) sightTileBlocked(x, y int, steppingX bool, intercept int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return true
	}
	tile := g.level.Tile(x, y)
	if tile.Door == nil {
		return tile.Solid
	}

	doorOpen := g.doorOpenness(x, y)
	if doorOpen <= 0 {
		return true
	}
	if doorOpen >= 0.999 {
		return false
	}

	openPos := int(doorOpen * 65535.0)
	return intercept > openPos
}

func (g *game) actorAreaAt(x, y int) int {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return -1
	}
	tile := g.level.Tile(x, y)
	if tile.Area >= 0 {
		return tile.Area
	}
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for _, d := range dirs {
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || ny < 0 || nx >= g.levelWidth || ny >= g.levelHeight {
			continue
		}
		if area := g.level.Tile(nx, ny).Area; area >= 0 {
			return area
		}
	}
	return -1
}
