package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"gd-wolf/internal/wl6"
)

type wolfsrcAIDecisionMode string

const (
	wolfsrcDecisionChase wolfsrcAIDecisionMode = "chase"
	wolfsrcDecisionDodge wolfsrcAIDecisionMode = "dodge"
	wolfsrcDecisionRun   wolfsrcAIDecisionMode = "run"
)

type enemyAIHarnessDecision struct {
	Mode         wolfsrcAIDecisionMode
	Dir          int
	DestX        int
	DestY        int
	HasMove      bool
	WaitForDoor  bool
	TurnaroundOK bool
}

type enemyAIHarnessFirstSighting struct {
	Alerted      bool
	FirstAttack  bool
	ReactionTics int
	AIState      ActorAIState
	HasGoal      bool
	GoalX        int
	GoalY        int
	MoveDistance float64
	Rotate       bool
	SequenceID   AnimSequenceID
}

type enemyAIHarnessAttackEntry struct {
	Started      bool
	AIState      ActorAIState
	HasGoal      bool
	GoalX        int
	GoalY        int
	MoveDistance float64
	Rotate       bool
	SequenceID   AnimSequenceID
}

type enemyAIHarnessRuntimeState struct {
	X            float64
	Y            float64
	TileX        int
	TileY        int
	Dir          int
	AIState      ActorAIState
	Alerted      bool
	Ambush       bool
	FirstAttack  bool
	ReactionTics int
	Area         int
	Rotate       bool
	SequenceID   AnimSequenceID
	HasGoal      bool
	GoalX        int
	GoalY        int
	MoveDistance float64
}

type enemyAIHarnessSnapshot struct {
	MapIndex     int
	EnemyKind    ActorKind
	EnemyAIState ActorAIState
	EnemyTileX   int
	EnemyTileY   int
	EnemyDir     int
	PlayerTileX  int
	PlayerTileY  int
	PlayerX      float64
	PlayerY      float64
	Mode         wolfsrcAIDecisionMode
	CanSeePlayer bool
}

type enemyAIHarnessMismatch struct {
	Layer        string                `json:"layer"`
	MapIndex     int                   `json:"map_index"`
	ActorIndex   int                   `json:"actor_index"`
	EnemyKind    ActorKind             `json:"enemy_kind"`
	EnemyState   ActorAIState          `json:"enemy_state"`
	EnemyTileX   int                   `json:"enemy_tile_x"`
	EnemyTileY   int                   `json:"enemy_tile_y"`
	EnemyDir     int                   `json:"enemy_dir"`
	PlayerX      float64               `json:"player_x"`
	PlayerY      float64               `json:"player_y"`
	PlayerTileX  int                   `json:"player_tile_x"`
	PlayerTileY  int                   `json:"player_tile_y"`
	CanSeePlayer bool                  `json:"can_see_player"`
	Mode         wolfsrcAIDecisionMode `json:"mode,omitempty"`
	Tics         int                   `json:"tics,omitempty"`
	Step         int                   `json:"step,omitempty"`
	Script       string                `json:"script,omitempty"`
	Diff         string                `json:"diff"`
	Start        any                   `json:"start,omitempty"`
	Want         any                   `json:"want,omitempty"`
	Got          any                   `json:"got,omitempty"`
}

type enemyAIHarnessReportWriter struct {
	file   *os.File
	writer *bufio.Writer
}

func openEnemyAIHarnessReport(path string) (*enemyAIHarnessReportWriter, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &enemyAIHarnessReportWriter{
		file:   f,
		writer: bufio.NewWriter(f),
	}, nil
}

func (w *enemyAIHarnessReportWriter) Write(rec enemyAIHarnessMismatch) error {
	if w == nil {
		return nil
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := w.writer.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func (w *enemyAIHarnessReportWriter) Close() error {
	if w == nil {
		return nil
	}
	if err := w.writer.Flush(); err != nil {
		_ = w.file.Close()
		return err
	}
	return w.file.Close()
}

func cloneAIHarnessGame(src *game, actorIndex int, playerPos [2]float64) *game {
	dst := *src
	dst.actors = []actorInstance{src.actors[actorIndex]}
	dst.staticSprites = append([]staticSprite(nil), src.staticSprites...)
	dst.doorOpen = append([]float64(nil), src.doorOpen...)
	dst.doorState = append([]byte(nil), src.doorState...)
	dst.doorTimer = append([]int(nil), src.doorTimer...)
	dst.playerAreas = append([]bool(nil), src.playerAreas...)
	dst.playerX = playerPos[0]
	dst.playerY = playerPos[1]
	dst.cameraX = dst.playerX
	dst.cameraY = dst.playerY
	dst.rng = testRNG(1)
	dst.rebuildPlayerAreas()
	return &dst
}

func normalizeHarnessActorForDecision(a *actorInstance) {
	a.aiState = actorStateChase
	a.alerted = true
	a.reactionTimer = 0
	a.hasGoal = false
	a.moveDistance = 0
}

func harnessDecisionMode(g *game, a *actorInstance) wolfsrcAIDecisionMode {
	if a == nil {
		return wolfsrcDecisionChase
	}
	if g.actorCanSeePlayer(a) {
		return wolfsrcDecisionDodge
	}
	return wolfsrcDecisionChase
}

func captureHarnessSnapshot(g *game, a *actorInstance, mode wolfsrcAIDecisionMode) enemyAIHarnessSnapshot {
	return enemyAIHarnessSnapshot{
		MapIndex:     g.mapIndex,
		EnemyKind:    a.kind,
		EnemyAIState: a.aiState,
		EnemyTileX:   a.tileX,
		EnemyTileY:   a.tileY,
		EnemyDir:     a.dir,
		PlayerTileX:  int(g.playerX),
		PlayerTileY:  int(g.playerY),
		PlayerX:      g.playerX,
		PlayerY:      g.playerY,
		Mode:         mode,
		CanSeePlayer: g.actorCanSeePlayer(a),
	}
}

func capturePortDecision(g *game, a *actorInstance, mode wolfsrcAIDecisionMode) enemyAIHarnessDecision {
	normalizeHarnessActorForDecision(a)
	switch mode {
	case wolfsrcDecisionDodge:
		g.actorChooseChaseGoal(a, true)
	case wolfsrcDecisionRun:
		g.actorChooseRunGoal(a)
	default:
		g.actorChooseChaseGoal(a, false)
	}
	if !a.hasGoal {
		return enemyAIHarnessDecision{Mode: mode}
	}
	destX := a.tileX
	destY := a.tileY
	return enemyAIHarnessDecision{
		Mode:        mode,
		Dir:         a.dir,
		DestX:       destX,
		DestY:       destY,
		HasMove:     true,
		WaitForDoor: g.level != nil && g.level.Tile(destX, destY).Door != nil && !g.isDoorOpen(destX, destY),
	}
}

func wolfsrcCheckDiagTile(g *game, x, y int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false
	}
	tile := g.level.Tile(x, y)
	if tile.Solid || tile.Door != nil {
		return false
	}
	for i := range g.actors {
		a := &g.actors[i]
		if !a.alive || !a.blocking {
			continue
		}
		if a.tileX == x && a.tileY == y {
			return false
		}
	}
	for _, spr := range g.staticSprites {
		if spr.alive && spr.blocking && int(spr.x) == x && int(spr.y) == y {
			return false
		}
	}
	return true
}

func wolfsrcCheckSideTile(g *game, a *actorInstance, x, y int) (ok bool, waitForDoor bool) {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false, false
	}
	tile := g.level.Tile(x, y)
	if tile.Door != nil {
		if tile.Door.Lock != 0 {
			return false, false
		}
		if !g.isDoorOpen(x, y) {
			return true, true
		}
	} else if tile.Solid {
		return false, false
	}
	for i := range g.actors {
		other := &g.actors[i]
		if other == a || !other.alive || !other.blocking {
			continue
		}
		if other.tileX == x && other.tileY == y {
			return false, false
		}
	}
	for _, spr := range g.staticSprites {
		if spr.alive && spr.blocking && int(spr.x) == x && int(spr.y) == y {
			return false, false
		}
	}
	return true, false
}

