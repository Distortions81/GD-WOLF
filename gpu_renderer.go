package main

import (
	_ "embed"
	"image"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed shaders/wolf_world.kage
var wolfWorldShaderSource []byte

//go:embed shaders/floor_light.kage
var wolfFloorLightShaderSource []byte

type wolfGPUTextureBatch struct {
	image    *ebiten.Image
	vertices []ebiten.Vertex
	indices  []uint16
}

type wolfGPUCommands struct {
	batches []wolfGPUTextureBatch
	used    int
}

type wolfGPURenderer struct {
	worldShader      *ebiten.Shader
	floorLightShader *ebiten.Shader
	frame            *ebiten.Image
	width            int
	height           int
	failed           bool
	commands         wolfGPUCommands
}

func (g *game) invalidateGPURenderer() {
	if g == nil || g.gpuRenderer == nil {
		return
	}
	if g.gpuRenderer.frame != nil {
		g.gpuRenderer.frame.Deallocate()
	}
	g.gpuRenderer = nil
}

func (g *game) beginGPUFrame() *wolfGPURenderer {
	if g == nil || g.layout.bufferWidth <= 0 || g.layout.bufferHeight <= 0 || g.gameplayBackgroundImage == nil {
		return nil
	}
	if g.gpuRenderer == nil {
		g.gpuRenderer = &wolfGPURenderer{}
	}
	r := g.gpuRenderer
	if r.failed {
		return nil
	}
	if r.worldShader == nil {
		shader, err := ebiten.NewShader(wolfWorldShaderSource)
		if err != nil {
			log.Printf("GPU renderer unavailable, using CPU: %v", err)
			r.failed = true
			return nil
		}
		r.worldShader = shader
		floorLightShader, err := ebiten.NewShader(wolfFloorLightShaderSource)
		if err != nil {
			log.Printf("GPU renderer unavailable, using CPU: %v", err)
			r.failed = true
			return nil
		}
		r.floorLightShader = floorLightShader
	}
	width, height := g.layout.bufferWidth, g.layout.bufferHeight
	if r.frame == nil || r.width != width || r.height != height {
		if r.frame != nil {
			r.frame.Deallocate()
		}
		r.width, r.height = width, height
		r.frame = ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), &ebiten.NewImageOptions{Unmanaged: true})
	}
	r.commands.reset()
	r.frame.DrawImage(g.gameplayBackgroundImage, nil)
	return r
}

func (g *game) renderGameplayFrameGPU() *ebiten.Image {
	r := g.beginGPUFrame()
	if r == nil {
		return nil
	}
	forwardX := math.Cos(g.playerA)
	forwardY := math.Sin(g.playerA)
	bufferWidth := g.layout.bufferWidth
	planeScale := math.Tan(fov / 2)
	planeX := -forwardY * planeScale
	planeY := forwardX * planeScale
	projPlaneDist := float64(bufferWidth) / (2 * planeScale)
	g.prepareRaycastDirections(0, bufferWidth, forwardX, forwardY, planeX, planeY)
	g.renderRaycastColumns(true, 0, bufferWidth, forwardX, forwardY, planeX, planeY, projPlaneDist)
	g.drawLightFixtureFloorGlowsGPU(r, forwardX, forwardY, planeX, planeY, projPlaneDist)
	g.drawWallColumnsGPU(r)
	g.drawSpritesGPU(r, forwardX, forwardY, planeX, planeY, projPlaneDist)
	g.drawWeaponOverlayGPU(r)
	r.commands.draw(r.frame, r.worldShader)
	return r.frame
}

func (c *wolfGPUCommands) reset() {
	c.used = 0
	for i := range c.batches {
		c.batches[i].vertices = c.batches[i].vertices[:0]
		c.batches[i].indices = c.batches[i].indices[:0]
	}
}

func splitFixed(v uint32) (whole, fraction float32) {
	return float32(v >> 16), float32(v & 0xffff)
}

