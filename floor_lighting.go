package main

import "math"

// HD light sprites contain only the fixture. Draw their soft light on the
// horizontal floor plane instead of carrying a painted floor in the billboard.
func (g *game) drawLightFixtureFloorGlows(forwardX, forwardY, projPlaneDist float64) {
	if !g.shouldUseHDAssets() || g.sprites == nil {
		return
	}
	for _, spr := range g.staticSprites {
		if !spr.alive {
			continue
		}
		var strength [3]float64
		switch spr.shapenum {
		case shapeSPR_STAT_3, shapeSPR_STAT_4: // Standing lamp and golden chandelier.
			strength = [3]float64{32, 28, 16}
		case shapeSPR_STAT_14: // Green hanging lamp.
			strength = [3]float64{42, 37, 25}
		default:
			continue
		}
		if sprite, ok := g.sprites.SpriteByShape(spr.shapenum); !ok || sprite.IndexedFormat {
			continue
		}
		g.drawFloorLight(spr.x, spr.y, strength, forwardX, forwardY, projPlaneDist)
	}
}

func (g *game) drawFloorLight(lightX, lightY float64, strength [3]float64, forwardX, forwardY, projPlaneDist float64) {
	// Keep the footprint within the light's map tile, preventing light leaking
	// into adjacent rooms through a wall.
	const radius = 0.48
	width, height := g.layout.bufferWidth, g.layout.bufferHeight
	dx, dy := lightX-g.playerX, lightY-g.playerY
	depth := dx*forwardX + dy*forwardY
	if depth+radius <= 0.01 || depth-radius >= maxRayDepth {
		return
	}
	lateral := -dx*forwardY + dy*forwardX
	nearDepth, farDepth := math.Max(0.01, depth-radius), depth+radius
	halfHeight := float64(height) / 2
	startY := maxInt(height/2+1, int(halfHeight+projPlaneDist/(2*farDepth)))
	endY := minInt(height, int(math.Ceil(halfHeight+projPlaneDist/(2*nearDepth)))+1)
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, d := range [2]float64{nearDepth, farDepth} {
		for _, side := range [2]float64{lateral - radius, lateral + radius} {
			x := float64(width)/2 + projPlaneDist*side/d
			minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		}
	}
	startX, endX := maxInt(0, int(math.Floor(minX))), minInt(width, int(math.Ceil(maxX))+1)
	for y := startY; y < endY; y++ {
		// The camera is halfway between the floor and ceiling. Intersect each
		// viewing ray with z=0 using the same projection as walls and sprites.
		floorDepth := projPlaneDist / (2 * (float64(y) + 0.5 - halfHeight))
		for x := startX; x < endX; x++ {
			if floorDepth >= g.zbuffer[x] {
				continue
			}
			worldX := g.playerX + g.rayDirXColumns[x]*floorDepth
			worldY := g.playerY + g.rayDirYColumns[x]*floorDepth
			lx, ly := worldX-lightX, worldY-lightY
			falloff := 1 - (lx*lx+ly*ly)/(radius*radius)
			if falloff <= 0 {
				continue
			}
			falloff *= falloff
			i := y*width + x
			pixel := g.gameplayFrame32[i]
			r := minInt(255, int(byte(pixel))+int(strength[0]*falloff))
			gr := minInt(255, int(byte(pixel>>8))+int(strength[1]*falloff))
			b := minInt(255, int(byte(pixel>>16))+int(strength[2]*falloff))
			g.gameplayFrame32[i] = uint32(r) | uint32(gr)<<8 | uint32(b)<<16 | pixel&0xff000000
		}
	}
}