func wolfsrcTryWalkDecision(g *game, a *actorInstance, dir int) (enemyAIHarnessDecision, bool) {
	dx, dy := dirStep(dir)
	destX := a.tileX + dx
	destY := a.tileY + dy

	if dx != 0 && dy != 0 {
		if !wolfsrcCheckDiagTile(g, destX, destY) || !wolfsrcCheckDiagTile(g, destX, a.tileY) || !wolfsrcCheckDiagTile(g, a.tileX, destY) {
			return enemyAIHarnessDecision{}, false
		}
		return enemyAIHarnessDecision{
			Dir:     dir & 7,
			DestX:   destX,
			DestY:   destY,
			HasMove: true,
		}, true
	}

	if a.kind == actorKindDog {
		if !wolfsrcCheckDiagTile(g, destX, destY) {
			return enemyAIHarnessDecision{}, false
		}
		return enemyAIHarnessDecision{
			Dir:     dir & 7,
			DestX:   destX,
			DestY:   destY,
			HasMove: true,
		}, true
	}

	ok, waitForDoor := wolfsrcCheckSideTile(g, a, destX, destY)
	if !ok {
		return enemyAIHarnessDecision{}, false
	}
	return enemyAIHarnessDecision{
		Dir:         dir & 7,
		DestX:       destX,
		DestY:       destY,
		HasMove:     true,
		WaitForDoor: waitForDoor,
	}, true
}

func wolfsrcReferenceDecision(g *game, a *actorInstance, mode wolfsrcAIDecisionMode) enemyAIHarnessDecision {
	ref := *a
	normalizeHarnessActorForDecision(&ref)
	turnaround := oppositeDir(ref.dir)
	turnaroundOK := true
	if mode == wolfsrcDecisionDodge && ref.firstAttack {
		turnaround = 8
		turnaroundOK = false
		ref.firstAttack = false
	}

	playerTileX := int(g.playerX)
	playerTileY := int(g.playerY)
	deltaX := playerTileX - ref.tileX
	deltaY := playerTileY - ref.tileY

	tryDirs := make([]int, 0, 8)
	switch mode {
	case wolfsrcDecisionDodge:
		dirTry := [5]int{}
		if deltaX > 0 {
			dirTry[1], dirTry[3] = 0, 4
		} else {
			dirTry[1], dirTry[3] = 4, 0
		}
		if deltaY > 0 {
			dirTry[2], dirTry[4] = 6, 2
		} else {
			dirTry[2], dirTry[4] = 2, 6
		}
		if absInt(deltaX) > absInt(deltaY) {
			dirTry[1], dirTry[2] = dirTry[2], dirTry[1]
			dirTry[3], dirTry[4] = dirTry[4], dirTry[3]
		}
		if g.rng == nil {
			g.rng = defaultRNG()
		}
		if g.rng.Intn(256) < 128 {
			dirTry[1], dirTry[2] = dirTry[2], dirTry[1]
			dirTry[3], dirTry[4] = dirTry[4], dirTry[3]
		}
		dirTry[0] = diagonalDir(dirTry[1], dirTry[2])
		for _, dir := range dirTry {
			if dir != 8 {
				tryDirs = append(tryDirs, dir)
			}
		}
	case wolfsrcDecisionRun:
		firstDir, secondDir := 4, 2
		if deltaX < 0 {
			firstDir = 0
		}
		if deltaY < 0 {
			secondDir = 6
		}
		if absInt(deltaY) > absInt(deltaX) {
			firstDir, secondDir = secondDir, firstDir
		}
		tryDirs = append(tryDirs, firstDir, secondDir)
		if g.rng == nil {
			g.rng = defaultRNG()
		}
		if g.rng.Intn(256) > 128 {
			tryDirs = append(tryDirs, 2, 0, 6, 4)
		} else {
			tryDirs = append(tryDirs, 4, 6, 0, 2)
		}
	default:
		firstDir, secondDir := 8, 8
		if deltaX > 0 {
			firstDir = 0
		} else if deltaX < 0 {
			firstDir = 4
		}
		if deltaY > 0 {
			secondDir = 6
		} else if deltaY < 0 {
			secondDir = 2
		}
		if absInt(deltaY) > absInt(deltaX) {
			firstDir, secondDir = secondDir, firstDir
		}
		tryDirs = append(tryDirs, firstDir, secondDir)
		if ref.dir != 8 {
			tryDirs = append(tryDirs, ref.dir)
		}
		if g.rng == nil {
			g.rng = defaultRNG()
		}
		if g.rng.Intn(256) > 128 {
			tryDirs = append(tryDirs, 2, 0, 6, 4)
		} else {
			tryDirs = append(tryDirs, 4, 6, 0, 2)
		}
	}

	seen := map[int]bool{}
	for _, dir := range tryDirs {
		if dir == 8 || seen[dir] {
			continue
		}
		if mode != wolfsrcDecisionRun && dir == turnaround {
			continue
		}
		seen[dir] = true
		if decision, ok := wolfsrcTryWalkDecision(g, &ref, dir); ok {
			decision.Mode = mode
			decision.TurnaroundOK = turnaroundOK
			return decision
		}
	}

	if mode != wolfsrcDecisionRun && turnaround != 8 {
		if decision, ok := wolfsrcTryWalkDecision(g, &ref, turnaround); ok {
			decision.Mode = mode
			decision.TurnaroundOK = turnaroundOK
			return decision
		}
	}

	return enemyAIHarnessDecision{Mode: mode, TurnaroundOK: turnaroundOK}
}

func capturePortFirstSighting(g *game, a *actorInstance) enemyAIHarnessFirstSighting {
	if a.kind == actorKindDog {
		g.dogFirstSighting(a)
	} else {
		g.guardFirstSighting(a)
	}
	return enemyAIHarnessFirstSighting{
		Alerted:      a.alerted,
		FirstAttack:  a.firstAttack,
		ReactionTics: a.reactionTimer,
		AIState:      a.aiState,
		HasGoal:      a.hasGoal,
		GoalX:        a.tileX,
		GoalY:        a.tileY,
		MoveDistance: a.moveDistance,
		Rotate:       a.rotate,
		SequenceID:   a.sequenceID,
	}
}

func wolfsrcReferenceFirstSighting(g *game, a *actorInstance) enemyAIHarnessFirstSighting {
	ref := *a
	ref.alerted = true
	ref.firstAttack = true
	ref.reactionTimer = 0
	ref.aiState = actorStateChase
	ref.rotate = actorUsesDirectionalRotation(ref.kind)
	if ref.kind == actorKindDog {
		ref.rotate = true
	}
	if ref.moveDistance < 0 {
		ref.moveDistance = 0
	}
	ref.sequenceID = ref.chaseSequence()
	return enemyAIHarnessFirstSighting{
		Alerted:      ref.alerted,
		FirstAttack:  ref.firstAttack,
		ReactionTics: ref.reactionTimer,
		AIState:      ref.aiState,
		HasGoal:      ref.hasGoal,
		GoalX:        ref.tileX,
		GoalY:        ref.tileY,
		MoveDistance: ref.moveDistance,
		Rotate:       ref.rotate,
		SequenceID:   ref.sequenceID,
	}
}

