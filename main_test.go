package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gd-wolf/internal/wl6"
)

func testRNG(seed int) *wolfRNG {
	return newWolfRNG(byte(seed))
}

func testRNGForPredicates(preds ...func(byte) bool) *wolfRNG {
	for seed := 0; seed < len(wolfRandomTable); seed++ {
		idx := byte(seed)
		ok := true
		for _, pred := range preds {
			idx++
			if !pred(wolfRandomTable[idx]) {
				ok = false
				break
			}
		}
		if ok {
			return newWolfRNG(byte(seed))
		}
	}
	panic("no wolf RNG seed matches predicates")
}

func rngValueRange(min, max byte) func(byte) bool {
	return func(v byte) bool {
		return v >= min && v <= max
	}
}

func rngValueAtLeast(min byte) func(byte) bool {
	return func(v byte) bool {
		return v >= min
	}
}

func rngValueLessThan(limit byte) func(byte) bool {
	return func(v byte) bool {
		return v < limit
	}
}

func rngValueNonZeroDamage(v byte) bool {
	return v >= 16
}

func rngModulo(mod, want byte) func(byte) bool {
	return func(v byte) bool {
		return v%mod == want
	}
}

func rngValueEquals(want byte) func(byte) bool {
	return func(v byte) bool {
		return v == want
	}
}

func testGameWithLevel(level *wl6.Level) *game {
	g := &game{
		level:        level,
		levelWidth:   level.Width,
		levelHeight:  level.Height,
		difficulty:   difficultyMedium,
		bestWeapon:   0,
		weapon:       1,
		chosenWeapon: 1,
		viewWidth:    320,
		viewHeight:   200,
		zbuffer:      make([]float64, 320),
		rng:          testRNG(1),
	}
	g.cachedWallIDs = make([]uint16, level.Width*level.Height)
	g.cachedDoorFlags = make([]byte, level.Width*level.Height)
	g.cachedDoorSides = make([]byte, level.Width*level.Height)
	g.renderableWalls = make([]bool, level.Width*level.Height)
	for i, tile := range level.Tiles {
		if tile.RenderWall || tile.Solid {
			g.cachedWallIDs[i] = renderWallTile(tile)
			g.renderableWalls[i] = true
		}
		if tile.Door != nil {
			if tile.Door.Vertical {
				g.cachedDoorFlags[i] = 2
			} else {
				g.cachedDoorFlags[i] = 1
			}
		}
	}
	g.doorOpen = make([]float64, level.Width*level.Height)
	g.doorState = make([]byte, level.Width*level.Height)
	g.doorTimer = make([]int, level.Width*level.Height)
	return g
}

func blankLevel(width, height int) *wl6.Level {
	return &wl6.Level{
		Width:  width,
		Height: height,
		Tiles:  make([]wl6.Tile, width*height),
	}
}

func setLevelTile(level *wl6.Level, x, y int, tile wl6.Tile) {
	if tile.Solid && tile.Door == nil {
		tile.RenderWall = true
	}
	level.Tiles[y*level.Width+x] = tile
}

func TestRebuildLayoutUsesWolfAspectStretch(t *testing.T) {
	g := &game{viewWidth: 1280, viewHeight: 800}
	g.rebuildLayout()

	if g.layout.renderLeft != 107 || g.layout.renderRight != 1174 {
		t.Fatalf("render x bounds = %d..%d, want 107..1174", g.layout.renderLeft, g.layout.renderRight)
	}
	if g.layout.renderTop != 0 || g.layout.renderBottom != 640 {
		t.Fatalf("render y bounds = %d..%d, want 0..640", g.layout.renderTop, g.layout.renderBottom)
	}
	if g.layout.renderWidth != 1067 || g.layout.renderHeight != 640 {
		t.Fatalf("render size = %dx%d, want 1067x640", g.layout.renderWidth, g.layout.renderHeight)
	}
	if math.Abs(g.layout.screenScaleY/g.layout.screenScaleX-1.2) > 1e-9 {
		t.Fatalf("vertical stretch ratio = %.6f, want 1.2", g.layout.screenScaleY/g.layout.screenScaleX)
	}
	if math.Abs(g.layout.statusY-640) > 1e-9 {
		t.Fatalf("status y = %.6f, want 640", g.layout.statusY)
	}
	if g.layout.bufferWidth != g.layout.renderWidth || g.layout.bufferHeight != g.layout.renderHeight {
		t.Fatalf("ultra buffer size = %dx%d, want render size %dx%d", g.layout.bufferWidth, g.layout.bufferHeight, g.layout.renderWidth, g.layout.renderHeight)
	}
}

func TestRebuildLayoutFixedRenderModesUseInternalBuffers(t *testing.T) {
	tests := []struct {
		name       string
		mode       renderMode
		wantWidth  int
		wantHeight int
	}{
		{name: "dos", mode: renderModeDOS, wantWidth: wolfScreenWidth, wantHeight: wolfGameplayLines},
		{name: "hq", mode: renderModeHQ, wantWidth: hqGameplayWidth, wantHeight: hqGameplayHeight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &game{viewWidth: 1280, viewHeight: 800, renderMode: tt.mode}
			g.rebuildLayout()
			if g.layout.bufferWidth != tt.wantWidth || g.layout.bufferHeight != tt.wantHeight {
				t.Fatalf("buffer size = %dx%d, want %dx%d", g.layout.bufferWidth, g.layout.bufferHeight, tt.wantWidth, tt.wantHeight)
			}
			if g.layout.renderWidth != 1067 || g.layout.renderHeight != 640 {
				t.Fatalf("screen render size = %dx%d, want 1067x640", g.layout.renderWidth, g.layout.renderHeight)
			}
		})
	}
}

func TestFitImageRectPreservesSourceAspectRatio(t *testing.T) {
	drawX, drawY, scale := fitImageRect(1920, 1080, 0, 0, wolfScreenWidth, wolfScreenHeight)

	if math.Abs(scale-float64(wolfScreenWidth)/1920.0) > 1e-9 {
		t.Fatalf("scale = %.12f, want %.12f", scale, float64(wolfScreenWidth)/1920.0)
	}
	if math.Abs(drawX) > 1e-9 {
		t.Fatalf("drawX = %.12f, want 0", drawX)
	}
	wantY := (float64(wolfScreenHeight) - 1080.0*scale) / 2
	if math.Abs(drawY-wantY) > 1e-9 {
		t.Fatalf("drawY = %.12f, want %.12f", drawY, wantY)
	}
}

func TestFitImageRectKeepsGetPsychedHDReplacementInOriginalBounds(t *testing.T) {
	drawX, drawY, scale := fitImageRect(448, 96, 48, 76, 224, 48)

	if math.Abs(scale-0.5) > 1e-9 {
		t.Fatalf("scale = %.12f, want 0.5", scale)
	}
	if math.Abs(drawX-48) > 1e-9 {
		t.Fatalf("drawX = %.12f, want 48", drawX)
	}
	if math.Abs(drawY-76) > 1e-9 {
		t.Fatalf("drawY = %.12f, want 76", drawY)
	}
}

func TestShouldDrawTitleDirectToScreenOnlyForHDTitleImages(t *testing.T) {
	lowRes := &game{titlePic: ebiten.NewImage(wolfScreenWidth, wolfScreenHeight)}
	if lowRes.shouldDrawTitleDirectToScreen() {
		t.Fatal("low-res title image should stay on the frontend canvas")
	}

	hd := &game{titlePic: ebiten.NewImage(1920, 1080)}
	if !hd.shouldDrawTitleDirectToScreen() {
		t.Fatal("hd title image should render directly to the screen")
	}
}

func TestRenderModeMenuMapping(t *testing.T) {
	tests := []struct {
		mode  renderMode
		index int
	}{
		{mode: renderModeDOS, index: 0},
		{mode: renderModeHQ, index: 1},
		{mode: renderModeUltra, index: 2},
	}

	items := renderModeMenuItems()
	if len(items) != 3 {
		t.Fatalf("render mode item count = %d, want 3", len(items))
	}

	for _, tt := range tests {
		if got := renderModeMenuIndex(tt.mode); got != tt.index {
			t.Fatalf("renderModeMenuIndex(%v) = %d, want %d", tt.mode, got, tt.index)
		}
		if got := renderModeMenuChoice(tt.index); got != tt.mode {
			t.Fatalf("renderModeMenuChoice(%d) = %v, want %v", tt.index, got, tt.mode)
		}
	}
}

func TestSetRenderModeRebuildsBuffersImmediately(t *testing.T) {
	g := &game{
		viewWidth:       1280,
		viewHeight:      800,
		frame:           make([]byte, 1280*800*4),
		background:      make([]byte, 1280*800*4),
		prevWallTops:    make([]int, 1280),
		prevWallBottoms: make([]int, 1280),
		renderMode:      renderModeUltra,
	}

	g.setRenderMode(renderModeDOS)
	if g.renderMode != renderModeDOS {
		t.Fatalf("render mode = %v, want DOS", g.renderMode)
	}
	if g.layout.bufferWidth != wolfScreenWidth || g.layout.bufferHeight != wolfGameplayLines {
		t.Fatalf("dos buffer size = %dx%d, want %dx%d", g.layout.bufferWidth, g.layout.bufferHeight, wolfScreenWidth, wolfGameplayLines)
	}

	g.setRenderMode(renderModeUltra)
	if g.renderMode != renderModeUltra {
		t.Fatalf("render mode = %v, want ULTRA", g.renderMode)
	}
	if g.layout.bufferWidth != g.layout.renderWidth || g.layout.bufferHeight != g.layout.renderHeight {
		t.Fatalf("ultra buffer size = %dx%d, want render size %dx%d", g.layout.bufferWidth, g.layout.bufferHeight, g.layout.renderWidth, g.layout.renderHeight)
	}
}

func TestSharewareSpriteCatalogUsesCanonicalLatestLayout(t *testing.T) {
	setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"})
	t.Cleanup(func() {
		setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"})
	})

	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("OpenEmbeddedShareware: %v", err)
	}
	setSpriteCatalogVariant(files.Variant)

	rocket, ok := LookupAnimSequence(seqProjectileRocket)
	if !ok {
		t.Fatal("seqProjectileRocket missing")
	}
	if got, want := rocket.Frames[0].Shape, 370; got != want {
		t.Fatalf("shareware rocket start = %d, want %d", got, want)
	}
	if got, want := rocket.Frames[len(rocket.Frames)-1].Shape, 377; got != want {
		t.Fatalf("shareware rocket end = %d, want %d", got, want)
	}

	boom, ok := LookupAnimSequence(seqProjectileBoom)
	if !ok {
		t.Fatal("seqProjectileBoom missing")
	}
	if got, want := boom.Frames[len(boom.Frames)-1].Shape, 384; got != want {
		t.Fatalf("shareware boom end = %d, want %d", got, want)
	}

	run, ok := LookupAnimSequence(seqVictoryBJRun)
	if !ok {
		t.Fatal("seqVictoryBJRun missing")
	}
	if got, want := run.Frames[0].Shape, 408; got != want {
		t.Fatalf("shareware BJ run start = %d, want %d", got, want)
	}
	if got, want := activeVictoryBJWalk1, 408; got != want {
		t.Fatalf("shareware active BJ walk shape = %d, want %d", got, want)
	}
}

func TestPopulateMissingWolfSoundsUsesOriginalAdLibChunks(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("OpenEmbeddedShareware: %v", err)
	}

	soundData := buildSoundBank(audioSampleRate)
	populateMissingWolfSounds(files, audioSampleRate, soundData)

	for _, id := range []soundID{
		soundNoWay,
		soundMenuMove,
		soundMenuConfirm,
		soundMenuBack,
		soundPickupKey,
		soundPickupAmmo,
		soundPickupMachineGun,
		soundPickupChaingun,
		soundPickupHealth1,
		soundPickupHealth2,
		soundPickupTreasure1,
		soundPickupTreasure2,
		soundPickupTreasure3,
		soundPickupTreasure4,
		soundPickupOneUp,
		soundKnife,
		soundHitEnemy,
	} {
		if len(soundData[id]) == 0 {
			t.Fatalf("sound %v missing rendered Wolf audio", id)
		}
	}
}

func TestCurrentPersistentConfigIncludesRenderPromptFlag(t *testing.T) {
	g := &game{
		renderMode:         renderModeDOS,
		renderModePrompted: true,
		hdTexturesEnabled:  true,
		sfxVolume:          0.5,
		musicVolume:        1.0,
		mouseLook:          defaultMouseLook,
		turnSpeed:          defaultTurnSpeed,
		mapMoveSpeed:       defaultMapMoveSpeed,
		vsyncEnabled:       true,
		keybinds:           defaultKeybinds(),
	}

	cfg := g.currentPersistentConfig()
	if !cfg.RenderModePrompted {
		t.Fatal("RenderModePrompted = false, want true")
	}
}

func TestApplyPersistentConfigSetsRenderPromptFlag(t *testing.T) {
	g := &game{keybinds: defaultKeybinds()}
	g.applyPersistentConfig(persistentConfig{
		SFXVolume:          0.5,
		MusicVolume:        1.0,
		MouseSensitivity:   defaultMouseLook,
		TurnSpeed:          defaultTurnSpeed,
		MapMoveSpeed:       defaultMapMoveSpeed,
		RenderMode:         renderModeHQ.label(),
		RenderModePrompted: true,
		HDTexturesEnabled:  false,
		VsyncEnabled:       true,
		Keybinds:           map[string]persistentKeybind{},
	})

	if !g.renderModePrompted {
		t.Fatal("renderModePrompted = false, want true")
	}
	if g.renderMode != renderModeHQ {
		t.Fatalf("renderMode = %v, want HQ", g.renderMode)
	}
	if g.hdTexturesEnabled {
		t.Fatal("hdTexturesEnabled = true, want false")
	}
}

func TestOptionsMenuItemsUseGraphicsSubmenu(t *testing.T) {
	g := &game{
		sfxVolume:         0.5,
		musicVolume:       1.0,
		mouseLook:         defaultMouseLook,
		turnSpeed:         defaultTurnSpeed,
		mapMoveSpeed:      defaultMapMoveSpeed,
		renderMode:        renderModeUltra,
		hdTexturesEnabled: true,
		vsyncEnabled:      true,
	}

	items := g.optionsMenuItems()
	if len(items) != 4 {
		t.Fatalf("options item count = %d, want 4", len(items))
	}
	if items[0] != "Graphics >" {
		t.Fatalf("options item 0 = %q, want Graphics >", items[0])
	}
	if items[1] != "Audio >" {
		t.Fatalf("options item 1 = %q, want Audio >", items[1])
	}
	if items[2] != "Controls >" {
		t.Fatalf("options item 2 = %q, want Controls >", items[2])
	}

	audioItems := g.audioMenuItems()
	if len(audioItems) != 3 {
		t.Fatalf("audio item count = %d, want 3", len(audioItems))
	}
	if audioItems[0] != "Sound Volume   50%" {
		t.Fatalf("audio item 0 = %q, want Sound Volume   50%%", audioItems[0])
	}
	if audioItems[1] != "Music Volume  100%" {
		t.Fatalf("audio item 1 = %q, want Music Volume  100%%", audioItems[1])
	}

	controls := g.controlsMenuItems()
	if len(controls) != 5 {
		t.Fatalf("controls item count = %d, want 5", len(controls))
	}
	if controls[3] != "Keybinds >" {
		t.Fatalf("controls item 3 = %q, want Keybinds >", controls[3])
	}

	graphics := g.graphicsMenuItems()
	if len(graphics) != 4 {
		t.Fatalf("graphics item count = %d, want 4", len(graphics))
	}
	if graphics[1] != "HD Textures   On" {
		t.Fatalf("graphics item 1 = %q, want HD Textures   On", graphics[1])
	}
}

func TestNextTitleInputStateHonorsPromptFlag(t *testing.T) {
	g := &game{}
	if got := g.nextTitleInputState(); got != uiStateRenderModePrompt {
		t.Fatalf("nextTitleInputState() = %v, want render mode prompt", got)
	}
	g.renderModePrompted = true
	if got := g.nextTitleInputState(); got != uiStateMainMenu {
		t.Fatalf("nextTitleInputState() = %v, want main menu", got)
	}
}

func TestFinishVictorySequenceTransitionsToIntermission(t *testing.T) {
	g := &game{
		uiState:            uiStatePlaying,
		mode:               modeMap,
		paused:             true,
		mousePrimed:        true,
		victoryActive:      true,
		victoryPhase:       victoryPhaseJump,
		victoryRunDistance: 1,
		victoryBJ:          staticSprite{alive: true},
	}

	if err := g.finishVictorySequence(); err != nil {
		t.Fatalf("finishVictorySequence: %v", err)
	}
	if g.victoryActive || g.victoryPhase != victoryPhaseNone || g.victoryRunDistance != 0 || g.victoryBJ.alive {
		t.Fatalf("victory state not cleared: active=%t phase=%v run=%.2f alive=%t", g.victoryActive, g.victoryPhase, g.victoryRunDistance, g.victoryBJ.alive)
	}
	if g.fadeAction == nil {
		t.Fatal("fadeAction = nil, want pending transition")
	}
	if err := g.fadeAction(); err != nil {
		t.Fatalf("fadeAction: %v", err)
	}
	if g.uiState != uiStateVictoryIntermission {
		t.Fatalf("uiState = %v, want victory intermission", g.uiState)
	}
	if g.paused {
		t.Fatal("paused = true, want false")
	}
	if g.mode != modeRaycast {
		t.Fatalf("mode = %d, want raycast", g.mode)
	}
	if g.menuReturn != uiStateMainMenu || g.menuIndex != 0 || g.mousePrimed {
		t.Fatalf("menu state = return %v index %d primed %t, want main menu/0/false", g.menuReturn, g.menuIndex, g.mousePrimed)
	}
}

func TestRebuildCameraColumnsUsesGameplayViewport(t *testing.T) {
	g := &game{viewWidth: 1280, viewHeight: 800}
	g.rebuildLayout()
	g.rebuildCameraColumns()

	if len(g.cameraColumns) != g.layout.bufferWidth {
		t.Fatalf("camera column count = %d, want %d", len(g.cameraColumns), g.layout.bufferWidth)
	}
	wantLeft := (2*0.5)/float64(g.layout.bufferWidth) - 1
	if math.Abs(g.cameraColumns[0]-wantLeft) > 1e-12 {
		t.Fatalf("left gameplay camera column = %.12f, want %.12f", g.cameraColumns[0], wantLeft)
	}
	wantRight := (2*(float64(g.layout.bufferWidth)-0.5))/float64(g.layout.bufferWidth) - 1
	if math.Abs(g.cameraColumns[g.layout.bufferWidth-1]-wantRight) > 1e-12 {
		t.Fatalf("right gameplay camera column = %.12f, want %.12f", g.cameraColumns[g.layout.bufferWidth-1], wantRight)
	}
}

func TestEnsureFrameRebuildsCameraColumnsWhenReusingSize(t *testing.T) {
	g := &game{
		viewWidth:       1280,
		viewHeight:      800,
		frame:           make([]byte, 1280*800*4),
		background:      make([]byte, 1280*800*4),
		zbuffer:         make([]float64, 1280),
		prevWallTops:    make([]int, 1280),
		prevWallBottoms: make([]int, 1280),
		cameraColumns:   make([]float64, 1280),
	}

	g.ensureFrame(1280, 800)

	if len(g.cameraColumns) != g.layout.bufferWidth {
		t.Fatalf("camera column count = %d, want %d", len(g.cameraColumns), g.layout.bufferWidth)
	}
	wantLeft := (2*0.5)/float64(g.layout.bufferWidth) - 1
	if math.Abs(g.cameraColumns[0]-wantLeft) > 1e-12 {
		t.Fatalf("left gameplay camera column = %.12f, want %.12f", g.cameraColumns[0], wantLeft)
	}
	wantRight := (2*(float64(g.layout.bufferWidth)-0.5))/float64(g.layout.bufferWidth) - 1
	if math.Abs(g.cameraColumns[g.layout.bufferWidth-1]-wantRight) > 1e-12 {
		t.Fatalf("right gameplay camera column = %.12f, want %.12f", g.cameraColumns[g.layout.bufferWidth-1], wantRight)
	}
}

