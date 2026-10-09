package main

import "math"

func (g *game) spawnActorProjectile(source *actorInstance, kind ActorKind) {
	// Original stores atan2 in a 32-bit float before converting to degrees.
	angle := float32(math.Atan2(source.y-g.playerY, g.playerX-source.x))
	if angle < 0 {
		angle = float32(2*math.Pi + float64(angle))
	}
	degrees := int(float64(angle) / (2 * math.Pi) * 360)
	sequence, speed, sound := seqActorNeedle, 0x2000, soundProjectileNeedle
	switch kind {
	case actorKindRocket:
		sequence, sound = seqActorRocket, soundProjectileRocket
	case actorKindFire:
		sequence, speed, sound = seqActorFire, 0x1200, soundProjectileFire
	}
	a := actorInstance{kind: kind, x: source.x, y: source.y, tileX: source.tileX, tileY: source.tileY,
		dir: 8, angle: degrees, projectileSpeed: speed, alive: true, demoActive: true, aiState: actorStateProjectile}
	g.startActorSequence(&a, sequence, true)
	seq, _ := LookupAnimSequence(sequence)
	a.frameTimer = seq.Frames[0].Tics - 1
	a.rotate = kind == actorKindRocket
	g.queueDemoActorSpawn(a, false)
	g.playWorldSound(sound, a.x, a.y)
}

func (g *game) spawnActorSmoke(source *actorInstance) {
	a := actorInstance{kind: actorKindSmoke, x: source.x, y: source.y, tileX: source.tileX, tileY: source.tileY,
		alive: true, demoActive: true, aiState: actorStateEffect}
	g.startActorSequence(&a, seqActorSmoke, false)
	a.frameTimer = -3 // s_smoke1 lasts three tics, but A_Smoke assigns ticcount=6.
	g.queueDemoActorSpawn(a, false)
}

func (g *game) projectilePositionClear(x, y int) bool {
	const radius = 0x2000
	for ty := (y - radius) >> 16; ty <= (y+radius)>>16; ty++ {
		for tx := (x - radius) >> 16; tx <= (x+radius)>>16; tx++ {
			if g.level == nil || tx < 0 || ty < 0 || tx >= g.levelWidth || ty >= g.levelHeight {
				return false
			}
			if g.demoPlayback != nil {
				if tag := g.demoActorTagAt(tx, ty); tag != 0 && tag < 256 {
					return false
				}
				continue
			}
			tile := g.level.Tile(tx, ty)
			if tile.Door != nil {
				if !g.isDoorOpen(tx, ty) {
					return false
				}
			} else if tile.Solid || g.blockingStaticAt(tx, ty) {
				return false
			}
		}
	}
	return true
}

func (g *game) updateActorProjectile(a *actorInstance, tics int) {
	speed := a.projectileSpeed * tics
	dx := wolfDemoFixedByFrac(speed, wolfDemoTrigTable[a.angle+90])
	dy := -wolfDemoFixedByFrac(speed, wolfDemoTrigTable[a.angle])
	// T_Projectile only caps positive components; preserve that asymmetry.
	dx, dy = minInt(dx, 65536), minInt(dy, 65536)
	x, y := int(math.Round(a.x*65536))+dx, int(math.Round(a.y*65536))+dy
	a.x, a.y = float64(x)/65536, float64(y)/65536
	if !g.projectilePositionClear(x, y) {
		if a.kind == actorKindRocket {
			g.playWorldSound(soundProjectileHit, a.x, a.y)
			seq, _ := LookupAnimSequence(a.sequenceID)
			remaining := seq.Frames[a.frameIndex].Tics - a.frameTimer
			g.startActorSequence(a, seqActorBoom, false)
			a.frameTimer = 6 - remaining // direct state assignment preserves ticcount.
			a.aiState, a.rotate = actorStateEffect, false
		} else {
			a.removed = true
		}
		return
	}
	if absInt(x-int(math.Round(g.playerX*65536))) < 0xc000 && absInt(y-int(math.Round(g.playerY*65536))) < 0xc000 {
		damage := g.rng.Intn(256) >> 3
		if a.kind == actorKindNeedle {
			damage += 20
		} else if a.kind == actorKindRocket {
			damage += 30
		}
		g.takePlayerDamageAt(damage, a.x, a.y, true)
		a.removed = true
		return
	}
	a.tileX, a.tileY = x>>16, y>>16
}