func diffHarnessFirstSighting(want, got enemyAIHarnessFirstSighting) string {
	fields := make([]string, 0, 8)
	if want.Alerted != got.Alerted {
		fields = append(fields, fmt.Sprintf("alerted want=%t got=%t", want.Alerted, got.Alerted))
	}
	if want.FirstAttack != got.FirstAttack {
		fields = append(fields, fmt.Sprintf("firstAttack want=%t got=%t", want.FirstAttack, got.FirstAttack))
	}
	if want.ReactionTics != got.ReactionTics {
		fields = append(fields, fmt.Sprintf("reactionTimer want=%d got=%d", want.ReactionTics, got.ReactionTics))
	}
	if want.AIState != got.AIState {
		fields = append(fields, fmt.Sprintf("aiState want=%v got=%v", want.AIState, got.AIState))
	}
	if want.HasGoal != got.HasGoal {
		fields = append(fields, fmt.Sprintf("hasGoal want=%t got=%t", want.HasGoal, got.HasGoal))
	}
	if want.GoalX != got.GoalX || want.GoalY != got.GoalY {
		fields = append(fields, fmt.Sprintf("goal want=(%d,%d) got=(%d,%d)", want.GoalX, want.GoalY, got.GoalX, got.GoalY))
	}
	if math.Abs(want.MoveDistance-got.MoveDistance) > 1e-9 {
		fields = append(fields, fmt.Sprintf("moveDistance want=%.5f got=%.5f", want.MoveDistance, got.MoveDistance))
	}
	if want.Rotate != got.Rotate {
		fields = append(fields, fmt.Sprintf("rotate want=%t got=%t", want.Rotate, got.Rotate))
	}
	if want.SequenceID != got.SequenceID {
		fields = append(fields, fmt.Sprintf("sequence want=%q got=%q", want.SequenceID, got.SequenceID))
	}
	return strings.Join(fields, ", ")
}

func capturePortAttackEntry(g *game, a *actorInstance, tics int) enemyAIHarnessAttackEntry {
	started := false
	if a.kind == actorKindDog {
		started = g.dogCanStartJump(a, tics)
		if started {
			g.startDogJump(a)
		}
	} else {
		started = g.guardTryStartShoot(a, tics)
	}
	return enemyAIHarnessAttackEntry{
		Started:      started,
		AIState:      a.aiState,
		HasGoal:      a.hasGoal,
		GoalX:        a.goalX,
		GoalY:        a.goalY,
		MoveDistance: a.moveDistance,
		Rotate:       a.rotate,
		SequenceID:   a.sequenceID,
	}
}

func wolfsrcReferenceAttackEntry(g *game, a *actorInstance, tics int) enemyAIHarnessAttackEntry {
	ref := *a
	if ref.kind == actorKindDog {
		started := g.dogCanStartJump(&ref, tics)
		if started {
			ref.aiState = actorStateJump
			ref.rotate = false
			ref.sequenceID = ref.jumpSequence()
		}
		return enemyAIHarnessAttackEntry{
			Started:      started,
			AIState:      ref.aiState,
			HasGoal:      ref.hasGoal,
			GoalX:        ref.goalX,
			GoalY:        ref.goalY,
			MoveDistance: ref.moveDistance,
			Rotate:       ref.rotate,
			SequenceID:   ref.sequenceID,
		}
	}
	if !g.actorCanSeePlayer(&ref) {
		return enemyAIHarnessAttackEntry{
			Started:      false,
			AIState:      ref.aiState,
			HasGoal:      ref.hasGoal,
			GoalX:        ref.goalX,
			GoalY:        ref.goalY,
			MoveDistance: ref.moveDistance,
			Rotate:       ref.rotate,
			SequenceID:   ref.sequenceID,
		}
	}
	dx := absInt(ref.tileX - int(g.playerX))
	dy := absInt(ref.tileY - int(g.playerY))
	dist := maxInt(dx, dy)
	chance := 300
	if !(dist == 0 || (dist == 1 && ref.moveDistance < 0.25)) {
		chance = (maxInt(tics, 1) << 4) / maxInt(1, dist)
	}
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	started := g.rng.Intn(256) < chance
	if started {
		ref.aiState = actorStateShoot
		ref.rotate = false
		ref.sequenceID = ref.shootSequence()
	}
	return enemyAIHarnessAttackEntry{
		Started:      started,
		AIState:      ref.aiState,
		HasGoal:      ref.hasGoal,
		GoalX:        ref.goalX,
		GoalY:        ref.goalY,
		MoveDistance: ref.moveDistance,
		Rotate:       ref.rotate,
		SequenceID:   ref.sequenceID,
	}
}

func diffHarnessAttackEntry(want, got enemyAIHarnessAttackEntry) string {
	fields := make([]string, 0, 7)
	if want.Started != got.Started {
		fields = append(fields, fmt.Sprintf("started want=%t got=%t", want.Started, got.Started))
	}
	if want.AIState != got.AIState {
		fields = append(fields, fmt.Sprintf("aiState want=%v got=%v", want.AIState, got.AIState))
	}
	if want.HasGoal != got.HasGoal {
		fields = append(fields, fmt.Sprintf("hasGoal want=%t got=%t", want.HasGoal, got.HasGoal))
	}
	if want.GoalX != got.GoalX || want.GoalY != got.GoalY {
		fields = append(fields, fmt.Sprintf("goal want=(%d,%d) got=(%d,%d)", want.GoalX, want.GoalY, got.GoalX, got.GoalY))
	}
	if math.Abs(want.MoveDistance-got.MoveDistance) > 1e-9 {
		fields = append(fields, fmt.Sprintf("moveDistance want=%.5f got=%.5f", want.MoveDistance, got.MoveDistance))
	}
	if want.Rotate != got.Rotate {
		fields = append(fields, fmt.Sprintf("rotate want=%t got=%t", want.Rotate, got.Rotate))
	}
	if want.SequenceID != got.SequenceID {
		fields = append(fields, fmt.Sprintf("sequence want=%q got=%q", want.SequenceID, got.SequenceID))
	}
	return strings.Join(fields, ", ")
}

func captureHarnessRuntimeState(a *actorInstance) enemyAIHarnessRuntimeState {
	return enemyAIHarnessRuntimeState{
		X:            a.x,
		Y:            a.y,
		TileX:        a.tileX,
		TileY:        a.tileY,
		Dir:          a.dir,
		AIState:      a.aiState,
		Alerted:      a.alerted,
		Ambush:       a.ambush,
		FirstAttack:  a.firstAttack,
		ReactionTics: a.reactionTimer,
		Area:         a.area,
		Rotate:       a.rotate,
		SequenceID:   a.sequenceID,
		HasGoal:      a.hasGoal,
		GoalX:        a.goalX,
		GoalY:        a.goalY,
		MoveDistance: a.moveDistance,
	}
}