func TestLookupStaticTable(t *testing.T) {
	for info := uint16(23); info <= 74; info++ {
		if info >= 71 && info <= 73 {
			if _, ok := LookupStatic(info); ok {
				t.Fatalf("LookupStatic(%d) unexpectedly found sparse entry", info)
			}
			continue
		}
		if _, ok := LookupStatic(info); !ok {
			t.Fatalf("LookupStatic(%d) missing", info)
		}
	}

	tests := []struct {
		info     uint16
		shape    int
		blocking bool
		pickup   pickupType
	}{
		{info: 24, shape: shapeSPR_STAT_1, blocking: true, pickup: pickupNone},
		{info: 49, shape: shapeSPR_STAT_26, blocking: false, pickup: pickupClip},
		{info: 50, shape: shapeSPR_STAT_27, blocking: false, pickup: pickupMachineGun},
		{info: 55, shape: shapeSPR_STAT_32, blocking: false, pickup: pickupCrown},
		{info: 74, shape: shapeSPR_STAT_26, blocking: false, pickup: pickupClip2},
		{info: 124, shape: shapeGuardDead, blocking: false, pickup: pickupNone},
	}
	for _, tt := range tests {
		def, ok := LookupStatic(tt.info)
		if !ok {
			t.Fatalf("LookupStatic(%d) missing", tt.info)
		}
		if def.Shape != tt.shape || def.Blocking != tt.blocking || def.Pickup != tt.pickup {
			t.Fatalf("LookupStatic(%d) = %+v, want shape=%d blocking=%t pickup=%v", tt.info, def, tt.shape, tt.blocking, tt.pickup)
		}
	}
}

func TestBuildStaticSpritesIncludesDeadGuardCorpse(t *testing.T) {
	level := blankLevel(2, 2)
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 124})

	g := testGameWithLevel(level)
	sprites, treasureTotal := g.buildStaticSprites()
	if treasureTotal != 0 {
		t.Fatalf("treasureTotal = %d, want 0", treasureTotal)
	}
	if len(sprites) != 1 {
		t.Fatalf("buildStaticSprites len = %d, want 1", len(sprites))
	}
	if sprites[0].shapenum != shapeGuardDead || sprites[0].blocking || sprites[0].pickup != pickupNone {
		t.Fatalf("corpse sprite = %+v, want shapenum=%d nonblocking no pickup", sprites[0], shapeGuardDead)
	}
}

func TestBuildStaticSpritesSkipsActiveActorSpawns(t *testing.T) {
	level := blankLevel(3, 1)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 152})
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 162})
	setLevelTile(level, 2, 0, wl6.Tile{RawInfo: 214})

	g := testGameWithLevel(level)
	g.difficulty = difficultyMedium
	sprites, treasureTotal := g.buildStaticSprites()
	if treasureTotal != 0 {
		t.Fatalf("treasureTotal = %d, want 0", treasureTotal)
	}
	if len(sprites) != 0 {
		t.Fatalf("buildStaticSprites len = %d, want 0 for active actor spawns", len(sprites))
	}
}

func TestStaticDefForPickup(t *testing.T) {
	tests := []struct {
		pickup pickupType
		shape  int
	}{
		{pickup: pickupClip, shape: shapeSPR_STAT_26},
		{pickup: pickupClip2, shape: shapeSPR_STAT_26},
		{pickup: pickupMachineGun, shape: shapeSPR_STAT_27},
	}
	for _, tt := range tests {
		def, ok := StaticDefForPickup(tt.pickup)
		if !ok {
			t.Fatalf("StaticDefForPickup(%v) missing", tt.pickup)
		}
		if def.Shape != tt.shape {
			t.Fatalf("StaticDefForPickup(%v) shape = %d, want %d", tt.pickup, def.Shape, tt.shape)
		}
	}
}

func TestLookupActorSpawnGuardRanges(t *testing.T) {
	tests := []struct {
		info       uint16
		difficulty gameDifficulty
		mode       ActorSpawnMode
		ok         bool
	}{
		{info: 108, difficulty: difficultyEasy, mode: actorSpawnStand, ok: true},
		{info: 112, difficulty: difficultyEasy, mode: actorSpawnPatrol, ok: true},
		{info: 144, difficulty: difficultyEasy, ok: false},
		{info: 144, difficulty: difficultyMedium, mode: actorSpawnStand, ok: true},
		{info: 148, difficulty: difficultyMedium, mode: actorSpawnPatrol, ok: true},
		{info: 180, difficulty: difficultyMedium, ok: false},
		{info: 180, difficulty: difficultyHard, mode: actorSpawnStand, ok: true},
	}
	for _, tt := range tests {
		def, ok := LookupActorSpawn(tt.info, tt.difficulty)
		if ok != tt.ok {
			t.Fatalf("LookupActorSpawn(%d,%v) ok = %t, want %t", tt.info, tt.difficulty, ok, tt.ok)
		}
		if !tt.ok {
			continue
		}
		if def.Kind != actorKindGuard || def.SpawnMode != tt.mode {
			t.Fatalf("LookupActorSpawn(%d,%v) = %+v, want guard mode %v", tt.info, tt.difficulty, def, tt.mode)
		}
	}
}

func TestLookupActorSpawnDogRanges(t *testing.T) {
	tests := []struct {
		info       uint16
		difficulty gameDifficulty
		mode       ActorSpawnMode
		ok         bool
	}{
		{info: 134, difficulty: difficultyEasy, mode: actorSpawnStand, ok: true},
		{info: 138, difficulty: difficultyEasy, mode: actorSpawnPatrol, ok: true},
		{info: 170, difficulty: difficultyEasy, ok: false},
		{info: 170, difficulty: difficultyMedium, mode: actorSpawnStand, ok: true},
		{info: 174, difficulty: difficultyMedium, mode: actorSpawnPatrol, ok: true},
		{info: 206, difficulty: difficultyMedium, ok: false},
		{info: 206, difficulty: difficultyHard, mode: actorSpawnStand, ok: true},
	}
	for _, tt := range tests {
		def, ok := LookupActorSpawn(tt.info, tt.difficulty)
		if ok != tt.ok {
			t.Fatalf("LookupActorSpawn(%d,%v) ok = %t, want %t", tt.info, tt.difficulty, ok, tt.ok)
		}
		if !tt.ok {
			continue
		}
		if def.Kind != actorKindDog || def.SpawnMode != tt.mode {
			t.Fatalf("LookupActorSpawn(%d,%v) = %+v, want dog mode %v", tt.info, tt.difficulty, def, tt.mode)
		}
	}
}

func TestLookupActorSpawnBossDifficultySelection(t *testing.T) {
	tests := []struct {
		difficulty gameDifficulty
		hp         int
	}{
		{difficulty: difficultyEasy, hp: 850},
		{difficulty: difficultyMedium, hp: 950},
		{difficulty: difficultyHard, hp: 1050},
	}
	for _, tt := range tests {
		def, ok := LookupActorSpawn(214, tt.difficulty)
		if !ok {
			t.Fatalf("LookupActorSpawn(214,%v) missing", tt.difficulty)
		}
		if def.Kind != actorKindBoss || def.HitPoints != tt.hp {
			t.Fatalf("LookupActorSpawn(214,%v) = %+v, want boss hp %d", tt.difficulty, def, tt.hp)
		}
	}
}

func TestLookupActorSpawnMutantDifficultySelection(t *testing.T) {
	tests := []struct {
		info       uint16
		difficulty gameDifficulty
		mode       ActorSpawnMode
		hp         int
		ok         bool
	}{
		{info: 216, difficulty: difficultyEasy, mode: actorSpawnStand, hp: 45, ok: true},
		{info: 220, difficulty: difficultyEasy, mode: actorSpawnPatrol, hp: 45, ok: true},
		{info: 234, difficulty: difficultyEasy, ok: false},
		{info: 234, difficulty: difficultyMedium, mode: actorSpawnStand, hp: 55, ok: true},
		{info: 238, difficulty: difficultyMedium, mode: actorSpawnPatrol, hp: 55, ok: true},
		{info: 252, difficulty: difficultyMedium, ok: false},
		{info: 252, difficulty: difficultyHard, mode: actorSpawnStand, hp: 55, ok: true},
		{info: 256, difficulty: difficultyHard, mode: actorSpawnPatrol, hp: 55, ok: true},
	}
	for _, tt := range tests {
		def, ok := LookupActorSpawn(tt.info, tt.difficulty)
		if ok != tt.ok {
			t.Fatalf("LookupActorSpawn(%d,%v) ok = %t, want %t", tt.info, tt.difficulty, ok, tt.ok)
		}
		if !tt.ok {
			continue
		}
		if def.Kind != actorKindMutant || def.SpawnMode != tt.mode || def.HitPoints != tt.hp {
			t.Fatalf("LookupActorSpawn(%d,%v) = %+v, want mutant mode %v hp %d", tt.info, tt.difficulty, def, tt.mode, tt.hp)
		}
	}
}

func TestBuildActorsMediumDifficulty(t *testing.T) {
	level := blankLevel(3, 3)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 108})
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 112})
	setLevelTile(level, 2, 0, wl6.Tile{RawInfo: 144})
	setLevelTile(level, 0, 1, wl6.Tile{RawInfo: 148})
	setLevelTile(level, 1, 1, wl6.Tile{RawInfo: 180})

	g := testGameWithLevel(level)
	g.difficulty = difficultyMedium
	actors := g.buildActors()
	if len(actors) != 4 {
		t.Fatalf("buildActors len = %d, want 4", len(actors))
	}
	stand := 0
	patrol := 0
	for _, actor := range actors {
		switch actor.spawnMode {
		case actorSpawnStand:
			stand++
		case actorSpawnPatrol:
			patrol++
		}
	}
	if stand != 2 || patrol != 2 {
		t.Fatalf("stand/patrol = %d/%d, want 2/2", stand, patrol)
	}
}

func TestBuildActorsIncludesDog(t *testing.T) {
	level := blankLevel(2, 2)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 170})
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 174})

	g := testGameWithLevel(level)
	g.difficulty = difficultyMedium
	actors := g.buildActors()
	if len(actors) != 2 {
		t.Fatalf("buildActors len = %d, want 2", len(actors))
	}
	if actors[0].kind != actorKindDog || actors[0].aiState != actorStateStand {
		t.Fatalf("first actor = %+v, want stand dog", actors[0])
	}
	if actors[1].kind != actorKindDog || actors[1].aiState != actorStatePatrol {
		t.Fatalf("second actor = %+v, want patrol dog", actors[1])
	}
	if actors[0].shapenum != shapeDogBase || !actors[0].rotate {
		t.Fatalf("dog actor shape/rotate = %d/%t, want %d/true", actors[0].shapenum, actors[0].rotate, shapeDogBase)
	}
}

func TestBuildActorsIncludesOfficer(t *testing.T) {
	level := blankLevel(2, 2)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 152})
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 156})

	g := testGameWithLevel(level)
	g.difficulty = difficultyMedium
	actors := g.buildActors()
	if len(actors) != 2 {
		t.Fatalf("buildActors len = %d, want 2", len(actors))
	}
	if actors[0].kind != actorKindOfficer || actors[0].aiState != actorStateStand {
		t.Fatalf("first actor = %+v, want stand officer", actors[0])
	}
	if actors[1].kind != actorKindOfficer || actors[1].aiState != actorStatePatrol {
		t.Fatalf("second actor = %+v, want patrol officer", actors[1])
	}
	if actors[0].shapenum != shapeOfficerBase || !actors[0].rotate {
		t.Fatalf("officer actor shape/rotate = %d/%t, want %d/true", actors[0].shapenum, actors[0].rotate, shapeOfficerBase)
	}
	if actors[0].health != 50 {
		t.Fatalf("officer health = %d, want 50", actors[0].health)
	}
	if actors[0].chaseSpeed <= actors[0].patrolSpeed {
		t.Fatalf("officer chase speed %.4f <= patrol speed %.4f", actors[0].chaseSpeed, actors[0].patrolSpeed)
	}
}

func TestBuildActorsIncludesSS(t *testing.T) {
	level := blankLevel(2, 2)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 162})
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 166})

	g := testGameWithLevel(level)
	g.difficulty = difficultyMedium
	g.bestWeapon = 0
	actors := g.buildActors()
	if len(actors) != 2 {
		t.Fatalf("buildActors len = %d, want 2", len(actors))
	}
	if actors[0].kind != actorKindSS || actors[0].aiState != actorStateStand {
		t.Fatalf("first actor = %+v, want stand ss", actors[0])
	}
	if actors[1].kind != actorKindSS || actors[1].aiState != actorStatePatrol {
		t.Fatalf("second actor = %+v, want patrol ss", actors[1])
	}
	if actors[0].shapenum != shapeSSBase || !actors[0].rotate {
		t.Fatalf("ss actor shape/rotate = %d/%t, want %d/true", actors[0].shapenum, actors[0].rotate, shapeSSBase)
	}
	if actors[0].health != 100 {
		t.Fatalf("ss health = %d, want 100", actors[0].health)
	}
	if actors[0].chaseSpeed <= actors[0].patrolSpeed {
		t.Fatalf("ss chase speed %.4f <= patrol speed %.4f", actors[0].chaseSpeed, actors[0].patrolSpeed)
	}
	if actors[0].dropPickup != pickupMachineGun {
		t.Fatalf("ss dropPickup = %v, want machinegun for low-weapon loadout", actors[0].dropPickup)
	}
}

func TestBuildActorsIncludesMutant(t *testing.T) {
	level := blankLevel(2, 2)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 234})
	setLevelTile(level, 1, 0, wl6.Tile{RawInfo: 238})

	g := testGameWithLevel(level)
	g.difficulty = difficultyMedium
	actors := g.buildActors()
	if len(actors) != 2 {
		t.Fatalf("buildActors len = %d, want 2", len(actors))
	}
	if actors[0].kind != actorKindMutant || actors[0].aiState != actorStateStand {
		t.Fatalf("first actor = %+v, want stand mutant", actors[0])
	}
	if actors[1].kind != actorKindMutant || actors[1].aiState != actorStatePatrol {
		t.Fatalf("second actor = %+v, want patrol mutant", actors[1])
	}
	if actors[0].shapenum != shapeMutantBase || !actors[0].rotate {
		t.Fatalf("mutant actor shape/rotate = %d/%t, want %d/true", actors[0].shapenum, actors[0].rotate, shapeMutantBase)
	}
	if actors[0].health != 55 {
		t.Fatalf("mutant health = %d, want 55", actors[0].health)
	}
	if actors[0].chaseSpeed <= actors[0].patrolSpeed {
		t.Fatalf("mutant chase speed %.4f <= patrol speed %.4f", actors[0].chaseSpeed, actors[0].patrolSpeed)
	}
}

func TestBuildActorsIncludesBoss(t *testing.T) {
	level := blankLevel(1, 1)
	setLevelTile(level, 0, 0, wl6.Tile{RawInfo: 214})

	tests := []struct {
		difficulty gameDifficulty
		hp         int
	}{
		{difficulty: difficultyEasy, hp: 850},
		{difficulty: difficultyMedium, hp: 950},
		{difficulty: difficultyHard, hp: 1050},
	}
	for _, tt := range tests {
		g := testGameWithLevel(level)
		g.difficulty = tt.difficulty
		actors := g.buildActors()
		if len(actors) != 1 {
			t.Fatalf("difficulty %v buildActors len = %d, want 1", tt.difficulty, len(actors))
		}
		if actors[0].kind != actorKindBoss || actors[0].aiState != actorStateStand {
			t.Fatalf("difficulty %v actor = %+v, want stand boss", tt.difficulty, actors[0])
		}
		if actors[0].shapenum != shapeBossBase || actors[0].rotate {
			t.Fatalf("difficulty %v boss shape/rotate = %d/%t, want %d/false", tt.difficulty, actors[0].shapenum, actors[0].rotate, shapeBossBase)
		}
		if actors[0].dir != 6 || actors[0].facingDir != 6 {
			t.Fatalf("difficulty %v boss dir/facing = %d/%d, want south/6", tt.difficulty, actors[0].dir, actors[0].facingDir)
		}
		if !actors[0].ambush {
			t.Fatalf("difficulty %v boss ambush = false, want true", tt.difficulty)
		}
		if actors[0].health != tt.hp {
			t.Fatalf("difficulty %v boss health = %d, want %d", tt.difficulty, actors[0].health, tt.hp)
		}
	}
}

func TestPatrolDirFromInfo(t *testing.T) {
	for info := uint16(90); info <= 97; info++ {
		dir, ok := patrolDirFromInfo(info)
		if !ok || dir != int(info-90) {
			t.Fatalf("patrolDirFromInfo(%d) = (%d,%t), want (%d,true)", info, dir, ok, int(info-90))
		}
	}
	if _, ok := patrolDirFromInfo(89); ok {
		t.Fatal("patrolDirFromInfo(89) unexpectedly succeeded")
	}
}

func TestActorStartShootChance(t *testing.T) {
	if got := actorStartShootChance(3, 1, false); got != 5 {
		t.Fatalf("chance dist=3 tics=1 = %d, want 5", got)
	}
	if got := actorStartShootChance(3, 4, false); got != 21 {
		t.Fatalf("chance dist=3 tics=4 = %d, want 21", got)
	}
	if got := actorStartShootChance(1, 1, true); got != 300 {
		t.Fatalf("point-blank chance = %d, want 300", got)
	}
}

func TestGuardPatrolAdvancesToArrow(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{RawInfo: 90, Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{RawInfo: 90, Area: 0})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:        actorKindGuard,
		x:           1.5,
		y:           1.5,
		tileX:       1,
		tileY:       1,
		goalX:       1,
		goalY:       1,
		alive:       true,
		blocking:    true,
		shootable:   true,
		aiState:     actorStatePatrol,
		spawnMode:   actorSpawnPatrol,
		patrolSpeed: 2,
	}
	g.actors = []actorInstance{actor}

	g.updateActors(1)
	if g.actors[0].tileX != 3 || g.actors[0].tileY != 1 {
		t.Fatalf("patrol moved to (%d,%d), want (3,1)", g.actors[0].tileX, g.actors[0].tileY)
	}
}

func TestBuildActorsPatrolStartsMovingFromSpawnFacing(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{RawInfo: 112, Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.difficulty = difficultyEasy
	g.actors = g.buildActors()
	if len(g.actors) != 1 {
		t.Fatalf("actor count = %d, want 1", len(g.actors))
	}
	if !g.actors[0].hasGoal || g.actors[0].goalX != 2 || g.actors[0].goalY != 1 {
		t.Fatalf("initial patrol goal = (%d,%d,%t), want (2,1,true)", g.actors[0].goalX, g.actors[0].goalY, g.actors[0].hasGoal)
	}

	startX := g.actors[0].x
	g.updateActors(1)
	if g.actors[0].x <= startX {
		t.Fatalf("spawn patrol x = %f, want greater than start %f", g.actors[0].x, startX)
	}
	if g.actors[0].goalX != 2 || g.actors[0].goalY != 1 {
		t.Fatalf("spawn patrol goal after update = (%d,%d), want (2,1)", g.actors[0].goalX, g.actors[0].goalY)
	}
}

func TestCollidesUsesWolfPlayerBox(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Solid: true})

	g := testGameWithLevel(level)
	if !g.collides(1.7, 1.5) {
		t.Fatal("expected Wolf-sized player box to collide with adjacent wall")
	}
}

func TestTryMoveSlidesAlongWall(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 2, 1, wl6.Tile{Solid: true})

	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5

	g.tryMove(0.3, 0.3)

	if g.playerX != 1.5 {
		t.Fatalf("playerX = %.3f, want 1.5 after blocked X move", g.playerX)
	}
	if g.playerY <= 1.5 {
		t.Fatalf("playerY = %.3f, want slide along open Y axis", g.playerY)
	}
}

