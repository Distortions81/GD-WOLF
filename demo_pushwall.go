package main

import "gd-wolf/internal/wl6"

func (g *game) updateDemoPushWall(tics int) {
	g.pushWall.tics += tics
	for g.pushWall.active && g.pushWall.tics >= pushWallStepTics {
		g.pushWall.tics -= pushWallStepTics
		area := g.playerArea()
		g.setLevelTile(g.pushWall.x, g.pushWall.y, wl6.Tile{Area: area})
		g.pushWall.x += g.pushWall.dx
		g.pushWall.y += g.pushWall.dy
		g.pushWall.steps++
		if g.pushWall.steps >= pushWallMaxSteps {
			g.finishPushWall()
			return
		}
		x, y := g.pushWall.x+g.pushWall.dx, g.pushWall.y+g.pushWall.dy
		if !g.pushWallDestinationClear(x, y) {
			g.finishPushWall()
			return
		}
		leading := g.pushWall.wall
		leading.Area = g.level.Tile(x, y).Area
		g.setLevelTile(x, y, leading)
		g.refreshLevelGeometry()
	}
}