func diffHarnessRuntimeState(want, got enemyAIHarnessRuntimeState) string {
	fields := make([]string, 0, 8)
	if math.Abs(want.X-got.X) > 1e-9 || math.Abs(want.Y-got.Y) > 1e-9 {
		fields = append(fields, fmt.Sprintf("pos want=(%.5f,%.5f) got=(%.5f,%.5f)", want.X, want.Y, got.X, got.Y))
	}
	if want.TileX != got.TileX || want.TileY != got.TileY {
		fields = append(fields, fmt.Sprintf("tile want=(%d,%d) got=(%d,%d)", want.TileX, want.TileY, got.TileX, got.TileY))
	}
	if want.Dir != got.Dir {
		fields = append(fields, fmt.Sprintf("dir want=%d got=%d", want.Dir, got.Dir))
	}
	if want.AIState != got.AIState {
		fields = append(fields, fmt.Sprintf("aiState want=%v got=%v", want.AIState, got.AIState))
	}
	if want.Alerted != got.Alerted {
		fields = append(fields, fmt.Sprintf("alerted want=%t got=%t", want.Alerted, got.Alerted))
	}
	if want.Ambush != got.Ambush {
		fields = append(fields, fmt.Sprintf("ambush want=%t got=%t", want.Ambush, got.Ambush))
	}
	if want.FirstAttack != got.FirstAttack {
		fields = append(fields, fmt.Sprintf("firstAttack want=%t got=%t", want.FirstAttack, got.FirstAttack))
	}
	if want.ReactionTics != got.ReactionTics {
		fields = append(fields, fmt.Sprintf("reactionTimer want=%d got=%d", want.ReactionTics, got.ReactionTics))
	}
	if want.Area != got.Area {
		fields = append(fields, fmt.Sprintf("area want=%d got=%d", want.Area, got.Area))
	}
	if want.Rotate != got.Rotate {
		fields = append(fields, fmt.Sprintf("rotate want=%t got=%t", want.Rotate, got.Rotate))
	}
	if want.SequenceID != got.SequenceID {
		fields = append(fields, fmt.Sprintf("sequence want=%q got=%q", want.SequenceID, got.SequenceID))
	}
	if want.HasGoal != got.HasGoal {
		fields = append(fields, fmt.Sprintf("hasGoal want=%t got=%t", want.HasGoal, got.HasGoal))
	}
	if want.GoalX != got.GoalX || want.GoalY != got.GoalY {
		fields = append(fields, fmt.Sprintf("goal want=(%d,%d) got=(%d,%d)", want.GoalX, want.GoalY, got.GoalX, got.GoalY))
	}
	if math.Abs(want.MoveDistance-got.MoveDistance) > 1e-9 {
		fields = append(fields, fmt.Sprintf("moveDistance want=%.5f got=%.5f", want.MoveDistance, got.MoveDistance))
	}
	return strings.Join(fields, ", ")
}

func diffHarnessDecision(want, got enemyAIHarnessDecision) string {
	fields := make([]string, 0, 6)
	if want.HasMove != got.HasMove {
		fields = append(fields, fmt.Sprintf("hasMove want=%t got=%t", want.HasMove, got.HasMove))
	}
	if want.Dir != got.Dir {
		fields = append(fields, fmt.Sprintf("dir want=%d got=%d", want.Dir, got.Dir))
	}
	if want.DestX != got.DestX || want.DestY != got.DestY {
		fields = append(fields, fmt.Sprintf("dest want=(%d,%d) got=(%d,%d)", want.DestX, want.DestY, got.DestX, got.DestY))
	}
	if want.WaitForDoor != got.WaitForDoor {
		fields = append(fields, fmt.Sprintf("waitForDoor want=%t got=%t", want.WaitForDoor, got.WaitForDoor))
	}
	return strings.Join(fields, ", ")
}

func compareEnemyAIMovementScenario(src *game, actorIndex int, playerPos [2]float64) *enemyAIHarnessMismatch {
	gdGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	refGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	gdActor := &gdGame.actors[0]
	refActor := &refGame.actors[0]

	mode := harnessDecisionMode(refGame, refActor)
	snap := captureHarnessSnapshot(refGame, refActor, mode)
	want := wolfsrcReferenceDecision(refGame, refActor, mode)
	got := capturePortDecision(gdGame, gdActor, mode)
	if diff := diffHarnessDecision(want, got); diff != "" {
		return &enemyAIHarnessMismatch{
			Layer:        "movement",
			MapIndex:     snap.MapIndex,
			ActorIndex:   actorIndex,
			EnemyKind:    snap.EnemyKind,
			EnemyState:   snap.EnemyAIState,
			EnemyTileX:   snap.EnemyTileX,
			EnemyTileY:   snap.EnemyTileY,
			EnemyDir:     snap.EnemyDir,
			PlayerX:      snap.PlayerX,
			PlayerY:      snap.PlayerY,
			PlayerTileX:  snap.PlayerTileX,
			PlayerTileY:  snap.PlayerTileY,
			CanSeePlayer: snap.CanSeePlayer,
			Mode:         snap.Mode,
			Diff:         diff,
			Want:         want,
			Got:          got,
		}
	}
	return nil
}

func wolfsrcReserveGoal(g *game, a *actorInstance, decision enemyAIHarnessDecision) {
	a.dir = decision.Dir & 7
	a.facingDir = a.dir
	a.reserveTileGoal(decision.DestX, decision.DestY)
	a.moveDistance = 1
	if decision.WaitForDoor {
		a.moveDistance = actorDoorWaitDistance
	}
	if g.level == nil || g.level.Tile(a.tileX, a.tileY).Door == nil {
		a.area = g.actorAreaAt(a.tileX, a.tileY)
	}
}

func wolfsrcHasInitialSight(g *game, a *actorInstance) bool {
	if a == nil {
		return false
	}
	dx := g.playerX - a.x
	dy := g.playerY - a.y
	const minSight = 1.5
	if dx > -minSight && dx < minSight && dy > -minSight && dy < minSight {
		return true
	}
	switch a.dir & 7 {
	case 2:
		if dy > 0 {
			return false
		}
	case 0:
		if dx < 0 {
			return false
		}
	case 6:
		if dy < 0 {
			return false
		}
	case 4:
		if dx > 0 {
			return false
		}
	}
	return g.actorCanSeePlayer(a)
}

func wolfsrcSightPlayer(g *game, a *actorInstance, tics int) bool {
	if a == nil {
		return false
	}
	if a.reactionTimer > 0 {
		a.reactionTimer -= tics
		if a.reactionTimer > 0 {
			return false
		}
		a.reactionTimer = 0
		if a.kind == actorKindDog {
			g.dogFirstSighting(a)
		} else {
			g.guardFirstSighting(a)
		}
		return true
	}
	if !g.isAreaConnectedToPlayer(a.area) {
		return false
	}
	hasSight := wolfsrcHasInitialSight(g, a)
	if a.ambush {
		if !hasSight {
			return false
		}
		a.ambush = false
	} else if !g.madeNoise && !hasSight {
		return false
	}
	a.reactionTimer = g.actorReactionTics(a)
	if a.reactionTimer == 0 {
		a.reactionTimer = 1
	}
	return false
}

func wolfsrcSelectPathDir(g *game, a *actorInstance) {
	if a == nil || g.level == nil {
		return
	}
	if dir, ok := patrolDirFromInfo(g.level.Tile(a.tileX, a.tileY).RawInfo); ok {
		a.dir = dir
	}
	decision, ok := wolfsrcTryWalkDecision(g, a, a.dir)
	if !ok {
		a.dir = 8
		a.clearTileGoal()
		return
	}
	wolfsrcReserveGoal(g, a, decision)
}

func wolfsrcMoveObj(g *game, a *actorInstance, move float64) bool {
	dx, dy := dirStep(a.dir)
	nextX := a.x + float64(dx)*move
	nextY := a.y + float64(dy)*move
	if g.isAreaConnectedToPlayer(a.area) && math.Abs(nextX-g.playerX) < playerBlockDist && math.Abs(nextY-g.playerY) < playerBlockDist {
		return false
	}
	a.x = nextX
	a.y = nextY
	return true
}

