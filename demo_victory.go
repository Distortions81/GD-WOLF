package main

import "math"

// BJ is an ordinary, non-shootable actor in the original linked list. Keeping
// him in that list preserves spawn order, door occupancy and activation.
const actorKindVictoryBJ ActorKind = 100
const actorStateVictoryRun ActorAIState = 9

func (g *game) spawnDemoVictoryBJ() {
	a := actorInstance{
		kind: actorKindVictoryBJ, x: g.playerX, y: g.playerY,
		tileX: int(g.playerX), tileY: int(g.playerY) + 1,
		dir: 2, facingDir: 2, alive: true, blocking: true,
		aiState: actorStateVictoryRun, reactionTimer: 6,
	}
	a.area = g.actorAreaAt(a.tileX, a.tileY)
	g.startActorSequence(&a, seqVictoryBJRun, true)
	remaining := g.rng.Intn(12)
	a.frameTimer, a.spawnAnimationFrozen = 12-remaining, remaining == 0
	g.queueDemoActorSpawn(a, true)
	g.drainActorSpawns()
	g.victoryActive = true
}

func (g *game) advanceDemoVictoryPlayer(tics int) {
	d := g.demoPlayback
	if d.angle > 270 {
		d.angle = maxInt(270, d.angle-3*tics)
	} else if d.angle < 270 {
		d.angle = minInt(270, d.angle+3*tics)
	}
	g.playerA = -float64(d.angle) * math.Pi / 180
	destY := float64((d.victoryTileY-5)*65536-0x3000) / 65536
	if g.playerY > destY {
		g.playerY = math.Max(destY, g.playerY-float64(tics*4096)/65536)
	}
}

func (g *game) updateDemoVictoryActor(a *actorInstance, tics int) {
	seq, _ := LookupAnimSequence(a.sequenceID)
	if !a.spawnAnimationFrozen {
		a.frameTimer += tics
		for a.frameTimer >= seq.Frames[a.frameIndex].Tics {
			a.frameTimer -= seq.Frames[a.frameIndex].Tics
			if a.aiState == actorStateJump {
				switch a.frameIndex {
				case 1:
					g.playWorldSound(soundVictoryYell, a.x, a.y)
				case 3:
					g.demoPlayback.levelExit = 6 // ex_victorious
				}
			}
			if a.aiState == actorStateJump && a.frameIndex == 3 {
				// s_bjjump4 points back to itself.
			} else {
				a.frameIndex = (a.frameIndex + 1) % len(seq.Frames)
			}
			a.shapenum = seq.Frames[a.frameIndex].Shape
		}
	}
	if a.aiState == actorStateJump {
		if a.frameIndex < 3 {
			g.moveDemoVictoryActor(a, 680*tics)
		}
		return
	}
	if a.frameIndex == 1 || a.frameIndex == 4 {
		return
	}
	move := 2048 * tics
	for move > 0 {
		distance := int(math.Round(a.moveDistance * 65536))
		if move < distance {
			g.moveDemoVictoryActor(a, move)
			return
		}
		a.x, a.y = float64(a.tileX)+0.5, float64(a.tileY)+0.5
		move -= distance
		g.selectDemoPathGoal(a)
		a.reactionTimer--
		if a.reactionTimer == 0 {
			a.aiState = actorStateJump
			g.startActorSequence(a, seqVictoryBJJump, false)
			return
		}
	}
}

func (g *game) moveDemoVictoryActor(a *actorInstance, move int) {
	if a.dir == 8 {
		return
	}
	dx, dy := dirStep(a.dir)
	x := a.x + float64(dx*move)/65536
	y := a.y + float64(dy*move)/65536
	// Original MoveObj includes both equality boundaries at MINACTORDIST.
	if g.isAreaConnectedToPlayer(a.area) && math.Abs(x-g.playerX) <= 1 && math.Abs(y-g.playerY) <= 1 {
		return
	}
	a.x, a.y = x, y
	a.moveDistance -= float64(move) / 65536
}

func (g *game) recordDemoBossKill() {
	if d := g.demoPlayback; d != nil {
		d.bossKillX, d.bossKillY = g.playerX, g.playerY
	}
}

func (g *game) startDemoBossDeathCam(a *actorInstance) {
	d := g.demoPlayback
	if d == nil {
		return
	}
	if g.victoryActive {
		d.levelExit = 6
		return
	}
	g.victoryActive, d.deathCam = true, true
	g.demoMemory().clearDirtyBlocks()
	g.attacking = false
	fangle := float32(math.Atan2(d.bossKillY-a.y, a.x-d.bossKillX))
	if fangle < 0 {
		fangle = float32(float64(fangle) + 2*math.Pi)
	}
	d.angle = int(float64(fangle) / (2 * math.Pi) * 360)
	g.playerA = -float64(d.angle) * math.Pi / 180
	for distance := 0x14000; ; distance += 0x1000 {
		dx := wolfDemoFixedByFrac(distance, wolfDemoTrigTable[d.angle+90])
		dy := -wolfDemoFixedByFrac(distance, wolfDemoTrigTable[d.angle])
		g.playerX, g.playerY = a.x-float64(dx)/65536, a.y-float64(dy)/65536
		if g.demoDeathCameraPositionClear() {
			break
		}
	}
	g.restartBossDeathForCamera(a)
	g.demoMemory().markPlayBorder()
}

func (g *game) demoDeathCameraPositionClear() bool {
	x, y := int(math.Round(g.playerX*65536)), int(math.Round(g.playerY*65536))
	for tileY := (y - 0x5800) >> 16; tileY <= (y+0x5800)>>16; tileY++ {
		for tileX := (x - 0x5800) >> 16; tileX <= (x+0x5800)>>16; tileX++ {
			if tag := g.demoActorTagAt(tileX, tileY); tag != 0 && tag < 256 {
				return false
			}
		}
	}
	return true
}