func TestDoorCollisionUsesThinCenterSlab(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	idx := 1*level.Width + 2
	g.doorOpen[idx] = 0
	g.doorState[idx] = 0
	if g.collides(1.52, 1.5) {
		t.Fatal("closed door should not block outside the 75 percent slab")
	}
	if !g.collides(1.79, 1.5) {
		t.Fatal("closed door should block at the center slab")
	}

	g.doorOpen[idx] = 1
	g.doorState[idx] = 2
	if g.collides(1.8, 1.5) {
		t.Fatal("fully open door should allow movement")
	}
}

func TestTryMoveDoesNotSlideAlongDoorSlab(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5

	g.tryMove(0.3, 0.3)

	if g.playerX != 1.5 {
		t.Fatalf("playerX = %.3f, want 1.5 when door blocks the move", g.playerX)
	}
	if g.playerY != 1.5 {
		t.Fatalf("playerY = %.3f, want 1.5 when door blocks the move", g.playerY)
	}
}

func TestMoveActorTowardGoalStopsBeforePlayerOverlap(t *testing.T) {
	level := blankLevel(4, 3)
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 1.5

	actor := &actorInstance{
		x:       1.5,
		y:       1.5,
		tileX:   2,
		tileY:   1,
		goalX:   2,
		goalY:   1,
		hasGoal: true,
		alive:   true,
	}

	g.moveActorTowardGoal(actor, 1.0)

	if actor.x != 1.5 || actor.y != 1.5 {
		t.Fatalf("actor moved to (%.3f, %.3f), want blocked at (1.5, 1.5)", actor.x, actor.y)
	}
	if !actor.hasGoal {
		t.Fatal("actor goal cleared despite blocked move")
	}
}

func TestMoveActorTowardGoalIgnoresPlayerWhenAreaDisconnected(t *testing.T) {
	level := blankLevel(4, 3)
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 1.5
	g.playerAreas = []bool{true, false}

	actor := &actorInstance{
		x:       1.5,
		y:       1.5,
		tileX:   2,
		tileY:   1,
		goalX:   2,
		goalY:   1,
		hasGoal: true,
		alive:   true,
		area:    1,
	}

	g.moveActorTowardGoal(actor, 1.0)

	if actor.x != 2.5 || actor.y != 1.5 {
		t.Fatalf("actor moved to (%.3f, %.3f), want advance through disconnected-area player overlap", actor.x, actor.y)
	}
	if actor.hasGoal {
		t.Fatal("actor goal should clear after reaching reserved tile")
	}
}

func TestClosingDoorStaysOpenWhenPlayerOverlapsDoorway(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	g.playerX = 1.8
	g.playerY = 1.5
	idx := 1*level.Width + 2
	g.doorOpen[idx] = 0.5
	g.doorState[idx] = 3
	g.doorTimer[idx] = 12

	g.updateDoors(1)

	if g.doorState[idx] != 1 {
		t.Fatalf("doorState = %d, want 1 (reopening)", g.doorState[idx])
	}
	if g.doorTimer[idx] != 0 {
		t.Fatalf("doorTimer = %d, want 0", g.doorTimer[idx])
	}
}

func TestOpenDoorDoesNotAutoCloseIntoPlayerPath(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	g.playerX = 1.8
	g.playerY = 1.5
	idx := 1*level.Width + 2
	g.doorOpen[idx] = 1
	g.doorState[idx] = 2
	g.doorTimer[idx] = doorOpenHoldTics

	g.updateDoors(1)

	if g.doorState[idx] != 2 {
		t.Fatalf("doorState = %d, want 2 (stay open)", g.doorState[idx])
	}
	if g.doorOpen[idx] != 1 {
		t.Fatalf("doorOpen = %f, want 1", g.doorOpen[idx])
	}
}

func TestClosingDoorReopensWhenPlayerBlocksSlab(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	g.playerX = 1.8
	g.playerY = 1.5
	idx := 1*level.Width + 2
	g.doorOpen[idx] = 0.5
	g.doorState[idx] = 3

	g.updateDoors(1)

	if g.doorState[idx] != 1 {
		t.Fatalf("doorState = %d, want 1 (reopening)", g.doorState[idx])
	}
	if g.doorTimer[idx] != 0 {
		t.Fatalf("doorTimer = %d, want 0", g.doorTimer[idx])
	}
}

func TestActorTryWalkBlocksDiagonalCornerCutting(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 2, 1, wl6.Tile{RawWall: 1, Solid: true})
	setLevelTile(level, 1, 2, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 2, wl6.Tile{Area: 0})

	g := testGameWithLevel(level)
	actor := &actorInstance{
		alive:    true,
		blocking: true,
		tileX:    1,
		tileY:    1,
		x:        1.5,
		y:        1.5,
	}

	if g.actorTryWalk(actor, 1) {
		t.Fatal("diagonal corner cut unexpectedly allowed")
	}
}

func TestActorChooseDirectChaseGoalUsesCardinalDirection(t *testing.T) {
	level := blankLevel(4, 4)
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 3.5
	actor := &actorInstance{
		x:      1.5,
		y:      1.5,
		tileX:  1,
		tileY:  1,
		alive:  true,
		dir:    8,
		rotate: true,
	}

	g.actorChooseChaseGoal(actor, false)
	if !actor.hasGoal {
		t.Fatal("expected chase goal to be chosen")
	}
	if actor.dir != 0 || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("direct chase picked dir=%d goal=(%d,%d), want east to (2,1)", actor.dir, actor.goalX, actor.goalY)
	}
}

func TestActorChooseDodgeGoalUsesDiagonalDirection(t *testing.T) {
	level := blankLevel(4, 4)
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 3.5
	g.rng = testRNG(1)
	actor := &actorInstance{
		x:      1.5,
		y:      1.5,
		tileX:  1,
		tileY:  1,
		alive:  true,
		dir:    8,
		rotate: true,
	}

	g.actorChooseChaseGoal(actor, true)
	if !actor.hasGoal {
		t.Fatal("expected dodge goal to be chosen")
	}
	if actor.dir != 7 || actor.goalX != 2 || actor.goalY != 2 {
		t.Fatalf("dodge chase picked dir=%d goal=(%d,%d), want southeast to (2,2)", actor.dir, actor.goalX, actor.goalY)
	}
}

func TestActorChooseDodgeGoalAllowsTurnaroundOnFirstAttack(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 2, 1, wl6.Tile{RawWall: 1, Solid: true})
	g := testGameWithLevel(level)
	g.playerX = 0.5
	g.playerY = 2.5
	g.rng = testRNG(1)
	actor := &actorInstance{
		x:           2.5,
		y:           2.5,
		tileX:       2,
		tileY:       2,
		alive:       true,
		dir:         0,
		rotate:      true,
		firstAttack: true,
	}

	g.actorChooseChaseGoal(actor, true)
	if !actor.hasGoal {
		t.Fatal("expected dodge goal to be chosen")
	}
	if actor.dir != 4 || actor.goalX != 1 || actor.goalY != 2 {
		t.Fatalf("first-attack dodge picked dir=%d goal=(%d,%d), want west to (1,2)", actor.dir, actor.goalX, actor.goalY)
	}
	if actor.firstAttack {
		t.Fatal("firstAttack should be consumed after first dodge selection")
	}
}

func TestActorChooseDirectChaseGoalMatchesSourceFallbackArc(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 3, 2, wl6.Tile{Solid: true})
	setLevelTile(level, 2, 3, wl6.Tile{Solid: true})
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 3.5
	g.rng = testRNGForPredicates(rngValueRange(0, 128))
	actor := &actorInstance{
		x:         2.5,
		y:         2.5,
		tileX:     2,
		tileY:     2,
		alive:     true,
		blocking:  true,
		shootable: true,
		dir:       0,
		facingDir: 0,
		area:      0,
	}

	g.actorChooseDirectChaseGoal(actor)
	if !actor.hasGoal {
		t.Fatal("expected direct chase fallback goal")
	}
	if actor.dir != 2 || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("fallback dir=%d goal=(%d,%d), want north to (2,1)", actor.dir, actor.goalX, actor.goalY)
	}
}

func TestActorChooseRunGoalMatchesSourceFallbackArc(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 2, 1, wl6.Tile{Solid: true})
	setLevelTile(level, 1, 2, wl6.Tile{Solid: true})
	setLevelTile(level, 3, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 2.5
	g.rng = testRNGForPredicates(rngValueAtLeast(129))
	actor := &actorInstance{
		x:         2.5,
		y:         3.5,
		tileX:     2,
		tileY:     3,
		alive:     true,
		blocking:  true,
		shootable: true,
		dir:       2,
		facingDir: 2,
		area:      0,
	}

	g.actorChooseRunGoal(actor)
	if !actor.hasGoal {
		t.Fatal("expected run fallback goal")
	}
	if actor.dir != 6 || actor.goalX != 2 || actor.goalY != 4 {
		t.Fatalf("run fallback dir=%d goal=(%d,%d), want south to (2,4)", actor.dir, actor.goalX, actor.goalY)
	}
}

func TestActorChooseDodgeGoalCanReservePlayerTile(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 1, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5
	g.rng = testRNGForPredicates(rngValueRange(0, 127))
	actor := &actorInstance{
		kind:        actorKindGuard,
		x:           2.5,
		y:           2.5,
		tileX:       2,
		tileY:       2,
		alive:       true,
		blocking:    true,
		shootable:   true,
		dir:         6,
		facingDir:   6,
		area:        0,
		firstAttack: false,
	}

	g.actorChooseDodgeGoal(actor)
	if !actor.hasGoal {
		t.Fatal("expected dodge goal")
	}
	if actor.dir != 3 || actor.goalX != 1 || actor.goalY != 1 {
		t.Fatalf("dodge dir=%d goal=(%d,%d), want northwest to player tile (1,1)", actor.dir, actor.goalX, actor.goalY)
	}
}

func TestDogCannotOpenDoors(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	dog := &actorInstance{kind: actorKindDog, tileX: 1, tileY: 1}
	guard := &actorInstance{kind: actorKindGuard, tileX: 1, tileY: 1}

	if passable, waitDoor := g.actorCardinalTilePassable(dog, 2, 1); passable || waitDoor {
		t.Fatal("dog unexpectedly treated closed door as passable")
	}
	idx := 1*level.Width + 2
	if g.doorState[idx] != 0 {
		t.Fatalf("dog changed doorState to %d, want 0", g.doorState[idx])
	}
	passable, waitDoor := g.actorCardinalTilePassable(guard, 2, 1)
	if !passable || !waitDoor {
		t.Fatalf("guard closed door result = (%t,%t), want (true,true)", passable, waitDoor)
	}
	if g.doorState[idx] != 1 {
		t.Fatalf("guard doorState = %d, want 1 (opening)", g.doorState[idx])
	}
}

func TestDogCanTraverseOpenDoors(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	idx := 1*level.Width + 2
	g.doorOpen[idx] = 1
	g.doorState[idx] = 2

	dog := &actorInstance{kind: actorKindDog, tileX: 1, tileY: 1}
	passable, waitDoor := g.actorCardinalTilePassable(dog, 2, 1)
	if !passable || waitDoor {
		t.Fatalf("dog open door result = (%t,%t), want (true,false)", passable, waitDoor)
	}
}

func TestActorDiagCheckDoesNotOpenClosedDoor(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	setLevelTile(level, 1, 2, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	actor := &actorInstance{
		kind:     actorKindGuard,
		tileX:    1,
		tileY:    1,
		x:        1.5,
		y:        1.5,
		alive:    true,
		blocking: true,
	}

	if g.actorTryWalk(actor, 1) {
		t.Fatal("diagonal move through closed door unexpectedly allowed")
	}
	idx := 1*level.Width + 2
	if g.doorState[idx] != 0 {
		t.Fatalf("diagonal check changed doorState to %d, want 0", g.doorState[idx])
	}
}

func TestGuardChaseWaitsForDoorGoalToOpen(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0, Door: &wl6.Door{Vertical: true}})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	actor := actorInstance{
		kind:       actorKindGuard,
		x:          1.5,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		area:       0,
		aiState:    actorStateChase,
		chaseSpeed: 0.2,
	}
	g.actors = []actorInstance{actor}

	g.updateActors(1)
	if !g.actors[0].hasGoal || g.actors[0].goalX != 2 || g.actors[0].goalY != 1 {
		t.Fatalf("guard goal = hasGoal %t target (%d,%d), want door tile (2,1)", g.actors[0].hasGoal, g.actors[0].goalX, g.actors[0].goalY)
	}
	if g.actors[0].x != 1.5 || g.actors[0].y != 1.5 {
		t.Fatalf("guard moved to (%.2f, %.2f), want to wait before door fully opens", g.actors[0].x, g.actors[0].y)
	}
	idx := 1*level.Width + 2
	if g.doorState[idx] != 1 {
		t.Fatalf("doorState = %d, want 1 (opening)", g.doorState[idx])
	}

	g.doorOpen[idx] = 0.75
	g.updateActors(1)
	if g.actors[0].x != 1.5 || g.actors[0].y != 1.5 {
		t.Fatalf("guard moved to (%.2f, %.2f), want to keep waiting while the door is not fully open", g.actors[0].x, g.actors[0].y)
	}

	g.doorState[idx] = 2
	g.doorOpen[idx] = 1
	g.updateActors(1)
	if g.actors[0].x <= 1.5 {
		t.Fatalf("guard x = %.2f, want movement once the door is fully open", g.actors[0].x)
	}
}

func TestDoorCollisionShrinksWithOpenness(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{Vertical: true}})

	g := testGameWithLevel(level)
	idx := 1*level.Width + 2

	if !g.doorCollisionAt(2, 1, 1.79-playerRadius, 1.79+playerRadius, 1.5-playerRadius, 1.5+playerRadius) {
		t.Fatal("closed door should still collide at the center slab")
	}

	g.doorState[idx] = 1
	g.doorOpen[idx] = 0.75
	if g.doorCollisionAt(2, 1, 1.79-playerRadius, 1.79+playerRadius, 1.5-playerRadius, 1.5+playerRadius) {
		t.Fatal("partially opened vertical door should stop colliding once the slab retracts enough")
	}
}

func TestCollidesUsesWolfActorBlockingDistance(t *testing.T) {
	level := blankLevel(5, 3)
	g := testGameWithLevel(level)
	g.actors = []actorInstance{{
		x:        2.5,
		y:        1.5,
		alive:    true,
		blocking: true,
	}}

	if !g.collides(1.8, 1.5) {
		t.Fatal("expected actor within Wolf blocking distance to block movement")
	}
}

func TestUseDoorAheadUsesSingleCardinalTile(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	setLevelTile(level, 1, 2, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5
	g.playerA = math.Pi / 4

	g.useDoorAhead()

	eastIdx := 1*level.Width + 2
	southIdx := 2*level.Width + 1
	if g.doorState[eastIdx] != 1 {
		t.Fatalf("east door state = %d, want 1 (opening)", g.doorState[eastIdx])
	}
	if g.doorState[southIdx] != 0 {
		t.Fatalf("south door state = %d, want 0", g.doorState[southIdx])
	}
}

func TestLineBlockedByClosedDoor(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})

	g := testGameWithLevel(level)
	if !g.lineBlocked(1.5, 1.5, 3.5, 1.5) {
		t.Fatal("expected closed door to block line of sight")
	}

	idx := 1*level.Width + 2
	g.doorOpen[idx] = 1
	g.doorState[idx] = 2
	if g.lineBlocked(1.5, 1.5, 3.5, 1.5) {
		t.Fatal("expected fully open door to allow line of sight")
	}
}

func TestDogPatrolAdvancesToArrow(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{RawInfo: 90, Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{RawInfo: 90, Area: 0})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:        actorKindDog,
		x:           1.5,
		y:           1.5,
		tileX:       1,
		tileY:       1,
		goalX:       1,
		goalY:       1,
		alive:       true,
		blocking:    true,
		shootable:   true,
		rotate:      true,
		patrolSeq:   seqActorDogPatrol,
		chaseSeq:    seqActorDogChase,
		jumpSeq:     seqActorDogJump,
		deathSeq:    seqActorDogDeath,
		aiState:     actorStatePatrol,
		spawnMode:   actorSpawnPatrol,
		patrolSpeed: 2,
		area:        0,
	}
	g.actors = []actorInstance{actor}

	g.updateActors(1)
	if g.actors[0].tileX != 3 || g.actors[0].tileY != 1 {
		t.Fatalf("dog patrol moved to (%d,%d), want (3,1)", g.actors[0].tileX, g.actors[0].tileY)
	}
}

func TestGuardPatrolBlockedByWall(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{RawInfo: 90, Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Solid: true})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:        actorKindGuard,
		x:           1.5,
		y:           1.5,
		tileX:       1,
		tileY:       1,
		goalX:       1,
		goalY:       1,
		alive:       true,
		blocking:    true,
		shootable:   true,
		aiState:     actorStatePatrol,
		spawnMode:   actorSpawnPatrol,
		patrolSpeed: 2,
	}
	g.actors = []actorInstance{actor}

	g.updateActors(1)
	if g.actors[0].tileX != 1 || g.actors[0].tileY != 1 {
		t.Fatalf("blocked patrol moved to (%d,%d), want (1,1)", g.actors[0].tileX, g.actors[0].tileY)
	}
}

func TestGuardPatrolContinuesAcrossNonArrowTile(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:        actorKindGuard,
		x:           2.5,
		y:           1.5,
		tileX:       2,
		tileY:       1,
		goalX:       2,
		goalY:       1,
		dir:         0,
		facingDir:   0,
		alive:       true,
		blocking:    true,
		shootable:   true,
		aiState:     actorStatePatrol,
		spawnMode:   actorSpawnPatrol,
		patrolSpeed: 2,
		area:        0,
	}
	g.actors = []actorInstance{actor}

	g.updateActors(1)
	if g.actors[0].tileX != 4 || g.actors[0].tileY != 1 {
		t.Fatalf("continued patrol tile = (%d,%d), want (4,1)", g.actors[0].tileX, g.actors[0].tileY)
	}
	if g.actors[0].x != 4.5 || g.actors[0].y != 1.5 {
		t.Fatalf("continued patrol position = (%.2f,%.2f), want (4.50,1.50)", g.actors[0].x, g.actors[0].y)
	}
	if g.actors[0].hasGoal {
		t.Fatal("continued patrol still has goal after completing step")
	}
}

func TestGuardStandSeesPlayerAndChases(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer == 0 {
		t.Fatal("expected reaction timer to be set")
	}
	for g.actors[0].aiState == actorStateStand {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("guard state = %v, want chase", g.actors[0].aiState)
	}
}

func TestGuardStandUsesGuardAlertSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.actors = []actorInstance{{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	for i := 0; i < 80 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("guard state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound != soundEnemyAlertGuard {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAlertGuard)
	}
}

func TestActorReactionTicsMatchSharewareSourceSplit(t *testing.T) {
	g := &game{rng: testRNGForPredicates(rngValueRange(32, 35))}

	if got := g.actorReactionTics(&actorInstance{kind: actorKindGuard}); got != 9 {
		t.Fatalf("guard reaction = %d, want 9", got)
	}
	g.rng = testRNGForPredicates(rngValueRange(12, 17))
	if got := g.actorReactionTics(&actorInstance{kind: actorKindOfficer}); got != 2 {
		t.Fatalf("officer reaction = %d, want 2", got)
	}
	g.rng = testRNGForPredicates(rngValueRange(12, 17))
	if got := g.actorReactionTics(&actorInstance{kind: actorKindSS}); got != 3 {
		t.Fatalf("ss reaction = %d, want 3", got)
	}
	g.rng = testRNGForPredicates(rngValueRange(192, 199))
	if got := g.actorReactionTics(&actorInstance{kind: actorKindDog}); got != 25 {
		t.Fatalf("dog reaction = %d, want 25", got)
	}
	if got := g.actorReactionTics(&actorInstance{kind: actorKindBoss}); got != 1 {
		t.Fatalf("boss reaction = %d, want 1", got)
	}
}

func TestOfficerStandUsesExactTwoTicReaction(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:      actorKindOfficer,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].aiState != actorStateStand {
		t.Fatalf("officer state after first tic = %v, want stand", g.actors[0].aiState)
	}
	if g.actors[0].reactionTimer != 2 {
		t.Fatalf("officer reactionTimer = %d, want 2 after first tic", g.actors[0].reactionTimer)
	}
	g.updateActors(1)
	if g.actors[0].aiState != actorStateStand {
		t.Fatalf("officer state after second tic = %v, want stand", g.actors[0].aiState)
	}
	if g.actors[0].reactionTimer != 1 {
		t.Fatalf("officer reactionTimer = %d, want 1 after second tic", g.actors[0].reactionTimer)
	}
	g.updateActors(1)
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("officer state after third tic = %v, want chase", g.actors[0].aiState)
	}
}

func TestBossStandWaitsOneTicBeforeChasing(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:      actorKindBoss,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].aiState != actorStateStand {
		t.Fatalf("boss state after first tic = %v, want stand", g.actors[0].aiState)
	}
	if g.actors[0].reactionTimer != 1 {
		t.Fatalf("boss reactionTimer = %d, want 1 after first tic", g.actors[0].reactionTimer)
	}
	g.updateActors(1)
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("boss state after second tic = %v, want chase", g.actors[0].aiState)
	}
}

func TestAmbushGuardNeedsSight(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Solid: true})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.madeNoise = true
	g.actors = []actorInstance{{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		ambush:    true,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer != 0 {
		t.Fatalf("ambush guard reaction timer = %d, want 0", g.actors[0].reactionTimer)
	}
}

func TestNoiseAlertsGuardAcrossConnectedArea(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0, Door: &wl6.Door{Vertical: true}})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 1})
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5
	g.doorState[1*level.Width+2] = 1
	g.playerAreas = g.computePlayerAreas()
	g.rebuildPlayerAreas()
	g.madeNoise = true
	g.actors = []actorInstance{{
		kind:      actorKindGuard,
		x:         3.5,
		y:         1.5,
		tileX:     3,
		tileY:     1,
		goalX:     3,
		goalY:     1,
		dir:       4,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      1,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer == 0 {
		t.Fatal("connected-area guard did not react to noise")
	}
}

func TestActorSetGoalUpdatesAreaToDestination(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 1})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
	}

	if !g.actorSetGoal(&actor, 0) {
		t.Fatal("actorSetGoal failed")
	}
	if actor.tileX != 2 || actor.tileY != 1 {
		t.Fatalf("reserved tile = (%d,%d), want (2,1)", actor.tileX, actor.tileY)
	}
	if actor.area != 1 {
		t.Fatalf("actor area = %d, want 1", actor.area)
	}
}

func TestActorSetDoorGoalPreservesCurrentAreaUntilDoorIsCrossed(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 1})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
	}

	if !g.actorSetGoal(&actor, 0) {
		t.Fatal("actorSetGoal failed for door goal")
	}
	if actor.tileX != 2 || actor.tileY != 1 {
		t.Fatalf("door reserved tile = (%d,%d), want (2,1)", actor.tileX, actor.tileY)
	}
	if actor.area != 0 {
		t.Fatalf("door-goal actor area = %d, want 0 until the door is crossed", actor.area)
	}
}

func TestActorSetGoalReservesDestinationTileImmediately(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
	}

	if !g.actorSetGoal(&actor, 0) {
		t.Fatal("actorSetGoal failed")
	}
	if !actor.hasGoal || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal = hasGoal %t target (%d,%d), want (2,1,true)", actor.hasGoal, actor.goalX, actor.goalY)
	}
	if actor.tileX != 2 || actor.tileY != 1 {
		t.Fatalf("reserved tile = (%d,%d), want (2,1)", actor.tileX, actor.tileY)
	}
	if actor.x != 1.5 || actor.y != 1.5 {
		t.Fatalf("world position = (%.2f,%.2f), want unchanged at (1.50,1.50)", actor.x, actor.y)
	}
}

func TestActorReservedTileBlocksOtherActors(t *testing.T) {
	level := blankLevel(6, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	moving := actorInstance{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
	}
	if !g.actorSetGoal(&moving, 0) {
		t.Fatal("moving actorSetGoal failed")
	}
	g.actors = []actorInstance{
		moving,
		{
			kind:      actorKindGuard,
			x:         3.5,
			y:         1.5,
			tileX:     3,
			tileY:     1,
			alive:     true,
			blocking:  true,
			shootable: true,
		},
	}

	if g.actorGoalTileClear(&g.actors[1], 2, 1) {
		t.Fatal("reserved destination tile should block other actors immediately")
	}
}

func TestClosedDoorWaitOccupiesDoorTileLikeWolfReservation(t *testing.T) {
	level := blankLevel(6, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	g := testGameWithLevel(level)
	g.actors = []actorInstance{
		{
			kind:         actorKindGuard,
			x:            1.5,
			y:            1.5,
			tileX:        2,
			tileY:        1,
			goalX:        2,
			goalY:        1,
			hasGoal:      true,
			moveDistance: actorDoorWaitDistance,
			alive:        true,
			blocking:     true,
			shootable:    true,
		},
		{
			kind:      actorKindGuard,
			x:         3.5,
			y:         1.5,
			tileX:     3,
			tileY:     1,
			alive:     true,
			blocking:  true,
			shootable: true,
		},
	}

	if g.actorGoalTileClear(&g.actors[1], 2, 1) {
		t.Fatal("closed door wait should reserve the door tile like Wolf's actorat occupancy")
	}
}

func TestActorTryWalkBlocksOnClosedDoorWaiter(t *testing.T) {
	level := blankLevel(6, 3)
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.actors = []actorInstance{
		{
			kind:         actorKindGuard,
			x:            1.5,
			y:            1.5,
			tileX:        2,
			tileY:        1,
			goalX:        2,
			goalY:        1,
			hasGoal:      true,
			moveDistance: actorDoorWaitDistance,
			alive:        true,
			blocking:     true,
			shootable:    true,
		},
		{
			kind:      actorKindGuard,
			x:         3.5,
			y:         1.5,
			tileX:     3,
			tileY:     1,
			dir:       4,
			alive:     true,
			blocking:  true,
			shootable: true,
		},
	}

	if g.actorTryWalk(&g.actors[1], 4) {
		t.Fatal("second actor should not be able to reserve a closed door tile already occupied by a waiting actor")
	}
}

func TestDoorGoalArrivalPreservesCurrentAreaUntilNextTile(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 1})
	g := testGameWithLevel(level)
	idx := 1*level.Width + 2
	g.doorState[idx] = 2
	g.doorOpen[idx] = 1
	actor := actorInstance{
		kind:         actorKindGuard,
		x:            1.5,
		y:            1.5,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		area:         0,
		moveDistance: 1,
	}

	consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(&actor, 1)
	if blocked || !reachedGoal || consumed != 1 {
		t.Fatalf("door step = consumed %.2f reached %t blocked %t, want 1/true/false", consumed, reachedGoal, blocked)
	}
	if actor.tileX != 2 || actor.tileY != 1 {
		t.Fatalf("door arrival tile = (%d,%d), want (2,1)", actor.tileX, actor.tileY)
	}
	if actor.area != 0 {
		t.Fatalf("door arrival area = %d, want 0 until the next non-door walk", actor.area)
	}
}

func TestMoveActorTowardGoalUsesWolfDiagonalAxisMovement(t *testing.T) {
	level := blankLevel(4, 4)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     2,
		tileY:     2,
		goalX:     2,
		goalY:     2,
		hasGoal:   true,
		dir:       7,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
	}

	g.moveActorTowardGoal(&actor, 0.2)
	if actor.x != 1.7 || actor.y != 1.7 {
		t.Fatalf("diagonal move = (%.2f, %.2f), want (1.70, 1.70)", actor.x, actor.y)
	}
}

func TestMoveActorTowardGoalStepResyncsStaleDistance(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	actor := actorInstance{
		kind:         actorKindGuard,
		x:            1.01,
		y:            1.5,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		area:         0,
		aiState:      actorStateChase,
		patrolSpeed:  0.1,
		chaseSpeed:   0.1,
		moveDistance: 0.01,
	}

	consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(&actor, 0.1)
	if blocked || reachedGoal {
		t.Fatalf("step blocked=%t reached=%t, want false/false", blocked, reachedGoal)
	}
	if math.Abs(consumed-0.1) > 1e-9 {
		t.Fatalf("consumed = %.4f, want 0.1", consumed)
	}
	if math.Abs(actor.x-1.11) > 1e-9 || math.Abs(actor.y-1.5) > 1e-9 {
		t.Fatalf("actor moved to (%.4f,%.4f), want (1.11,1.50)", actor.x, actor.y)
	}
	if math.Abs(actor.moveDistance-1.39) > 1e-9 {
		t.Fatalf("moveDistance = %.4f after resync, want 1.39", actor.moveDistance)
	}
}

func TestMoveActorTowardGoalStepAllowsWolfStyleOffCenterRouting(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 0, wl6.Tile{Solid: true, RenderWall: true, RawWall: 1})
	g := testGameWithLevel(level)
	actor := actorInstance{
		kind:         actorKindGuard,
		x:            1.8,
		y:            1.2,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		area:         0,
		aiState:      actorStateChase,
		patrolSpeed:  0.1,
		chaseSpeed:   0.1,
		moveDistance: 0.7,
	}

	consumed, reachedGoal, blocked := g.moveActorTowardGoalStep(&actor, 0.1)
	if blocked || reachedGoal || consumed != 0.1 {
		t.Fatalf("step blocked=%t reached=%t consumed=%.2f, want false/false/0.1", blocked, reachedGoal, consumed)
	}
	if actor.x <= 1.8 || actor.y != 1.2 {
		t.Fatalf("actor moved to (%.3f, %.3f), want continued progress through the valid goal tile", actor.x, actor.y)
	}
}

func TestSavedActorPreservesMoveDistanceAcrossRoundTrip(t *testing.T) {
	level := blankLevel(16, 48)
	setLevelTile(level, 10, 37, wl6.Tile{Area: 0})
	setLevelTile(level, 10, 38, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 12.5
	g.playerY = 40.5
	g.actors = []actorInstance{{
		kind:         actorKindGuard,
		x:            10.5,
		y:            37.203125,
		tileX:        10,
		tileY:        38,
		goalX:        10,
		goalY:        38,
		hasGoal:      true,
		dir:          6,
		facingDir:    6,
		alive:        true,
		blocking:     true,
		shootable:    true,
		alerted:      true,
		area:         0,
		aiState:      actorStateChase,
		patrolSpeed:  7.0 / 256.0,
		chaseSpeed:   7.0 / 256.0,
		moveDistance: 75.0 / 256.0,
	}}

	var w saveBinaryWriter
	w.writeActor(saveActorFromGame(g.actors[0]))
	if w.err != nil {
		t.Fatalf("writeActor: %v", w.err)
	}
	r := &saveBinaryReader{data: w.buf.Bytes()}
	restored := actorFromSave(r.readActor())
	if r.err != nil {
		t.Fatalf("readActor: %v", r.err)
	}
	if math.Abs(restored.moveDistance-g.actors[0].moveDistance) > 1e-9 {
		t.Fatalf("moveDistance = %.6f after round-trip, want %.6f", restored.moveDistance, g.actors[0].moveDistance)
	}

	g.actors[0] = restored
	g.updateActors(1)
}

func TestDogFirstSightingPreservesMidTilePatrolGoal(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	actor := &actorInstance{
		kind:         actorKindDog,
		x:            1.81,
		y:            1.5,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		facingDir:    0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		area:         0,
		aiState:      actorStatePatrol,
		moveDistance: 0.19,
	}

	g.dogFirstSighting(actor)

	if actor.aiState != actorStateChase {
		t.Fatalf("aiState = %v, want chase", actor.aiState)
	}
	if !actor.hasGoal {
		t.Fatal("expected current patrol goal to be preserved on first sighting")
	}
	if actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal = (%d,%d), want (2,1)", actor.goalX, actor.goalY)
	}
	if math.Abs(actor.moveDistance-0.19) > 1e-9 {
		t.Fatalf("moveDistance = %.4f, want 0.19", actor.moveDistance)
	}
}

func TestUpdateActorsMovementScalesWithTics(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})

	newGame := func() *game {
		g := testGameWithLevel(level)
		g.playerX = 4.5
		g.playerY = 1.5
		g.actors = []actorInstance{{
			kind:        actorKindGuard,
			x:           1.5,
			y:           1.5,
			tileX:       2,
			tileY:       1,
			goalX:       2,
			goalY:       1,
			hasGoal:     true,
			dir:         0,
			alive:       true,
			blocking:    true,
			shootable:   true,
			alerted:     false,
			area:        0,
			aiState:     actorStatePatrol,
			patrolSpeed: 0.1,
		}}
		return g
	}

	g1 := newGame()
	g1.updateActors(1)
	if g1.actors[0].x != 1.6 {
		t.Fatalf("1-tic move x = %.2f, want 1.60", g1.actors[0].x)
	}

	g4 := newGame()
	g4.updateActors(4)
	if g4.actors[0].x != 1.9 {
		t.Fatalf("4-tic move x = %.2f, want 1.90", g4.actors[0].x)
	}
}

func TestPatrolCanCrossMultipleTilesInOneUpdate(t *testing.T) {
	level := blankLevel(6, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0, RawInfo: 90})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0, RawInfo: 90})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 5.5
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:        actorKindGuard,
		x:           1.5,
		y:           1.5,
		tileX:       1,
		tileY:       1,
		goalX:       1,
		goalY:       1,
		dir:         0,
		alive:       true,
		blocking:    true,
		shootable:   true,
		area:        0,
		aiState:     actorStatePatrol,
		patrolSpeed: 1,
	}}

	g.updateActors(2)
	if g.actors[0].tileX != 3 || g.actors[0].tileY != 1 {
		t.Fatalf("patrol tile = (%d,%d), want (3,1)", g.actors[0].tileX, g.actors[0].tileY)
	}
	if g.actors[0].x != 3.5 || g.actors[0].y != 1.5 {
		t.Fatalf("patrol pos = (%.2f, %.2f), want (3.50, 1.50)", g.actors[0].x, g.actors[0].y)
	}
}

func TestGuardStandDoesNotNoticePlayerBehindWithoutNoise(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		dir:       4,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer != 0 {
		t.Fatalf("rear-facing guard reaction timer = %d, want 0", g.actors[0].reactionTimer)
	}
	if g.actors[0].aiState != actorStateStand {
		t.Fatalf("rear-facing guard state = %v, want stand", g.actors[0].aiState)
	}
}

func TestGuardStandNoticesPlayerAtCloseRangeEvenBehind(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.4
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		dir:       4,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer == 0 {
		t.Fatal("close-range guard did not start reacting")
	}
}

func TestLineBlockedAllowsSightThroughBarelyOpenDoorLikeWolf(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	idx := 1*level.Width + 2
	g.doorState[idx] = 1
	g.doorOpen[idx] = 0.01

	if g.lineBlocked(1.5, 1.5, 3.5, 1.5) {
		t.Fatal("expected barely open door to allow sight like WOLFSRC CheckLine")
	}
}

func TestDogStandSeesPlayerAndChases(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.actors = []actorInstance{{
		kind:      actorKindDog,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		rotate:    true,
		area:      0,
		chaseSeq:  seqActorDogChase,
		jumpSeq:   seqActorDogJump,
		deathSeq:  seqActorDogDeath,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer == 0 {
		t.Fatal("expected dog reaction timer to be set")
	}
	for i := 0; i < 50 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("dog state = %v, want chase", g.actors[0].aiState)
	}
}

func TestDogStandUsesDogAlertSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.actors = []actorInstance{{
		kind:      actorKindDog,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		rotate:    true,
		area:      0,
		chaseSeq:  seqActorDogChase,
		jumpSeq:   seqActorDogJump,
		deathSeq:  seqActorDogDeath,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	for i := 0; i < 80 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("dog state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound != soundEnemyAlertDog {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAlertDog)
	}
}

func TestMutantStandChasesWithoutAlertSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.lastPlayedSound = soundHitEnemy
	g.actors = []actorInstance{{
		kind:      actorKindMutant,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		rotate:    true,
		area:      0,
		chaseSeq:  seqActorMutantChase,
		painSeq:   seqActorMutantPain,
		shootSeq:  seqActorMutantShoot,
		deathSeq:  seqActorMutantDeath,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	for i := 0; i < 80 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("mutant state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound != soundHitEnemy {
		t.Fatalf("lastPlayedSound = %v, want no mutant alert sound replay", g.lastPlayedSound)
	}
}

func TestDogChaseCanStartJumpWithoutCurrentSight(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Solid: true})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.0
	g.playerY = 1.5
	actor := actorInstance{
		kind:       actorKindDog,
		x:          1.82,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		area:       0,
		aiState:    actorStateChase,
		chaseSpeed: 0.05,
	}

	if !g.dogCanStartJump(&actor, 20) {
		t.Fatal("expected chase dog to enter jump range without re-checking sight")
	}
}

func TestDogChaseCanStartJumpWithoutConnectedArea(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 1})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.0
	g.playerY = 1.5
	g.playerAreas = []bool{true, false}
	actor := actorInstance{
		kind:       actorKindDog,
		x:          1.82,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		area:       1,
		aiState:    actorStateChase,
		chaseSpeed: 0.05,
	}

	if !g.dogCanStartJump(&actor, 20) {
		t.Fatal("expected chase dog to enter jump range without connected-area gating")
	}
}

func TestOfficerStandUsesOfficerAlertSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.actors = []actorInstance{{
		kind:      actorKindOfficer,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	for i := 0; i < 80 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("officer state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound != soundEnemyAlertOfficer {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAlertOfficer)
	}
}

func TestGuardShootDamagesPlayer(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.health = 100
	g.playerMovingFast = false
	g.rng = testRNG(1)
	actor := actorInstance{
		kind:       actorKindGuard,
		x:          1.5,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		aiState:    actorStateShoot,
		sequenceID: seqActorGuardShoot,
	}
	g.startActorSequence(&actor, seqActorGuardShoot, false)
	g.actors = []actorInstance{actor}

	for i := 0; i < 50 && g.health == 100; i++ {
		g.updateActors(1)
	}
	if g.health >= 100 {
		t.Fatal("expected guard shot to reduce player health")
	}
}

func TestGuardShootStillPlaysSoundOnMiss(t *testing.T) {
	level := blankLevel(10, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 8, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 8.5
	g.playerY = 1.5
	g.health = 100
	g.playerMovingFast = false
	g.rng = testRNGForPredicates(rngValueAtLeast(200))
	actor := actorInstance{
		kind:       actorKindGuard,
		x:          1.5,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		aiState:    actorStateShoot,
		sequenceID: seqActorGuardShoot,
	}

	g.guardTryShoot(&actor)
	if g.health != 100 {
		t.Fatalf("health = %d, want 100 on miss", g.health)
	}
	if g.lastPlayedSound != soundEnemyAttackGuard {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAttackGuard)
	}
}

func TestSharewareRangedShootersPlayExpectedSoundOnMiss(t *testing.T) {
	tests := []struct {
		name    string
		kind    ActorKind
		sound   soundID
		rngSeed func() *wolfRNG
	}{
		{name: "guard", kind: actorKindGuard, sound: soundEnemyAttackGuard, rngSeed: func() *wolfRNG { return testRNGForPredicates(rngValueAtLeast(200)) }},
		{name: "officer", kind: actorKindOfficer, sound: soundEnemyAttackGuard, rngSeed: func() *wolfRNG { return testRNGForPredicates(rngValueAtLeast(200)) }},
		{name: "ss", kind: actorKindSS, sound: soundEnemyAttackSS, rngSeed: func() *wolfRNG { return testRNGForPredicates(rngValueAtLeast(224)) }},
		{name: "boss", kind: actorKindBoss, sound: soundEnemyAttackBoss, rngSeed: func() *wolfRNG { return testRNGForPredicates(rngValueAtLeast(224)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := blankLevel(10, 3)
			setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
			setLevelTile(level, 8, 1, wl6.Tile{Area: 0})
			g := testGameWithLevel(level)
			g.playerX = 8.5
			g.playerY = 1.5
			g.health = 100
			g.playerMovingFast = false
			g.rng = tt.rngSeed()
			actor := actorInstance{
				kind:      tt.kind,
				x:         1.5,
				y:         1.5,
				tileX:     1,
				tileY:     1,
				alive:     true,
				blocking:  true,
				shootable: true,
				area:      0,
				alerted:   true,
			}

			g.guardTryShoot(&actor)
			if g.health != 100 {
				t.Fatalf("health = %d, want 100 on miss", g.health)
			}
			if g.lastPlayedSound != tt.sound {
				t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, tt.sound)
			}
		})
	}
}

func TestGuardShootUsesVisibilityAdjustedHitchance(t *testing.T) {
	level := blankLevel(10, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 8, 1, wl6.Tile{Area: 0})

	newActor := func() actorInstance {
		return actorInstance{
			kind:      actorKindGuard,
			x:         1.5,
			y:         1.5,
			tileX:     1,
			tileY:     1,
			alive:     true,
			blocking:  true,
			shootable: true,
			area:      0,
			alerted:   true,
		}
	}

	gVisible := testGameWithLevel(level)
	gVisible.playerX = 8.5
	gVisible.playerY = 1.5
	gVisible.playerA = math.Pi
	gVisible.health = 100
	gVisible.playerMovingFast = false
	gVisible.rng = testRNGForPredicates(rngValueAtLeast(144))
	visibleActor := newActor()
	gVisible.guardTryShoot(&visibleActor)
	if gVisible.health != 100 {
		t.Fatalf("visible shot health = %d, want 100 on miss", gVisible.health)
	}

	gHidden := testGameWithLevel(level)
	gHidden.playerX = 8.5
	gHidden.playerY = 1.5
	gHidden.playerA = 0
	gHidden.health = 100
	gHidden.playerMovingFast = false
	gHidden.rng = testRNGForPredicates(rngValueLessThan(200), rngValueNonZeroDamage)
	hiddenActor := newActor()
	gHidden.guardTryShoot(&hiddenActor)
	if gHidden.health >= 100 {
		t.Fatal("expected hidden shooter to hit with looser hitchance branch")
	}
}

func TestGuardStartShootUsesRemainingTileDistanceForPointBlankChance(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 1.5

	pointBlank := actorInstance{
		kind:         actorKindGuard,
		x:            1.5,
		y:            1.5,
		tileX:        1,
		tileY:        1,
		alive:        true,
		blocking:     true,
		shootable:    true,
		alerted:      true,
		aiState:      actorStateChase,
		shootSeq:     seqActorGuardShoot,
		moveDistance: 0.2,
	}
	g.rng = testRNGForPredicates(rngValueRange(100, 103))
	if !g.guardTryStartShoot(&pointBlank, 1) {
		t.Fatal("expected point-blank shooter to enter shoot state")
	}

	initialPointBlank := actorInstance{
		kind:      actorKindGuard,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		alerted:   true,
		aiState:   actorStateChase,
		shootSeq:  seqActorGuardShoot,
	}
	g.rng = testRNGForPredicates(rngValueRange(100, 103))
	if !g.guardTryStartShoot(&initialPointBlank, 1) {
		t.Fatal("expected newly alerted adjacent shooter to use point-blank chance")
	}

	notPointBlank := actorInstance{
		kind:         actorKindGuard,
		x:            1.5,
		y:            1.5,
		tileX:        1,
		tileY:        1,
		alive:        true,
		blocking:     true,
		shootable:    true,
		alerted:      true,
		aiState:      actorStateChase,
		shootSeq:     seqActorGuardShoot,
		moveDistance: 0.3,
	}
	g.rng = testRNGForPredicates(rngValueRange(100, 103))
	if g.guardTryStartShoot(&notPointBlank, 1) {
		t.Fatal("expected non-point-blank shooter to stay in chase state")
	}
}

func TestGuardStartShootPreservesReservedGoalLikeWolf(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 1.5
	g.rng = testRNGForPredicates(rngValueRange(100, 103))
	actor := actorInstance{
		kind:         actorKindGuard,
		x:            1.7,
		y:            1.5,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		alerted:      true,
		aiState:      actorStateChase,
		shootSeq:     seqActorGuardShoot,
		moveDistance: 0.2,
	}

	if !g.guardTryStartShoot(&actor, 1) {
		t.Fatal("expected shooter to enter shoot state")
	}
	if !actor.hasGoal || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal after shoot entry = hasGoal %t target (%d,%d), want true (2,1)", actor.hasGoal, actor.goalX, actor.goalY)
	}
	if math.Abs(actor.moveDistance-0.2) > 1e-9 {
		t.Fatalf("moveDistance after shoot entry = %.4f, want 0.2", actor.moveDistance)
	}
}

func TestDogJumpStartPreservesReservedGoalLikeWolf(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	actor := actorInstance{
		kind:         actorKindDog,
		x:            1.45,
		y:            1.5,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		alerted:      true,
		area:         0,
		aiState:      actorStateChase,
		jumpSeq:      seqActorDogJump,
		moveDistance: 0.2,
	}

	g.startDogJump(&actor)
	if actor.aiState != actorStateJump {
		t.Fatalf("dog state = %v, want jump", actor.aiState)
	}
	if !actor.hasGoal || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal after jump entry = hasGoal %t target (%d,%d), want true (2,1)", actor.hasGoal, actor.goalX, actor.goalY)
	}
	if math.Abs(actor.moveDistance-0.2) > 1e-9 {
		t.Fatalf("moveDistance after jump entry = %.4f, want 0.2", actor.moveDistance)
	}
}

func TestFirstSightingKeepsReservedGoalAndCancelsDoorWaitLikeWolf(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{}})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	actor := actorInstance{
		kind:         actorKindGuard,
		x:            1.5,
		y:            1.5,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		dir:          0,
		alive:        true,
		blocking:     true,
		shootable:    true,
		area:         0,
		chaseSeq:     seqActorGuardChase,
		moveDistance: actorDoorWaitDistance,
	}

	g.guardFirstSighting(&actor)
	if actor.aiState != actorStateChase {
		t.Fatalf("state after first sighting = %v, want chase", actor.aiState)
	}
	if !actor.hasGoal || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal after first sighting = hasGoal %t target (%d,%d), want true (2,1)", actor.hasGoal, actor.goalX, actor.goalY)
	}
	if actor.moveDistance != 0 {
		t.Fatalf("moveDistance after first sighting = %.4f, want 0", actor.moveDistance)
	}
}

func TestGuardResumeChasePreservesReservedGoalLikeWolf(t *testing.T) {
	g := &game{}
	actor := actorInstance{
		kind:         actorKindGuard,
		alive:        true,
		alerted:      true,
		rotate:       false,
		aiState:      actorStateShoot,
		tileX:        2,
		tileY:        1,
		goalX:        2,
		goalY:        1,
		hasGoal:      true,
		moveDistance: 0.3,
		chaseSeq:     seqActorGuardChase,
		shootSeq:     seqActorGuardShoot,
	}

	g.guardResumeChase(&actor)
	if actor.aiState != actorStateChase {
		t.Fatalf("state after resume = %v, want chase", actor.aiState)
	}
	if !actor.hasGoal || actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal after resume = hasGoal %t target (%d,%d), want true (2,1)", actor.hasGoal, actor.goalX, actor.goalY)
	}
	if math.Abs(actor.moveDistance-0.3) > 1e-9 {
		t.Fatalf("moveDistance after resume = %.4f, want 0.3", actor.moveDistance)
	}
}

func TestSSStandSeesPlayerAndChases(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.actors = []actorInstance{{
		kind:      actorKindSS,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		rotate:    true,
		area:      0,
		chaseSeq:  seqActorSSChase,
		painSeq:   seqActorSSPain,
		shootSeq:  seqActorSSShoot,
		deathSeq:  seqActorSSDeath,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	if g.actors[0].reactionTimer == 0 {
		t.Fatal("expected ss reaction timer to be set")
	}
	for i := 0; i < 80 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("ss state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound != soundEnemyAlertSS {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAlertSS)
	}
}

func TestSSShootUsesSSAttackSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.health = 100
	g.playerMovingFast = false
	g.rng = testRNG(1)
	actor := actorInstance{
		kind:       actorKindSS,
		x:          1.5,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		aiState:    actorStateShoot,
		sequenceID: seqActorSSShoot,
	}
	g.startActorSequence(&actor, seqActorSSShoot, false)
	g.actors = []actorInstance{actor}

	for i := 0; i < 50 && g.lastPlayedSound != soundEnemyAttackSS; i++ {
		g.updateActors(1)
	}
	if g.lastPlayedSound != soundEnemyAttackSS {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAttackSS)
	}
}

func TestBossStandUsesBossAlertSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.rng = testRNG(1)
	g.actors = []actorInstance{{
		kind:      actorKindBoss,
		x:         1.5,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		goalX:     1,
		goalY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      0,
		chaseSeq:  seqActorBossChase,
		shootSeq:  seqActorBossShoot,
		deathSeq:  seqActorBossDeath,
		aiState:   actorStateStand,
	}}

	g.updateActors(1)
	for i := 0; i < 80 && g.actors[0].aiState == actorStateStand; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("boss state = %v, want chase", g.actors[0].aiState)
	}
	if g.actors[0].rotate {
		t.Fatal("boss should remain non-rotating after first sighting")
	}
	if g.lastPlayedSound != soundEnemyAlertBoss {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAlertBoss)
	}
}

func TestBossShootUsesBossAttackSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.health = 100
	g.playerMovingFast = false
	g.rng = testRNG(1)
	actor := actorInstance{
		kind:       actorKindBoss,
		x:          1.5,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		aiState:    actorStateShoot,
		sequenceID: seqActorBossShoot,
	}
	g.startActorSequence(&actor, seqActorBossShoot, false)
	g.actors = []actorInstance{actor}

	for i := 0; i < 80 && g.lastPlayedSound != soundEnemyAttackBoss; i++ {
		g.updateActors(1)
	}
	if g.lastPlayedSound != soundEnemyAttackBoss {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAttackBoss)
	}
}

func TestBossShootFinishesBackInNonRotatingChase(t *testing.T) {
	g := &game{}
	actor := actorInstance{
		kind:     actorKindBoss,
		alive:    true,
		alerted:  true,
		aiState:  actorStateShoot,
		chaseSeq: seqActorBossChase,
		shootSeq: seqActorBossShoot,
		rotate:   false,
	}
	g.startActorSequence(&actor, seqActorBossShoot, false)
	g.actors = []actorInstance{actor}

	for i := 0; i < 120 && g.actors[0].aiState != actorStateChase; i++ {
		g.updateActors(1)
	}

	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("boss state = %v, want chase", g.actors[0].aiState)
	}
	if g.actors[0].rotate {
		t.Fatal("boss should remain non-rotating after shoot")
	}
	if g.actors[0].shapenum != shapeBossBase {
		t.Fatalf("boss chase shape = %d, want %d", g.actors[0].shapenum, shapeBossBase)
	}
}

func TestMutantShootUsesDefaultAttackSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 3, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 3.5
	g.playerY = 1.5
	g.health = 100
	g.playerMovingFast = false
	g.rng = testRNG(1)
	actor := actorInstance{
		kind:       actorKindMutant,
		x:          1.5,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		aiState:    actorStateShoot,
		sequenceID: seqActorMutantShoot,
	}
	g.startActorSequence(&actor, seqActorMutantShoot, false)
	g.actors = []actorInstance{actor}

	for i := 0; i < 80 && g.lastPlayedSound != soundEnemyAttackGuard; i++ {
		g.updateActors(1)
	}
	if g.lastPlayedSound != soundEnemyAttackGuard {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAttackGuard)
	}
}

func TestDogChaseStartsJump(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	g.actors = []actorInstance{{
		kind:       actorKindDog,
		x:          1.45,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		rotate:     true,
		area:       0,
		alerted:    true,
		chaseSeq:   seqActorDogChase,
		jumpSeq:    seqActorDogJump,
		deathSeq:   seqActorDogDeath,
		aiState:    actorStateChase,
		chaseSpeed: 0.05,
	}}

	g.updateActors(1)
	if g.actors[0].aiState != actorStateJump {
		t.Fatalf("dog state = %v, want jump", g.actors[0].aiState)
	}
	if g.actors[0].shapenum != shapeDogJump1 || g.actors[0].rotate {
		t.Fatalf("dog jump state = shape %d rotate %t, want %d false", g.actors[0].shapenum, g.actors[0].rotate, shapeDogJump1)
	}
}

func TestDogChaseStartsJumpAtPlayerBlockDistance(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	actor := actorInstance{
		kind:       actorKindDog,
		x:          g.playerX - 0.8,
		y:          g.playerY,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		rotate:     true,
		area:       0,
		alerted:    true,
		chaseSeq:   seqActorDogChase,
		jumpSeq:    seqActorDogJump,
		deathSeq:   seqActorDogDeath,
		aiState:    actorStateChase,
		chaseSpeed: 0.05,
	}
	if math.Abs(g.playerX-actor.x) >= playerBlockDist {
		t.Fatalf("test setup distance = %f, want inside playerBlockDist %f", math.Abs(g.playerX-actor.x), playerBlockDist)
	}
	g.actors = []actorInstance{actor}

	g.updateActors(1)
	if g.actors[0].aiState != actorStateJump {
		t.Fatalf("dog state = %v, want jump", g.actors[0].aiState)
	}
	if g.actors[0].shapenum != shapeDogJump1 || g.actors[0].rotate {
		t.Fatalf("dog jump state = shape %d rotate %t, want %d false", g.actors[0].shapenum, g.actors[0].rotate, shapeDogJump1)
	}
}

func TestDogChaseStartsJumpWithTicScaledReach(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	actor := actorInstance{
		kind:       actorKindDog,
		x:          g.playerX - 1.18,
		y:          g.playerY,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		rotate:     true,
		area:       0,
		alerted:    true,
		chaseSeq:   seqActorDogChase,
		jumpSeq:    seqActorDogJump,
		deathSeq:   seqActorDogDeath,
		aiState:    actorStateChase,
		chaseSpeed: 0.05,
	}
	g.actors = []actorInstance{actor}

	g.updateActors(4)
	if g.actors[0].aiState != actorStateJump {
		t.Fatalf("dog state = %v, want jump", g.actors[0].aiState)
	}
}

func TestDogJumpBiteDamagesPlayer(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	g.health = 100
	g.rng = testRNGForPredicates(rngValueLessThan(180), rngValueNonZeroDamage)
	actor := actorInstance{
		kind:       actorKindDog,
		x:          1.45,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		goalX:      1,
		goalY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		chaseSeq:   seqActorDogChase,
		jumpSeq:    seqActorDogJump,
		deathSeq:   seqActorDogDeath,
		aiState:    actorStateJump,
		chaseSpeed: 0.05,
	}
	g.startActorSequence(&actor, seqActorDogJump, false)
	g.actors = []actorInstance{actor}

	for i := 0; i < 10+1; i++ {
		g.updateActors(1)
	}
	if g.health >= 100 {
		t.Fatal("expected dog bite to reduce player health")
	}
	for i := 0; i < 40+2 && g.actors[0].aiState != actorStateChase; i++ {
		g.updateActors(1)
	}
	if g.actors[0].aiState != actorStateChase || g.actors[0].shapenum != shapeDogBase || !g.actors[0].rotate {
		t.Fatalf("dog post-jump state = %+v, want chase on walk base with rotation", g.actors[0])
	}
}

func TestDogJumpBiteStillPlaysSoundOnMiss(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	g.health = 100
	g.rng = testRNGForPredicates(rngValueAtLeast(180))
	actor := actorInstance{
		kind:       actorKindDog,
		x:          1.45,
		y:          1.5,
		tileX:      1,
		tileY:      1,
		alive:      true,
		blocking:   true,
		shootable:  true,
		area:       0,
		alerted:    true,
		chaseSeq:   seqActorDogChase,
		jumpSeq:    seqActorDogJump,
		deathSeq:   seqActorDogDeath,
		aiState:    actorStateJump,
		chaseSpeed: 0.05,
	}

	g.dogTryBite(&actor)
	if g.health != 100 {
		t.Fatalf("health = %d, want 100 on miss", g.health)
	}
	if g.lastPlayedSound != soundEnemyAttackDog {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyAttackDog)
	}
}

func TestDogJumpBiteDoesNotRequireSightOrAreaConnection(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 1, 2, wl6.Tile{RawWall: 98, Solid: true})
	g := testGameWithLevel(level)
	g.playerX = 2.0
	g.playerY = 1.5
	g.health = 100
	g.rng = testRNGForPredicates(rngValueLessThan(180), rngValueNonZeroDamage)
	actor := actorInstance{
		kind:      actorKindDog,
		x:         1.45,
		y:         1.5,
		tileX:     1,
		tileY:     1,
		alive:     true,
		blocking:  true,
		shootable: true,
		area:      -1,
		alerted:   true,
		aiState:   actorStateJump,
	}

	g.dogTryBite(&actor)
	if g.health >= 100 {
		t.Fatal("expected dog bite to reduce player health")
	}
}

func TestGuardPainFinishesWithoutReplayingAlert(t *testing.T) {
	g := &game{}
	actor := actorInstance{
		kind:     actorKindGuard,
		alive:    true,
		alerted:  true,
		rotate:   false,
		aiState:  actorStatePain,
		chaseSeq: seqActorGuardChase,
		painSeq:  seqActorGuardPain,
	}
	g.startActorSequence(&actor, seqActorGuardPain, false)
	g.lastPlayedSound = soundHitEnemy
	g.actors = []actorInstance{actor}

	for i := 0; i < 50 && g.actors[0].aiState != actorStateChase; i++ {
		g.updateActors(1)
	}

	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("guard state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound != soundHitEnemy {
		t.Fatalf("last sound = %v, want unchanged %v", g.lastPlayedSound, soundHitEnemy)
	}
}

func TestDogJumpFinishesWithoutReplayingAlert(t *testing.T) {
	g := &game{}
	actor := actorInstance{
		kind:     actorKindDog,
		alive:    true,
		alerted:  true,
		rotate:   false,
		aiState:  actorStateJump,
		chaseSeq: seqActorDogChase,
		jumpSeq:  seqActorDogJump,
	}
	g.startActorSequence(&actor, seqActorDogJump, false)
	g.lastPlayedSound = soundHitEnemy
	g.actors = []actorInstance{actor}

	for i := 0; i < 80 && g.actors[0].aiState != actorStateChase; i++ {
		g.updateActors(1)
	}

	if g.actors[0].aiState != actorStateChase {
		t.Fatalf("dog state = %v, want chase", g.actors[0].aiState)
	}
	if g.lastPlayedSound == soundEnemyAlertDog {
		t.Fatalf("last sound = %v, did not expect alert replay", g.lastPlayedSound)
	}
}

func TestDamageActorTransitions(t *testing.T) {
	g := &game{rng: testRNGForPredicates(rngModulo(8, 1))}
	actor := actorInstance{
		kind:       actorKindGuard,
		alive:      true,
		blocking:   true,
		shootable:  true,
		health:     25,
		scoreValue: 100,
		dropPickup: pickupClip2,
	}

	g.damageActor(&actor, 5)
	if actor.health != 15 {
		t.Fatalf("actor health = %d, want 15", actor.health)
	}
	if actor.aiState != actorStatePain && actor.aiState != actorStateChase {
		t.Fatalf("actor state after hit = %v, want pain/chase", actor.aiState)
	}
	if !actor.alerted {
		t.Fatal("actor should become alerted after taking damage")
	}
}