func wolfsrcAdvancePathRuntime(g *game, a *actorInstance, tics int) {
	if a == nil || !a.alive {
		return
	}
	if wolfsrcSightPlayer(g, a, tics) {
		return
	}
	remaining := a.patrolSpeed * float64(tics)
	for remaining > 0 {
		if !a.hasGoal {
			wolfsrcSelectPathDir(g, a)
			if a.dir == 8 || !a.hasGoal {
				return
			}
		}
		if a.moveDistance < 0 {
			if g.level != nil {
				if tile := g.level.Tile(a.tileX, a.tileY); tile.Door != nil {
					g.openDoorAt(a.tileX, a.tileY)
					if !g.isDoorOpen(a.tileX, a.tileY) {
						return
					}
				}
			}
			a.moveDistance = 1
		}
		if remaining < a.moveDistance {
			if wolfsrcMoveObj(g, a, remaining) {
				a.moveDistance -= remaining
			}
			return
		}
		a.x = float64(a.tileX) + 0.5
		a.y = float64(a.tileY) + 0.5
		remaining -= a.moveDistance
		if g.level == nil || g.level.Tile(a.tileX, a.tileY).Door == nil {
			a.area = g.actorAreaAt(a.tileX, a.tileY)
		}
		a.clearTileGoal()
		a.moveDistance = 0
		wolfsrcSelectPathDir(g, a)
		if a.dir == 8 || !a.hasGoal {
			return
		}
	}
}

func wolfsrcAdvanceChaseRuntime(g *game, a *actorInstance, tics int) {
	if a == nil || !a.alive {
		return
	}
	remaining := a.chaseSpeed * float64(tics)
	dodge := g.actorCanSeePlayer(a)
	if a.kind == actorKindDog {
		dodge = true
		if g.dogCanStartJump(a, tics) {
			a.aiState = actorStateJump
			a.rotate = false
			a.sequenceID = a.jumpSequence()
			return
		}
	} else {
		attack := wolfsrcReferenceAttackEntry(g, a, tics)
		if attack.Started {
			a.aiState = attack.AIState
			a.rotate = false
			a.sequenceID = a.shootSequence()
			return
		}
	}
	for remaining > 0 {
		if !a.hasGoal {
			mode := wolfsrcDecisionChase
			consumeFirstAttack := false
			if dodge {
				mode = wolfsrcDecisionDodge
				consumeFirstAttack = a.firstAttack
			}
			decision := wolfsrcReferenceDecision(g, a, mode)
			if !decision.HasMove {
				return
			}
			if consumeFirstAttack {
				a.firstAttack = false
			}
			wolfsrcReserveGoal(g, a, decision)
		}
		if a.moveDistance < 0 {
			if g.level != nil {
				if tile := g.level.Tile(a.tileX, a.tileY); tile.Door != nil {
					g.openDoorAt(a.tileX, a.tileY)
					if !g.isDoorOpen(a.tileX, a.tileY) {
						return
					}
				}
			}
			a.moveDistance = 1
		}
		if remaining < a.moveDistance {
			if wolfsrcMoveObj(g, a, remaining) {
				a.moveDistance -= remaining
			}
			return
		}
		a.x = float64(a.tileX) + 0.5
		a.y = float64(a.tileY) + 0.5
		remaining -= a.moveDistance
		if g.level == nil || g.level.Tile(a.tileX, a.tileY).Door == nil {
			a.area = g.actorAreaAt(a.tileX, a.tileY)
		}
		a.clearTileGoal()
		a.moveDistance = 0
	}
}

func compareEnemyAIRuntimeScenario(src *game, actorIndex int, playerPos [2]float64, tics int) *enemyAIHarnessMismatch {
	gdGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	refGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	gdGame.rng = testRNG(9)
	refGame.rng = testRNG(9)
	gdGame.updateDoors(tics)
	refGame.updateDoors(tics)
	gdActor := &gdGame.actors[0]
	refActor := &refGame.actors[0]
	if gdActor.aiState != refActor.aiState {
		return nil
	}
	start := captureHarnessRuntimeState(refActor)
	switch gdActor.aiState {
	case actorStateStand:
		if gdActor.kind == actorKindDog {
			gdGame.updateDogStand(gdActor, tics)
		} else {
			gdGame.updateGuardStand(gdActor, tics)
		}
		_ = wolfsrcSightPlayer(refGame, refActor, tics)
	case actorStatePatrol:
		if gdActor.kind == actorKindDog {
			gdGame.updateDogPatrol(gdActor, tics)
		} else {
			gdGame.updateGuardPatrol(gdActor, tics)
		}
		wolfsrcAdvancePathRuntime(refGame, refActor, tics)
	case actorStateChase:
		if !gdActor.alerted || !refActor.alerted {
			return nil
		}
		if gdActor.kind == actorKindDog {
			gdGame.updateDogChase(gdActor, tics)
		} else {
			gdGame.updateGuardChase(gdActor, tics)
		}
		wolfsrcAdvanceChaseRuntime(refGame, refActor, tics)
	default:
		return nil
	}
	got := captureHarnessRuntimeState(gdActor)
	want := captureHarnessRuntimeState(refActor)
	if diff := diffHarnessRuntimeState(want, got); diff != "" {
		return &enemyAIHarnessMismatch{
			Layer:        "runtime_move",
			MapIndex:     refGame.mapIndex,
			ActorIndex:   actorIndex,
			EnemyKind:    refActor.kind,
			EnemyState:   refActor.aiState,
			EnemyTileX:   refActor.tileX,
			EnemyTileY:   refActor.tileY,
			EnemyDir:     refActor.dir,
			PlayerX:      refGame.playerX,
			PlayerY:      refGame.playerY,
			PlayerTileX:  int(refGame.playerX),
			PlayerTileY:  int(refGame.playerY),
			CanSeePlayer: refGame.actorCanSeePlayer(refActor),
			Tics:         tics,
			Diff:         diff,
			Start:        start,
			Want:         want,
			Got:          got,
		}
	}
	return nil
}

func compareEnemyAIFirstSightingScenario(src *game, actorIndex int, playerPos [2]float64) *enemyAIHarnessMismatch {
	gdGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	refGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	got := capturePortFirstSighting(gdGame, &gdGame.actors[0])
	want := wolfsrcReferenceFirstSighting(refGame, &refGame.actors[0])
	if diff := diffHarnessFirstSighting(want, got); diff != "" {
		a := &refGame.actors[0]
		return &enemyAIHarnessMismatch{
			Layer:        "first_sighting",
			MapIndex:     refGame.mapIndex,
			ActorIndex:   actorIndex,
			EnemyKind:    a.kind,
			EnemyState:   a.aiState,
			EnemyTileX:   a.tileX,
			EnemyTileY:   a.tileY,
			EnemyDir:     a.dir,
			PlayerX:      refGame.playerX,
			PlayerY:      refGame.playerY,
			PlayerTileX:  int(refGame.playerX),
			PlayerTileY:  int(refGame.playerY),
			CanSeePlayer: refGame.actorCanSeePlayer(a),
			Diff:         diff,
			Want:         want,
			Got:          got,
		}
	}
	return nil
}

func compareEnemyAIAttackEntryScenario(src *game, actorIndex int, playerPos [2]float64, tics int) *enemyAIHarnessMismatch {
	gdGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	refGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	gdGame.rng = testRNG(9)
	refGame.rng = testRNG(9)
	got := capturePortAttackEntry(gdGame, &gdGame.actors[0], tics)
	want := wolfsrcReferenceAttackEntry(refGame, &refGame.actors[0], tics)
	if diff := diffHarnessAttackEntry(want, got); diff != "" {
		a := &refGame.actors[0]
		return &enemyAIHarnessMismatch{
			Layer:        "attack_entry",
			MapIndex:     refGame.mapIndex,
			ActorIndex:   actorIndex,
			EnemyKind:    a.kind,
			EnemyState:   a.aiState,
			EnemyTileX:   a.tileX,
			EnemyTileY:   a.tileY,
			EnemyDir:     a.dir,
			PlayerX:      refGame.playerX,
			PlayerY:      refGame.playerY,
			PlayerTileX:  int(refGame.playerX),
			PlayerTileY:  int(refGame.playerY),
			CanSeePlayer: refGame.actorCanSeePlayer(a),
			Tics:         tics,
			Diff:         diff,
			Want:         want,
			Got:          got,
		}
	}
	return nil
}

