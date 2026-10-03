package main

import "math"

// BuildTables/CalcProjection use a float radtoint and float atan result.
var demoFineTangents, demoPixelAngles = func() ([900]int, [demoViewWidth]int) {
	var tangents [900]int
	var pixels [demoViewWidth]int
	radToInt := float32(3600 / 2 / 3.141592657)
	for i := 0; i < 450; i++ {
		tangent := math.Tan((float64(i) + 0.5) / float64(radToInt))
		tangents[i] = int(tangent * 65536)
		tangents[899-i] = int(65536 / tangent)
	}
	for i := 0; i < demoViewWidth/2; i++ {
		tangent := float64(i*65536/demoViewWidth) / float64(0x5700+0x5800)
		angle := int(float32(math.Atan(tangent)) * radToInt)
		pixels[demoViewWidth/2-1-i], pixels[demoViewWidth/2+i] = angle, -angle
	}
	return tangents, pixels
}()

// demoVisibleTiles translates the traversal in WL_DR_A.ASM AsmRefresh.
// Only floor visibility is needed; wall heights and texture drawing are omitted.
func (g *game) demoVisibleTiles(viewX, viewY int) []bool {
	visible := make([]bool, len(g.level.Tiles))
	for _, offset := range demoPixelAngles {
		angle := (g.demoPlayback.angle*10 + offset + 3600) % 3600
		var dx, dy, xStep, yStep, xPartial, yPartial int
		switch {
		case angle < 900:
			dx, dy = 1, -1
			xStep, yStep = demoFineTangents[899-angle], -demoFineTangents[angle]
		case angle < 1800:
			dx, dy = -1, -1
			xStep, yStep = -demoFineTangents[angle-900], -demoFineTangents[1799-angle]
		case angle < 2700:
			dx, dy = -1, 1
			xStep, yStep = -demoFineTangents[2699-angle], demoFineTangents[angle-1800]
		default:
			dx, dy = 1, 1
			xStep, yStep = demoFineTangents[angle-2700], demoFineTangents[3599-angle]
		}
		xPartial, yPartial = viewX&65535, viewY&65535
		if dx > 0 {
			xPartial = (65536 - xPartial) & 65535
		}
		if dy > 0 {
			yPartial = (65536 - yPartial) & 65535
		}
		// Assembly partial products truncate magnitude before restoring sign.
		yIntercept := viewY + int(int64(yStep)*int64(xPartial)/65536)
		xIntercept := viewX + int(int64(xStep)*int64(yPartial)/65536)
		xTile, yTile := (viewX>>16)+dx, (viewY>>16)+dy
		vertical := true
		for steps := 0; steps < 256; steps++ {
			if vertical {
				if (dy < 0 && yIntercept>>16 <= yTile) || (dy > 0 && yIntercept>>16 >= yTile) {
					vertical = false
					continue
				}
				if !g.demoRayPassesTile(xTile, yIntercept>>16, yIntercept, yStep) {
					break
				}
				visible[(yIntercept>>16)*g.levelWidth+xTile] = true
				xTile += dx
				yIntercept += yStep
			} else {
				if (dx < 0 && xIntercept>>16 <= xTile) || (dx > 0 && xIntercept>>16 >= xTile) {
					// horizcheck branches straight to vertentry, avoiding a
					// second comparison at a grid corner.
					if !g.demoRayPassesTile(xTile, yIntercept>>16, yIntercept, yStep) {
						break
					}
					visible[(yIntercept>>16)*g.levelWidth+xTile] = true
					xTile += dx
					yIntercept += yStep
					vertical = true
					continue
				}
				if !g.demoRayPassesTile(xIntercept>>16, yTile, xIntercept, xStep) {
					break
				}
				visible[yTile*g.levelWidth+(xIntercept>>16)] = true
				yTile += dy
				xIntercept += xStep
			}
		}
	}
	return visible
}

func (g *game) demoRayPassesTile(x, y, intercept, step int) bool {
	if x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false
	}
	if g.pushWall.active && x == g.pushWall.x && y == g.pushWall.y {
		shifted := intercept + int(int64(step)*int64(g.pushWall.tics/2)>>6)
		return shifted>>16 != intercept>>16
	}
	tile := g.level.Tile(x, y)
	if tile.Door != nil {
		middle := intercept + (step >> 1)
		return middle>>16 != intercept>>16 || middle&65535 < int(math.Round(g.doorOpen[y*g.levelWidth+x]*65535))
	}
	return !tile.Solid
}

func (g *game) demoActorTileVisible(a *actorInstance, visible []bool) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := a.tileX+dx, a.tileY+dy
			if x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
				continue
			}
			if visible[y*g.levelWidth+x] && (dx == 0 && dy == 0 || !g.isBlockingTile(x, y) && g.level.Tile(x, y).Door == nil) {
				return true
			}
		}
	}
	return false
}
