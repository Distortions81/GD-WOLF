package main

import "math"

// The original chase/path loops call MoveObj only for a partial tile step.
// Reaching the reserved tile snaps directly to its center, without a second
// collision check. TryWalk already validated and reserved the destination.
func (g *game) moveDemoActorTowardGoalStep(a *actorInstance, move float64) (consumed float64, reachedGoal bool, blocked bool) {
	if a.moveDistance < 0 {
		door := g.demoDoorByIndex(int(-a.moveDistance) - 1)
		g.openDoorAt(door.x, door.y)
		if !g.isDoorOpen(door.x, door.y) {
			return 0, false, true
		}
		a.moveDistance = 1
	}
	if move >= a.moveDistance {
		consumed = a.moveDistance
		a.x, a.y = float64(a.tileX)+0.5, float64(a.tileY)+0.5
		a.clearTileGoal()
		a.moveDistance = 0
		return consumed, true, false
	}
	if a.dir == 8 {
		return 0, false, true
	}
	dx, dy := dirStep(a.dir)
	x, y := a.x+float64(dx)*move, a.y+float64(dy)*move
	if g.isAreaConnectedToPlayer(a.area) && math.Abs(x-g.playerX) <= playerBlockDist && math.Abs(y-g.playerY) <= playerBlockDist {
		if a.kind == actorKindGhost {
			g.takePlayerDamageAt(a.actionTics*2, x, y, true)
		}
		return 0, false, true
	}
	a.x, a.y = x, y
	a.moveDistance -= move
	return move, false, false
}