type enemyAIHarnessTraceScript string

const (
	enemyAITraceCircleStrafe enemyAIHarnessTraceScript = "circle_strafe"
	enemyAITraceStationary   enemyAIHarnessTraceScript = "stationary"
	enemyAITraceBackpedal    enemyAIHarnessTraceScript = "backpedal"
	enemyAITraceZigZagStrafe enemyAIHarnessTraceScript = "zigzag_strafe"
	enemyAITraceDoorBait     enemyAIHarnessTraceScript = "door_bait"
)

func moveEnemyAITracePlayer(g *game, actor *actorInstance, moveX, moveY float64) {
	prevX, prevY := g.playerX, g.playerY
	g.tryMove(moveX, moveY)
	g.playerMovingFast = math.Hypot(g.playerX-prevX, g.playerY-prevY) > walkSpeed
	g.playerA = math.Atan2(actor.y-g.playerY, actor.x-g.playerX)
	g.cameraX = g.playerX
	g.cameraY = g.playerY
	g.rebuildPlayerAreas()
}

func applyEnemyAITraceScript(g *game, actor *actorInstance, script enemyAIHarnessTraceScript, tickSeed, step, tics int) {
	switch script {
	case enemyAITraceCircleStrafe:
		fuzzCircleStrafePlayer(g, actor, tickSeed, step, tics)
	case enemyAITraceStationary:
		g.playerMovingFast = false
		g.playerA = math.Atan2(actor.y-g.playerY, actor.x-g.playerX)
		g.cameraX = g.playerX
		g.cameraY = g.playerY
		g.rebuildPlayerAreas()
	case enemyAITraceBackpedal:
		dx := g.playerX - actor.x
		dy := g.playerY - actor.y
		dist := math.Hypot(dx, dy)
		if dist < 1e-6 {
			dx, dy = 1, 0
			dist = 1
		}
		speed := walkSpeed * float64(tics)
		if ((tickSeed >> ((step + 2) % 8)) & 1) != 0 {
			speed = runSpeed * float64(tics)
		}
		moveEnemyAITracePlayer(g, actor, dx/dist*speed, dy/dist*speed)
	case enemyAITraceZigZagStrafe:
		dx := g.playerX - actor.x
		dy := g.playerY - actor.y
		dist := math.Hypot(dx, dy)
		if dist < 1e-6 {
			dx, dy = 1, 0
			dist = 1
		}
		strafeDir := 1.0
		if step%2 != 0 {
			strafeDir = -1
		}
		tangentX := -dy / dist * strafeDir
		tangentY := dx / dist * strafeDir
		// Bias slightly outward so the player keeps crossing attack thresholds
		// instead of settling into a perfect orbit.
		radialX := dx / dist * 0.35
		radialY := dy / dist * 0.35
		speed := walkSpeed * float64(tics)
		if ((tickSeed >> ((step + 5) % 8)) & 1) != 0 {
			speed = runSpeed * float64(tics)
		}
		moveEnemyAITracePlayer(g, actor, (tangentX+radialX)*speed, (tangentY+radialY)*speed)
	case enemyAITraceDoorBait:
		dx := g.playerX - actor.x
		dy := g.playerY - actor.y
		dist := math.Hypot(dx, dy)
		if dist < 1e-6 {
			dx, dy = 1, 0
			dist = 1
		}
		doorDx, doorDy := 0.0, 0.0
		playerTileX := int(g.playerX)
		playerTileY := int(g.playerY)
		for _, stepDir := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			tile := g.level.Tile(playerTileX+stepDir[0], playerTileY+stepDir[1])
			if tile.Door != nil {
				doorDx = float64(stepDir[0])
				doorDy = float64(stepDir[1])
				break
			}
		}
		if doorDx == 0 && doorDy == 0 {
			// Fall back to a lateral weave if the player drifted away from the
			// door-adjacent seed tile.
			doorDx = -dy / dist
			doorDy = dx / dist
		}
		swingDir := 1.0
		if step%2 != 0 {
			swingDir = -1
		}
		speed := walkSpeed * float64(tics)
		if ((tickSeed >> ((step + 1) % 8)) & 1) != 0 {
			speed = runSpeed * float64(tics)
		}
		moveEnemyAITracePlayer(g, actor, doorDx*swingDir*speed, doorDy*swingDir*speed)
	default:
		panic(fmt.Sprintf("unknown enemy ai trace script %q", script))
	}
}

func compareEnemyAITraceScenario(t *testing.T, src *game, actorIndex int, playerPos [2]float64, script enemyAIHarnessTraceScript, tickSeed, stepCount int) *enemyAIHarnessMismatch {
	t.Helper()

	if actorIndex < 0 || actorIndex >= len(src.actors) {
		t.Fatalf("actorIndex %d out of range for trace compare", actorIndex)
	}
	if stepCount <= 0 {
		t.Fatalf("stepCount = %d, want > 0", stepCount)
	}

	g := cloneEnemyAIFuzzBaseline(src, byte(17+tickSeed%200))
	g.playerX = playerPos[0]
	g.playerY = playerPos[1]
	g.cameraX = g.playerX
	g.cameraY = g.playerY
	g.playerA = math.Atan2(g.actors[actorIndex].y-g.playerY, g.actors[actorIndex].x-g.playerX)
	g.rebuildPlayerAreas()

	for step := 0; step < stepCount; step++ {
		if actorIndex >= len(g.actors) {
			t.Fatalf("actorIndex %d went out of range during trace", actorIndex)
		}
		actor := &g.actors[actorIndex]
		tics := 1 + absInt(tickSeed+step*11)%8
		applyEnemyAITraceScript(g, actor, script, tickSeed, step, tics)
		if ((tickSeed >> (step % 8)) & 1) != 0 {
			g.madeNoise = true
		} else {
			g.madeNoise = false
		}

		currentPos := [2]float64{g.playerX, g.playerY}
		for _, m := range []*enemyAIHarnessMismatch{
			compareEnemyAIFirstSightingScenario(g, actorIndex, currentPos),
			compareEnemyAIAttackEntryScenario(g, actorIndex, currentPos, tics),
			compareEnemyAIMovementScenario(g, actorIndex, currentPos),
			compareEnemyAIRuntimeScenario(g, actorIndex, currentPos, tics),
		} {
			if m == nil {
				continue
			}
			m.Step = step
			m.Script = string(script)
			return m
		}

		g.updateDoors(tics)
		g.updateActors(tics)
		fuzzAssertActorState(t, g)
	}
	return nil
}

func minimizeEnemyAITraceMismatch(t *testing.T, src *game, actorIndex int, playerPos [2]float64, script enemyAIHarnessTraceScript, tickSeed, stepCount int, first *enemyAIHarnessMismatch) *enemyAIHarnessMismatch {
	t.Helper()
	if first == nil {
		return nil
	}
	best := *first
	if best.Step > 0 {
		stepCount = minInt(stepCount, best.Step+1)
	}
	for candidateSteps := 1; candidateSteps < stepCount; candidateSteps++ {
		m := compareEnemyAITraceScenario(t, src, actorIndex, playerPos, script, tickSeed, candidateSteps)
		if m == nil {
			continue
		}
		best = *m
		stepCount = candidateSteps
		break
	}
	return &best
}

