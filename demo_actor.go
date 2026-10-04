package main

// advanceDemoActorSequence follows DoActor: decrement the current timer,
// execute actions on state exit, then think only in the resulting state.
// action is supplied so the compiled-C scheduler comparison can record the
// same firing/bite callbacks without involving unverified combat decisions.
func (g *game) advanceDemoActorSequence(a *actorInstance, tics int, action func(AnimAction)) bool {
	seq, ok := LookupAnimSequence(a.sequenceID)
	if !ok || len(seq.Frames) == 0 {
		return false
	}
	if a.spawnAnimationFrozen || seq.Frames[a.frameIndex].Tics == 0 {
		return demoActorFrameThinks(a)
	}
	a.frameTimer += tics
	for a.frameTimer >= seq.Frames[a.frameIndex].Tics {
		frame := seq.Frames[a.frameIndex]
		a.frameTimer -= frame.Tics
		if a.aiState == actorStateDead && a.frameIndex == 0 {
			action(animActionDeathScream)
		}
		if frame.Action != animActionNone {
			action(frame.Action)
		}
		a.frameIndex++
		if a.frameIndex >= len(seq.Frames) {
			if a.sequenceLoop {
				a.frameIndex = 0
			} else if a.kind == actorKindDog && a.aiState == actorStateDead {
				// WL_ACT2.C links s_dogdead to itself with a 15-tic timer.
				a.frameIndex = len(seq.Frames) - 1
			} else {
				// Pain, shooting and dog jump states link directly to chase.
				// Preserve excess elapsed time, rather than running that chase
				// state for a whole additional command.
				carry := a.frameTimer
				switch a.aiState {
				case actorStatePain, actorStateShoot, actorStateJump:
					if a.kind == actorKindDog {
						g.dogResumeChase(a)
					} else {
						g.guardResumeChase(a)
					}
					a.frameTimer = carry
					seq, _ = LookupAnimSequence(a.sequenceID)
				default:
					// Dead terminal states hold their last shape.
					a.frameIndex = len(seq.Frames) - 1
					a.frameTimer = 0
					return false
				}
			}
		}
		a.shapenum = seq.Frames[a.frameIndex].Shape
		if seq.Frames[a.frameIndex].Tics == 0 {
			a.frameTimer = 0
			break
		}
	}
	return demoActorFrameThinks(a)
}

func demoActorFrameThinks(a *actorInstance) bool {
	switch a.aiState {
	case actorStateStand:
		return true
	case actorStatePatrol, actorStateChase:
		// WOLFSRC path1s/path3s and chase1s/chase3s have no think callback.
		return a.frameIndex != 1 && a.frameIndex != 4
	default:
		return false
	}
}

func (g *game) selectDemoPathGoal(a *actorInstance) {
	dir := a.dir
	if override, ok := patrolDirFromInfo(g.level.Tile(a.tileX, a.tileY).RawInfo); ok {
		dir = override
	}
	a.moveDistance = 1
	if !g.actorSetGoal(a, dir) {
		a.dir = 8
	}
}

func (g *game) updateDemoActors(tics int) {
	g.rebuildPlayerAreas()
	for i := range g.actors {
		a := &g.actors[i]
		if !a.alive && a.aiState != actorStateDead {
			continue
		}
		// DoActor suspends any inactive actor in a disconnected area.
		// DrawScaleds permanently activates actors once their tile is visible.
		if !a.demoActive && !g.isAreaConnectedToPlayer(a.area) {
			continue
		}
		prevX, prevY := a.x, a.y
		if g.advanceDemoActorSequence(a, tics, func(action AnimAction) {
			g.actorSequenceActionFrame(a, action)
		}) {
			if a.kind == actorKindDog {
				g.updateDogActor(a, tics)
			} else {
				g.updateGuardActor(a, tics)
			}
		}
		g.fatalIfActorExceededSpeedBudget(a, prevX, prevY, tics)
	}
}