func TestDamageActorChoosesHumanoidPainFrameByRemainingHealthParity(t *testing.T) {
	tests := []struct {
		name      string
		kind      ActorKind
		health    int
		damage    int
		wantSeq   AnimSequenceID
		wantShape int
	}{
		{name: "guard odd", kind: actorKindGuard, health: 25, damage: 4, wantSeq: seqActorGuardPain, wantShape: shapeGuardPain1},
		{name: "guard even", kind: actorKindGuard, health: 25, damage: 5, wantSeq: seqActorGuardPain2, wantShape: shapeGuardPain2},
		{name: "officer odd", kind: actorKindOfficer, health: 50, damage: 3, wantSeq: seqActorOfficerPain, wantShape: shapeOfficerPain1},
		{name: "officer even", kind: actorKindOfficer, health: 50, damage: 4, wantSeq: seqActorOfficerPain2, wantShape: shapeOfficerPain2},
		{name: "ss odd", kind: actorKindSS, health: 100, damage: 3, wantSeq: seqActorSSPain, wantShape: shapeSSPain1},
		{name: "ss even", kind: actorKindSS, health: 100, damage: 4, wantSeq: seqActorSSPain2, wantShape: shapeSSPain2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &game{rng: testRNG(1)}
			actor := actorInstance{
				kind:      tt.kind,
				alive:     true,
				blocking:  true,
				shootable: true,
				alerted:   true,
				health:    tt.health,
			}

			g.damageActor(&actor, tt.damage)

			if actor.aiState != actorStatePain {
				t.Fatalf("actor state = %v, want pain", actor.aiState)
			}
			if actor.sequenceID != tt.wantSeq {
				t.Fatalf("sequenceID = %q, want %q", actor.sequenceID, tt.wantSeq)
			}
			if actor.shapenum != tt.wantShape {
				t.Fatalf("pain shape = %d, want %d", actor.shapenum, tt.wantShape)
			}
		})
	}
}

func TestDamageActorDeathSpawnsClip(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:       actorKindGuard,
		x:          2.5,
		y:          3.5,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		health:     4,
		scoreValue: 100,
		dropPickup: pickupClip2,
	}

	g.damageActor(&actor, 4)
	if actor.aiState != actorStateDead {
		t.Fatalf("actor state = %v, want dead", actor.aiState)
	}
	if actor.blocking || actor.shootable {
		t.Fatal("dead actor should no longer block or be shootable")
	}
	if len(g.staticSprites) != 1 {
		t.Fatalf("dropped pickups = %d, want 1", len(g.staticSprites))
	}
	if g.staticSprites[0].shapenum != shapeSPR_STAT_26 {
		t.Fatalf("dropped pickup shape = %d, want %d", g.staticSprites[0].shapenum, shapeSPR_STAT_26)
	}
}

func TestDamageDogActorDeathNoDrop(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:       actorKindDog,
		x:          2.5,
		y:          3.5,
		alive:      true,
		blocking:   true,
		shootable:  true,
		health:     1,
		scoreValue: 200,
		deathSeq:   seqActorDogDeath,
	}

	g.damageActor(&actor, 1)
	if actor.aiState != actorStateDead {
		t.Fatalf("dog actor state = %v, want dead", actor.aiState)
	}
	if actor.blocking || actor.shootable {
		t.Fatal("dead dog should no longer block or be shootable")
	}
	if actor.shapenum != shapeDogDie1 {
		t.Fatalf("dog death start shape = %d, want %d", actor.shapenum, shapeDogDie1)
	}
	if len(g.staticSprites) != 0 {
		t.Fatalf("dead dog dropped %d pickups, want 0", len(g.staticSprites))
	}
}

func TestDamageSSActorDeathDropsMachineGun(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:       actorKindSS,
		x:          2.5,
		y:          3.5,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		health:     4,
		scoreValue: 500,
		dropPickup: pickupMachineGun,
		deathSeq:   seqActorSSDeath,
	}

	g.damageActor(&actor, 4)
	if actor.aiState != actorStateDead {
		t.Fatalf("ss actor state = %v, want dead", actor.aiState)
	}
	if actor.shapenum != shapeSSDie1 {
		t.Fatalf("ss death start shape = %d, want %d", actor.shapenum, shapeSSDie1)
	}
	if len(g.staticSprites) != 1 {
		t.Fatalf("dead ss dropped %d pickups, want 1", len(g.staticSprites))
	}
	if g.staticSprites[0].shapenum != shapeSPR_STAT_27 {
		t.Fatalf("dropped pickup shape = %d, want %d", g.staticSprites[0].shapenum, shapeSPR_STAT_27)
	}
}

func TestDamageBossActorDeathDropsGoldKey(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:       actorKindBoss,
		x:          2.5,
		y:          3.5,
		alive:      true,
		blocking:   true,
		shootable:  true,
		alerted:    true,
		health:     4,
		scoreValue: 5000,
		dropPickup: pickupKey1,
		deathSeq:   seqActorBossDeath,
	}

	g.damageActor(&actor, 4)
	if actor.aiState != actorStateDead {
		t.Fatalf("boss actor state = %v, want dead", actor.aiState)
	}
	if actor.shapenum != shapeBossDie1 {
		t.Fatalf("boss death start shape = %d, want %d", actor.shapenum, shapeBossDie1)
	}
	if len(g.staticSprites) != 1 {
		t.Fatalf("dead boss dropped %d pickups, want 1", len(g.staticSprites))
	}
	if g.staticSprites[0].pickup != pickupKey1 {
		t.Fatalf("dropped pickup = %v, want %v", g.staticSprites[0].pickup, pickupKey1)
	}
	if g.staticSprites[0].shapenum != shapeSPR_STAT_20 {
		t.Fatalf("dropped pickup shape = %d, want %d", g.staticSprites[0].shapenum, shapeSPR_STAT_20)
	}
}

func TestDamageActorDeathDoesNotPlayAlertSound(t *testing.T) {
	g := &game{rng: testRNGForPredicates(rngModulo(8, 0))}
	actor := actorInstance{
		kind:      actorKindGuard,
		x:         2.5,
		y:         3.5,
		alive:     true,
		blocking:  true,
		shootable: true,
		health:    4,
	}

	g.damageActor(&actor, 4)

	if g.lastPlayedSound != soundEnemyDeathGuard {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyDeathGuard)
	}
	if actor.alive {
		t.Fatal("dead actor should not remain alive")
	}
}

func TestDamageOfficerActorDeathUsesOfficerSound(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:      actorKindOfficer,
		x:         2.5,
		y:         3.5,
		alive:     true,
		blocking:  true,
		shootable: true,
		health:    4,
	}

	g.damageActor(&actor, 4)

	if g.lastPlayedSound != soundEnemyDeathOfficer {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyDeathOfficer)
	}
}

func TestDamageSSActorDeathUsesSSSound(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:      actorKindSS,
		x:         2.5,
		y:         3.5,
		alive:     true,
		blocking:  true,
		shootable: true,
		health:    4,
	}

	g.damageActor(&actor, 4)

	if g.lastPlayedSound != soundEnemyDeathSS {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyDeathSS)
	}
}

func TestDamageMutantActorDeathUsesMutantSound(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:      actorKindMutant,
		x:         2.5,
		y:         3.5,
		alive:     true,
		blocking:  true,
		shootable: true,
		health:    4,
	}

	g.damageActor(&actor, 4)

	if g.lastPlayedSound != soundEnemyDeathGuard8 {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyDeathGuard8)
	}
}

func TestBossMapSpecialDeathScreamOverridesSharewareNonBossDeaths(t *testing.T) {
	tests := []struct {
		name string
		kind ActorKind
	}{
		{name: "guard", kind: actorKindGuard},
		{name: "officer", kind: actorKindOfficer},
		{name: "ss", kind: actorKindSS},
		{name: "dog", kind: actorKindDog},
		{name: "mutant", kind: actorKindMutant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &game{
				rng:      testRNGForPredicates(rngValueEquals(0)),
				mapIndex: 9,
			}
			actor := actorInstance{
				kind:      tt.kind,
				x:         2.5,
				y:         3.5,
				alive:     true,
				blocking:  true,
				shootable: true,
				health:    4,
				deathSeq:  seqActorDogDeath,
			}

			g.damageActor(&actor, 4)

			if g.lastPlayedSound != soundEnemyDeathGuard6 {
				t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyDeathGuard6)
			}
		})
	}
}

func TestDamageBossActorDeathUsesBossSound(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:      actorKindBoss,
		x:         2.5,
		y:         3.5,
		alive:     true,
		blocking:  true,
		shootable: true,
		health:    4,
	}

	g.damageActor(&actor, 4)

	if g.lastPlayedSound != soundEnemyDeathBoss {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundEnemyDeathBoss)
	}
}

func TestBossDeathDoesNotStartVictorySequence(t *testing.T) {
	g := &game{rng: testRNG(1)}
	actor := actorInstance{
		kind:      actorKindBoss,
		x:         2.5,
		y:         3.5,
		alive:     true,
		blocking:  true,
		shootable: true,
		health:    4,
		deathSeq:  seqActorBossDeath,
	}

	g.damageActor(&actor, 4)
	if g.victoryActive {
		t.Fatal("boss death should not start the exit-tile victory sequence")
	}
}

func TestExitTileStartsVictorySequence(t *testing.T) {
	level := blankLevel(6, 6)
	setLevelTile(level, 2, 2, wl6.Tile{Area: 0, RawInfo: wolfExitTile})
	g := testGameWithLevel(level)
	g.playerX = 2.5
	g.playerY = 2.5

	if !g.checkVictoryTile() {
		t.Fatal("expected exit tile to start victory sequence")
	}
	if !g.victoryActive {
		t.Fatal("victoryActive = false, want true")
	}
	if g.victoryPhase != victoryPhaseRun {
		t.Fatalf("victoryPhase = %v, want run", g.victoryPhase)
	}
	if g.victoryBJ.sequenceID != seqVictoryBJRun {
		t.Fatalf("victory BJ sequence = %q, want %q", g.victoryBJ.sequenceID, seqVictoryBJRun)
	}
}

func TestVictorySequenceTransitionsAfterBJSequence(t *testing.T) {
	g := &game{
		rng:        testRNG(1),
		uiState:    uiStatePlaying,
		mode:       modeRaycast,
		menuReturn: uiStatePauseMenu,
	}
	g.startVictorySequence()

	for i := 0; i < 500 && g.fadePhase == 0; i++ {
		if err := g.updateVictorySequence(1); err != nil {
			t.Fatalf("updateVictorySequence failed: %v", err)
		}
	}
	if g.fadePhase == 0 {
		t.Fatal("victory sequence should start a fade transition")
	}

	for i := 0; i < 30 && g.fadePhase != 0; i++ {
		if err := g.updateFade(); err != nil {
			t.Fatalf("updateFade failed: %v", err)
		}
	}
	if g.uiState != uiStateVictoryIntermission {
		t.Fatalf("uiState = %v, want victory intermission", g.uiState)
	}
	if g.menuReturn != uiStateMainMenu {
		t.Fatalf("menuReturn = %v, want main menu", g.menuReturn)
	}
}

func TestReturnToMainMenuResetsPauseState(t *testing.T) {
	g := &game{
		uiState:     uiStatePauseMenu,
		menuReturn:  uiStatePauseMenu,
		menuIndex:   4,
		mode:        modeMap,
		paused:      true,
		mousePrimed: true,
	}

	if err := g.returnToMainMenu(); err != nil {
		t.Fatalf("returnToMainMenu failed: %v", err)
	}
	for i := 0; i < 30 && g.fadePhase != 0; i++ {
		if err := g.updateFade(); err != nil {
			t.Fatalf("updateFade failed: %v", err)
		}
	}

	if g.uiState != uiStateMainMenu {
		t.Fatalf("uiState = %v, want %v", g.uiState, uiStateMainMenu)
	}
	if g.menuReturn != uiStateMainMenu {
		t.Fatalf("menuReturn = %v, want %v", g.menuReturn, uiStateMainMenu)
	}
	if g.menuIndex != 0 {
		t.Fatalf("menuIndex = %d, want 0", g.menuIndex)
	}
	if g.mode != modeRaycast {
		t.Fatalf("mode = %v, want %v", g.mode, modeRaycast)
	}
	if g.paused {
		t.Fatal("paused should be false")
	}
	if g.mousePrimed {
		t.Fatal("mousePrimed should be false")
	}
}

func TestShouldDrawActorSpriteKeepsDeadActorsVisible(t *testing.T) {
	if !shouldDrawActorSprite(actorInstance{alive: true}) {
		t.Fatal("alive actor should be drawable")
	}
	if !shouldDrawActorSprite(actorInstance{alive: false, aiState: actorStateDead}) {
		t.Fatal("dead actor in death state should remain drawable")
	}
	if shouldDrawActorSprite(actorInstance{alive: false, aiState: actorStateChase}) {
		t.Fatal("non-dead inactive actor should not be drawable")
	}
}

func TestAnimSequences(t *testing.T) {
	guardDeath, ok := LookupAnimSequence(seqActorGuardDeath)
	if !ok {
		t.Fatal("guard death sequence missing")
	}
	wantGuardDeath := []int{shapeGuardDie1, shapeGuardDie2, shapeGuardDie3, shapeGuardDead}
	if len(guardDeath.Frames) != len(wantGuardDeath) {
		t.Fatalf("guard death frames = %d, want %d", len(guardDeath.Frames), len(wantGuardDeath))
	}
	for i, shape := range wantGuardDeath {
		if guardDeath.Frames[i].Shape != shape {
			t.Fatalf("guard death frame %d = %d, want %d", i, guardDeath.Frames[i].Shape, shape)
		}
	}

	guardShoot, ok := LookupAnimSequence(seqActorGuardShoot)
	if !ok {
		t.Fatal("guard shoot sequence missing")
	}
	if len(guardShoot.Frames) != 3 || guardShoot.Frames[1].Action != animActionFireActor {
		t.Fatalf("guard shoot sequence = %+v, want middle fire action", guardShoot.Frames)
	}

	officerShoot, ok := LookupAnimSequence(seqActorOfficerShoot)
	if !ok {
		t.Fatal("officer shoot sequence missing")
	}
	wantOfficerShoot := []AnimFrame{
		{Shape: shapeOfficerShoot1, Tics: 6},
		{Shape: shapeOfficerShoot2, Tics: 20, Action: animActionFireActor},
		{Shape: shapeOfficerShoot3, Tics: 10},
	}
	if len(officerShoot.Frames) != len(wantOfficerShoot) {
		t.Fatalf("officer shoot frames = %d, want %d", len(officerShoot.Frames), len(wantOfficerShoot))
	}
	for i, want := range wantOfficerShoot {
		if officerShoot.Frames[i] != want {
			t.Fatalf("officer shoot frame %d = %+v, want %+v", i, officerShoot.Frames[i], want)
		}
	}

	ssShoot, ok := LookupAnimSequence(seqActorSSShoot)
	if !ok {
		t.Fatal("ss shoot sequence missing")
	}
	wantSSShoot := []AnimFrame{
		{Shape: shapeSSShoot1, Tics: 20},
		{Shape: shapeSSShoot2, Tics: 20, Action: animActionFireActor},
		{Shape: shapeSSShoot3, Tics: 10},
		{Shape: shapeSSShoot2, Tics: 10, Action: animActionFireActor},
		{Shape: shapeSSShoot3, Tics: 10},
		{Shape: shapeSSShoot2, Tics: 10, Action: animActionFireActor},
		{Shape: shapeSSShoot3, Tics: 10},
		{Shape: shapeSSShoot2, Tics: 10, Action: animActionFireActor},
		{Shape: shapeSSShoot3, Tics: 10},
	}
	if len(ssShoot.Frames) != len(wantSSShoot) {
		t.Fatalf("ss shoot frames = %d, want %d", len(ssShoot.Frames), len(wantSSShoot))
	}
	for i, want := range wantSSShoot {
		if ssShoot.Frames[i] != want {
			t.Fatalf("ss shoot frame %d = %+v, want %+v", i, ssShoot.Frames[i], want)
		}
	}

	bossShoot, ok := LookupAnimSequence(seqActorBossShoot)
	if !ok {
		t.Fatal("boss shoot sequence missing")
	}
	wantBossShoot := []AnimFrame{
		{Shape: shapeBossShoot1, Tics: 30},
		{Shape: shapeBossShoot2, Tics: 10, Action: animActionFireActor},
		{Shape: shapeBossShoot3, Tics: 10, Action: animActionFireActor},
		{Shape: shapeBossShoot2, Tics: 10, Action: animActionFireActor},
		{Shape: shapeBossShoot3, Tics: 10, Action: animActionFireActor},
		{Shape: shapeBossShoot2, Tics: 10, Action: animActionFireActor},
		{Shape: shapeBossShoot3, Tics: 10, Action: animActionFireActor},
		{Shape: shapeBossShoot1, Tics: 10},
	}
	if len(bossShoot.Frames) != len(wantBossShoot) {
		t.Fatalf("boss shoot frames = %d, want %d", len(bossShoot.Frames), len(wantBossShoot))
	}
	for i, want := range wantBossShoot {
		if bossShoot.Frames[i] != want {
			t.Fatalf("boss shoot frame %d = %+v, want %+v", i, bossShoot.Frames[i], want)
		}
	}

	bossDeath, ok := LookupAnimSequence(seqActorBossDeath)
	if !ok {
		t.Fatal("boss death sequence missing")
	}
	wantBossDeath := []AnimFrame{
		{Shape: shapeBossDie1, Tics: 15},
		{Shape: shapeBossDie2, Tics: 15},
		{Shape: shapeBossDie3, Tics: 15},
		{Shape: shapeBossDead, Tics: 0},
	}
	if len(bossDeath.Frames) != len(wantBossDeath) {
		t.Fatalf("boss death frames = %d, want %d", len(bossDeath.Frames), len(wantBossDeath))
	}
	for i, want := range wantBossDeath {
		if bossDeath.Frames[i] != want {
			t.Fatalf("boss death frame %d = %+v, want %+v", i, bossDeath.Frames[i], want)
		}
	}

	dogPatrol, ok := LookupAnimSequence(seqActorDogPatrol)
	if !ok {
		t.Fatal("dog patrol sequence missing")
	}
	wantDogPatrol := []int{shapeDogBase, shapeDogBase, shapeDogWalk2, shapeDogWalk3, shapeDogWalk3, shapeDogWalk4}
	if len(dogPatrol.Frames) != len(wantDogPatrol) {
		t.Fatalf("dog patrol frames = %d, want %d", len(dogPatrol.Frames), len(wantDogPatrol))
	}
	for i, shape := range wantDogPatrol {
		if dogPatrol.Frames[i].Shape != shape {
			t.Fatalf("dog patrol frame %d = %d, want %d", i, dogPatrol.Frames[i].Shape, shape)
		}
	}

	dogChase, ok := LookupAnimSequence(seqActorDogChase)
	if !ok {
		t.Fatal("dog chase sequence missing")
	}
	wantDogChase := []int{shapeDogBase, shapeDogBase, shapeDogWalk2, shapeDogWalk3, shapeDogWalk3, shapeDogWalk4}
	if len(dogChase.Frames) != len(wantDogChase) {
		t.Fatalf("dog chase frames = %d, want %d", len(dogChase.Frames), len(wantDogChase))
	}
	for i, shape := range wantDogChase {
		if dogChase.Frames[i].Shape != shape {
			t.Fatalf("dog chase frame %d = %d, want %d", i, dogChase.Frames[i].Shape, shape)
		}
	}

	dogJump, ok := LookupAnimSequence(seqActorDogJump)
	if !ok {
		t.Fatal("dog jump sequence missing")
	}
	wantDogJump := []int{shapeDogJump1, shapeDogJump2, shapeDogJump3, shapeDogJump1, shapeDogBase}
	if len(dogJump.Frames) != len(wantDogJump) {
		t.Fatalf("dog jump frames = %d, want %d", len(dogJump.Frames), len(wantDogJump))
	}
	for i, shape := range wantDogJump {
		if dogJump.Frames[i].Shape != shape {
			t.Fatalf("dog jump frame %d = %d, want %d", i, dogJump.Frames[i].Shape, shape)
		}
	}
	if dogJump.Frames[1].Action != animActionBiteActor {
		t.Fatalf("dog jump middle action = %v, want bite", dogJump.Frames[1].Action)
	}

	dogDeath, ok := LookupAnimSequence(seqActorDogDeath)
	if !ok {
		t.Fatal("dog death sequence missing")
	}
	wantDogDeath := []int{shapeDogDie1, shapeDogDie2, shapeDogDie3, shapeDogDead}
	if len(dogDeath.Frames) != len(wantDogDeath) {
		t.Fatalf("dog death frames = %d, want %d", len(dogDeath.Frames), len(wantDogDeath))
	}
	for i, shape := range wantDogDeath {
		if dogDeath.Frames[i].Shape != shape || dogDeath.Frames[i].Tics != 15 {
			t.Fatalf("dog death frame %d = %+v, want shape=%d tics=15", i, dogDeath.Frames[i], shape)
		}
	}
}