func runEnemyAIHarnessScenario(t *testing.T, src *game, actorIndex int, playerPos [2]float64) {
	t.Helper()
	if m := compareEnemyAIMovementScenario(src, actorIndex, playerPos); m != nil {
		t.Fatalf("enemy ai harness mismatch: map=%d kind=%v state=%v mode=%s enemyTile=(%d,%d) enemyDir=%d player=(%.2f,%.2f) playerTile=(%d,%d) canSee=%t: %s",
			m.MapIndex, m.EnemyKind, m.EnemyState, m.Mode, m.EnemyTileX, m.EnemyTileY, m.EnemyDir,
			m.PlayerX, m.PlayerY, m.PlayerTileX, m.PlayerTileY, m.CanSeePlayer, m.Diff)
	}
}

func runEnemyAIFirstSightingHarnessScenario(t *testing.T, src *game, actorIndex int, playerPos [2]float64) {
	t.Helper()
	if m := compareEnemyAIFirstSightingScenario(src, actorIndex, playerPos); m != nil {
		t.Fatalf("enemy ai first-sighting mismatch: map=%d kind=%v enemyTile=(%d,%d) player=(%.2f,%.2f): %s",
			m.MapIndex, m.EnemyKind, m.EnemyTileX, m.EnemyTileY, m.PlayerX, m.PlayerY, m.Diff)
	}
}

func runEnemyAIAttackEntryHarnessScenario(t *testing.T, src *game, actorIndex int, playerPos [2]float64, tics int) {
	t.Helper()
	if m := compareEnemyAIAttackEntryScenario(src, actorIndex, playerPos, tics); m != nil {
		t.Fatalf("enemy ai attack-entry mismatch: map=%d kind=%v enemyTile=(%d,%d) player=(%.2f,%.2f) tics=%d: %s",
			m.MapIndex, m.EnemyKind, m.EnemyTileX, m.EnemyTileY, m.PlayerX, m.PlayerY, tics, m.Diff)
	}
}

func runEnemyAIRunDecisionHarnessScenario(t *testing.T, src *game, actorIndex int, playerPos [2]float64) {
	t.Helper()
	gdGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	refGame := cloneAIHarnessGame(src, actorIndex, playerPos)
	gdActor := &gdGame.actors[0]
	refActor := &refGame.actors[0]
	mode := wolfsrcDecisionRun
	snap := captureHarnessSnapshot(refGame, refActor, mode)
	want := wolfsrcReferenceDecision(refGame, refActor, mode)
	got := capturePortDecision(gdGame, gdActor, mode)
	if diff := diffHarnessDecision(want, got); diff != "" {
		t.Fatalf("enemy ai run-decision mismatch: map=%d kind=%v state=%v mode=%s enemyTile=(%d,%d) enemyDir=%d player=(%.2f,%.2f) playerTile=(%d,%d) canSee=%t: %s",
			snap.MapIndex, snap.EnemyKind, snap.EnemyAIState, snap.Mode, snap.EnemyTileX, snap.EnemyTileY, snap.EnemyDir,
			snap.PlayerX, snap.PlayerY, snap.PlayerTileX, snap.PlayerTileY, snap.CanSeePlayer, diff)
	}
}

func TestEnemyAIHarnessSmoke(t *testing.T) {
	cases := []struct {
		mapIndex  int
		actorPick int
		playerPos [2]float64
	}{
		{mapIndex: 0, actorPick: 0, playerPos: [2]float64{18.5, 55.5}},
		{mapIndex: 1, actorPick: 1, playerPos: [2]float64{53.5, 49.5}},
		{mapIndex: 2, actorPick: 0, playerPos: [2]float64{35.5, 29.5}},
	}

	for _, tc := range cases {
		g := fuzzTestGameWithSharewareMap(t, tc.mapIndex, 7)
		live := fuzzLiveEnemyIndices(g)
		if len(live) == 0 {
			t.Fatalf("map %d has no live enemies", tc.mapIndex)
		}
		runEnemyAIHarnessScenario(t, g, live[tc.actorPick%len(live)], tc.playerPos)
	}
}

func TestEnemyAIFirstSightingHarnessSmoke(t *testing.T) {
	g := fuzzTestGameWithSharewareMap(t, 0, 7)
	live := fuzzLiveEnemyIndices(g)
	if len(live) == 0 {
		t.Fatal("map 0 has no live enemies")
	}
	runEnemyAIFirstSightingHarnessScenario(t, g, live[0], [2]float64{18.5, 55.5})
}

func TestEnemyAIAttackEntryHarnessSmoke(t *testing.T) {
	g := fuzzTestGameWithSharewareMap(t, 0, 7)
	live := fuzzLiveEnemyIndices(g)
	if len(live) == 0 {
		t.Fatal("map 0 has no live enemies")
	}
	for _, actorIndex := range live {
		a := &g.actors[actorIndex]
		positions := fuzzNearbyPlayerPositions(g, a)
		if len(positions) == 0 {
			continue
		}
		runEnemyAIAttackEntryHarnessScenario(t, g, actorIndex, positions[len(positions)/2], 4)
		return
	}
	t.Fatal("no actor had nearby player placements for attack-entry smoke test")
}

func TestEnemyAIRunDecisionHarnessSmoke(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 2, 4, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 2.5
	g.actors = []actorInstance{{
		kind:       actorKindGuard,
		x:          2.5,
		y:          3.5,
		tileX:      2,
		tileY:      3,
		dir:        2,
		facingDir:  2,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		area:       0,
		aiState:    actorStateChase,
		chaseSpeed: 0.2,
	}}
	runEnemyAIRunDecisionHarnessScenario(t, g, 0, [2]float64{2.5, 2.5})
}

func TestEnemyAITraceHarnessSmoke(t *testing.T) {
	g := fuzzTestGameWithSharewareMap(t, 0, 19)
	live := fuzzLiveEnemyIndices(g)
	if len(live) == 0 {
		t.Fatal("map 0 has no live enemies")
	}
	actorIndex := live[0]
	positions := fuzzNearbyPlayerPositions(g, &g.actors[actorIndex])
	if len(positions) == 0 {
		t.Fatal("selected actor had no nearby placements for trace smoke test")
	}
	if m := compareEnemyAITraceScenario(t, g, actorIndex, positions[len(positions)/2], enemyAITraceCircleStrafe, 23, 8); m != nil {
		t.Fatalf("enemy ai trace mismatch: script=%s step=%d map=%d kind=%v enemyTile=(%d,%d) player=(%.2f,%.2f): %s",
			m.Script, m.Step, m.MapIndex, m.EnemyKind, m.EnemyTileX, m.EnemyTileY, m.PlayerX, m.PlayerY, m.Diff)
	}
	if m := compareEnemyAITraceScenario(t, g, actorIndex, positions[len(positions)/3], enemyAITraceBackpedal, 29, 8); m != nil {
		t.Fatalf("enemy ai trace mismatch: script=%s step=%d map=%d kind=%v enemyTile=(%d,%d) player=(%.2f,%.2f): %s",
			m.Script, m.Step, m.MapIndex, m.EnemyKind, m.EnemyTileX, m.EnemyTileY, m.PlayerX, m.PlayerY, m.Diff)
	}
}

func TestEnemyAIRuntimeHarnessSmoke(t *testing.T) {
	g := fuzzTestGameWithSharewareMap(t, 0, 23)
	live := fuzzLiveEnemyIndices(g)
	if len(live) == 0 {
		t.Fatal("map 0 has no live enemies")
	}
	actorIndex := live[0]
	positions := fuzzNearbyPlayerPositions(g, &g.actors[actorIndex])
	if len(positions) == 0 {
		t.Fatal("selected actor had no nearby placements for runtime smoke test")
	}
	if m := compareEnemyAIRuntimeScenario(g, actorIndex, positions[len(positions)/2], 4); m != nil {
		t.Fatalf("enemy ai runtime mismatch: map=%d kind=%v enemyTile=(%d,%d) player=(%.2f,%.2f) tics=%d: %s",
			m.MapIndex, m.EnemyKind, m.EnemyTileX, m.EnemyTileY, m.PlayerX, m.PlayerY, m.Tics, m.Diff)
	}
}

