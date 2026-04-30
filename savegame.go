package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/zeebo/blake3"

	"gd-wolf/internal/wl6"
)

const (
	saveGameVersion   = 4
	maxSaveNameLen    = 31
	saveFileMagic     = "GWLF"
	saveFileExtension = ".sav"
	saveFooterMagic   = "B3CK"
	saveChecksumSize  = 32
	autosaveSlotName  = "Autosave"
	autosaveSlotCount = 3
)

var errSaveUnsupported = errors.New("save games are not supported on this build")

type saveSlotSummary struct {
	Used            bool
	Path            string
	Name            string
	MapIndex        int
	Timestamp       time.Time
	Thumbnail       *saveThumbnailData
	IsAutosave      bool
	AutosaveOrdinal int

	thumbnailImage *ebiten.Image
}

type saveThumbnailData struct {
	PNG []byte `json:"png"`
}

type saveGameData struct {
	Version       int                `json:"version"`
	Name          string             `json:"name"`
	MapIndex      int                `json:"map_index"`
	Timestamp     time.Time          `json:"timestamp"`
	Thumbnail     *saveThumbnailData `json:"thumbnail,omitempty"`
	ContentTag    string             `json:"content_tag,omitempty"`
	ContentDigest []byte             `json:"content_digest,omitempty"`

	Difficulty         gameDifficulty   `json:"difficulty"`
	SelectedEpisode    int              `json:"selected_episode"`
	SelectedLevel      int              `json:"selected_level"`
	PendingMap         int              `json:"pending_map"`
	RNGIndex           byte             `json:"rng_index"`
	GameplayTickAccum  int              `json:"gameplay_tick_accum"`
	VictoryActive      bool             `json:"victory_active"`
	VictoryPhase       victoryPhase     `json:"victory_phase"`
	VictoryBJ          saveStaticSprite `json:"victory_bj"`
	VictoryRunDistance float64          `json:"victory_run_distance"`

	PlayerX float64 `json:"player_x"`
	PlayerY float64 `json:"player_y"`
	PlayerA float64 `json:"player_a"`
	CameraX float64 `json:"camera_x"`
	CameraY float64 `json:"camera_y"`
	Zoom    float64 `json:"zoom"`

	Weapon       int  `json:"weapon"`
	BestWeapon   int  `json:"best_weapon"`
	ChosenWeapon int  `json:"chosen_weapon"`
	Health       int  `json:"health"`
	Ammo         int  `json:"ammo"`
	Lives        int  `json:"lives"`
	Keys         byte `json:"keys"`
	Score        int  `json:"score"`

	SecretTotal   int `json:"secret_total"`
	SecretCount   int `json:"secret_count"`
	TreasureTotal int `json:"treasure_total"`
	TreasureCount int `json:"treasure_count"`

	Attacking       bool           `json:"attacking"`
	WeaponSequence  AnimSequenceID `json:"weapon_sequence"`
	WeaponFrameIdx  int            `json:"weapon_frame_idx"`
	WeaponFrameTics int            `json:"weapon_frame_tics"`
	DamageFlash     int            `json:"damage_flash"`
	BonusFlash      int            `json:"bonus_flash"`
	GodMode         bool           `json:"god_mode"`

	LevelState    saveLevelState     `json:"level_state"`
	PushWall      savePushWallState  `json:"push_wall"`
	StaticSprites []saveStaticSprite `json:"static_sprites"`
	Actors        []saveActor        `json:"actors"`
}

type saveLevelState struct {
	TileOverrides []saveTileOverride `json:"tile_overrides,omitempty"`
	Doors         []saveDoorRuntime  `json:"doors,omitempty"`
}

type saveTileOverride struct {
	Index int      `json:"index"`
	Tile  saveTile `json:"tile"`
}

type saveDoorRuntime struct {
	Index int     `json:"index"`
	Open  float64 `json:"open"`
	State byte    `json:"state"`
	Timer int     `json:"timer"`
}

type saveTile struct {
	RawWall    uint16    `json:"raw_wall"`
	RawInfo    uint16    `json:"raw_info"`
	Solid      bool      `json:"solid"`
	RenderWall bool      `json:"render_wall"`
	Area       int       `json:"area"`
	Ambush     bool      `json:"ambush"`
	Door       *wl6.Door `json:"door,omitempty"`
}

type savePushWallState struct {
	Active bool     `json:"active"`
	X      int      `json:"x"`
	Y      int      `json:"y"`
	DX     int      `json:"dx"`
	DY     int      `json:"dy"`
	Steps  int      `json:"steps"`
	Tics   int      `json:"tics"`
	Wall   saveTile `json:"wall"`
}

type saveStaticSprite struct {
	X          float64        `json:"x"`
	Y          float64        `json:"y"`
	ShapeNum   int            `json:"shape_num"`
	Rotate     bool           `json:"rotate"`
	FacingDir  int            `json:"facing_dir"`
	Blocking   bool           `json:"blocking"`
	Shootable  bool           `json:"shootable"`
	Alive      bool           `json:"alive"`
	Pickup     pickupType     `json:"pickup"`
	DropPickup pickupType     `json:"drop_pickup"`
	ScoreValue int            `json:"score_value"`
	DeathSeq   AnimSequenceID `json:"death_seq"`
	SequenceID AnimSequenceID `json:"sequence_id"`
	FrameIndex int            `json:"frame_index"`
	FrameTimer int            `json:"frame_timer"`
}