func TestCanOpenDoorLock(t *testing.T) {
	if canOpenDoorLock(0, 0) != true {
		t.Fatal("normal door should open without keys")
	}
	if canOpenDoorLock(1, 0) {
		t.Fatal("lock 1 should require key 1")
	}
	if !canOpenDoorLock(1, 1<<0) {
		t.Fatal("key 1 should open lock 1")
	}
	if canOpenDoorLock(4, 1<<0) {
		t.Fatal("wrong key should not open lock 4")
	}
	if !canOpenDoorLock(5, 0) {
		t.Fatal("elevator lock should not require a key in this engine")
	}
}

func TestApplyPickup(t *testing.T) {
	g := &game{
		health:       50,
		ammo:         0,
		weapon:       0,
		bestWeapon:   0,
		chosenWeapon: 0,
	}

	if !g.applyPickup(pickupFirstAid) {
		t.Fatal("expected first aid pickup to apply")
	}
	if g.health != 75 {
		t.Fatalf("health = %d, want 75", g.health)
	}

	if !g.applyPickup(pickupMachineGun) {
		t.Fatal("expected machine gun pickup to apply")
	}
	if g.ammo != 6 {
		t.Fatalf("ammo = %d, want 6", g.ammo)
	}
	if g.weapon != 2 || g.bestWeapon != 2 || g.chosenWeapon != 2 {
		t.Fatalf("weapons = (%d,%d,%d), want all 2", g.weapon, g.bestWeapon, g.chosenWeapon)
	}

	if !g.applyPickup(pickupCross) {
		t.Fatal("expected cross pickup to apply")
	}
	if g.score != 100 || g.treasureCount != 1 {
		t.Fatalf("score/treasure = %d/%d, want 100/1", g.score, g.treasureCount)
	}
}

func TestResetLoadoutMatchesWolf3DStart(t *testing.T) {
	g := &game{
		weapon:          3,
		bestWeapon:      3,
		chosenWeapon:    3,
		ammo:            99,
		attacking:       true,
		weaponSequence:  "attack",
		weaponFrameIdx:  2,
		weaponFrameTics: 5,
	}

	g.resetLoadout()

	if g.weapon != 1 || g.bestWeapon != 1 || g.chosenWeapon != 1 {
		t.Fatalf("weapons = (%d,%d,%d), want all 1", g.weapon, g.bestWeapon, g.chosenWeapon)
	}
	if g.ammo != startAmmo {
		t.Fatalf("ammo = %d, want %d", g.ammo, startAmmo)
	}
	if g.attacking || g.weaponSequence != "" || g.weaponFrameIdx != 0 || g.weaponFrameTics != 0 {
		t.Fatalf("weapon state not reset: attacking=%v sequence=%q frame=%d tics=%d", g.attacking, g.weaponSequence, g.weaponFrameIdx, g.weaponFrameTics)
	}
}

func TestStartNewGameMatchesWolf3DState(t *testing.T) {
	g := &game{
		health:        12,
		lives:         1,
		keys:          3,
		score:         900,
		treasureCount: 4,
		weapon:        3,
		bestWeapon:    3,
		chosenWeapon:  3,
		ammo:          99,
	}

	g.startNewGame()

	if g.health != 100 {
		t.Fatalf("health = %d, want 100", g.health)
	}
	if g.lives != 3 {
		t.Fatalf("lives = %d, want 3", g.lives)
	}
	if g.keys != 0 || g.score != 0 || g.treasureCount != 0 {
		t.Fatalf("keys/score/treasure = %d/%d/%d, want 0/0/0", g.keys, g.score, g.treasureCount)
	}
	if g.weapon != 1 || g.bestWeapon != 1 || g.chosenWeapon != 1 || g.ammo != startAmmo {
		t.Fatalf("loadout = (%d,%d,%d,%d), want (1,1,1,%d)", g.weapon, g.bestWeapon, g.chosenWeapon, g.ammo, startAmmo)
	}
}

func TestResetLevelStatePreservesLoadoutAndScore(t *testing.T) {
	g := &game{
		health:        67,
		lives:         2,
		keys:          3,
		score:         1200,
		treasureCount: 5,
		weapon:        3,
		bestWeapon:    3,
		chosenWeapon:  3,
		ammo:          42,
	}

	g.resetLevelState()

	if g.health != 67 || g.lives != 2 || g.score != 1200 {
		t.Fatalf("health/lives/score = %d/%d/%d, want 67/2/1200", g.health, g.lives, g.score)
	}
	if g.weapon != 3 || g.bestWeapon != 3 || g.chosenWeapon != 3 || g.ammo != 42 {
		t.Fatalf("loadout = (%d,%d,%d,%d), want (3,3,3,42)", g.weapon, g.bestWeapon, g.chosenWeapon, g.ammo)
	}
	if g.keys != 0 || g.treasureCount != 0 {
		t.Fatalf("keys/treasure = %d/%d, want 0/0", g.keys, g.treasureCount)
	}
}

func TestTakePlayerDamageHonorsGodMode(t *testing.T) {
	g := &game{
		health:  100,
		godMode: true,
	}

	g.takePlayerDamage(25)

	if g.health != 100 {
		t.Fatalf("health = %d, want 100", g.health)
	}
}

func TestApplyItemCheatMatchesWolfUpgradeFlow(t *testing.T) {
	g := &game{
		health:       40,
		weapon:       1,
		bestWeapon:   1,
		chosenWeapon: 1,
		ammo:         8,
		score:        0,
	}

	g.applyItemCheat()

	if g.score != 100000 {
		t.Fatalf("score = %d, want 100000", g.score)
	}
	if g.health != 100 {
		t.Fatalf("health = %d, want 100", g.health)
	}
	if g.weapon != 2 || g.bestWeapon != 2 || g.chosenWeapon != 2 {
		t.Fatalf("weapons = (%d,%d,%d), want all 2", g.weapon, g.bestWeapon, g.chosenWeapon)
	}
	if g.ammo != 64 {
		t.Fatalf("ammo = %d, want 64", g.ammo)
	}
}

func TestApplyExtraStuffCheatGivesKeys(t *testing.T) {
	g := &game{}

	g.applyExtraStuffCheat()

	if g.keys != 0x0f {
		t.Fatalf("keys = %04b, want 1111", g.keys)
	}
}

func TestNextElevatorMap(t *testing.T) {
	tests := []struct {
		name   string
		mapIdx int
		secret bool
		want   int
	}{
		{name: "normal floor to next", mapIdx: 0, secret: false, want: 1},
		{name: "secret elevator enters secret floor", mapIdx: 0, secret: true, want: 9},
		{name: "secret floor returns to configured floor", mapIdx: 9, secret: false, want: 1},
		{name: "episode three secret floor returns to configured floor", mapIdx: 29, secret: false, want: 27},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &game{mapIndex: tt.mapIdx}
			if got := g.nextElevatorMap(tt.secret); got != tt.want {
				t.Fatalf("nextElevatorMap(%v) = %d, want %d", tt.secret, got, tt.want)
			}
		})
	}
}

func TestUseElevatorFlipsSwitchWall(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	setLevelTile(level, 2, 1, wl6.Tile{RawWall: wolfElevatorTile, Solid: true, RenderWall: true})
	g := testGameWithLevel(level)
	g.mapIndex = 0
	g.playerX = 1.5
	g.playerY = 1.5
	g.playerA = 0

	if err := g.useElevator(); err != nil {
		t.Fatalf("useElevator failed: %v", err)
	}
	if tile := g.level.Tile(2, 1); tile.RawWall != wolfElevatorUsedTile {
		t.Fatalf("elevator switch wall = %d, want %d", tile.RawWall, wolfElevatorUsedTile)
	}
	if g.fadePhase == 0 {
		t.Fatal("useElevator should start a fade transition")
	}
}

func TestSaveMenuSkipsDedicatedAutosaveSlot(t *testing.T) {
	g := &game{
		saveSlots: []saveSlotSummary{
			{Used: true, Path: autosaveSlotPath(0), IsAutosave: true, AutosaveOrdinal: 1, Name: autosaveSlotName},
			{Used: true, Path: autosaveSlotPath(1), IsAutosave: true, AutosaveOrdinal: 2, Name: autosaveSlotName},
			{Used: false, Path: autosaveSlotPath(2), IsAutosave: true, AutosaveOrdinal: 3, Name: autosaveSlotName},
			{Used: true, Path: "manual-1.sav", Name: "Manual One"},
			{Used: true, Path: "manual-2.sav", Name: "Manual Two"},
		},
	}

	if got := g.loadGameMenuCount(); got != 4 {
		t.Fatalf("loadGameMenuCount = %d, want 4", got)
	}
	if got := g.saveGameMenuCount(); got != 3 {
		t.Fatalf("saveGameMenuCount = %d, want 3", got)
	}
	g.menuIndex = 0
	if got := g.loadMenuSelectionSlot(); got != 0 {
		t.Fatalf("loadMenuSelectionSlot for first visible slot = %d, want 0", got)
	}
	g.menuIndex = 2
	if got := g.loadMenuSelectionSlot(); got != 3 {
		t.Fatalf("loadMenuSelectionSlot for first manual slot = %d, want 3", got)
	}
	g.menuIndex = 3
	if summary := g.selectedSaveSlotSummary(false); summary == nil || summary.Name != "Manual Two" {
		t.Fatalf("selected load summary = %+v, want Manual Two", summary)
	}
	g.menuIndex = 1
	if got := g.saveMenuSelectionSlot(); got != 3 {
		t.Fatalf("saveMenuSelectionSlot for first manual slot = %d, want 3", got)
	}
	g.menuIndex = 2
	if got := g.saveMenuSelectionSlot(); got != 4 {
		t.Fatalf("saveMenuSelectionSlot for second manual slot = %d, want 4", got)
	}
	if summary := g.selectedSaveSlotSummary(true); summary == nil || summary.Name != "Manual Two" {
		t.Fatalf("selected save summary = %+v, want Manual Two", summary)
	}
}

func TestConsumePeriodicAutosaveTriggers(t *testing.T) {
	g := &game{}

	if got := g.consumePeriodicAutosaveTriggers(autosaveIntervalTics - 1); got != 0 {
		t.Fatalf("triggers before interval = %d, want 0", got)
	}
	if got := g.consumePeriodicAutosaveTriggers(1); got != 1 {
		t.Fatalf("triggers at interval = %d, want 1", got)
	}
	if g.autosaveTickAccum != 0 {
		t.Fatalf("autosaveTickAccum after exact interval = %d, want 0", g.autosaveTickAccum)
	}
	if got := g.consumePeriodicAutosaveTriggers(autosaveIntervalTics*2 + 10); got != 2 {
		t.Fatalf("triggers over two intervals = %d, want 2", got)
	}
	if g.autosaveTickAccum != 10 {
		t.Fatalf("autosaveTickAccum remainder = %d, want 10", g.autosaveTickAccum)
	}
}

func withTempWorkingDir(t *testing.T) string {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	return tmp
}

func newAutosaveTestGame() *game {
	level := blankLevel(4, 3)
	setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
	mapData := &wl6.MapData{
		Header: wl6.MapHeader{
			Width:  uint16(level.Width),
			Height: uint16(level.Height),
			Name:   "TEST",
		},
		Planes: [2][]uint16{
			make([]uint16, level.Width*level.Height),
			make([]uint16, level.Width*level.Height),
		},
	}
	g := testGameWithLevel(level)
	g.mapData = mapData
	g.mapIndex = 0
	g.playerX = 1.5
	g.playerY = 1.5
	g.playerA = 0
	g.health = 100
	g.ammo = 8
	g.lives = 3
	return g
}

func TestPerformAutosavesWritesAutosaveSlot(t *testing.T) {
	tmp := withTempWorkingDir(t)
	g := newAutosaveTestGame()

	if err := g.performAutosaves(1); err != nil {
		t.Fatalf("performAutosaves failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, autosaveSlotPath(0))); err != nil {
		t.Fatalf("autosave file missing: %v", err)
	}
	if g.hudNotice != "Autosaved" {
		t.Fatalf("hudNotice = %q, want Autosaved", g.hudNotice)
	}
	if g.loadGameMenuCount() != 1 {
		t.Fatalf("loadGameMenuCount after autosave = %d, want 1", g.loadGameMenuCount())
	}
}

func TestUpdatePausedDoesNotAdvanceAutosaveTimer(t *testing.T) {
	g := &game{
		uiState: uiStatePlaying,
		paused:  true,
	}

	if err := g.Update(); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if g.autosaveTickAccum != 0 {
		t.Fatalf("autosaveTickAccum = %d, want 0 while paused", g.autosaveTickAccum)
	}
}

func TestUseElevatorTriggersAutosave(t *testing.T) {
	tmp := withTempWorkingDir(t)
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{RawWall: wolfElevatorTile, Solid: true, RenderWall: true})
	setLevelTile(level, 1, 1, wl6.Tile{RawWall: wolfElevatorTile})
	mapData := &wl6.MapData{
		Header: wl6.MapHeader{
			Width:  uint16(level.Width),
			Height: uint16(level.Height),
			Name:   "ELEVATOR",
		},
		Planes: [2][]uint16{
			make([]uint16, level.Width*level.Height),
			make([]uint16, level.Width*level.Height),
		},
	}
	g := testGameWithLevel(level)
	g.mapData = mapData
	g.mapIndex = 0
	g.pendingMap = 0
	g.playerX = 1.5
	g.playerY = 1.5
	g.playerA = 0
	g.health = 100
	g.ammo = 8
	g.lives = 3

	if err := g.useElevator(); err != nil {
		t.Fatalf("useElevator failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, autosaveSlotPath(0))); err != nil {
		t.Fatalf("level-transition autosave missing: %v", err)
	}
	if g.hudNotice != "Autosaved" {
		t.Fatalf("hudNotice = %q, want Autosaved", g.hudNotice)
	}
	if g.loadGameMenuCount() != 1 {
		t.Fatalf("loadGameMenuCount after level-transition autosave = %d, want 1", g.loadGameMenuCount())
	}
}

func TestCastRayDoesNotClampFarWallDistance(t *testing.T) {
	level := blankLevel(40, 3)
	setLevelTile(level, 30, 1, wl6.Tile{RawWall: 1, Solid: true, RenderWall: true})
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5

	dist, wallID, _, _, _ := g.castRay(1, 0)
	if wallID != 1 {
		t.Fatalf("castRay wallID = %d, want 1", wallID)
	}
	if dist != 28.5 {
		t.Fatalf("castRay distance = %v, want 28.5", dist)
	}
}

func TestShootAheadGunHasNoHardRangeCap(t *testing.T) {
	level := blankLevel(32, 3)
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5
	g.playerA = 0
	g.weapon = 1
	g.chosenWeapon = 1
	g.bestWeapon = 1
	g.staticSprites = []staticSprite{{
		x:         20.5,
		y:         1.5,
		alive:     true,
		shootable: true,
	}}

	g.shootAhead()

	if g.staticSprites[0].alive {
		t.Fatal("gun shot should hit a distant centered target without a hard max range cap")
	}
}

func TestShootAheadKnifeRespectsWolfRange(t *testing.T) {
	level := blankLevel(8, 3)
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 1.5
	g.playerA = 0
	g.weapon = 0
	g.chosenWeapon = 0
	g.bestWeapon = 0
	g.staticSprites = []staticSprite{{
		x:         3.5,
		y:         1.5,
		alive:     true,
		shootable: true,
	}}

	g.shootAhead()

	if !g.staticSprites[0].alive {
		t.Fatal("knife shot should miss beyond Wolf's short melee range")
	}
}

func TestPlayerAttackTileDistanceMatchesWolfMetric(t *testing.T) {
	g := &game{playerX: 1.5, playerY: 1.5}

	if got := g.playerAttackTileDistance(4.9, 2.1); got != 3 {
		t.Fatalf("playerAttackTileDistance = %d, want 3", got)
	}
}

func TestDoorDoesNotCloseOnBlockingActor(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{
		Door: &wl6.Door{Vertical: true},
	})
	g := testGameWithLevel(level)
	i := 1*level.Width + 2
	g.doorState[i] = 2
	g.doorOpen[i] = 1
	g.doorTimer[i] = doorOpenHoldTics
	g.actors = []actorInstance{{
		alive:    true,
		blocking: true,
		tileX:    2,
		tileY:    1,
		x:        2.5,
		y:        1.5,
	}}

	g.updateDoors(1)

	if g.doorState[i] != 2 {
		t.Fatalf("doorState = %d, want 2 while blocked by actor", g.doorState[i])
	}
}

func TestDoorDoesNotCloseOnAdjacentActorCrossingPlane(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 2, 1, wl6.Tile{
		Door: &wl6.Door{Vertical: true},
	})
	g := testGameWithLevel(level)
	i := 1*level.Width + 2
	g.doorState[i] = 2
	g.doorOpen[i] = 1
	g.doorTimer[i] = doorOpenHoldTics
	g.actors = []actorInstance{{
		alive:    true,
		blocking: true,
		tileX:    1,
		tileY:    1,
		x:        1.8,
		y:        1.5,
	}}

	g.updateDoors(1)

	if g.doorState[i] != 2 {
		t.Fatalf("doorState = %d, want 2 while adjacent actor crosses vertical door plane", g.doorState[i])
	}
}

func TestDoorMayCloseWhenAdjacentActorIsClearOfPlane(t *testing.T) {
	level := blankLevel(5, 3)
	setLevelTile(level, 2, 1, wl6.Tile{
		Door: &wl6.Door{Vertical: true},
	})
	g := testGameWithLevel(level)
	i := 1*level.Width + 2
	g.doorState[i] = 2
	g.doorOpen[i] = 1
	g.doorTimer[i] = doorOpenHoldTics
	g.actors = []actorInstance{{
		alive:    true,
		blocking: true,
		tileX:    1,
		tileY:    1,
		x:        1.5,
		y:        1.5,
	}}

	g.updateDoors(1)

	if g.doorState[i] != 3 {
		t.Fatalf("doorState = %d, want 3 when adjacent actor is clear of vertical door plane", g.doorState[i])
	}
}

func TestOpenDoorAtReopensClosingDoor(t *testing.T) {
	level := blankLevel(4, 3)
	setLevelTile(level, 2, 1, wl6.Tile{
		Door: &wl6.Door{Vertical: true},
	})
	g := testGameWithLevel(level)
	i := 1*level.Width + 2
	g.doorState[i] = 3
	g.doorOpen[i] = 0.4

	g.openDoorAt(2, 1)

	if g.doorState[i] != 1 {
		t.Fatalf("doorState = %d, want 1 after reopen", g.doorState[i])
	}
	if g.doorTimer[i] != 0 {
		t.Fatalf("doorTimer = %d, want 0 after reopen", g.doorTimer[i])
	}
}