func (c *wolfGPUCommands) rect(texture *ebiten.Image, x0, y0, x1, y1 int, baseU, baseV, stepU, stepV uint32) {
	if texture == nil || x1 < x0 || y1 < y0 {
		return
	}
	var batch *wolfGPUTextureBatch
	if c.used > 0 {
		batch = &c.batches[c.used-1]
	}
	if batch == nil || batch.image != texture || len(batch.vertices)+4 > 65532 {
		if c.used == len(c.batches) {
			c.batches = append(c.batches, wolfGPUTextureBatch{})
		}
		batch = &c.batches[c.used]
		c.used++
		batch.image = texture
	}
	baseUWhole, baseUFraction := splitFixed(baseU)
	baseVWhole, baseVFraction := splitFixed(baseV)
	stepUWhole, stepUFraction := splitFixed(stepU)
	stepVWhole, stepVFraction := splitFixed(stepV)
	vertex := ebiten.Vertex{
		SrcX:    float32(x0),
		SrcY:    float32(y0),
		ColorR:  baseUWhole,
		ColorG:  baseUFraction,
		ColorB:  baseVWhole,
		ColorA:  baseVFraction,
		Custom0: stepUWhole,
		Custom1: stepUFraction,
		Custom2: stepVWhole,
		Custom3: stepVFraction,
	}
	offset := uint16(len(batch.vertices))
	for _, p := range [4][2]int{{x0, y0}, {x1 + 1, y0}, {x0, y1 + 1}, {x1 + 1, y1 + 1}} {
		vertex.DstX = float32(p[0])
		vertex.DstY = float32(p[1])
		batch.vertices = append(batch.vertices, vertex)
	}
	batch.indices = append(batch.indices, offset, offset+1, offset+2, offset+1, offset+2, offset+3)
}

func (c *wolfGPUCommands) draw(dst *ebiten.Image, shader *ebiten.Shader) {
	for i := 0; i < c.used; i++ {
		batch := &c.batches[i]
		op := &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{batch.image}}
		dst.DrawTrianglesShader(batch.vertices, batch.indices, shader, op)
	}
}

func (g *game) drawWallColumnsGPU(r *wolfGPURenderer) {
	if len(g.wallColumns) < g.layout.bufferWidth {
		return
	}
	for x := 0; x < g.layout.bufferWidth; x++ {
		column := g.wallColumns[x]
		if !column.hit || column.drawBottom <= column.drawTop {
			continue
		}
		texture := g.pickWallTexture(column.wallID, column.side, column.texOverride)
		image, ok := g.wallTextureImage(texture)
		if !ok {
			continue
		}
		textureWidth, textureHeight := wallTextureDimensions(texture)
		fullHeight := column.origBottom - column.origTop
		if textureWidth <= 0 || textureHeight <= 0 || fullHeight <= 0 {
			continue
		}
		texX := textureXFromU(column.texU, textureWidth)
		texStep := uint32(textureHeight<<16) / uint32(fullHeight)
		texPos := uint32(column.drawTop-column.origTop) * texStep
		r.commands.rect(image, x, column.drawTop, x, column.drawBottom-1, uint32(texX<<16), texPos, 0, texStep)
	}
}

func (g *game) drawSpritesGPU(r *wolfGPURenderer, forwardX, forwardY, planeX, planeY, projPlaneDist float64) {
	if g.sprites == nil || len(g.zbuffer) < g.layout.bufferWidth {
		return
	}
	invDet := 1.0 / (planeX*forwardY - forwardX*planeY)
	bufferWidth, bufferHeight := g.layout.bufferWidth, g.layout.bufferHeight
	for _, visible := range g.visibleSprites(forwardX, forwardY) {
		transformX := invDet * (forwardY*visible.x - forwardX*visible.y)
		transformY := invDet * (-planeY*visible.x + planeX*visible.y)
		if transformY <= 0.01 || transformY >= maxSpriteDrawDepth {
			continue
		}
		shape := visible.shapenum
		if visible.rotate {
			shape += g.spriteRotateOffset(visible.x, visible.y, visible.facingDir)
		}
		sprite, ok := g.sprites.SpriteByShape(shape)
		if !ok {
			continue
		}
		texture, ok := g.spriteImageByShape(shape)
		if !ok {
			continue
		}
		spriteSize := int(projPlaneDist / transformY)
		if spriteSize <= 0 {
			continue
		}
		spriteScreenX := int((float64(bufferWidth) / 2) * (1 + transformX/transformY))
		spriteTop := int((float64(bufferHeight) - float64(spriteSize)) / 2)
		drawTop := maxInt(0, spriteTop)
		drawBottom := minInt(bufferHeight, spriteTop+spriteSize)
		spriteLeft := spriteScreenX - spriteSize/2
		startX := maxInt(0, spriteLeft)
		endX := minInt(bufferWidth, spriteLeft+spriteSize)
		if endX <= startX || drawBottom <= drawTop {
			continue
		}
		texXStep := uint32(sprite.Width<<16) / uint32(spriteSize)
		texYStep := uint32(sprite.Height<<16) / uint32(spriteSize)
		baseV := uint32(drawTop-spriteTop) * texYStep
		for x := startX; x < endX; {
			for x < endX && transformY >= g.zbuffer[x] {
				x++
			}
			if x >= endX {
				break
			}
			runStart := x
			for x < endX && transformY < g.zbuffer[x] {
				x++
			}
			baseU := uint32(runStart-spriteLeft) * texXStep
			r.commands.rect(texture, runStart, drawTop, x-1, drawBottom-1, baseU, baseV, texXStep, texYStep)
		}
	}
}

