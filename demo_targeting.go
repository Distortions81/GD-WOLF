package main

import "math"

const demoViewWidth = 240 // Original default viewsize 15, SetViewSize.

type demoActorProjection struct {
	visible bool
	viewX   int
	transX  int
}

type demoActorHit struct {
	index  int
	damage int
}

// The original player attacks using the projection from the previous refresh.
// Keep it independent of the desktop viewport and port's wider/adjustable view.
func (g *game) refreshDemoActorProjections() {
	d := g.demoPlayback
	if len(d.projections) != len(g.actors) {
		d.projections = make([]demoActorProjection, len(g.actors))
	}
	cos, sin := wolfDemoTrigTable[d.angle+90], wolfDemoTrigTable[d.angle]
	px, py := int(math.Round(g.playerX*65536)), int(math.Round(g.playerY*65536))
	viewX := px - wolfDemoFixedByFrac(0x5700, cos)
	viewY := py + wolfDemoFixedByFrac(0x5700, sin)
	visible := g.demoVisibleTiles(viewX, viewY)
	d.visibleTiles = visible
	for i := range g.actors {
		a := &g.actors[i]
		p := &d.projections[i]
		if !g.demoActorTileVisible(a, visible) {
			p.visible = false
			continue
		}
		a.demoActive = true
		if transformDemoActor(a, p, viewX, viewY, cos, sin) {
			p.visible = true
		}
	}
}

func transformDemoActor(a *actorInstance, p *demoActorProjection, viewX, viewY int, cos, sin uint32) bool {
	gx := int(math.Round(a.x*65536)) - viewX
	gy := int(math.Round(a.y*65536)) - viewY
	nx := wolfDemoSignedFixedByFrac(gx, cos) - wolfDemoSignedFixedByFrac(gy, sin) - 0x4000
	ny := wolfDemoSignedFixedByFrac(gy, cos) + wolfDemoSignedFixedByFrac(gx, sin)
	p.transX = nx
	if nx < 0x5800 {
		// DrawScaleds continues without clearing FL_VISABLE; the old
		// screen X survives TransformActor's too-close early return.
		return false
	}
	scale := (demoViewWidth / 2) * (0x5700 + 0x5800) / 32768
	p.viewX = demoViewWidth/2 - 1 + ny*scale/nx
	return true
}

func wolfDemoSignedFixedByFrac(value int, fraction uint32) int {
	if value < 0 {
		return -wolfDemoFixedByFrac(-value, fraction)
	}
	return wolfDemoFixedByFrac(value, fraction)
}

// DrawScaleds collects a visible bonus using TransformTile, rather than the
// player's floor tile. The focal point sits behind the player's position.
func (g *game) demoPickupInReach(x, y int) bool {
	d := g.demoPlayback
	index := y*g.levelWidth + x
	if index < 0 || index >= len(d.visibleTiles) || !d.visibleTiles[index] {
		return false
	}
	cos, sin := wolfDemoTrigTable[d.angle+90], wolfDemoTrigTable[d.angle]
	viewX := int(math.Round(g.playerX*65536)) - wolfDemoFixedByFrac(0x5700, cos)
	viewY := int(math.Round(g.playerY*65536)) + wolfDemoFixedByFrac(0x5700, sin)
	gx, gy := x*65536+32768-viewX, y*65536+32768-viewY
	nx := wolfDemoSignedFixedByFrac(gx, cos) - wolfDemoSignedFixedByFrac(gy, sin) - 0x2000
	ny := wolfDemoSignedFixedByFrac(gy, cos) + wolfDemoSignedFixedByFrac(gx, sin)
	return nx >= 0x5800 && nx < 65536 && ny > -32768 && ny < 32768
}

func (g *game) shootDemoAhead() {
	g.demoPlayback.shots++
	if g.weapon != 0 {
		g.madeNoise = true
	}
	best, distance := -1, int(^uint(0)>>1)
	for i, a := range g.actors {
		if !a.alive || !a.shootable || i >= len(g.demoPlayback.projections) {
			continue
		}
		p := g.demoPlayback.projections[i]
		if !p.visible || absInt(p.viewX-(demoViewWidth/2-1)) >= demoViewWidth/10 || p.transX >= distance {
			continue
		}
		best, distance = i, p.transX
	}
	if best < 0 || (g.weapon == 0 && distance > 0x18000) {
		return
	}
	a := &g.actors[best]
	if g.weapon != 0 && g.lineBlocked(a.x, a.y, g.playerX, g.playerY) {
		return
	}
	tileDistance := maxInt(absInt(a.tileX-int(g.playerX)), absInt(a.tileY-int(g.playerY)))
	damage := 0
	if g.weapon != 0 && tileDistance >= 4 {
		// A range miss returns without DamageActor. A real hit rolling zero
		// still alerts/stuns the actor, so these outcomes must stay distinct.
		if g.rng.Intn(256)/12 < tileDistance {
			return
		}
		damage = g.rng.Intn(256) / 6
	} else {
		damage = g.playerAttackDamage(tileDistance)
	}
	g.demoPlayback.hits = append(g.demoPlayback.hits, demoActorHit{best, damage})
	g.damageActor(a, damage)
}