func TestSpawnDroppedPickupCentersOnTile(t *testing.T) {
	g := &game{}

	g.spawnDroppedPickup(10.92, 7.08, pickupClip2)

	if len(g.staticSprites) != 1 {
		t.Fatalf("dropped sprites = %d, want 1", len(g.staticSprites))
	}
	if g.staticSprites[0].x != 10.5 || g.staticSprites[0].y != 7.5 {
		t.Fatalf("drop position = (%.2f, %.2f), want (10.50, 7.50)", g.staticSprites[0].x, g.staticSprites[0].y)
	}
}

func TestUseDoorAheadActivatesSecretWall(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 2, 2, wl6.Tile{RawWall: 1, RawInfo: pushableTile, Solid: true, RenderWall: true})
	setLevelTile(level, 3, 2, wl6.Tile{Area: 0})
	setLevelTile(level, 4, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 2.5
	g.playerA = 0

	g.useDoorAhead()

	if !g.pushWall.active {
		t.Fatal("push wall did not activate")
	}
	if g.secretCount != 1 {
		t.Fatalf("secretCount = %d, want 1", g.secretCount)
	}
	if tile := g.level.Tile(2, 2); tile.RawInfo != 0 || tile.Solid || tile.RenderWall {
		t.Fatalf("origin tile after activation = %+v, want cleared floor tile", tile)
	}
}

func TestUpdatePushWallMovesTwoTiles(t *testing.T) {
	level := blankLevel(6, 5)
	setLevelTile(level, 2, 2, wl6.Tile{RawWall: 7, RawInfo: pushableTile, Solid: true, RenderWall: true})
	setLevelTile(level, 3, 2, wl6.Tile{Area: 0})
	setLevelTile(level, 4, 2, wl6.Tile{Area: 0})
	setLevelTile(level, 5, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 2.5
	g.playerA = 0

	g.useDoorAhead()
	g.updatePushWall(pushWallStepTics * pushWallMaxSteps)

	if g.pushWall.active {
		t.Fatal("push wall still active after max steps")
	}
	if tile := g.level.Tile(2, 2); tile.Solid {
		t.Fatalf("origin tile still solid: %+v", tile)
	}
	if tile := g.level.Tile(3, 2); tile.Solid {
		t.Fatalf("middle tile still solid: %+v", tile)
	}
	tile := g.level.Tile(4, 2)
	if !tile.Solid || tile.RawWall != 7 {
		t.Fatalf("final wall tile = %+v, want solid wall 7", tile)
	}
}

func TestCollidesBlockingStaticUsesTightBounds(t *testing.T) {
	level := blankLevel(5, 5)
	g := testGameWithLevel(level)
	g.staticSprites = []staticSprite{{
		x:        2.5,
		y:        2.5,
		blocking: true,
		alive:    true,
	}}

	if g.collides(1.6, 2.5) {
		t.Fatal("blocking static collided from too far away")
	}
	if !g.collides(1.9, 2.5) {
		t.Fatal("blocking static did not collide at close range")
	}
}

func TestCastPushWallHitsShiftedBlockFromAllSides(t *testing.T) {
	g := testGameWithLevel(blankLevel(6, 6))
	g.pushWall = pushWallState{
		active: true,
		x:      2,
		y:      2,
		dx:     1,
		wall:   wl6.Tile{RawWall: 7, Solid: true, RenderWall: true},
		tics:   pushWallStepTics / 2,
	}

	g.playerX = 4.5
	g.playerY = 2.5
	if hit, dist, side, _ := g.castPushWall(2, 2, -1, 0); !hit || side != 1 || math.Abs(dist-1) > 0.001 {
		t.Fatalf("rear push-wall hit = hit:%v dist:%f side:%d, want hit at dist 1 on vertical face", hit, dist, side)
	}

	g.playerX = 3.0
	g.playerY = 1.5
	if hit, dist, side, _ := g.castPushWall(2, 2, 0, 1); !hit || side != 2 || math.Abs(dist-0.5) > 0.001 {
		t.Fatalf("side push-wall hit = hit:%v dist:%f side:%d, want hit at dist 0.5 on horizontal face", hit, dist, side)
	}

	g.playerX = 4.5
	g.playerY = 2.5
	if hit, dist, side, _ := g.castPushWall(3, 2, -1, 0); !hit || side != 1 || math.Abs(dist-1) > 0.001 {
		t.Fatalf("overlap-tile push-wall hit = hit:%v dist:%f side:%d, want hit at dist 1 on vertical face", hit, dist, side)
	}
}

func TestCollidesPushWallUsesShiftedBlockBounds(t *testing.T) {
	g := testGameWithLevel(blankLevel(6, 6))
	g.pushWall = pushWallState{
		active: true,
		x:      2,
		y:      2,
		dx:     1,
		wall:   wl6.Tile{RawWall: 7, Solid: true, RenderWall: true},
		tics:   pushWallStepTics / 2,
	}

	if !g.collides(3.2, 2.5) {
		t.Fatal("shifted push wall did not block inside protruding next tile")
	}
}

func TestApplyPickupUsesWolfStyleSoundEvents(t *testing.T) {
	tests := []struct {
		name   string
		game   game
		pickup pickupType
		want   soundID
	}{
		{name: "food", game: game{health: 50}, pickup: pickupFood, want: soundPickupHealth1},
		{name: "first aid", game: game{health: 50}, pickup: pickupFirstAid, want: soundPickupHealth2},
		{name: "gibs", game: game{health: 10}, pickup: pickupGibs, want: soundPickupGibs},
		{name: "dog food", game: game{health: 50}, pickup: pickupAlpo, want: soundPickupHealth1},
		{name: "ammo", game: game{ammo: 0}, pickup: pickupClip, want: soundPickupAmmo},
		{name: "machine gun", game: game{}, pickup: pickupMachineGun, want: soundPickupMachineGun},
		{name: "chaingun", game: game{}, pickup: pickupChaingun, want: soundPickupChaingun},
		{name: "key", game: game{}, pickup: pickupKey1, want: soundPickupKey},
		{name: "cross", game: game{}, pickup: pickupCross, want: soundPickupTreasure1},
		{name: "chalice", game: game{}, pickup: pickupChalice, want: soundPickupTreasure2},
		{name: "bible", game: game{}, pickup: pickupBible, want: soundPickupTreasure3},
		{name: "crown", game: game{}, pickup: pickupCrown, want: soundPickupTreasure4},
		{name: "full heal", game: game{health: 10}, pickup: pickupFullHeal, want: soundPickupOneUp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.game
			if !g.applyPickup(tt.pickup) {
				t.Fatal("applyPickup returned false")
			}
			if g.lastPlayedSound != tt.want {
				t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, tt.want)
			}
		})
	}
}

func TestStartPushWallUsesPushWallSound(t *testing.T) {
	level := blankLevel(5, 5)
	setLevelTile(level, 2, 2, wl6.Tile{RawWall: 1, RawInfo: pushableTile, Solid: true, RenderWall: true})
	setLevelTile(level, 3, 2, wl6.Tile{Area: 0})
	g := testGameWithLevel(level)
	g.playerX = 1.5
	g.playerY = 2.5
	g.playerA = 0

	g.useDoorAhead()

	if g.lastPlayedSound != soundPushWall {
		t.Fatalf("lastPlayedSound = %v, want %v", g.lastPlayedSound, soundPushWall)
	}
}

func TestDamageActorDoesNotPlayGenericHitConfirm(t *testing.T) {
	g := &game{lastPlayedSound: soundMenuBack}
	a := actorInstance{
		alive:     true,
		shootable: true,
		alerted:   true,
		health:    10,
		kind:      actorKindGuard,
	}

	if !g.damageActor(&a, 1) {
		t.Fatal("damageActor returned false")
	}
	if g.lastPlayedSound != soundMenuBack {
		t.Fatalf("lastPlayedSound = %v, want unchanged %v", g.lastPlayedSound, soundMenuBack)
	}
}

func TestGiveAmmoRestoresChosenWeaponAfterDryFire(t *testing.T) {
	g := &game{
		ammo:         0,
		weapon:       0,
		bestWeapon:   2,
		chosenWeapon: 2,
	}

	g.giveAmmo(4)

	if g.ammo != 4 {
		t.Fatalf("ammo = %d, want 4", g.ammo)
	}
	if g.weapon != 2 {
		t.Fatalf("weapon = %d, want 2", g.weapon)
	}
}

func TestTakePlayerDamageEntersDeathSequence(t *testing.T) {
	g := &game{
		level:        blankLevel(3, 3),
		health:       5,
		lives:        3,
		ammo:         40,
		weapon:       2,
		bestWeapon:   2,
		chosenWeapon: 2,
		keys:         0x03,
		playerX:      4.5,
		playerY:      4.5,
	}

	g.takePlayerDamage(10)

	if !g.playerDying {
		t.Fatal("playerDying = false, want true")
	}
	if g.deathPhase != deathPhaseFizzle {
		t.Fatalf("deathPhase = %v, want %v", g.deathPhase, deathPhaseFizzle)
	}
	if g.deathTimer != deathFadeTics {
		t.Fatalf("deathTimer = %d, want %d", g.deathTimer, deathFadeTics)
	}
	if g.lives != 3 {
		t.Fatalf("lives = %d, want 3 before death sequence completes", g.lives)
	}
	if g.health != 0 {
		t.Fatalf("health = %d, want 0 during death sequence", g.health)
	}
	if g.hudNotice != "" {
		t.Fatalf("hudNotice = %q, want empty during death sequence", g.hudNotice)
	}
	if alpha := g.deathOverlayAlpha(); alpha != 1 {
		t.Fatalf("deathOverlayAlpha = %.2f, want 1 during fizzle", alpha)
	}
}

func TestDeathOverlayAlphaRampsAndHolds(t *testing.T) {
	g := &game{playerDying: true, deathPhase: deathPhaseRotate}

	if alpha := g.deathOverlayAlpha(); alpha != 0 {
		t.Fatalf("deathOverlayAlpha in rotate = %.2f, want 0", alpha)
	}

	g.deathPhase = deathPhaseFizzle
	g.deathTimer = deathFadeTics / 2
	if alpha := g.deathOverlayAlpha(); alpha != 1 {
		t.Fatalf("deathOverlayAlpha in fizzle = %.2f, want 1", alpha)
	}

	g.viewWidth = 320
	g.viewHeight = 200
	g.layout.renderLeft = 0
	g.layout.renderRight = 320
	g.layout.renderTop = 0
	g.layout.renderBottom = 160
	if reveal := g.deathFizzleRevealCount(); reveal != 320*160/2 {
		t.Fatalf("deathFizzleRevealCount = %d, want %d", reveal, 320*160/2)
	}

	g.deathPhase = deathPhaseHold
	g.deathTimer = deathHoldTics
	if alpha := g.deathOverlayAlpha(); alpha != 1 {
		t.Fatalf("deathOverlayAlpha hold = %.2f, want 1", alpha)
	}
}

func TestBeginPlayerDeathRotatesTowardAttacker(t *testing.T) {
	g := &game{
		playerX: 1,
		playerY: 1,
		playerA: 0,
	}

	g.beginPlayerDeath(1, 2, true)

	if g.deathPhase != deathPhaseRotate {
		t.Fatalf("deathPhase = %v, want %v", g.deathPhase, deathPhaseRotate)
	}
	if err := g.advancePlayerDeathTics(10); err != nil {
		t.Fatalf("advancePlayerDeathTics error: %v", err)
	}
	if math.Abs(g.playerA-(20*math.Pi/180)) > 0.001 {
		t.Fatalf("playerA = %.4f, want %.4f", g.playerA, 20*math.Pi/180)
	}
	if g.deathPhase != deathPhaseRotate {
		t.Fatalf("deathPhase after partial rotate = %v, want %v", g.deathPhase, deathPhaseRotate)
	}
	if err := g.advancePlayerDeathTics(35); err != nil {
		t.Fatalf("advancePlayerDeathTics error: %v", err)
	}
	if math.Abs(g.playerA-math.Pi/2) > 0.001 {
		t.Fatalf("playerA = %.4f, want %.4f", g.playerA, math.Pi/2)
	}
	if g.deathPhase != deathPhaseFizzle {
		t.Fatalf("deathPhase after rotate = %v, want %v", g.deathPhase, deathPhaseFizzle)
	}
}

func TestUpdatePlayerDeathRespawnsAfterDeathSequence(t *testing.T) {
	g := &game{
		level:        blankLevel(3, 3),
		health:       5,
		lives:        3,
		ammo:         40,
		weapon:       2,
		bestWeapon:   2,
		chosenWeapon: 2,
		keys:         0x03,
		playerX:      4.5,
		playerY:      4.5,
	}

	g.takePlayerDamage(10)

	if err := g.advancePlayerDeathTics(deathFadeTics + deathHoldTics); err != nil {
		t.Fatalf("advancePlayerDeathTics error: %v", err)
	}
	if g.deathPhase != deathPhaseWaitSound {
		t.Fatalf("deathPhase = %v, want %v before sound completion", g.deathPhase, deathPhaseWaitSound)
	}
	if err := g.advancePlayerDeathTics(1); err != nil {
		t.Fatalf("advancePlayerDeathTics error: %v", err)
	}

	if g.playerDying {
		t.Fatal("playerDying = true, want false after respawn")
	}
	if g.lives != 2 {
		t.Fatalf("lives = %d, want 2", g.lives)
	}
	if g.health != 100 {
		t.Fatalf("health = %d, want 100", g.health)
	}
	if g.weapon != 1 || g.bestWeapon != 1 || g.chosenWeapon != 1 {
		t.Fatalf("weapons = (%d,%d,%d), want (1,1,1)", g.weapon, g.bestWeapon, g.chosenWeapon)
	}
	if g.ammo != startAmmo {
		t.Fatalf("ammo = %d, want %d", g.ammo, startAmmo)
	}
	if g.keys != 0 {
		t.Fatalf("keys = %04b, want 0000", g.keys)
	}
	if g.hudNotice != "" {
		t.Fatalf("hudNotice = %q, want empty after respawn", g.hudNotice)
	}
}

func TestUpdatePlayerDeathGameOverAtNoLives(t *testing.T) {
	g := &game{
		health:  5,
		lives:   0,
		score:   1234,
		uiState: uiStatePlaying,
	}

	g.takePlayerDamage(10)

	if err := g.advancePlayerDeathTics(deathFadeTics + deathHoldTics); err != nil {
		t.Fatalf("advancePlayerDeathTics error: %v", err)
	}
	if err := g.advancePlayerDeathTics(1); err != nil {
		t.Fatalf("advancePlayerDeathTics error: %v", err)
	}

	if g.uiState != uiStateMainMenu {
		t.Fatalf("uiState = %v, want %v", g.uiState, uiStateMainMenu)
	}
	if g.lives != 3 {
		t.Fatalf("lives = %d, want 3 after new game reset", g.lives)
	}
	if g.score != 0 {
		t.Fatalf("score = %d, want 0 after game over reset", g.score)
	}
	if g.health != 100 {
		t.Fatalf("health = %d, want 100 after game over reset", g.health)
	}
}

func TestTakePlayerDamageStartsDamageFlash(t *testing.T) {
	g := &game{health: 100, lives: 3}

	g.takePlayerDamage(12)

	if g.damageFlash != 12 {
		t.Fatalf("damageFlash = %d, want 12", g.damageFlash)
	}
	clr, ok := g.currentScreenFlash()
	if !ok || clr.R != 255 || clr.G != 0 || clr.B != 0 || clr.A != uint8((255*2)/(redSteps*3)) {
		t.Fatalf("currentScreenFlash = %#v ok=%v, want red flash", clr, ok)
	}
}

func TestApplyPickupStartsBonusFlash(t *testing.T) {
	g := &game{health: 50}

	if !g.applyPickup(pickupFood) {
		t.Fatal("applyPickup(food) = false, want true")
	}
	if g.bonusFlash != numWhiteShifts*whiteTics {
		t.Fatalf("bonusFlash = %d, want %d", g.bonusFlash, numWhiteShifts*whiteTics)
	}
	clr, ok := g.currentScreenFlash()
	if !ok || clr.R != 255 || clr.G != 247 || clr.B != 0 || clr.A != uint8((255*numWhiteShifts)/(whiteSteps*3)) {
		t.Fatalf("currentScreenFlash = %#v ok=%v, want white flash", clr, ok)
	}
}

func TestStartSpriteSequence(t *testing.T) {
	g := &game{}
	spr := &staticSprite{}
	if !g.startSpriteSequence(spr, seqActorGuardDeath) {
		t.Fatal("startSpriteSequence failed")
	}
	if spr.sequenceID != seqActorGuardDeath || spr.shapenum != shapeGuardDie1 {
		t.Fatalf("sprite sequence state = %+v, want sequence %q shape %d", spr, seqActorGuardDeath, shapeGuardDie1)
	}
}

func TestSpriteRotateOffsetMatchesAngleReference(t *testing.T) {
	g := &game{}
	reference := func(spriteDX, spriteDY float64, facingDir int) int {
		angleToPlayer := math.Atan2(-spriteDY, -spriteDX)
		if angleToPlayer < 0 {
			angleToPlayer += 2 * math.Pi
		}

		facingAngle := facingDirAngle(facingDir)
		relative := facingAngle - angleToPlayer
		for relative < 0 {
			relative += 2 * math.Pi
		}
		for relative >= 2*math.Pi {
			relative -= 2 * math.Pi
		}
		relative += math.Pi / 8
		if relative >= 2*math.Pi {
			relative -= 2 * math.Pi
		}
		return int(relative / (math.Pi / 4))
	}

	for facingDir := 0; facingDir < 8; facingDir++ {
		for yi := -12; yi <= 12; yi++ {
			for xi := -12; xi <= 12; xi++ {
				if xi == 0 && yi == 0 {
					continue
				}
				spriteDX := float64(xi) / 3
				spriteDY := float64(yi) / 3
				want := reference(spriteDX, spriteDY, facingDir)
				got := g.spriteRotateOffset(spriteDX, spriteDY, facingDir)
				if got != want {
					t.Fatalf("spriteRotateOffset(%.3f, %.3f, %d) = %d, want %d", spriteDX, spriteDY, facingDir, got, want)
				}
			}
		}
	}
}

func TestColumnCoverageReserveMergesOverlaps(t *testing.T) {
	var c columnCoverage
	c.reserve(10, 20)
	c.reserve(30, 40)
	c.reserve(18, 35)

	if c.count != 1 {
		t.Fatalf("count = %d, want 1", c.count)
	}
	if got := c.spans[0]; got.start != 10 || got.end != 40 {
		t.Fatalf("merged span = %+v, want [10,40)", got)
	}
}

func TestColumnCoverageSubtractSplitsIntoVisibleSegments(t *testing.T) {
	var c columnCoverage
	c.reserve(10, 20)
	c.reserve(30, 40)

	var out [maxColumnSpanCount]columnSpan
	n := c.subtract(0, 50, &out)
	if n != 3 {
		t.Fatalf("segment count = %d, want 3", n)
	}
	want := [3]columnSpan{{0, 10}, {20, 30}, {40, 50}}
	for i := 0; i < n; i++ {
		if out[i] != want[i] {
			t.Fatalf("segment %d = %+v, want %+v", i, out[i], want[i])
		}
	}
}

func TestColumnCoverageSubtractFullyCovered(t *testing.T) {
	var c columnCoverage
	c.reserve(10, 40)

	var out [maxColumnSpanCount]columnSpan
	if n := c.subtract(15, 20, &out); n != 0 {
		t.Fatalf("segment count = %d, want 0", n)
	}
}