func (g *game) drawWeaponOverlayGPU(r *wolfGPURenderer) {
	if g.playerDying || g.sprites == nil {
		return
	}
	shape, ok := g.currentWeaponShape()
	if !ok {
		return
	}
	sprite, ok := g.sprites.SpriteByShape(shape)
	if !ok {
		return
	}
	texture, ok := g.spriteImageByShape(shape)
	if !ok {
		return
	}
	geometry := buildWeaponOverlayGeometry(&g.weaponOverlayScaled, g.layout.bufferWidth, g.layout.bufferHeight, 0, 0)
	if geometry.endX <= geometry.startX || geometry.drawBottom <= geometry.drawTop {
		return
	}
	texXStep := uint32(sprite.Width<<16) / uint32(geometry.spriteSize)
	texYStep := uint32(sprite.Height<<16) / uint32(geometry.spriteSize)
	spriteLeft := geometry.spriteScreenX - geometry.spriteSize/2
	baseU := uint32(geometry.startX-spriteLeft) * texXStep
	baseV := uint32(geometry.drawTop-geometry.spriteTop) * texYStep
	r.commands.rect(texture, geometry.startX, geometry.drawTop, geometry.endX-1, geometry.drawBottom-1, baseU, baseV, texXStep, texYStep)
}

func (g *game) drawLightFixtureFloorGlowsGPU(r *wolfGPURenderer, forwardX, forwardY, planeX, planeY, projPlaneDist float64) {
	if !g.shouldUseHDAssets() || g.sprites == nil {
		return
	}
	for _, sprite := range g.staticSprites {
		if !sprite.alive {
			continue
		}
		var strength [3]float32
		switch sprite.shapenum {
		case shapeSPR_STAT_3, shapeSPR_STAT_4:
			strength = [3]float32{32, 28, 16}
		case shapeSPR_STAT_14:
			strength = [3]float32{42, 37, 25}
		default:
			continue
		}
		asset, ok := g.sprites.SpriteByShape(sprite.shapenum)
		if !ok || asset.IndexedFormat {
			continue
		}
		r.drawFloorLight(g, sprite.x, sprite.y, strength, forwardX, forwardY, planeX, planeY, projPlaneDist)
	}
}

func (r *wolfGPURenderer) drawFloorLight(g *game, lightX, lightY float64, strength [3]float32, forwardX, forwardY, planeX, planeY, projPlaneDist float64) {
	const radius = 0.48
	width, height := r.width, r.height
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
	startX := maxInt(0, int(math.Floor(minX)))
	endX := minInt(width, int(math.Ceil(maxX))+1)
	if endX <= startX || endY <= startY {
		return
	}
	op := &ebiten.DrawRectShaderOptions{
		Blend: ebiten.BlendLighter,
		Uniforms: map[string]any{
			"ViewSize":   []float32{float32(width), float32(height)},
			"Player":     []float32{float32(g.playerX), float32(g.playerY)},
			"Forward":    []float32{float32(forwardX), float32(forwardY)},
			"Plane":      []float32{float32(planeX), float32(planeY)},
			"Projection": float32(projPlaneDist),
			"Light":      []float32{float32(lightX), float32(lightY)},
			"Radius":     float32(radius),
			"Strength":   strength[:],
		},
	}
	op.GeoM.Translate(float64(startX), float64(startY))
	r.frame.DrawRectShader(endX-startX, endY-startY, r.floorLightShader, op)
}
