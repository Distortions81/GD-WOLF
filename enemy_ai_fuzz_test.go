package main

import (
	"math"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"gd-wolf/internal/wl6"
)

type enemyAIFuzzMapSnapshot struct {
	base        *game
	liveEnemies []int
	placements  map[int][][2]float64
}

var (
	enemyAIFuzzSnapshotsOnce sync.Once
	enemyAIFuzzSnapshotsErr  error
	enemyAIFuzzSnapshots     [10]enemyAIFuzzMapSnapshot
)

func buildEnemyAIFuzzBaseline(files *wl6.Files, mapIndex int) (*game, error) {
	g := &game{
		files:         files,
		difficulty:    difficultyMedium,
		bestWeapon:    1,
		weapon:        1,
		chosenWeapon:  1,
		selectedLevel: mapIndex,
		pendingMap:    mapIndex,
		rng:           newWolfRNG(0),
	}
	g.ensureFrame(320, 200)
	if err := g.setMap(mapIndex); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneEnemyAIFuzzBaseline(src *game, rngSeed byte) *game {
	dst := *src
	dst.actors = append([]actorInstance(nil), src.actors...)
	dst.staticSprites = append([]staticSprite(nil), src.staticSprites...)
	dst.doorOpen = append([]float64(nil), src.doorOpen...)
	dst.doorState = append([]byte(nil), src.doorState...)
	dst.doorTimer = append([]int(nil), src.doorTimer...)
	dst.playerAreas = append([]bool(nil), src.playerAreas...)
	dst.cachedWallIDs = append([]uint16(nil), src.cachedWallIDs...)
	dst.cachedDoorFlags = append([]byte(nil), src.cachedDoorFlags...)
	dst.cachedDoorSides = append([]byte(nil), src.cachedDoorSides...)
	dst.reachable = append([]bool(nil), src.reachable...)
	dst.renderableWalls = append([]bool(nil), src.renderableWalls...)
	dst.rng = newWolfRNG(rngSeed)
	return &dst
}

func enemyAIFuzzSnapshot(t *testing.T, mapIndex int) enemyAIFuzzMapSnapshot {
	t.Helper()
	enemyAIFuzzSnapshotsOnce.Do(func() {
		files, err := wl6.OpenEmbeddedShareware()
		if err != nil {
			enemyAIFuzzSnapshotsErr = err
			return
		}
		for i := 0; i < len(enemyAIFuzzSnapshots); i++ {
			base, err := buildEnemyAIFuzzBaseline(files, i)
			if err != nil {
				enemyAIFuzzSnapshotsErr = err
				return
			}
			live := fuzzLiveEnemyIndices(base)
			placements := make(map[int][][2]float64, len(live))
			for _, actorIndex := range live {
				placements[actorIndex] = fuzzNearbyPlayerPositions(base, &base.actors[actorIndex])
			}
			enemyAIFuzzSnapshots[i] = enemyAIFuzzMapSnapshot{
				base:        base,
				liveEnemies: live,
				placements:  placements,
			}
		}
	})
	if enemyAIFuzzSnapshotsErr != nil {
		t.Fatalf("build enemy ai fuzz snapshots: %v", enemyAIFuzzSnapshotsErr)
	}
	return enemyAIFuzzSnapshots[mapIndex]
}

func fuzzTestGameWithSharewareMap(t *testing.T, mapIndex int, rngSeed byte) *game {
	t.Helper()
	snapshot := enemyAIFuzzSnapshot(t, mapIndex)
	return cloneEnemyAIFuzzBaseline(snapshot.base, rngSeed)
}

func fuzzLiveEnemyIndices(g *game) []int {
	indices := make([]int, 0, len(g.actors))
	for i := range g.actors {
		a := &g.actors[i]
		if !a.alive || !a.blocking || !a.shootable {
			continue
		}
		indices = append(indices, i)
	}
	return indices
}

func fuzzNearbyPlayerPositions(g *game, a *actorInstance) [][2]float64 {
	if g == nil || a == nil || g.level == nil {
		return nil
	}

	offsets := []float64{0.25, 0.5, 0.75}
	positions := make([][2]float64, 0, 128)
	for radius := 1; radius <= 3; radius++ {
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if maxInt(absInt(dx), absInt(dy)) != radius {
					continue
				}
				tileX := a.tileX + dx
				tileY := a.tileY + dy
				if !g.isReachableTile(tileX, tileY) {
					continue
				}
				tile := g.level.Tile(tileX, tileY)
				if tile.Solid || tile.Door != nil {
					continue
				}
				for _, oy := range offsets {
					for _, ox := range offsets {
						x := float64(tileX) + ox
						y := float64(tileY) + oy
						if g.collides(x, y) {
							continue
						}
						positions = append(positions, [2]float64{x, y})
					}
				}
			}
		}
	}
	return positions
}