func TestEnemyAIDoorTraceHarnessSmoke(t *testing.T) {
	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		g := fuzzTestGameWithSharewareMap(t, mapIndex, 31)
		for _, actorIndex := range fuzzLiveEnemyIndices(g) {
			positions := fuzzNearbyDoorPlayerPositions(g, &g.actors[actorIndex])
			if len(positions) == 0 {
				continue
			}
			if m := compareEnemyAITraceScenario(t, g, actorIndex, positions[len(positions)/2], enemyAITraceDoorBait, 37, 8); m != nil {
				t.Fatalf("enemy ai door trace mismatch: script=%s step=%d map=%d kind=%v enemyTile=(%d,%d) player=(%.2f,%.2f): %s",
					m.Script, m.Step, m.MapIndex, m.EnemyKind, m.EnemyTileX, m.EnemyTileY, m.PlayerX, m.PlayerY, m.Diff)
			}
			return
		}
	}
	t.Fatal("no actor had a nearby door-adjacent placement for door trace smoke test")
}

func TestEnemyAIHarnessScanSharewareNearbyPlacements(t *testing.T) {
	if os.Getenv("GDWOLF_SCAN_WOLFSRC") == "" {
		t.Skip("set GDWOLF_SCAN_WOLFSRC=1 to run the real-map wolfsrc comparison scan")
	}
	reportPath := strings.TrimSpace(os.Getenv("GDWOLF_SCAN_WOLFSRC_REPORT"))
	report, err := openEnemyAIHarnessReport(reportPath)
	if err != nil {
		t.Fatalf("open harness report: %v", err)
	}
	defer func() {
		if err := report.Close(); err != nil {
			t.Fatalf("close harness report: %v", err)
		}
	}()

	mismatchCount := 0
	const attackEntryTics = 4

	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		g := fuzzTestGameWithSharewareMap(t, mapIndex, 11)
		live := fuzzLiveEnemyIndices(g)
		for _, actorIndex := range live {
			positions := fuzzNearbyPlayerPositions(g, &g.actors[actorIndex])
			for _, pos := range positions {
				if math.IsNaN(pos[0]) || math.IsNaN(pos[1]) {
					t.Fatalf("invalid player position for map=%d actor=%d", mapIndex, actorIndex)
				}
				for _, m := range []*enemyAIHarnessMismatch{
					compareEnemyAIFirstSightingScenario(g, actorIndex, pos),
					compareEnemyAIAttackEntryScenario(g, actorIndex, pos, attackEntryTics),
					compareEnemyAIMovementScenario(g, actorIndex, pos),
					compareEnemyAIRuntimeScenario(g, actorIndex, pos, attackEntryTics),
				} {
					if m == nil {
						continue
					}
					mismatchCount++
					if err := report.Write(*m); err != nil {
						t.Fatalf("write harness report: %v", err)
					}
				}
			}
		}
	}
	if mismatchCount > 0 {
		if reportPath != "" {
			t.Fatalf("enemy ai harness scan found %d mismatches; report=%s", mismatchCount, reportPath)
		}
		t.Fatalf("enemy ai harness scan found %d mismatches", mismatchCount)
	}
}

func TestEnemyAIHarnessScanSharewareTraceScripts(t *testing.T) {
	if os.Getenv("GDWOLF_SCAN_WOLFSRC_TRACE") == "" {
		t.Skip("set GDWOLF_SCAN_WOLFSRC_TRACE=1 to run the trace-script wolfsrc comparison scan")
	}

	reportPath := strings.TrimSpace(os.Getenv("GDWOLF_SCAN_WOLFSRC_REPORT"))
	report, err := openEnemyAIHarnessReport(reportPath)
	if err != nil {
		t.Fatalf("open harness report: %v", err)
	}
	defer func() {
		if err := report.Close(); err != nil {
			t.Fatalf("close harness report: %v", err)
		}
	}()

	traceScripts := []enemyAIHarnessTraceScript{
		enemyAITraceStationary,
		enemyAITraceCircleStrafe,
		enemyAITraceBackpedal,
		enemyAITraceZigZagStrafe,
	}
	mismatchCount := 0

	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		snapshot := enemyAIFuzzSnapshot(t, mapIndex)
		for actorPick, actorIndex := range snapshot.liveEnemies {
			positions := snapshot.placements[actorIndex]
			if len(positions) == 0 {
				continue
			}
			base := cloneEnemyAIFuzzBaseline(snapshot.base, byte(41+mapIndex*7+actorPick))
			playerPos := positions[(mapIndex+actorPick)%len(positions)]
			for scriptIndex, script := range traceScripts {
				tickSeed := mapIndex*97 + actorPick*13 + scriptIndex*17
				if m := compareEnemyAITraceScenario(t, base, actorIndex, playerPos, script, tickSeed, 10); m != nil {
					m = minimizeEnemyAITraceMismatch(t, base, actorIndex, playerPos, script, tickSeed, 10, m)
					mismatchCount++
					if err := report.Write(*m); err != nil {
						t.Fatalf("write harness report: %v", err)
					}
				}
			}
		}
	}

	if mismatchCount > 0 {
		if reportPath != "" {
			t.Fatalf("enemy ai trace scan found %d mismatches; report=%s", mismatchCount, reportPath)
		}
		t.Fatalf("enemy ai trace scan found %d mismatches", mismatchCount)
	}
}

func TestEnemyAIHarnessScanSharewareDoorTraceScripts(t *testing.T) {
	if os.Getenv("GDWOLF_SCAN_WOLFSRC_TRACE_DOORS") == "" {
		t.Skip("set GDWOLF_SCAN_WOLFSRC_TRACE_DOORS=1 to run the door-trace wolfsrc comparison scan")
	}

	reportPath := strings.TrimSpace(os.Getenv("GDWOLF_SCAN_WOLFSRC_REPORT"))
	report, err := openEnemyAIHarnessReport(reportPath)
	if err != nil {
		t.Fatalf("open harness report: %v", err)
	}
	defer func() {
		if err := report.Close(); err != nil {
			t.Fatalf("close harness report: %v", err)
		}
	}()

	mismatchCount := 0
	for mapIndex := 0; mapIndex < 10; mapIndex++ {
		snapshot := enemyAIFuzzSnapshot(t, mapIndex)
		for actorPick, actorIndex := range snapshot.liveEnemies {
			base := cloneEnemyAIFuzzBaseline(snapshot.base, byte(71+mapIndex*11+actorPick))
			positions := fuzzNearbyDoorPlayerPositions(base, &base.actors[actorIndex])
			if len(positions) == 0 {
				continue
			}
			playerPos := positions[(mapIndex+actorPick)%len(positions)]
			tickSeed := mapIndex*131 + actorPick*19
			if m := compareEnemyAITraceScenario(t, base, actorIndex, playerPos, enemyAITraceDoorBait, tickSeed, 10); m != nil {
				m = minimizeEnemyAITraceMismatch(t, base, actorIndex, playerPos, enemyAITraceDoorBait, tickSeed, 10, m)
				mismatchCount++
				if err := report.Write(*m); err != nil {
					t.Fatalf("write harness report: %v", err)
				}
			}
		}
	}

	if mismatchCount > 0 {
		if reportPath != "" {
			t.Fatalf("enemy ai door trace scan found %d mismatches; report=%s", mismatchCount, reportPath)
		}
		t.Fatalf("enemy ai door trace scan found %d mismatches", mismatchCount)
	}
}
