package main

import "math"

func isEnemyActorKind(kind ActorKind) bool {
	return kind >= actorKindGuard && kind <= actorKindGhost
}

func isTransientActorKind(kind ActorKind) bool {
	return kind >= actorKindNeedle && kind <= actorKindSmoke
}

func actorMarkFlags(kind ActorKind) int {
	switch kind {
	case actorKindNeedle, actorKindRocket:
		return 128 // FL_NONMARK
	case actorKindFire, actorKindSmoke:
		return 4 // FL_NEVERMARK
	default:
		return 0
	}
}

func wolfActorMarkFlags(a *actorInstance) int {
	return a.markFlags | actorMarkFlags(a.kind)
}

func actorHasDeathCamera(kind ActorKind) bool {
	return kind == actorKindSchabbs || kind == actorKindGift || kind == actorKindFat || kind == actorKindHitler
}

func (g *game) restartBossDeathForCamera(a *actorInstance) {
	g.startActorSequence(a, a.deathSequence(), false)
	a.frameIndex = -1 // DoActor follows deathcam.next after the action returns.
	a.frameTimer = -1
	if a.kind == actorKindHitler {
		a.frameTimer = -10
	}
}

func wolfActorOriginalSpeed(a *actorInstance) int {
	if isTransientActorKind(a.kind) {
		return a.projectileSpeed
	}
	speed := a.patrolSpeed
	if a.alerted || a.kind == actorKindGhost || a.kind == actorKindHitler {
		speed = a.chaseSpeed
	}
	return int(math.Round(speed * 65536))
}

func (g *game) initializeRegisteredActor(a *actorInstance) {
	if a.kind < actorKindSchabbs || a.kind > actorKindGhost {
		return
	}
	a.ambush = true
	a.dir = 6
	if a.kind == actorKindGift || a.kind == actorKindFake {
		a.dir = 2
	}
	if a.kind == actorKindGhost {
		a.dir = 0
		a.aiState = actorStateChase
	}
	a.facingDir = a.dir
}

// Spawn callbacks run before the current actor's state transition finishes.
// Defer slice growth until it finishes so append cannot invalidate its pointer.
// The index-based update loop still visits the new tail in this same command.
func (g *game) drainActorSpawns() {
	if len(g.pendingActors) == 0 {
		return
	}
	if d := g.demoPlayback; d != nil {
		for len(d.projections) < len(g.actors)+len(g.pendingActors) {
			d.projections = append(d.projections, demoActorProjection{})
		}
	}
	first := len(g.actors)
	g.actors = append(g.actors, g.pendingActors...)
	if p := g.demoActorPool; g.demoPlayback != nil && p != nil {
		for i := first; i < len(g.actors); i++ {
			p.slots[g.actors[i].poolSlot].index = i
		}
	}
	g.pendingActors = nil
}

func (g *game) pruneRemovedActors() {
	kept := 0
	for i := range g.actors {
		if g.actors[i].removed {
			continue
		}
		g.actors[kept] = g.actors[i]
		if p := g.demoActorPool; g.demoPlayback != nil && p != nil {
			p.slots[g.actors[kept].poolSlot].index = kept
		}
		if d := g.demoPlayback; d != nil && i < len(d.projections) {
			d.projections[kept] = d.projections[i]
		}
		kept++
	}
	g.actors = g.actors[:kept]
	if p := g.demoActorPool; g.demoPlayback != nil && p != nil {
		keptSlots := 0
		for _, slot := range p.order {
			if slot == 0 {
				continue
			}
			p.order[keptSlots] = slot
			p.slots[slot].orderIndex = keptSlots
			keptSlots++
		}
		p.order = p.order[:keptSlots]
	}
	if d := g.demoPlayback; d != nil && len(d.projections) > kept {
		d.projections = d.projections[:kept]
	}
}

func (g *game) updateRegisteredActor(a *actorInstance, tics int) {
	if isTransientActorKind(a.kind) {
		if a.aiState == actorStateProjectile && a.kind != actorKindFire {
			g.updateActorProjectile(a, tics)
		}
		return
	}
	if a.aiState != actorStateChase {
		g.updateGuardActor(a, tics)
		return
	}
	switch a.kind {
	case actorKindSchabbs, actorKindGift, actorKindFat, actorKindFake, actorKindGhost:
		g.updateRegisteredChase(a, tics)
	default:
		g.updateGuardActor(a, tics)
	}
}

