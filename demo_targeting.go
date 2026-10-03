package main

import "math"

const demoViewWidth = 304 // Original default viewsize 15, SetViewSize.

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
	d.projections = make([]demoActorProjection, len(g.actors))
	cos, sin := wolfDemoTrigTable[d.angle+90], wolfDemoTrigTable[d.angle]
	px, py := int(math.Round(g.playerX*65536)), int(math.Round(g.playerY*65536))
	viewX := px - wolfDemoFixedByFrac(0x5700, cos)
	viewY := py + wolfDemoFixedByFrac(0x5700, sin)
	scale := (demoViewWidth / 2) * (0x5700 + 0x5800) / 32768
	for i, a := range g.actors {
		gx := int(math.Round(a.x*65536)) - viewX
		gy := int(math.Round(a.y*65536)) - viewY
		nx := wolfDemoSignedFixedByFrac(gx, cos) - wolfDemoSignedFixedByFrac(gy, sin) - 0x4000
		ny := wolfDemoSignedFixedByFrac(gy, cos) + wolfDemoSignedFixedByFrac(gx, sin)
		p := &d.projections[i]
		p.transX = nx
		if nx < 0x5800 {
			continue
		}
		p.viewX = demoViewWidth/2 - 1 + ny*scale/nx
		// Visibility coverage is refined separately from TransformActor.
		// For targets near the crosshair, CheckLine establishes a clear path.
		p.visible = p.viewX >= -demoViewWidth/2 && p.viewX < demoViewWidth*3/2 &&
			!g.lineBlocked(a.x, a.y, g.playerX, g.playerY)
	}
}

func wolfDemoSignedFixedByFrac(value int, fraction uint32) int {
	if value < 0 {
		return -wolfDemoFixedByFrac(-value, fraction)
	}
	return wolfDemoFixedByFrac(value, fraction)
}

func (g *game) shootDemoAhead() {
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
		if g.weapon != 0 && g.lineBlocked(a.x, a.y, g.playerX, g.playerY) {
			continue
		}
		best, distance = i, p.transX
	}
	if best < 0 || (g.weapon == 0 && distance > 0x18000) {
		return
	}
	a := &g.actors[best]
	tileDistance := maxInt(absInt(a.tileX-int(g.playerX)), absInt(a.tileY-int(g.playerY)))
	damage := g.playerAttackDamage(tileDistance)
	g.demoPlayback.hits = append(g.demoPlayback.hits, demoActorHit{best, damage})
	g.damageActor(a, damage)
}