func fuzzNearbyDoorPlayerPositions(g *game, a *actorInstance) [][2]float64 {
	if g == nil || a == nil || g.level == nil {
		return nil
	}

	offsets := []float64{0.25, 0.5, 0.75}
	positions := make([][2]float64, 0, 64)
	for radius := 1; radius <= 6; radius++ {
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if maxInt(absInt(dx), absInt(dy)) != radius {
					continue
				}
				tileX := a.tileX + dx
				tileY := a.tileY + dy
				if !g.isReachableTile(tileX, tileY) {
					continue
				}
				tile := g.level.Tile(tileX, tileY)
				if tile.Solid || tile.Door != nil {
					continue
				}
				adjacentDoor := false
				for _, step := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					neighbor := g.level.Tile(tileX+step[0], tileY+step[1])
					if neighbor.Door != nil {
						adjacentDoor = true
						break
					}
				}
				if !adjacentDoor {
					continue
				}
				for _, oy := range offsets {
					for _, ox := range offsets {
						x := float64(tileX) + ox
						y := float64(tileY) + oy
						if g.collides(x, y) {
							continue
						}
						positions = append(positions, [2]float64{x, y})
					}
				}
			}
		}
	}
	return positions
}

func fuzzAssertActorState(t *testing.T, g *game) {
	t.Helper()

	for i := range g.actors {
		a := &g.actors[i]
		if math.IsNaN(a.x) || math.IsNaN(a.y) || math.IsInf(a.x, 0) || math.IsInf(a.y, 0) {
			t.Fatalf("actor %d has invalid position (%v,%v)", i, a.x, a.y)
		}
		if a.x < 0 || a.y < 0 || a.x > float64(g.levelWidth) || a.y > float64(g.levelHeight) {
			t.Fatalf("actor %d out of bounds at (%.4f, %.4f) on %dx%d map", i, a.x, a.y, g.levelWidth, g.levelHeight)
		}
		if a.tileX < 0 || a.tileY < 0 || a.tileX >= g.levelWidth || a.tileY >= g.levelHeight {
			t.Fatalf("actor %d tile out of bounds at (%d,%d) on %dx%d map", i, a.tileX, a.tileY, g.levelWidth, g.levelHeight)
		}
		if a.hasGoal && (a.goalX < 0 || a.goalY < 0 || a.goalX >= g.levelWidth || a.goalY >= g.levelHeight) {
			t.Fatalf("actor %d goal out of bounds at (%d,%d) on %dx%d map", i, a.goalX, a.goalY, g.levelWidth, g.levelHeight)
		}
	}
}

func fuzzCircleStrafePlayer(g *game, actor *actorInstance, tickSeed, step, tics int) {
	if g == nil || actor == nil || tics <= 0 {
		return
	}

	dx := g.playerX - actor.x
	dy := g.playerY - actor.y
	dist := math.Hypot(dx, dy)
	if dist < 1e-6 {
		dx, dy = 1, 0
		dist = 1
	}

	orbitDir := 1.0
	if ((tickSeed >> (step % 8)) & 1) != 0 {
		orbitDir = -1
	}

	// Keep the player moving laterally around the enemy while nudging back
	// toward a reasonable combat ring if the orbit starts drifting.
	targetRadius := 2.0 + float64(absInt(tickSeed+step)%3)*0.5
	radialError := dist - targetRadius
	radialScale := radialError * 0.12
	tangentX := -dy / dist * orbitDir
	tangentY := dx / dist * orbitDir
	radialX := -dx / dist * radialScale
	radialY := -dy / dist * radialScale

	speed := walkSpeed * float64(tics)
	if ((tickSeed >> ((step + 3) % 8)) & 1) != 0 {
		speed = runSpeed * float64(tics)
	}
	moveX := (tangentX + radialX) * speed
	moveY := (tangentY + radialY) * speed

	prevX, prevY := g.playerX, g.playerY
	g.tryMove(moveX, moveY)
	g.playerMovingFast = math.Hypot(g.playerX-prevX, g.playerY-prevY) > walkSpeed
	g.playerA = math.Atan2(actor.y-g.playerY, actor.x-g.playerX)
	g.cameraX = g.playerX
	g.cameraY = g.playerY
	g.rebuildPlayerAreas()
}