// T_Schabb, T_Gift and T_Fat share this chase loop. T_Fake always
// dodges and shoots less often; T_Ghosts only selects direct chase goals.
func (g *game) updateRegisteredChase(a *actorInstance, tics int) {
	ghost, fake := a.kind == actorKindGhost, a.kind == actorKindFake
	dodge := fake
	dist := maxInt(absInt(a.tileX-int(g.playerX)), absInt(a.tileY-int(g.playerY)))
	if !ghost && g.actorCanSeePlayer(a) {
		chance := tics << 3
		if fake {
			chance = tics << 1
		}
		if g.rng.Intn(256) < chance {
			a.aiState = actorStateShoot
			g.startActorSequence(a, a.shootSequence(), false)
			return
		}
		dodge = true
	}
	choose := func(atGoal bool) {
		if atGoal && !fake && !ghost && dist < 4 {
			g.actorChooseRunGoal(a)
		} else {
			g.actorChooseChaseGoal(a, dodge)
		}
	}
	if a.dir == 8 {
		choose(false)
		if a.dir == 8 {
			return
		}
	}
	remaining := a.chaseSpeed * float64(tics)
	for remaining > 0 {
		if !a.hasGoal {
			a.x, a.y = float64(a.tileX)+0.5, float64(a.tileY)+0.5
			choose(true)
			if a.dir == 8 {
				return
			}
		}
		previousDistance := a.moveDistance
		consumed, reached, blocked := g.moveActorTowardGoalStep(a, remaining)
		if blocked || consumed <= 0 {
			return
		}
		remaining -= consumed
		if !reached {
			return
		}
		choose(true)
		if a.dir == 8 {
			a.moveDistance = previousDistance
			return
		}
	}
}

func (g *game) registeredActorAction(a *actorInstance, action AnimAction) {
	switch action {
	case animActionThrowNeedle:
		g.spawnActorProjectile(a, actorKindNeedle)
	case animActionThrowRocket:
		g.spawnActorProjectile(a, actorKindRocket)
	case animActionThrowFire:
		g.spawnActorProjectile(a, actorKindFire)
	case animActionMoveProjectile:
		g.updateActorProjectile(a, a.actionTics)
	case animActionSmoke:
		g.spawnActorSmoke(a)
	case animActionHitlerMorph:
		g.spawnHitler(a)
	case animActionMechaStep:
		if g.isAreaConnectedToPlayer(a.area) {
			g.playWorldSound(soundMechaStep, a.x, a.y)
		}
	case animActionSlurpie:
		g.playSound(soundPickupGibs)
	case animActionBossDeathCam:
		g.startDemoBossDeathCam(a)
	}
}

func (g *game) spawnHitler(a *actorInstance) {
	health := []int{700, 800, 900}[g.difficulty]
	speed := wolfSpeedToUnits(512 * 5)
	if g.demoPlayback != nil {
		speed = 512.0 * 5 / 65536
	}
	hitler := actorInstance{kind: actorKindHitler, x: a.x, y: a.y, tileX: a.tileX, tileY: a.tileY,
		goalX: a.tileX, goalY: a.tileY, dir: a.dir, facingDir: a.dir, health: health,
		alive: true, shootable: true, blocking: true, alerted: a.alerted, firstAttack: a.firstAttack,
		ambush: a.ambush, area: g.actorAreaAt(a.tileX, a.tileY), aiState: actorStateChase,
		markFlags:    wolfActorMarkFlags(a),
		moveDistance: a.moveDistance, hasGoal: a.moveDistance != 0, patrolSpeed: speed, chaseSpeed: speed,
		chaseSeq: seqActorHitlerChase, shootSeq: seqActorHitlerShoot, deathSeq: seqActorHitlerDeath, scoreValue: 5000}
	g.startActorSequence(&hitler, seqActorHitlerChase, true)
	remaining := g.rng.Intn(6)
	hitler.frameTimer, hitler.spawnAnimationFrozen = 6-remaining, remaining == 0
	g.queueDemoActorSpawn(hitler, true)
}

func (g *game) demoActorFrameTics(a *actorInstance, frame AnimFrame) int {
	if a.aiState == actorStateDead && a.frameIndex == 1 {
		switch a.kind {
		case actorKindSchabbs, actorKindGift, actorKindFat, actorKindHitler:
			if g.demoPlayback != nil && g.demoPlayback.sound != nil && g.demoPlayback.sound.mode == "adlib-digi" {
				return 140
			}
			return 5
		}
	}
	return frame.Tics
}