type saveActor struct {
	Kind      ActorKind `json:"kind"`
	ShapeNum  int       `json:"shape_num"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	TileX     int       `json:"tile_x"`
	TileY     int       `json:"tile_y"`
	GoalX     int       `json:"goal_x"`
	GoalY     int       `json:"goal_y"`
	HasGoal   bool      `json:"has_goal"`
	Dir       int       `json:"dir"`
	FacingDir int       `json:"facing_dir"`
	Rotate    bool      `json:"rotate"`

	Blocking    bool `json:"blocking"`
	Shootable   bool `json:"shootable"`
	Alive       bool `json:"alive"`
	Alerted     bool `json:"alerted"`
	Ambush      bool `json:"ambush"`
	FirstAttack bool `json:"first_attack"`
	Area        int  `json:"area"`

	Health      int            `json:"health"`
	PatrolSpeed float64        `json:"patrol_speed"`
	ChaseSpeed  float64        `json:"chase_speed"`
	MoveDistance float64       `json:"move_distance"`
	ScoreValue  int            `json:"score_value"`
	DropPickup  pickupType     `json:"drop_pickup"`
	StandSeq    AnimSequenceID `json:"stand_seq"`
	PatrolSeq   AnimSequenceID `json:"patrol_seq"`
	ChaseSeq    AnimSequenceID `json:"chase_seq"`
	PainSeq     AnimSequenceID `json:"pain_seq"`
	ShootSeq    AnimSequenceID `json:"shoot_seq"`
	JumpSeq     AnimSequenceID `json:"jump_seq"`
	DeathSeq    AnimSequenceID `json:"death_seq"`

	AIState         ActorAIState   `json:"ai_state"`
	SpawnMode       ActorSpawnMode `json:"spawn_mode"`
	ReactionTimer   int            `json:"reaction_timer"`
	SequenceID      AnimSequenceID `json:"sequence_id"`
	SequenceLoop    bool           `json:"sequence_loop"`
	FrameIndex      int            `json:"frame_index"`
	FrameTimer      int            `json:"frame_timer"`
	FrameActionDone bool           `json:"frame_action_done"`
}

func slotSummaryLabel(summary saveSlotSummary) string {
	if summary.IsAutosave {
		if summary.AutosaveOrdinal > 0 {
			return fmt.Sprintf("%s %d", autosaveSlotName, summary.AutosaveOrdinal)
		}
		return autosaveSlotName
	}
	if !summary.Used {
		return "      - Empty -"
	}
	name := strings.TrimSpace(summary.Name)
	if name == "" {
		name = "Unnamed Save"
	}
	return name
}

func clampSaveName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	if len(runes) > maxSaveNameLen {
		runes = runes[:maxSaveNameLen]
	}
	return string(runes)
}

func (g *game) defaultSaveName() string {
	name := fmt.Sprintf("Floor %02d", g.mapIndex+1)
	if g.mapData != nil {
		mapName := strings.TrimSpace(g.mapData.Name())
		if mapName != "" {
			name = fmt.Sprintf("%02d %s", g.mapIndex, mapName)
		}
	}
	return clampSaveName(name)
}

func (g *game) autosaveName() string {
	if g.mapIndex >= 0 {
		return clampSaveName(fmt.Sprintf("%s Floor %02d", autosaveSlotName, g.mapIndex+1))
	}
	return autosaveSlotName
}

func (g *game) reloadSaveSlots() error {
	slots, err := g.listSaveSlots()
	if err != nil {
		g.saveSlots = nil
		return err
	}
	g.saveSlots = slots
	return nil
}

func autosavePlaceholderSummaries() []saveSlotSummary {
	autosaves := make([]saveSlotSummary, autosaveSlotCount)
	for i := range autosaves {
		autosaves[i] = saveSlotSummary{
			Used:            false,
			Path:            autosaveSlotPath(i),
			Name:            autosaveSlotName,
			IsAutosave:      true,
			AutosaveOrdinal: i + 1,
		}
	}
	return autosaves
}

func saveTileFromLevelTile(tile wl6.Tile) saveTile {
	return saveTile{
		RawWall:    tile.RawWall,
		RawInfo:    tile.RawInfo,
		Solid:      tile.Solid,
		RenderWall: tile.RenderWall,
		Area:       tile.Area,
		Ambush:     tile.Ambush,
		Door:       tile.Door,
	}
}

func levelTileFromSave(tile saveTile) wl6.Tile {
	return wl6.Tile{
		RawWall:    tile.RawWall,
		RawInfo:    tile.RawInfo,
		Solid:      tile.Solid,
		RenderWall: tile.RenderWall,
		Area:       tile.Area,
		Ambush:     tile.Ambush,
		Door:       tile.Door,
	}
}

func saveTilesEqual(a, b saveTile) bool {
	if a.RawWall != b.RawWall || a.RawInfo != b.RawInfo || a.Solid != b.Solid || a.RenderWall != b.RenderWall || a.Area != b.Area || a.Ambush != b.Ambush {
		return false
	}
	if a.Door == nil || b.Door == nil {
		return a.Door == nil && b.Door == nil
	}
	return a.Door.Vertical == b.Door.Vertical && a.Door.Lock == b.Door.Lock
}

func saveStaticSpriteFromGame(s staticSprite) saveStaticSprite {
	return saveStaticSprite{
		X:          s.x,
		Y:          s.y,
		ShapeNum:   s.shapenum,
		Rotate:     s.rotate,
		FacingDir:  s.facingDir,
		Blocking:   s.blocking,
		Shootable:  s.shootable,
		Alive:      s.alive,
		Pickup:     s.pickup,
		DropPickup: s.dropPickup,
		ScoreValue: s.scoreValue,
		DeathSeq:   s.deathSequence,
		SequenceID: s.sequenceID,
		FrameIndex: s.frameIndex,
		FrameTimer: s.frameTimer,
	}
}

func staticSpriteFromSave(s saveStaticSprite) staticSprite {
	return staticSprite{
		x:             s.X,
		y:             s.Y,
		shapenum:      s.ShapeNum,
		rotate:        s.Rotate,
		facingDir:     s.FacingDir,
		blocking:      s.Blocking,
		shootable:     s.Shootable,
		alive:         s.Alive,
		pickup:        s.Pickup,
		dropPickup:    s.DropPickup,
		scoreValue:    s.ScoreValue,
		deathSequence: s.DeathSeq,
		sequenceID:    s.SequenceID,
		frameIndex:    s.FrameIndex,
		frameTimer:    s.FrameTimer,
	}
}

func saveActorFromGame(a actorInstance) saveActor {
	return saveActor{
		Kind:            a.kind,
		ShapeNum:        a.shapenum,
		X:               a.x,
		Y:               a.y,
		TileX:           a.tileX,
		TileY:           a.tileY,
		GoalX:           a.tileX,
		GoalY:           a.tileY,
		HasGoal:         a.hasGoal,
		Dir:             a.dir,
		FacingDir:       a.facingDir,
		Rotate:          a.rotate,
		Blocking:        a.blocking,
		Shootable:       a.shootable,
		Alive:           a.alive,
		Alerted:         a.alerted,
		Ambush:          a.ambush,
		FirstAttack:     a.firstAttack,
		Area:            a.area,
		Health:          a.health,
		PatrolSpeed:     a.patrolSpeed,
		ChaseSpeed:      a.chaseSpeed,
		MoveDistance:    a.moveDistance,
		ScoreValue:      a.scoreValue,
		DropPickup:      a.dropPickup,
		StandSeq:        a.standSeq,
		PatrolSeq:       a.patrolSeq,
		ChaseSeq:        a.chaseSeq,
		PainSeq:         a.painSeq,
		ShootSeq:        a.shootSeq,
		JumpSeq:         a.jumpSeq,
		DeathSeq:        a.deathSeq,
		AIState:         a.aiState,
		SpawnMode:       a.spawnMode,
		ReactionTimer:   a.reactionTimer,
		SequenceID:      a.sequenceID,
		SequenceLoop:    a.sequenceLoop,
		FrameIndex:      a.frameIndex,
		FrameTimer:      a.frameTimer,
		FrameActionDone: a.frameActionDone,
	}
}

func actorFromSave(a saveActor) actorInstance {
	actor := actorInstance{
		kind:            a.Kind,
		shapenum:        a.ShapeNum,
		x:               a.X,
		y:               a.Y,
		tileX:           a.TileX,
		tileY:           a.TileY,
		dir:             a.Dir,
		facingDir:       a.FacingDir,
		rotate:          a.Rotate,
		blocking:        a.Blocking,
		shootable:       a.Shootable,
		alive:           a.Alive,
		alerted:         a.Alerted,
		ambush:          a.Ambush,
		firstAttack:     a.FirstAttack,
		area:            a.Area,
		health:          a.Health,
		patrolSpeed:     a.PatrolSpeed,
		chaseSpeed:      a.ChaseSpeed,
		moveDistance:    a.MoveDistance,
		scoreValue:      a.ScoreValue,
		dropPickup:      a.DropPickup,
		standSeq:        a.StandSeq,
		patrolSeq:       a.PatrolSeq,
		chaseSeq:        a.ChaseSeq,
		painSeq:         a.PainSeq,
		shootSeq:        a.ShootSeq,
		jumpSeq:         a.JumpSeq,
		deathSeq:        a.DeathSeq,
		aiState:         a.AIState,
		spawnMode:       a.SpawnMode,
		reactionTimer:   a.ReactionTimer,
		sequenceID:      a.SequenceID,
		sequenceLoop:    a.SequenceLoop,
		frameIndex:      a.FrameIndex,
		frameTimer:      a.FrameTimer,
		frameActionDone: a.FrameActionDone,
	}
	if a.HasGoal {
		actor.reserveTileGoal(a.TileX, a.TileY)
	} else {
		actor.clearTileGoal()
	}
	return actor
}

func (g *game) captureSaveGame(name string) saveGameData {
	levelState := saveLevelState{}
	var baseLevel *wl6.Level
	if g.mapData != nil {
		baseLevel = g.mapData.Level()
	}
	if baseLevel != nil && len(baseLevel.Tiles) == len(g.level.Tiles) {
		for i, tile := range g.level.Tiles {
			savedTile := saveTileFromLevelTile(tile)
			baseTile := saveTileFromLevelTile(baseLevel.Tiles[i])
			if saveTilesEqual(savedTile, baseTile) {
				continue
			}
			levelState.TileOverrides = append(levelState.TileOverrides, saveTileOverride{
				Index: i,
				Tile:  savedTile,
			})
		}
	} else {
		for i, tile := range g.level.Tiles {
			levelState.TileOverrides = append(levelState.TileOverrides, saveTileOverride{
				Index: i,
				Tile:  saveTileFromLevelTile(tile),
			})
		}
	}
	for i := range g.doorOpen {
		if g.doorOpen[i] == 0 && g.doorState[i] == 0 && g.doorTimer[i] == 0 {
			continue
		}
		levelState.Doors = append(levelState.Doors, saveDoorRuntime{
			Index: i,
			Open:  g.doorOpen[i],
			State: g.doorState[i],
			Timer: g.doorTimer[i],
		})
	}
	statics := make([]saveStaticSprite, len(g.staticSprites))
	for i, spr := range g.staticSprites {
		statics[i] = saveStaticSpriteFromGame(spr)
	}
	actors := make([]saveActor, len(g.actors))
	for i, actor := range g.actors {
		actors[i] = saveActorFromGame(actor)
	}
	return saveGameData{
		Version:            saveGameVersion,
		Name:               clampSaveName(name),
		MapIndex:           g.mapIndex,
		Timestamp:          time.Now(),
		Thumbnail:          g.captureSaveThumbnail(),
		ContentTag:         g.saveContentTag(),
		ContentDigest:      g.saveContentDigest(),
		Difficulty:         g.difficulty,
		SelectedEpisode:    g.selectedEpisode,
		SelectedLevel:      g.selectedLevel,
		PendingMap:         g.pendingMap,
		RNGIndex:           g.rng.Index(),
		GameplayTickAccum:  g.gameplayTickAccum,
		VictoryActive:      g.victoryActive,
		VictoryPhase:       g.victoryPhase,
		VictoryBJ:          saveStaticSpriteFromGame(g.victoryBJ),
		VictoryRunDistance: g.victoryRunDistance,
		PlayerX:            g.playerX,
		PlayerY:            g.playerY,
		PlayerA:            g.playerA,
		CameraX:            g.cameraX,
		CameraY:            g.cameraY,
		Zoom:               g.zoom,
		Weapon:             g.weapon,
		BestWeapon:         g.bestWeapon,
		ChosenWeapon:       g.chosenWeapon,
		Health:             g.health,
		Ammo:               g.ammo,
		Lives:              g.lives,
		Keys:               g.keys,
		Score:              g.score,
		SecretTotal:        g.secretTotal,
		SecretCount:        g.secretCount,
		TreasureTotal:      g.treasureTotal,
		TreasureCount:      g.treasureCount,
		Attacking:          g.attacking,
		WeaponSequence:     g.weaponSequence,
		WeaponFrameIdx:     g.weaponFrameIdx,
		WeaponFrameTics:    g.weaponFrameTics,
		DamageFlash:        g.damageFlash,
		BonusFlash:         g.bonusFlash,
		GodMode:            g.godMode,
		LevelState:         levelState,
		PushWall: savePushWallState{
			Active: g.pushWall.active,
			X:      g.pushWall.x,
			Y:      g.pushWall.y,
			DX:     g.pushWall.dx,
			DY:     g.pushWall.dy,
			Steps:  g.pushWall.steps,
			Tics:   g.pushWall.tics,
			Wall:   saveTileFromLevelTile(g.pushWall.wall),
		},
		StaticSprites: statics,
		Actors:        actors,
	}
}

func (g *game) saveContentTag() string {
	if g == nil || g.files == nil {
		return ""
	}
	return g.files.Variant.Ext
}

func (g *game) saveContentDigest() []byte {
	if g == nil || g.files == nil {
		return nil
	}
	hasher := blake3.New()
	hasher.Write([]byte(g.files.Variant.Ext))
	hasher.Write(g.files.MapHead)
	hasher.Write(g.files.GameMaps)
	sum := hasher.Sum(nil)
	return append([]byte(nil), sum...)
}

func (g *game) captureSaveThumbnail() *saveThumbnailData {
	if g == nil || g.walls == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
		}
	}()
	pngData, err := g.renderSaveThumbnailPNG()
	if err != nil || len(pngData) == 0 {
		return nil
	}
	return &saveThumbnailData{PNG: pngData}
}

func (g *game) renderSaveThumbnailPNG() ([]byte, error) {
	thumbGame := *g
	thumbGame.renderThreads = 1
	thumbGame.paused = false
	thumbGame.viewWidth = 0
	thumbGame.viewHeight = 0
	thumbGame.frame32 = nil
	thumbGame.frame = nil
	thumbGame.background32 = nil
	thumbGame.background = nil
	thumbGame.backgroundImage = nil
	thumbGame.cameraColumns = nil
	thumbGame.rayDirXColumns = nil
	thumbGame.rayDirYColumns = nil
	thumbGame.prevWallTops = nil
	thumbGame.prevWallBottoms = nil
	thumbGame.zbuffer = nil
	thumbGame.gameplayImage = nil
	thumbGame.gameplayFrame32 = nil
	thumbGame.gameplayFrame = nil
	thumbGame.gameplayBackground32 = nil
	thumbGame.gameplayBackground = nil
	thumbGame.raycastJobs = nil
	thumbGame.raycastWorkerCount = 0
	thumbGame.layout = wolfRenderLayout{}
	thumbGame.ensureFrame(wolfScreenWidth, wolfDisplayHeight)

	screen := ebiten.NewImage(wolfScreenWidth, wolfDisplayHeight)
	thumbGame.drawRaycast(screen)

	pixels := make([]byte, wolfScreenWidth*wolfDisplayHeight*4)
	screen.ReadPixels(pixels)
	return encodeThumbnailPNG(pixels, wolfScreenWidth, wolfDisplayHeight)
}

func (summary *saveSlotSummary) thumbnailImageForMenu() *ebiten.Image {
	if summary == nil || summary.Thumbnail == nil {
		return nil
	}
	thumb := summary.Thumbnail
	if len(thumb.PNG) == 0 {
		return nil
	}
	if summary.thumbnailImage == nil {
		img, _, err := image.Decode(bytes.NewReader(thumb.PNG))
		if err != nil {
			return nil
		}
		summary.thumbnailImage = ebiten.NewImageFromImage(img)
	}
	return summary.thumbnailImage
}

func encodeThumbnailPNG(pixels []byte, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 || len(pixels) != width*height*4 {
		return nil, fmt.Errorf("invalid thumbnail buffer %dx%d", width, height)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, pixels)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type saveBinaryWriter struct {
	buf bytes.Buffer
	err error
}

type saveBinaryReader struct {
	data []byte
	pos  int
	err  error
}

func (w *saveBinaryWriter) writeBytes(data []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.buf.Write(data)
}

func (w *saveBinaryWriter) writeUint8(v byte) {
	if w.err != nil {
		return
	}
	w.err = w.buf.WriteByte(v)
}

func (w *saveBinaryWriter) writeBool(v bool) {
	if v {
		w.writeUint8(1)
		return
	}
	w.writeUint8(0)
}

func (w *saveBinaryWriter) writeUint16(v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	w.writeBytes(buf[:])
}

func (w *saveBinaryWriter) writeUint32(v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	w.writeBytes(buf[:])
}

func (w *saveBinaryWriter) writeInt32(v int) {
	if w.err != nil {
		return
	}
	if v < -1<<31 || v > 1<<31-1 {
		w.err = fmt.Errorf("save integer %d out of int32 range", v)
		return
	}
	w.writeUint32(uint32(int32(v)))
}

func (w *saveBinaryWriter) writeInt64(v int64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(v))
	w.writeBytes(buf[:])
}

func (w *saveBinaryWriter) writeFloat64(v float64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
	w.writeBytes(buf[:])
}

func (w *saveBinaryWriter) writeString(v string) {
	w.writeBlob([]byte(v))
}

func (w *saveBinaryWriter) writeBlob(data []byte) {
	if w.err != nil {
		return
	}
	if len(data) > int(^uint32(0)) {
		w.err = fmt.Errorf("save blob length %d overflows uint32", len(data))
		return
	}
	w.writeUint32(uint32(len(data)))
	w.writeBytes(data)
}

func (r *saveBinaryReader) read(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.pos+n > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return nil
	}
	out := r.data[r.pos : r.pos+n]
	r.pos += n
	return out
}

func (r *saveBinaryReader) readUint8() byte {
	data := r.read(1)
	if data == nil {
		return 0
	}
	return data[0]
}

func (r *saveBinaryReader) readBool() bool {
	return r.readUint8() != 0
}

func (r *saveBinaryReader) readUint16() uint16 {
	data := r.read(2)
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

func (r *saveBinaryReader) readUint32() uint32 {
	data := r.read(4)
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}

func (r *saveBinaryReader) readInt32() int {
	return int(int32(r.readUint32()))
}

func (r *saveBinaryReader) readInt64() int64 {
	data := r.read(8)
	if data == nil {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(data))
}

func (r *saveBinaryReader) readFloat64() float64 {
	data := r.read(8)
	if data == nil {
		return 0
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(data))
}

func (r *saveBinaryReader) readBlob() []byte {
	length := r.readUint32()
	if r.err != nil {
		return nil
	}
	return append([]byte(nil), r.read(int(length))...)
}

func (r *saveBinaryReader) readString() string {
	return string(r.readBlob())
}

func (w *saveBinaryWriter) writeDoor(door *wl6.Door) {
	w.writeBool(door != nil)
	if door == nil {
		return
	}
	w.writeBool(door.Vertical)
	w.writeInt32(door.Lock)
}

func (r *saveBinaryReader) readDoor() *wl6.Door {
	if !r.readBool() {
		return nil
	}
	return &wl6.Door{
		Vertical: r.readBool(),
		Lock:     r.readInt32(),
	}
}

func (w *saveBinaryWriter) writeTile(tile saveTile) {
	w.writeUint16(tile.RawWall)
	w.writeUint16(tile.RawInfo)
	w.writeBool(tile.Solid)
	w.writeBool(tile.RenderWall)
	w.writeInt32(tile.Area)
	w.writeBool(tile.Ambush)
	w.writeDoor(tile.Door)
}

func (r *saveBinaryReader) readTile() saveTile {
	return saveTile{
		RawWall:    r.readUint16(),
		RawInfo:    r.readUint16(),
		Solid:      r.readBool(),
		RenderWall: r.readBool(),
		Area:       r.readInt32(),
		Ambush:     r.readBool(),
		Door:       r.readDoor(),
	}
}

func (w *saveBinaryWriter) writeStaticSprite(s saveStaticSprite) {
	w.writeFloat64(s.X)
	w.writeFloat64(s.Y)
	w.writeInt32(s.ShapeNum)
	w.writeBool(s.Rotate)
	w.writeInt32(s.FacingDir)
	w.writeBool(s.Blocking)
	w.writeBool(s.Shootable)
	w.writeBool(s.Alive)
	w.writeInt32(int(s.Pickup))
	w.writeInt32(int(s.DropPickup))
	w.writeInt32(s.ScoreValue)
	w.writeString(string(s.DeathSeq))
	w.writeString(string(s.SequenceID))
	w.writeInt32(s.FrameIndex)
	w.writeInt32(s.FrameTimer)
}

func (r *saveBinaryReader) readStaticSprite() saveStaticSprite {
	return saveStaticSprite{
		X:          r.readFloat64(),
		Y:          r.readFloat64(),
		ShapeNum:   r.readInt32(),
		Rotate:     r.readBool(),
		FacingDir:  r.readInt32(),
		Blocking:   r.readBool(),
		Shootable:  r.readBool(),
		Alive:      r.readBool(),
		Pickup:     pickupType(r.readInt32()),
		DropPickup: pickupType(r.readInt32()),
		ScoreValue: r.readInt32(),
		DeathSeq:   AnimSequenceID(r.readString()),
		SequenceID: AnimSequenceID(r.readString()),
		FrameIndex: r.readInt32(),
		FrameTimer: r.readInt32(),
	}
}

func (w *saveBinaryWriter) writeActor(a saveActor) {
	w.writeInt32(int(a.Kind))
	w.writeInt32(a.ShapeNum)
	w.writeFloat64(a.X)
	w.writeFloat64(a.Y)
	w.writeInt32(a.TileX)
	w.writeInt32(a.TileY)
	w.writeInt32(a.GoalX)
	w.writeInt32(a.GoalY)
	w.writeBool(a.HasGoal)
	w.writeInt32(a.Dir)
	w.writeInt32(a.FacingDir)
	w.writeBool(a.Rotate)
	w.writeBool(a.Blocking)
	w.writeBool(a.Shootable)
	w.writeBool(a.Alive)
	w.writeBool(a.Alerted)
	w.writeBool(a.Ambush)
	w.writeBool(a.FirstAttack)
	w.writeInt32(a.Area)
	w.writeInt32(a.Health)
	w.writeFloat64(a.PatrolSpeed)
	w.writeFloat64(a.ChaseSpeed)
	w.writeFloat64(a.MoveDistance)
	w.writeInt32(a.ScoreValue)
	w.writeInt32(int(a.DropPickup))
	w.writeString(string(a.StandSeq))
	w.writeString(string(a.PatrolSeq))
	w.writeString(string(a.ChaseSeq))
	w.writeString(string(a.PainSeq))
	w.writeString(string(a.ShootSeq))
	w.writeString(string(a.JumpSeq))
	w.writeString(string(a.DeathSeq))
	w.writeInt32(int(a.AIState))
	w.writeInt32(int(a.SpawnMode))
	w.writeInt32(a.ReactionTimer)
	w.writeString(string(a.SequenceID))
	w.writeBool(a.SequenceLoop)
	w.writeInt32(a.FrameIndex)
	w.writeInt32(a.FrameTimer)
	w.writeBool(a.FrameActionDone)
}

func (r *saveBinaryReader) readActor() saveActor {
	return saveActor{
		Kind:            ActorKind(r.readInt32()),
		ShapeNum:        r.readInt32(),
		X:               r.readFloat64(),
		Y:               r.readFloat64(),
		TileX:           r.readInt32(),
		TileY:           r.readInt32(),
		GoalX:           r.readInt32(),
		GoalY:           r.readInt32(),
		HasGoal:         r.readBool(),
		Dir:             r.readInt32(),
		FacingDir:       r.readInt32(),
		Rotate:          r.readBool(),
		Blocking:        r.readBool(),
		Shootable:       r.readBool(),
		Alive:           r.readBool(),
		Alerted:         r.readBool(),
		Ambush:          r.readBool(),
		FirstAttack:     r.readBool(),
		Area:            r.readInt32(),
		Health:          r.readInt32(),
		PatrolSpeed:     r.readFloat64(),
		ChaseSpeed:      r.readFloat64(),
		MoveDistance:    r.readFloat64(),
		ScoreValue:      r.readInt32(),
		DropPickup:      pickupType(r.readInt32()),
		StandSeq:        AnimSequenceID(r.readString()),
		PatrolSeq:       AnimSequenceID(r.readString()),
		ChaseSeq:        AnimSequenceID(r.readString()),
		PainSeq:         AnimSequenceID(r.readString()),
		ShootSeq:        AnimSequenceID(r.readString()),
		JumpSeq:         AnimSequenceID(r.readString()),
		DeathSeq:        AnimSequenceID(r.readString()),
		AIState:         ActorAIState(r.readInt32()),
		SpawnMode:       ActorSpawnMode(r.readInt32()),
		ReactionTimer:   r.readInt32(),
		SequenceID:      AnimSequenceID(r.readString()),
		SequenceLoop:    r.readBool(),
		FrameIndex:      r.readInt32(),
		FrameTimer:      r.readInt32(),
		FrameActionDone: r.readBool(),
	}
}

func writeSaveGamePayload(data saveGameData) ([]byte, error) {
	var w saveBinaryWriter
	w.writeInt32(data.Version)
	w.writeString(data.Name)
	w.writeInt32(data.MapIndex)
	w.writeInt64(data.Timestamp.UnixNano())
	w.writeBool(data.Thumbnail != nil)
	if data.Thumbnail != nil {
		w.writeBlob(data.Thumbnail.PNG)
	}
	w.writeString(data.ContentTag)
	w.writeBlob(data.ContentDigest)
	w.writeInt32(int(data.Difficulty))
	w.writeInt32(data.SelectedEpisode)
	w.writeInt32(data.SelectedLevel)
	w.writeInt32(data.PendingMap)
	w.writeUint8(data.RNGIndex)
	w.writeInt32(data.GameplayTickAccum)
	w.writeBool(data.VictoryActive)
	w.writeInt32(int(data.VictoryPhase))
	w.writeStaticSprite(data.VictoryBJ)
	w.writeFloat64(data.VictoryRunDistance)
	w.writeFloat64(data.PlayerX)
	w.writeFloat64(data.PlayerY)
	w.writeFloat64(data.PlayerA)
	w.writeFloat64(data.CameraX)
	w.writeFloat64(data.CameraY)
	w.writeFloat64(data.Zoom)
	w.writeInt32(data.Weapon)
	w.writeInt32(data.BestWeapon)
	w.writeInt32(data.ChosenWeapon)
	w.writeInt32(data.Health)
	w.writeInt32(data.Ammo)
	w.writeInt32(data.Lives)
	w.writeUint8(data.Keys)
	w.writeInt32(data.Score)
	w.writeInt32(data.SecretTotal)
	w.writeInt32(data.SecretCount)
	w.writeInt32(data.TreasureTotal)
	w.writeInt32(data.TreasureCount)
	w.writeBool(data.Attacking)
	w.writeString(string(data.WeaponSequence))
	w.writeInt32(data.WeaponFrameIdx)
	w.writeInt32(data.WeaponFrameTics)
	w.writeInt32(data.DamageFlash)
	w.writeInt32(data.BonusFlash)
	w.writeBool(data.GodMode)
	w.writeUint32(uint32(len(data.LevelState.TileOverrides)))
	for _, override := range data.LevelState.TileOverrides {
		w.writeInt32(override.Index)
		w.writeTile(override.Tile)
	}
	w.writeUint32(uint32(len(data.LevelState.Doors)))
	for _, door := range data.LevelState.Doors {
		w.writeInt32(door.Index)
		w.writeFloat64(door.Open)
		w.writeUint8(door.State)
		w.writeInt32(door.Timer)
	}
	w.writeBool(data.PushWall.Active)
	w.writeInt32(data.PushWall.X)
	w.writeInt32(data.PushWall.Y)
	w.writeInt32(data.PushWall.DX)
	w.writeInt32(data.PushWall.DY)
	w.writeInt32(data.PushWall.Steps)
	w.writeInt32(data.PushWall.Tics)
	w.writeTile(data.PushWall.Wall)
	w.writeUint32(uint32(len(data.StaticSprites)))
	for _, spr := range data.StaticSprites {
		w.writeStaticSprite(spr)
	}
	w.writeUint32(uint32(len(data.Actors)))
	for _, actor := range data.Actors {
		w.writeActor(actor)
	}
	if w.err != nil {
		return nil, w.err
	}
	return w.buf.Bytes(), nil
}

func readSaveGamePayload(payload []byte) (saveGameData, error) {
	r := &saveBinaryReader{data: payload}
	save := saveGameData{
		Version:   r.readInt32(),
		Name:      r.readString(),
		MapIndex:  r.readInt32(),
		Timestamp: time.Unix(0, r.readInt64()).UTC(),
	}
	if r.readBool() {
		save.Thumbnail = &saveThumbnailData{PNG: r.readBlob()}
	}
	save.ContentTag = r.readString()
	save.ContentDigest = r.readBlob()
	save.Difficulty = gameDifficulty(r.readInt32())
	save.SelectedEpisode = r.readInt32()
	save.SelectedLevel = r.readInt32()
	save.PendingMap = r.readInt32()
	save.RNGIndex = r.readUint8()
	save.GameplayTickAccum = r.readInt32()
	save.VictoryActive = r.readBool()
	save.VictoryPhase = victoryPhase(r.readInt32())
	save.VictoryBJ = r.readStaticSprite()
	save.VictoryRunDistance = r.readFloat64()
	save.PlayerX = r.readFloat64()
	save.PlayerY = r.readFloat64()
	save.PlayerA = r.readFloat64()
	save.CameraX = r.readFloat64()
	save.CameraY = r.readFloat64()
	save.Zoom = r.readFloat64()
	save.Weapon = r.readInt32()
	save.BestWeapon = r.readInt32()
	save.ChosenWeapon = r.readInt32()
	save.Health = r.readInt32()
	save.Ammo = r.readInt32()
	save.Lives = r.readInt32()
	save.Keys = r.readUint8()
	save.Score = r.readInt32()
	save.SecretTotal = r.readInt32()
	save.SecretCount = r.readInt32()
	save.TreasureTotal = r.readInt32()
	save.TreasureCount = r.readInt32()
	save.Attacking = r.readBool()
	save.WeaponSequence = AnimSequenceID(r.readString())
	save.WeaponFrameIdx = r.readInt32()
	save.WeaponFrameTics = r.readInt32()
	save.DamageFlash = r.readInt32()
	save.BonusFlash = r.readInt32()
	save.GodMode = r.readBool()

	tileCount := r.readUint32()
	save.LevelState.TileOverrides = make([]saveTileOverride, int(tileCount))
	for i := range save.LevelState.TileOverrides {
		save.LevelState.TileOverrides[i] = saveTileOverride{
			Index: r.readInt32(),
			Tile:  r.readTile(),
		}
	}
	doorCount := r.readUint32()
	save.LevelState.Doors = make([]saveDoorRuntime, int(doorCount))
	for i := range save.LevelState.Doors {
		save.LevelState.Doors[i] = saveDoorRuntime{
			Index: r.readInt32(),
			Open:  r.readFloat64(),
			State: r.readUint8(),
			Timer: r.readInt32(),
		}
	}
	save.PushWall = savePushWallState{
		Active: r.readBool(),
		X:      r.readInt32(),
		Y:      r.readInt32(),
		DX:     r.readInt32(),
		DY:     r.readInt32(),
		Steps:  r.readInt32(),
		Tics:   r.readInt32(),
		Wall:   r.readTile(),
	}
	staticCount := r.readUint32()
	save.StaticSprites = make([]saveStaticSprite, int(staticCount))
	for i := range save.StaticSprites {
		save.StaticSprites[i] = r.readStaticSprite()
	}
	actorCount := r.readUint32()
	save.Actors = make([]saveActor, int(actorCount))
	for i := range save.Actors {
		save.Actors[i] = r.readActor()
	}
	if r.err != nil {
		return save, r.err
	}
	if r.pos != len(r.data) {
		return save, fmt.Errorf("save payload has %d trailing bytes", len(r.data)-r.pos)
	}
	return save, nil
}

func marshalSaveGame(data saveGameData) ([]byte, error) {
	payload, err := writeSaveGamePayload(data)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	encoded.WriteString(saveFileMagic)
	gz := gzip.NewWriter(&encoded)
	if _, err := gz.Write(payload); err != nil {
		gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	digest := blake3.Sum256(encoded.Bytes())
	encoded.WriteString(saveFooterMagic)
	encoded.Write(digest[:])
	return encoded.Bytes(), nil
}

func parseSaveGame(data []byte) (saveGameData, error) {
	var save saveGameData
	if !bytes.HasPrefix(data, []byte(saveFileMagic)) {
		return save, errors.New("unsupported save format")
	}
	minSize := len(saveFileMagic) + len(saveFooterMagic) + saveChecksumSize
	if len(data) < minSize {
		return save, errors.New("save file is truncated")
	}
	footerStart := len(data) - len(saveFooterMagic) - saveChecksumSize
	if !bytes.Equal(data[footerStart:footerStart+len(saveFooterMagic)], []byte(saveFooterMagic)) {
		return save, errors.New("save file footer is missing")
	}
	payloadEnd := footerStart
	expected := data[footerStart+len(saveFooterMagic):]
	digest := blake3.Sum256(data[:payloadEnd])
	if !bytes.Equal(expected, digest[:]) {
		return save, errors.New("save file checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data[len(saveFileMagic):payloadEnd]))
	if err != nil {
		return save, err
	}
	defer gz.Close()
	payload, err := io.ReadAll(gz)
	if err != nil {
		return save, err
	}
	save, err = readSaveGamePayload(payload)
	if err != nil {
		return save, err
	}
	if save.Version != saveGameVersion {
		return save, fmt.Errorf("unsupported save version %d", save.Version)
	}
	return save, nil
}

func (g *game) applySaveGame(save saveGameData) error {
	if g.files == nil {
		return errors.New("game data is not loaded")
	}
	if err := g.validateSaveContent(save); err != nil {
		return err
	}
	g.difficulty = save.Difficulty
	if err := g.setMap(save.MapIndex); err != nil {
		return err
	}
	for _, override := range save.LevelState.TileOverrides {
		if override.Index < 0 || override.Index >= len(g.level.Tiles) {
			return fmt.Errorf("save tile override index %d out of range", override.Index)
		}
		g.level.Tiles[override.Index] = levelTileFromSave(override.Tile)
	}
	g.cachedWallIDs = g.computeWallIDs()
	g.cachedDoorFlags = g.computeDoorFlags()
	g.reachable = g.computeReachableTiles()
	g.renderableWalls = g.computeRenderableWalls()
	g.cachedDoorSides = g.computeDoorSides()

	for _, door := range save.LevelState.Doors {
		if door.Index < 0 || door.Index >= len(g.doorOpen) {
			return fmt.Errorf("save door index %d out of range", door.Index)
		}
		g.doorOpen[door.Index] = door.Open
		g.doorState[door.Index] = door.State
		g.doorTimer[door.Index] = door.Timer
	}
	g.pushWall = pushWallState{
		active: save.PushWall.Active,
		x:      save.PushWall.X,
		y:      save.PushWall.Y,
		dx:     save.PushWall.DX,
		dy:     save.PushWall.DY,
		steps:  save.PushWall.Steps,
		tics:   save.PushWall.Tics,
		wall:   levelTileFromSave(save.PushWall.Wall),
	}

	g.staticSprites = make([]staticSprite, len(save.StaticSprites))
	for i, spr := range save.StaticSprites {
		g.staticSprites[i] = staticSpriteFromSave(spr)
	}
	g.actors = make([]actorInstance, len(save.Actors))
	for i, actor := range save.Actors {
		g.actors[i] = actorFromSave(actor)
	}

	g.selectedEpisode = save.SelectedEpisode
	g.selectedLevel = save.SelectedLevel
	g.pendingMap = save.PendingMap
	g.rng = newWolfRNG(save.RNGIndex)
	g.gameplayTickAccum = save.GameplayTickAccum
	g.autosaveTickAccum = 0
	g.victoryActive = save.VictoryActive
	g.victoryPhase = save.VictoryPhase
	g.victoryBJ = staticSpriteFromSave(save.VictoryBJ)
	g.victoryRunDistance = save.VictoryRunDistance
	g.playerX = save.PlayerX
	g.playerY = save.PlayerY
	g.playerA = normalizeAngle(save.PlayerA)
	g.cameraX = save.CameraX
	g.cameraY = save.CameraY
	g.zoom = max(minZoom, min(maxZoom, save.Zoom))
	g.weapon = save.Weapon
	g.bestWeapon = save.BestWeapon
	g.chosenWeapon = save.ChosenWeapon
	g.health = save.Health
	g.ammo = save.Ammo
	g.lives = save.Lives
	g.keys = save.Keys
	g.score = save.Score
	g.secretTotal = save.SecretTotal
	g.secretCount = save.SecretCount
	g.treasureTotal = save.TreasureTotal
	g.treasureCount = save.TreasureCount
	g.attacking = save.Attacking
	g.weaponSequence = save.WeaponSequence
	g.weaponFrameIdx = save.WeaponFrameIdx
	g.weaponFrameTics = save.WeaponFrameTics
	g.damageFlash = save.DamageFlash
	g.bonusFlash = save.BonusFlash
	g.godMode = save.GodMode

	g.playerDying = false
	g.deathPhase = deathPhaseNone
	g.deathTimer = 0
	g.deathHasKiller = false
	g.deathKillerX = 0
	g.deathKillerY = 0
	g.deathFizzleOrder = nil
	g.deathFizzlePixels = nil
	g.deathFizzleFilled = 0
	g.deathFizzleImage = nil
	g.hudNotice = ""
	g.hudNoticeTimer = 0
	g.paused = false
	g.mode = modeRaycast
	g.mousePrimed = false
	g.debugPrompt = debugPromptNone
	g.debugPromptInput = ""
	g.debugGraphicsTest = false

	g.playerAreas = g.computePlayerAreas()
	g.rebuildPlayerAreas()
	g.rebuildBackground()
	g.rebuildHUDText()
	g.syncMusicTrack()
	return nil
}

func (g *game) validateSaveContent(save saveGameData) error {
	if g == nil || g.files == nil {
		return nil
	}
	if save.ContentTag != "" && save.ContentTag != g.files.Variant.Ext {
		return fmt.Errorf("save content %q does not match loaded data %q", save.ContentTag, g.files.Variant.Ext)
	}
	if len(save.ContentDigest) > 0 {
		current := g.saveContentDigest()
		if !bytes.Equal(save.ContentDigest, current) {
			return errors.New("save content does not match loaded data set")
		}
	}
	return nil
}

func (g *game) saveGame(slot int, name string) error {
	if g.mapData == nil || g.level == nil {
		return errors.New("no active game to save")
	}
	if g.playerDying || g.fadePhase != 0 {
		return errors.New("cannot save right now")
	}
	var existingPath string
	if slot >= 0 {
		if slot >= len(g.saveSlots) {
			return fmt.Errorf("save slot %d out of range", slot)
		}
		existingPath = g.saveSlots[slot].Path
	}
	save := g.captureSaveGame(name)
	data, err := marshalSaveGame(save)
	if err != nil {
		return err
	}
	if _, err := g.writeSaveSlot(existingPath, save.Name, data); err != nil {
		return err
	}
	return g.reloadSaveSlots()
}

func (g *game) saveAutosave() error {
	if g.mapData == nil || g.level == nil {
		return errors.New("no active game to save")
	}
	save := g.captureSaveGame(g.autosaveName())
	data, err := marshalSaveGame(save)
	if err != nil {
		return err
	}
	targetPath := autosaveSlotPath(0)
	oldestSet := false
	var oldest time.Time
	for i := range g.saveSlots {
		slot := g.saveSlots[i]
		if !slot.IsAutosave {
			continue
		}
		if !slot.Used {
			targetPath = slot.Path
			oldestSet = false
			break
		}
		if !oldestSet || slot.Timestamp.Before(oldest) {
			targetPath = slot.Path
			oldest = slot.Timestamp
			oldestSet = true
		}
	}
	if _, err := g.writeSaveSlot(targetPath, save.Name, data); err != nil {
		return err
	}
	return g.reloadSaveSlots()
}

func (g *game) loadGame(slot int) error {
	if slot < 0 || slot >= len(g.saveSlots) {
		return fmt.Errorf("save slot %d out of range", slot)
	}
	data, err := g.readSaveSlot(g.saveSlots[slot].Path)
	if err != nil {
		return err
	}
	save, err := parseSaveGame(data)
	if err != nil {
		return err
	}
	return g.applySaveGame(save)
}