func runEnemyAIFuzzScenario(t *testing.T, g *game, actorIndex int, positions [][2]float64, placementPick, tickSeed int) bool {
	t.Helper()

	if actorIndex < 0 || actorIndex >= len(g.actors) {
		t.Fatalf("actorIndex %d out of range", actorIndex)
	}
	if len(positions) == 0 {
		return false
	}
	actor := &g.actors[actorIndex]

	playerPos := positions[absInt(placementPick)%len(positions)]
	g.playerX = playerPos[0]
	g.playerY = playerPos[1]
	g.cameraX = g.playerX
	g.cameraY = g.playerY
	angleDegrees := ((tickSeed % 360) + 360) % 360
	g.playerA = float64(angleDegrees) * math.Pi / 180
	g.rebuildPlayerAreas()

	stepCount := 1 + absInt(tickSeed)%6
	for step := 0; step < stepCount; step++ {
		tics := 1 + absInt(tickSeed+step*7)%20
		fuzzCircleStrafePlayer(g, actor, tickSeed, step, tics)
		if ((tickSeed >> (step % 8)) & 1) != 0 {
			g.madeNoise = true
		} else {
			g.madeNoise = false
		}
		g.updateDoors(tics)
		g.updateActors(tics)
		fuzzAssertActorState(t, g)
	}
	return true
}

func enemyAISoakPerLevel() time.Duration {
	raw := os.Getenv("GDWOLF_ENEMY_SOAK_PER_LEVEL")
	if raw == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

func enemyAISoakRunsPerMap() int {
	raw := os.Getenv("GDWOLF_ENEMY_SOAK_RUNS_PER_MAP")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func TestEnemyAISequentialLevelSoak(t *testing.T) {
	if os.Getenv("GDWOLF_ENEMY_SOAK") == "" {
		t.Skip("set GDWOLF_ENEMY_SOAK=1 to run the sequential enemy AI soak")
	}

	perLevel := enemyAISoakPerLevel()
	runsPerMap := enemyAISoakRunsPerMap()
	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		snapshot := enemyAIFuzzSnapshot(t, mapIndex)
		start := time.Now()
		deadline := start.Add(perLevel)
		scenarios := 0
		for {
			if runsPerMap > 0 {
				if scenarios >= runsPerMap {
					break
				}
			} else if !time.Now().Before(deadline) {
				break
			}
			if len(snapshot.liveEnemies) == 0 {
				break
			}
			actorIndex := snapshot.liveEnemies[scenarios%len(snapshot.liveEnemies)]
			g := cloneEnemyAIFuzzBaseline(snapshot.base, byte((scenarios*17+mapIndex*11)&0xff))
			if !runEnemyAIFuzzScenario(t, g, actorIndex, snapshot.placements[actorIndex], scenarios*13+mapIndex, scenarios*29+mapIndex*7) {
				scenarios++
				continue
			}
			scenarios++
		}
		if runsPerMap > 0 {
			t.Logf("enemy-ai-soak map=%d runs_per_map=%d scenarios=%d elapsed=%s", mapIndex, runsPerMap, scenarios, time.Since(start).Round(time.Millisecond))
		} else {
			t.Logf("enemy-ai-soak map=%d per_level=%s scenarios=%d elapsed=%s", mapIndex, perLevel, scenarios, time.Since(start).Round(time.Millisecond))
		}
	}
}

func FuzzEnemyAINearbyPlacements(f *testing.F) {
	seeds := [][5]int{
		{0, 0, 0, 1, 0},
		{1, 1, 4, 3, 17},
		{2, 2, 7, 5, 33},
		{3, 3, 9, 7, 49},
		{4, 4, 12, 11, 65},
		{5, 5, 18, 13, 81},
		{6, 6, 21, 17, 97},
		{7, 7, 24, 19, 113},
		{8, 8, 27, 23, 129},
		{9, 9, 31, 29, 145},
	}
	for _, seed := range seeds {
		f.Add(seed[0], seed[1], seed[2], seed[3], seed[4])
	}

	f.Fuzz(func(t *testing.T, mapIndex, enemyPick, placementPick, tickSeed, rngSeed int) {
		mapIndex = absInt(mapIndex) % 10
		snapshot := enemyAIFuzzSnapshot(t, mapIndex)
		if len(snapshot.liveEnemies) == 0 {
			t.Skip()
		}
		actorIndex := snapshot.liveEnemies[absInt(enemyPick)%len(snapshot.liveEnemies)]
		g := cloneEnemyAIFuzzBaseline(snapshot.base, byte(rngSeed))
		if !runEnemyAIFuzzScenario(t, g, actorIndex, snapshot.placements[actorIndex], placementPick, tickSeed) {
			t.Skip()
		}
	})
}
