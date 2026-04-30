package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"io"
	"log"
	"math"
	"math/bits"
	"math/rand"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"gd-wolf/internal/wl6"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	defaultScreenWidth  = 1280
	defaultScreenHeight = 960
	mainMenuWindowX     = 68
	mainMenuWindowY     = 52
	mainMenuWindowW     = 178
	mainMenuWindowH     = 136

	baseTileSize        = 16
	minZoom             = 0.5
	maxZoom             = 32.0
	mapWheelZoomStep    = 1.12
	mapKeyZoomStep      = 1.035
	mapPanPixels        = 8.0
	mapPanZoomPower     = 0.5
	mapFineControl      = 0.1
	defaultMapMoveSpeed = 1.0
	mapPlayerShape      = 408
	targetFrameRate     = 300

	modeMap = iota
	modeRaycast

	runSpeed               = 0.11
	walkSpeed              = 0.045
	defaultTurnSpeed       = 0.045
	defaultMouseLook       = 0.0035
	maxRayDepth            = 24.0
	maxSpriteDrawDepth     = maxRayDepth * 2
	fov                    = math.Pi / 3
	playerRadius           = 0.34375
	enemyRadius            = 0.3
	staticBlockRadius      = 0.3
	playerBlockDist        = 1.0
	knifeRange             = 1.5
	doorOpenRatePerTic     = 1024.0 / 65535.0
	doorOpenHoldTics       = 300
	audioSampleRate        = 44100
	startAmmo              = 8
	pushableTile           = 98
	pushWallMaxSteps       = 2
	pushWallStepTics       = 128
	doorCollisionThickness = 0.75
	numRedShifts           = 6
	redSteps               = 8
	numWhiteShifts         = 3
	whiteSteps             = 20
	whiteTics              = 6
	engineTPS              = 60
	wolfTPS                = 70
	deathRotateDegrees     = 2
	deathFadeTics          = 70
	deathHoldTics          = 100
	bjRunSpeedPerTic       = 2048.0 / 65536.0
	bjJumpSpeedPerTic      = 680.0 / 65536.0
	wolfExitTile           = 99
	wolfElevatorTile       = 21
	wolfElevatorUsedTile   = 22
	wolfAltElevatorTile    = 107
	autosaveIntervalTics   = 5 * 60 * wolfTPS

	wolfScreenWidth   = 320
	wolfScreenHeight  = 200
	wolfDisplayHeight = 240
	wolfGameplayLines = 160
	wolfStatusLines   = 40
	hqGameplayWidth   = 640
	hqGameplayHeight  = 320
)

var vgaCeiling = [...]byte{
	0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0xbf,
	0x4e, 0x4e, 0x4e, 0x1d, 0x8d, 0x4e, 0x1d, 0x2d, 0x1d, 0x8d,
	0x1d, 0x1d, 0x1d, 0x1d, 0x1d, 0x2d, 0xdd, 0x1d, 0x1d, 0x98,

	0x1d, 0x9d, 0x2d, 0xdd, 0xdd, 0x9d, 0x2d, 0x4d, 0x1d, 0xdd,
	0x7d, 0x1d, 0x2d, 0x2d, 0xdd, 0xd7, 0x1d, 0x1d, 0x1d, 0x2d,
	0x1d, 0x1d, 0x1d, 0x1d, 0xdd, 0xdd, 0x7d, 0xdd, 0xdd, 0xdd,
}

var wolfElevatorBackTo = [...]int{1, 1, 7, 3, 5, 3}

var flashOverlayPixel *ebiten.Image

type deathPhase int

const (
	deathPhaseNone deathPhase = iota
	deathPhaseRotate
	deathPhaseFizzle
	deathPhaseHold
	deathPhaseWaitSound
)

type victoryPhase int

const (
	victoryPhaseNone victoryPhase = iota
	victoryPhaseRun
	victoryPhaseJump
)

type renderMode int

const (
	renderModeUltra renderMode = iota
	renderModeDOS
	renderModeHQ
)

type game struct {
	files               *wl6.Files
	summaries           []wl6.MapSummary
	mapIndex            int
	mapData             *wl6.MapData
	level               *wl6.Level
	levelWidth          int
	levelHeight         int
	reachable           []bool
	renderableWalls     []bool
	cachedWallIDs       []uint16
	cachedDoorFlags     []byte
	cachedDoorSides     []byte
	walls               *wl6.WallSet
	sprites             *wl6.SpriteSet
	hdAssetRoot         string
	hdTexturesEnabled   bool
	frontendCanvas      *ebiten.Image
	titlePic            *ebiten.Image
	getPsychedPic       *ebiten.Image
	pausedPic           *ebiten.Image
	pauseShade          *ebiten.Image
	pictureCache        map[int]*ebiten.Image
	pictureSizeCache    map[int]pictureChunkSize
	textureCache        map[*wl6.WallTexture]*ebiten.Image
	spriteImageCache    map[int]*ebiten.Image
	fonts               map[wolfFontKind]*wolfFontRenderer
	statusBarPic        *ebiten.Image
	statusBarState      statusBarSnapshot
	frame32             []uint32
	frame               []byte
	background32        []uint32
	background          []byte
	backgroundImage     *ebiten.Image
	cameraColumns       []float64
	rayDirXColumns      []float64
	rayDirYColumns      []float64
	wallColumns         []wallColumn
	columnCoverage      []columnCoverage
	prevWallTops        []int
	prevWallBottoms     []int
	zbuffer             []float64
	weaponOverlayScreen weaponOverlayGeometry
	weaponOverlayScaled weaponOverlayGeometry
	ceilingColor        color.RGBA
	floorColor          color.RGBA
	hudText             string
	fpsText             string
	lastFPSUpdate       time.Time
	staticSprites       []staticSprite
	actors              []actorInstance
	doorOpen            []float64
	doorState           []byte
	doorTimer           []int
	pushWall            pushWallState
	playerAreas         []bool
	renderThreads       int
	difficulty          gameDifficulty
	weapon              int
	bestWeapon          int
	chosenWeapon        int
	health              int
	ammo                int
	lives               int
	keys                byte
	score               int
	secretTotal         int
	secretCount         int
	treasureTotal       int
	treasureCount       int
	attacking           bool
	weaponSequence      AnimSequenceID
	weaponFrameIdx      int
	weaponFrameTics     int
	hudNotice           string
	hudNoticeTimer      int
	damageFlash         int
	bonusFlash          int
	playerDying         bool
	victoryActive       bool
	victoryPhase        victoryPhase
	victoryBJ           staticSprite
	victoryRunDistance  float64
	deathPhase          deathPhase
	deathTimer          int
	gameplayTickAccum   int
	autosaveTickAccum   int
	deathKillerX        float64
	deathKillerY        float64
	deathHasKiller      bool
	deathFizzleOrder    []int
	deathFizzlePixels   []byte
	deathFizzleFilled   int
	deathFizzleImage    *ebiten.Image
	audioContext        *audio.Context
	soundData           map[soundID][]byte
	soundBanks          map[soundID][]*soundVoice
	soundBankIndex      map[soundID]int
	music               *musicController
	musicError          string
	sfxVolume           float64
	musicVolume         float64
	mouseLook           float64
	turnSpeed           float64
	mapMoveSpeed        float64
	vsyncEnabled        bool
	renderMode          renderMode
	renderModePrompted  bool
	rng                 *wolfRNG
	lastPlayedSound     soundID

	mode            int
	uiState         uiState
	menuIndex       int
	menuReturn      uiState
	selectedEpisode int
	selectedLevel   int
	pendingMap      int
	psychedStart    time.Time
	saveSlots       []saveSlotSummary
	saveNameInput   string
	saveNameActive  bool
	saveStatusText  string

	paused            bool
	madeNoise         bool
	playerMovingFast  bool
	godMode           bool
	debugSlowMotion   bool
	debugSlowTick     int
	debugExtraVBLs    int
	debugBorderColor  byte
	debugShowStats    bool
	debugShowCoords   bool
	debugShowMemory   bool
	debugGraphicsTest bool
	debugTexturePage  int
	debugPrompt       debugPromptKind
	debugPromptInput  string
	fadeFrame         int
	fadeFrames        int
	fadePhase         int
	fadeAction        func() error

	cameraX float64
	cameraY float64
	zoom    float64

	playerX float64
	playerY float64
	playerA float64

	viewWidth            int
	viewHeight           int
	gameplayImage        *ebiten.Image
	gameplayFrame32      []uint32
	gameplayFrame        []byte
	gameplayBackground32 []uint32
	gameplayBackground   []byte
	spriteVisBuf         []spriteVis
	raycastJobs          chan raycastJob
	raycastWorkerCount   int

	lastMouseX          int
	lastMouseY          int
	mousePrimed         bool
	tabMapPending       bool
	tabDebugChordUsed   bool
	mapDragActive       bool
	mapDragLastX        int
	mapDragLastY        int
	layout              wolfRenderLayout
	keybinds            []keyBinding
	keybindField        int
	keybindCapture      bool
	keybindCaptureLabel string
}

type wolfRenderLayout struct {
	screenOffsetX float64
	screenOffsetY float64
	screenScaleX  float64
	screenScaleY  float64
	renderLeft    int
	renderRight   int
	renderTop     int
	renderBottom  int
	renderWidth   int
	renderHeight  int
	bufferWidth   int
	bufferHeight  int
	statusX       float64
	statusY       float64
	statusScaleX  float64
	statusScaleY  float64
}

type pushWallState struct {
	active bool
	x      int
	y      int
	dx     int
	dy     int
	steps  int
	tics   int
	wall   wl6.Tile
}

type keyBinding struct {
	id        string
	label     string
	primary   ebiten.Key
	secondary ebiten.Key
}

type keybindAction int

type debugPromptKind int

const (
	keybindMoveForward keybindAction = iota
	keybindMoveBackward
	keybindStrafeLeft
	keybindStrafeRight
	keybindTurnLeft
	keybindTurnRight
	keybindWalk
	keybindUse
	keybindToggleMap
	keybindZoomIn
	keybindZoomOut
	keybindPauseBack
)

const (
	debugPromptNone debugPromptKind = iota
	debugPromptBorderColor
	debugPromptExtraVBLs
	debugPromptWarpLevel
)

const (
	fadePhaseOut = iota + 1
	fadePhaseIn
)

const keyUnbound ebiten.Key = -1

type statusBarSnapshot struct {
	level  int
	score  int
	lives  int
	health int
	ammo   int
	weapon int
	keys   byte
	face   int
}

type uiState int

const (
	uiStateTitle uiState = iota
	uiStateRenderModePrompt
	uiStateMainMenu
	uiStatePauseMenu
	uiStateOptionsMenu
	uiStateAudioMenu
	uiStateControlsMenu
	uiStateGraphicsMenu
	uiStateKeybindsMenu
	uiStateLoadGame
	uiStateSaveGame
	uiStateEpisodeSelect
	uiStateDifficultySelect
	uiStateLevelSelect
	uiStateGetPsyched
	uiStateVictoryIntermission
	uiStatePlaying
)

type menuActionButton struct {
	label   string
	x       int
	y       int
	w       int
	h       int
	enabled bool
}

type staticSprite struct {
	x          float64
	y          float64
	shapenum   int
	rotate     bool
	facingDir  int
	blocking   bool
	shootable  bool
	alive      bool
	pickup     pickupType
	dropPickup pickupType
	scoreValue int

	deathSequence AnimSequenceID
	sequenceID    AnimSequenceID
	frameIndex    int
	frameTimer    int
}

type spriteVis struct {
	index     int
	x         float64
	y         float64
	shapenum  int
	rotate    bool
	facingDir int
	tileDist  int
	dist2     float64
}

type spriteVisByDist []spriteVis

func (s spriteVisByDist) Len() int { return len(s) }
func (s spriteVisByDist) Less(i, j int) bool {
	if s[i].tileDist != s[j].tileDist {
		return s[i].tileDist > s[j].tileDist
	}
	return s[i].dist2 > s[j].dist2
}
func (s spriteVisByDist) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type raycastJob struct {
	startX          int
	endX            int
	scaled          bool
	forwardX        float64
	forwardY        float64
	planeX          float64
	planeY          float64
	projPlaneDist   float64
	rayDirX         []float64
	rayDirY         []float64
	zbuffer         []float64
	dst             []uint32
	stride          int
	bufferHeight    int
	renderTop       int
	renderBottom    int
	renderHeight    int
	prevWallTops    []int
	prevWallBottoms []int
	wg              *sync.WaitGroup
}

const maxColumnSpanCount = 8

type columnSpan struct {
	start int
	end   int
}

type columnCoverage struct {
	count int
	spans [maxColumnSpanCount]columnSpan
}

type wallColumn struct {
	hit         bool
	dist        float64
	wallID      uint16
	side        int
	texU        float64
	texOverride int
	origTop     int
	origBottom  int
	drawTop     int
	drawBottom  int
}

type weaponOverlayGeometry struct {
	valid         bool
	screenW       int
	screenH       int
	left          int
	top           int
	startX        int
	endX          int
	drawTop       int
	drawBottom    int
	spriteTop     int
	spriteScreenX int
	spriteSize    int
}

type pictureChunkSize struct {
	width  int
	height int
}

type pickupType int

const (
	pickupNone pickupType = iota
	pickupFood
	pickupFirstAid
	pickupClip
	pickupMachineGun
	pickupChaingun
	pickupCross
	pickupChalice
	pickupBible
	pickupCrown
	pickupFullHeal
	pickupGibs
	pickupClip2
	pickupAlpo
	pickupKey1
	pickupKey2
	pickupKey3
	pickupKey4
)

type pcmBufferSource struct {
	buf []byte
	pos int64
}

type soundVoice struct {
	player *audio.Player
	src    *pcmBufferSource
	base   []byte
	buf    []byte
}

type wolfFontKind int

const (
	wolfFontSmall wolfFontKind = iota
	wolfFontLarge
)

type wolfTextStyle struct {
	font   wolfFontKind
	color  color.Color
	shadow color.Color
	scale  int
}

type wolfFontRenderer struct {
	font   *wl6.Font
	raw    []byte
	glyphs map[uint32]*ebiten.Image
}

func synthSquareBytes(sampleRate int, freq float64, duration time.Duration, volume float64) []byte {
	samples := int(float64(sampleRate) * duration.Seconds())
	if samples <= 0 {
		return nil
	}
	data := make([]byte, samples*4)
	halfPeriod := float64(sampleRate) / (freq * 2)
	if halfPeriod < 1 {
		halfPeriod = 1
	}
	amp := int16(32767 * volume)
	for i := 0; i < samples; i++ {
		v := amp
		if int(float64(i)/halfPeriod)%2 == 1 {
			v = -amp
		}
		binary.LittleEndian.PutUint16(data[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[i*4+2:], uint16(v))
	}
	return data
}

func synthNoiseBurstBytes(sampleRate int, duration time.Duration, volume float64, decay bool) []byte {
	samples := int(float64(sampleRate) * duration.Seconds())
	if samples <= 0 {
		return nil
	}
	data := make([]byte, samples*4)
	state := uint32(0x13579bdf)
	for i := 0; i < samples; i++ {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		scale := volume
		if decay {
			scale *= 1 - float64(i)/float64(samples)
		}
		v := int16((float64(int32(state&0xffff)-32768) / 32768.0) * 32767 * scale)
		binary.LittleEndian.PutUint16(data[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[i*4+2:], uint16(v))
	}
	return data
}

func mixPCMBytes(parts ...[]byte) []byte {
	maxLen := 0
	for _, part := range parts {
		if len(part) > maxLen {
			maxLen = len(part)
		}
	}
	if maxLen == 0 {
		return nil
	}
	out := make([]byte, maxLen)
	for i := 0; i+1 < maxLen; i += 2 {
		sum := 0
		for _, part := range parts {
			if i+1 >= len(part) {
				continue
			}
			sum += int(int16(binary.LittleEndian.Uint16(part[i : i+2])))
		}
		if sum > math.MaxInt16 {
			sum = math.MaxInt16
		}
		if sum < math.MinInt16 {
			sum = math.MinInt16
		}
		binary.LittleEndian.PutUint16(out[i:], uint16(int16(sum)))
	}
	return out
}

func buildSoundBank(sampleRate int) map[soundID][]byte {
	_ = sampleRate
	return map[soundID][]byte{}
}

func populateMissingWolfSounds(files *wl6.Files, sampleRate int, soundData map[soundID][]byte) {
	if files == nil {
		return
	}
	for id, wolfSound := range wolfSoundIDs {
		if len(soundData[id]) != 0 {
			continue
		}
		pcm, err := files.RenderAdLibSoundPCM(wolfSound, sampleRate)
		if err != nil || len(pcm) == 0 {
			continue
		}
		soundData[id] = pcm
	}
}

var wolfSoundIDs = map[soundID]int{
	soundKnife:             23, // ATKKNIFESND
	soundPistol:            24, // ATKPISTOLSND
	soundMachineGun:        26, // ATKMACHINEGUNSND
	soundChainGun:          11, // ATKGATLINGSND
	soundPickupAmmo:        31, // GETAMMOSND
	soundEnemyAlertGuard:   21, // HALTSND
	soundEnemyAlertOfficer: 66, // SPIONSND
	soundEnemyAlertSS:      51, // SCHUTZADSND
	soundEnemyAlertBoss:    55, // GUTENTAGSND
	soundEnemyAlertDog:     41, // DOGBARKSND
	soundEnemyAttackGuard:  58, // NAZIFIRESND
	soundEnemyAttackSS:     60, // SSFIRESND
	soundEnemyAttackBoss:   59, // BOSSFIRESND
	soundEnemyAttackDog:    68, // DOGATTACKSND
	soundEnemyDeathGuard:   29, // DEATHSCREAM1SND
	soundEnemyDeathGuard2:  22, // DEATHSCREAM2SND
	soundEnemyDeathGuard3:  25, // DEATHSCREAM3SND
	soundEnemyDeathGuard4:  34, // DEATHSCREAM4SND
	soundEnemyDeathGuard5:  35, // DEATHSCREAM5SND
	soundEnemyDeathGuard6:  39, // DEATHSCREAM6SND
	soundEnemyDeathGuard7:  40, // DEATHSCREAM7SND
	soundEnemyDeathGuard8:  52, // AHHHGSND
	soundEnemyDeathGuard9:  67, // NEINSOVASSND
	soundEnemyDeathOfficer: 67, // NEINSOVASSND
	soundEnemyDeathSS:      63, // MEINGOTTSND
	soundEnemyDeathBoss:    53, // DIESND
	soundEnemyDeathDog:     10, // DOGDEATHSND
	soundPlayerDeath:       9,  // PLAYERDEATHSND
	soundPlayerHurt:        16, // TAKEDAMAGESND
	soundDoorOpen:          18, // OPENDOORSND
	soundDoorClose:         19, // CLOSEDOORSND
	soundPushWall:          46, // PUSHWALLSND
	soundNoWay:             6,  // NOWAYSND
	soundMenuMove:          5,  // MOVEGUN1SND
	soundMenuConfirm:       32, // SHOOTSND
	soundMenuBack:          39, // ESCPRESSEDSND
	soundPickupKey:         12, // GETKEYSND
	soundPickupHealth1:     33, // HEALTH1SND
	soundPickupHealth2:     34, // HEALTH2SND
	soundPickupTreasure1:   35, // BONUS1SND
	soundPickupTreasure2:   36, // BONUS2SND
	soundPickupTreasure3:   37, // BONUS3SND
	soundPickupTreasure4:   45, // BONUS4SND
	soundPickupMachineGun:  30, // GETMACHINESND
	soundPickupChaingun:    38, // GETGATLINGSND
	soundPickupOneUp:       44, // BONUS1UPSND
	soundPickupGibs:        61, // SLURPIESND
	soundHitEnemy:          27, // HITENEMYSND
}

func soundVoiceCount(id soundID) int {
	switch id {
	case soundChainGun:
		return 6
	case soundMachineGun, soundPistol:
		return 4
	case soundEnemyAlertGuard, soundEnemyAlertOfficer, soundEnemyAlertSS, soundEnemyAlertBoss, soundEnemyAlertDog, soundEnemyAttackGuard, soundEnemyAttackSS, soundEnemyAttackBoss, soundEnemyAttackDog:
		return 3
	default:
		return 2
	}
}

func defaultKeybinds() []keyBinding {
	return []keyBinding{
		{id: "forward", label: "Forward", primary: ebiten.KeyW, secondary: ebiten.KeyUp},
		{id: "backward", label: "Backward", primary: ebiten.KeyS, secondary: ebiten.KeyDown},
		{id: "strafe_left", label: "Strafe Left", primary: ebiten.KeyA, secondary: keyUnbound},
		{id: "strafe_right", label: "Strafe Right", primary: ebiten.KeyD, secondary: keyUnbound},
		{id: "turn_left", label: "Turn Left", primary: ebiten.KeyLeft, secondary: ebiten.KeyQ},
		{id: "turn_right", label: "Turn Right", primary: ebiten.KeyRight, secondary: ebiten.KeyE},
		{id: "walk", label: "Walk", primary: ebiten.KeyShift, secondary: keyUnbound},
		{id: "use", label: "Use / Open", primary: ebiten.KeyF, secondary: keyUnbound},
		{id: "toggle_map", label: "Toggle Map", primary: ebiten.KeyTab, secondary: ebiten.KeyM},
		{id: "zoom_in", label: "Map Zoom In", primary: ebiten.KeyEqual, secondary: ebiten.KeyKPAdd},
		{id: "zoom_out", label: "Map Zoom Out", primary: ebiten.KeyMinus, secondary: ebiten.KeyKPSubtract},
		{id: "pause_back", label: "Pause / Back", primary: ebiten.KeyEscape, secondary: keyUnbound},
	}
}

func normalizeBindingKey(key ebiten.Key) ebiten.Key {
	switch key {
	case ebiten.KeyShiftLeft, ebiten.KeyShiftRight:
		return ebiten.KeyShift
	case ebiten.KeyControlLeft, ebiten.KeyControlRight:
		return ebiten.KeyControl
	default:
		return key
	}
}

func keyPressed(key ebiten.Key) bool {
	switch key {
	case keyUnbound:
		return false
	case ebiten.KeyShift:
		return ebiten.IsKeyPressed(ebiten.KeyShift) ||
			ebiten.IsKeyPressed(ebiten.KeyShiftLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	case ebiten.KeyControl:
		return ebiten.IsKeyPressed(ebiten.KeyControl) ||
			ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyControlRight)
	default:
		return ebiten.IsKeyPressed(key)
	}
}

func keyJustPressed(key ebiten.Key) bool {
	switch key {
	case keyUnbound:
		return false
	case ebiten.KeyShift:
		return inpututil.IsKeyJustPressed(ebiten.KeyShift) ||
			inpututil.IsKeyJustPressed(ebiten.KeyShiftLeft) ||
			inpututil.IsKeyJustPressed(ebiten.KeyShiftRight)
	case ebiten.KeyControl:
		return inpututil.IsKeyJustPressed(ebiten.KeyControl) ||
			inpututil.IsKeyJustPressed(ebiten.KeyControlLeft) ||
			inpututil.IsKeyJustPressed(ebiten.KeyControlRight)
	default:
		return inpututil.IsKeyJustPressed(key)
	}
}

func keyName(key ebiten.Key) string {
	switch key {
	case keyUnbound:
		return "-"
	case ebiten.KeyUp:
		return "Up"
	case ebiten.KeyDown:
		return "Down"
	case ebiten.KeyLeft:
		return "Left"
	case ebiten.KeyRight:
		return "Right"
	case ebiten.KeyPageUp:
		return "PgUp"
	case ebiten.KeyPageDown:
		return "PgDn"
	case ebiten.KeyBracketLeft:
		return "["
	case ebiten.KeyBracketRight:
		return "]"
	case ebiten.KeyEqual:
		return "="
	case ebiten.KeyMinus:
		return "-"
	case ebiten.KeyKPAdd:
		return "Num+"
	case ebiten.KeyKPSubtract:
		return "Num-"
	case ebiten.KeySpace:
		return "Space"
	case ebiten.KeyEscape:
		return "Esc"
	case ebiten.KeyShift:
		return "Shift"
	case ebiten.KeyControl:
		return "Ctrl"
	default:
		name := key.String()
		if name == "" {
			return "?"
		}
		return strings.TrimPrefix(name, "Key")
	}
}

func (s *pcmBufferSource) Reset() {
	if s == nil {
		return
	}
	s.pos = 0
}

func (s *pcmBufferSource) Read(p []byte) (int, error) {
	if s == nil || s.pos >= int64(len(s.buf)) {
		return 0, io.EOF
	}
	n := copy(p, s.buf[s.pos:])
	s.pos += int64(n)
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (s *pcmBufferSource) Seek(offset int64, whence int) (int64, error) {
	if s == nil {
		return 0, io.ErrClosedPipe
	}
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = s.pos + offset
	case io.SeekEnd:
		next = int64(len(s.buf)) + offset
	default:
		return 0, io.ErrUnexpectedEOF
	}
	if next < 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if next > int64(len(s.buf)) {
		next = int64(len(s.buf))
	}
	s.pos = next
	return s.pos, nil
}

type soundID int

const (
	soundKnife soundID = iota
	soundPistol
	soundMachineGun
	soundChainGun
	soundPickupAmmo
	soundPickupWeapon
	soundEnemyAlertGuard
	soundEnemyAlertOfficer
	soundEnemyAlertSS
	soundEnemyAlertBoss
	soundEnemyAlertDog
	soundEnemyAttackGuard
	soundEnemyAttackSS
	soundEnemyAttackBoss
	soundEnemyAttackDog
	soundEnemyDeathGuard
	soundEnemyDeathGuard2
	soundEnemyDeathGuard3
	soundEnemyDeathGuard4
	soundEnemyDeathGuard5
	soundEnemyDeathGuard6
	soundEnemyDeathGuard7
	soundEnemyDeathGuard8
	soundEnemyDeathGuard9
	soundEnemyDeathOfficer
	soundEnemyDeathSS
	soundEnemyDeathBoss
	soundEnemyDeathDog
	soundPlayerDeath
	soundPlayerHurt
	soundDoorOpen
	soundDoorClose
	soundPushWall
	soundNoWay
	soundMenuMove
	soundMenuConfirm
	soundMenuBack
	soundPickupKey
	soundPickupHealth1
	soundPickupHealth2
	soundPickupTreasure1
	soundPickupTreasure2
	soundPickupTreasure3
	soundPickupTreasure4
	soundPickupMachineGun
	soundPickupChaingun
	soundPickupOneUp
	soundPickupGibs
	soundHitEnemy
)

func main() {
	dataDir := flag.String("data", "", "path to Wolfenstein 3D data directory")
	startMap := flag.Int("map", 0, "initial map index")
	threads := flag.Int("threads", 0, "number of render worker threads (0 = NumCPU)")
	flag.Parse()
	startMapSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "map" {
			startMapSet = true
		}
	})

	var (
		files *wl6.Files
		err   error
	)
	if *dataDir == "" {
		files, _, err = wl6.OpenDefault()
	} else {
		files, err = wl6.Open(*dataDir)
	}
	if err != nil {
		log.Fatal(err)
	}
	setSpriteCatalogVariant(files.Variant)

	soundData := buildSoundBank(audioSampleRate)
	if digi, err := files.LoadDigitizedSoundsResampled(audioSampleRate); err == nil {
		if 5 < len(digi.Samples) && len(digi.Samples[5]) > 0 {
			soundData[soundPistol] = digi.Samples[5]
		}
		if 4 < len(digi.Samples) && len(digi.Samples[4]) > 0 {
			soundData[soundMachineGun] = digi.Samples[4]
		}
		if 6 < len(digi.Samples) && len(digi.Samples[6]) > 0 {
			soundData[soundChainGun] = digi.Samples[6]
		}
		if 0 < len(digi.Samples) && len(digi.Samples[0]) > 0 {
			soundData[soundEnemyAlertGuard] = digi.Samples[0]
		}
		if 27 < len(digi.Samples) && len(digi.Samples[27]) > 0 {
			soundData[soundEnemyAlertOfficer] = digi.Samples[27]
		}
		if 7 < len(digi.Samples) && len(digi.Samples[7]) > 0 {
			soundData[soundEnemyAlertSS] = digi.Samples[7]
		}
		if 8 < len(digi.Samples) && len(digi.Samples[8]) > 0 {
			soundData[soundEnemyAlertBoss] = digi.Samples[8]
		}
		if 1 < len(digi.Samples) && len(digi.Samples[1]) > 0 {
			soundData[soundEnemyAlertDog] = digi.Samples[1]
		}
		if 21 < len(digi.Samples) && len(digi.Samples[21]) > 0 {
			soundData[soundEnemyAttackGuard] = digi.Samples[21]
		}
		if 11 < len(digi.Samples) && len(digi.Samples[11]) > 0 {
			soundData[soundEnemyAttackSS] = digi.Samples[11]
		}
		if 10 < len(digi.Samples) && len(digi.Samples[10]) > 0 {
			soundData[soundEnemyAttackBoss] = digi.Samples[10]
		}
		if 29 < len(digi.Samples) && len(digi.Samples[29]) > 0 {
			soundData[soundEnemyAttackDog] = digi.Samples[29]
		}
		if 12 < len(digi.Samples) && len(digi.Samples[12]) > 0 {
			soundData[soundEnemyDeathGuard] = digi.Samples[12]
		}
		if 13 < len(digi.Samples) && len(digi.Samples[13]) > 0 {
			soundData[soundEnemyDeathGuard2] = digi.Samples[13]
			soundData[soundEnemyDeathGuard3] = digi.Samples[13]
		}
		if 34 < len(digi.Samples) && len(digi.Samples[34]) > 0 {
			soundData[soundEnemyDeathGuard4] = digi.Samples[34]
		}
		if 35 < len(digi.Samples) && len(digi.Samples[35]) > 0 {
			soundData[soundEnemyDeathGuard5] = digi.Samples[35]
		}
		if 39 < len(digi.Samples) && len(digi.Samples[39]) > 0 {
			soundData[soundEnemyDeathGuard6] = digi.Samples[39]
		}
		if 40 < len(digi.Samples) && len(digi.Samples[40]) > 0 {
			soundData[soundEnemyDeathGuard7] = digi.Samples[40]
		}
		if 41 < len(digi.Samples) && len(digi.Samples[41]) > 0 {
			soundData[soundEnemyDeathGuard8] = digi.Samples[41]
		}
		if 42 < len(digi.Samples) && len(digi.Samples[42]) > 0 {
			soundData[soundEnemyDeathGuard9] = digi.Samples[42]
		}
		if 28 < len(digi.Samples) && len(digi.Samples[28]) > 0 {
			soundData[soundEnemyDeathOfficer] = digi.Samples[28]
		}
		if 20 < len(digi.Samples) && len(digi.Samples[20]) > 0 {
			soundData[soundEnemyDeathSS] = digi.Samples[20]
		}
		if 9 < len(digi.Samples) && len(digi.Samples[9]) > 0 {
			soundData[soundEnemyDeathBoss] = digi.Samples[9]
		}
		if 16 < len(digi.Samples) && len(digi.Samples[16]) > 0 {
			soundData[soundEnemyDeathDog] = digi.Samples[16]
		}
		if 9 < len(digi.Samples) && len(digi.Samples[9]) > 0 {
			soundData[soundPlayerDeath] = digi.Samples[9]
		}
		if 14 < len(digi.Samples) && len(digi.Samples[14]) > 0 {
			soundData[soundPlayerHurt] = digi.Samples[14]
		}
		if 3 < len(digi.Samples) && len(digi.Samples[3]) > 0 {
			soundData[soundDoorOpen] = digi.Samples[3]
		}
		if 2 < len(digi.Samples) && len(digi.Samples[2]) > 0 {
			soundData[soundDoorClose] = digi.Samples[2]
		}
		if 15 < len(digi.Samples) && len(digi.Samples[15]) > 0 {
			soundData[soundPushWall] = digi.Samples[15]
		}
		if 22 < len(digi.Samples) && len(digi.Samples[22]) > 0 {
			soundData[soundPickupGibs] = digi.Samples[22]
		}
	}
	populateMissingWolfSounds(files, audioSampleRate, soundData)

	summaries, err := files.Maps()
	if err != nil {
		log.Fatal(err)
	}

	g := &game{
		files:             files,
		summaries:         summaries,
		zoom:              4.0,
		mode:              modeRaycast,
		uiState:           uiStateTitle,
		menuReturn:        uiStateMainMenu,
		difficulty:        difficultyMedium,
		playerA:           0,
		renderThreads:     *threads,
		viewWidth:         defaultScreenWidth,
		viewHeight:        defaultScreenHeight,
		frame32:           make([]uint32, defaultScreenWidth*defaultScreenHeight),
		background32:      make([]uint32, defaultScreenWidth*defaultScreenHeight),
		zbuffer:           make([]float64, defaultScreenWidth),
		sfxVolume:         0.5,
		musicVolume:       1.0,
		mouseLook:         defaultMouseLook,
		turnSpeed:         defaultTurnSpeed,
		mapMoveSpeed:      defaultMapMoveSpeed,
		vsyncEnabled:      true,
		renderMode:        renderModeUltra,
		hdTexturesEnabled: true,
		rng:               defaultRNG(),
		keybinds:          defaultKeybinds(),
		saveSlots:         nil,
	}
	g.frame = byteViewFromU32(g.frame32)
	g.background = byteViewFromU32(g.background32)
	if err := g.loadPersistentConfig(); err != nil {
		log.Printf("config load failed: %v", err)
	}
	if err := g.reloadSaveSlots(); err != nil {
		log.Printf("save slot scan failed: %v", err)
	}
	g.installBrowserSaveBridge()
	g.startNewGame()
	g.startFadeIn(12)
	g.frontendCanvas = ebiten.NewImage(wolfScreenWidth, wolfScreenHeight)
	if *startMap >= 0 && *startMap < len(summaries) {
		g.selectedLevel = *startMap
	}
	g.audioContext = audio.NewContext(audioSampleRate)
	g.soundData = soundData
	if err := g.rebuildSoundBanks(); err != nil {
		log.Fatal(err)
	}
	g.music, err = newMusicController(g.audioContext, files)
	if err != nil {
		log.Fatal(err)
	}
	g.applyMusicVolume()
	g.prevWallTops = make([]int, defaultScreenWidth)
	g.prevWallBottoms = make([]int, defaultScreenWidth)
	g.ensureRenderBuffers()
	g.hdAssetRoot = locateHDAssetsRoot()
	g.loadWolfFonts()
	if err := g.refreshRuntimeImageAssets(); err != nil {
		log.Fatal(err)
	}
	if startMapSet {
		g.startNewGame()
		if err := g.setMap(g.selectedLevel); err != nil {
			log.Fatal(err)
		}
		g.pendingMap = g.selectedLevel
		g.ensureFrame(g.viewWidth, g.viewHeight)
		g.uiState = uiStatePlaying
	}
	g.syncMusicTrack()

	ebiten.SetWindowSize(defaultScreenWidth, defaultScreenHeight)
	ebiten.SetWindowTitle("GD-WOLF")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetVsyncEnabled(g.vsyncEnabled)
	if g.uiState == uiStatePlaying {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
	if err := ebiten.RunGame(g); err != nil {
		if errors.Is(err, ebiten.Termination) {
			return
		}
		log.Fatal(err)
	}
}

func (g *game) Update() error {
	g.updateFPS()
	g.syncMusicTrack()
	if g.hudNoticeTimer > 0 {
		g.hudNoticeTimer--
		if g.hudNoticeTimer == 0 {
			g.hudNotice = ""
		}
	}

	if g.uiState != uiStatePlaying {
		if g.fadePhase != 0 {
			return g.updateFade()
		}
		return g.updateFrontend()
	}
	if g.fadePhase != 0 {
		return g.updateFade()
	}

	if g.debugExtraVBLs > 0 {
		time.Sleep(time.Duration(g.debugExtraVBLs) * time.Second / 70)
	}
	if g.debugPrompt != debugPromptNone {
		return g.updateDebugPrompt()
	}
	if handled, err := g.handleDebugCheatChord(); handled || err != nil {
		g.rebuildHUDText()
		return err
	}
	if g.debugGraphicsTest {
		return g.updateGraphicsTest()
	}

	if g.keybindJustPressed(keybindPauseBack) {
		g.openPauseMenu()
	}
	if g.updateTabMapState() {
		g.toggleMapMode()
	}

	if g.mode == modeMap {
		g.updateMapMode()
		g.rebuildHUDText()
		return nil
	}
	if g.paused {
		g.rebuildHUDText()
		return nil
	}
	if g.debugSlowMotion {
		g.debugSlowTick = (g.debugSlowTick + 1) % 4
		if g.debugSlowTick != 0 {
			g.rebuildHUDText()
			return nil
		}
	}

	gameplayTics := g.consumeWolfTics(&g.gameplayTickAccum)
	if g.playerDying {
		return g.updatePlayerDeath(gameplayTics)
	}
	if g.victoryActive {
		return g.updateVictorySequence(gameplayTics)
	}
	autosavesDue := g.consumePeriodicAutosaveTriggers(gameplayTics)

	g.madeNoise = false
	g.updateDoors(gameplayTics)
	g.updatePushWall(gameplayTics)
	g.updateSpriteAnimations(gameplayTics)
	g.updateScreenFlashes(gameplayTics)
	g.updateRaycastMode(gameplayTics)
	if err := g.performAutosaves(autosavesDue); err != nil && !errors.Is(err, errSaveUnsupported) {
		log.Printf("periodic autosave failed: %v", err)
	}
	g.rebuildHUDText()

	return nil
}

func (g *game) consumeWolfTics(accum *int) int {
	*accum += wolfTPS
	tics := *accum / engineTPS
	*accum -= tics * engineTPS
	return tics
}

func (g *game) consumePeriodicAutosaveTriggers(tics int) int {
	if tics <= 0 {
		return 0
	}
	g.autosaveTickAccum += tics
	triggers := g.autosaveTickAccum / autosaveIntervalTics
	g.autosaveTickAccum -= triggers * autosaveIntervalTics
	return triggers
}

func (g *game) performAutosaves(count int) error {
	for ; count > 0; count-- {
		if err := g.triggerAutosave("periodic"); err != nil {
			return err
		}
	}
	return nil
}

func (g *game) triggerAutosave(reason string) error {
	if err := g.saveAutosave(); err != nil {
		return err
	}
	g.setNotice("Autosaved")
	if reason == "" {
		log.Printf("autosave completed")
	} else {
		log.Printf("%s autosave completed", reason)
	}
	return nil
}

func (g *game) toggleMapMode() {
	if g.mode == modeMap {
		g.mode = modeRaycast
		if g.paused {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		}
		g.mousePrimed = false
		return
	}

	g.mode = modeMap
	g.centerMapOnPlayer()
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.mousePrimed = false
}

func (g *game) resumePlaying() {
	g.paused = false
	g.uiState = uiStatePlaying
	if g.mode == modeRaycast {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	}
	g.mousePrimed = false
}

func (g *game) openPauseMenu() {
	g.paused = true
	g.uiState = uiStatePauseMenu
	g.menuIndex = 0
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.mousePrimed = false
}

func (g *game) returnToMainMenu() error {
	return g.fadeToUIState(uiStateMainMenu, func() {
		g.paused = false
		g.menuReturn = uiStateMainMenu
		g.menuIndex = 0
		g.mode = modeRaycast
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		g.mousePrimed = false
	})
}

func (g *game) updateMapMode() {
	mouseX, mouseY := ebiten.CursorPosition()
	wheelX, wheelY := ebiten.Wheel()
	zoomStep := mapKeyZoomStep
	panPixels := mapPanPixels * g.mapMoveSpeed
	if g.keybindPressed(keybindWalk) {
		zoomStep = 1 + (mapKeyZoomStep-1)*mapFineControl
		panPixels *= mapFineControl
	}
	if wheelX != 0 || wheelY != 0 {
		prevZoom := g.zoom
		g.zoom = max(minZoom, min(maxZoom, g.zoom*math.Pow(mapWheelZoomStep, float64(wheelY))))
		if g.zoom != prevZoom {
			g.zoomMapAroundScreenPoint(mouseX, mouseY, prevZoom)
		}
	}
	if g.keybindPressed(keybindZoomIn) {
		prevZoom := g.zoom
		g.zoom = min(maxZoom, g.zoom*zoomStep)
		if g.zoom != prevZoom {
			g.zoomMapAroundScreenPoint(g.viewWidth/2, g.viewHeight/2, prevZoom)
		}
	}
	if g.keybindPressed(keybindZoomOut) {
		prevZoom := g.zoom
		g.zoom = max(minZoom, g.zoom/zoomStep)
		if g.zoom != prevZoom {
			g.zoomMapAroundScreenPoint(g.viewWidth/2, g.viewHeight/2, prevZoom)
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.mapDragActive = true
		g.mapDragLastX = mouseX
		g.mapDragLastY = mouseY
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.mapDragActive = false
	}
	if g.mapDragActive {
		dx := mouseX - g.mapDragLastX
		dy := mouseY - g.mapDragLastY
		if dx != 0 || dy != 0 {
			tileSize := baseTileSize * g.zoom
			if tileSize > 0 {
				g.cameraX -= float64(dx) / tileSize
				g.cameraY -= float64(dy) / tileSize
			}
			g.mapDragLastX = mouseX
			g.mapDragLastY = mouseY
		}
	}

	panSpeed := panPixels / (baseTileSize * math.Pow(g.zoom, mapPanZoomPower))
	if g.keybindPressed(keybindTurnLeft) || g.keybindPressed(keybindStrafeLeft) {
		g.cameraX -= panSpeed
	}
	if g.keybindPressed(keybindTurnRight) || g.keybindPressed(keybindStrafeRight) {
		g.cameraX += panSpeed
	}
	if g.keybindPressed(keybindMoveForward) {
		g.cameraY -= panSpeed
	}
	if g.keybindPressed(keybindMoveBackward) {
		g.cameraY += panSpeed
	}
}

func (g *game) zoomMapAroundScreenPoint(screenX, screenY int, prevZoom float64) {
	prevTileSize := baseTileSize * prevZoom
	nextTileSize := baseTileSize * g.zoom
	if prevTileSize <= 0 || nextTileSize <= 0 {
		return
	}
	worldX := g.cameraX + (float64(screenX)-float64(g.viewWidth)/2)/prevTileSize
	worldY := g.cameraY + (float64(screenY)-float64(g.viewHeight)/2)/prevTileSize
	g.cameraX = worldX - (float64(screenX)-float64(g.viewWidth)/2)/nextTileSize
	g.cameraY = worldY - (float64(screenY)-float64(g.viewHeight)/2)/nextTileSize
}

func (g *game) updateRaycastMode(gameplayTics int) {
	mouseX, _ := ebiten.CursorPosition()
	if g.mousePrimed {
		g.playerA += float64(mouseX-g.lastMouseX) * g.mouseLook
	}
	g.lastMouseX = mouseX
	g.mousePrimed = true

	if g.keybindPressed(keybindTurnLeft) {
		g.playerA -= g.turnSpeed
	}
	if g.keybindPressed(keybindTurnRight) {
		g.playerA += g.turnSpeed
	}

	forwardX := math.Cos(g.playerA)
	forwardY := math.Sin(g.playerA)
	rightX := math.Cos(g.playerA + math.Pi/2)
	rightY := math.Sin(g.playerA + math.Pi/2)
	speed := runSpeed
	if g.keybindPressed(keybindWalk) {
		speed = walkSpeed
	}

	var moveX, moveY float64
	if g.keybindPressed(keybindMoveForward) {
		moveX += forwardX * speed
		moveY += forwardY * speed
	}
	if g.keybindPressed(keybindMoveBackward) {
		moveX -= forwardX * speed
		moveY -= forwardY * speed
	}
	if g.keybindPressed(keybindStrafeLeft) {
		moveX -= rightX * speed
		moveY -= rightY * speed
	}
	if g.keybindPressed(keybindStrafeRight) {
		moveX += rightX * speed
		moveY += rightY * speed
	}
	if g.keybindJustPressed(keybindUse) {
		g.useDoorAhead()
	}
	g.updateWeaponSelection()
	g.updateWeaponAttack(gameplayTics)

	g.playerMovingFast = math.Hypot(moveX, moveY) > walkSpeed
	g.tryMove(moveX, moveY)
	if g.checkVictoryTile() {
		return
	}
	g.updateActors(gameplayTics)
	g.collectPickups()
}

func (g *game) Draw(screen *ebiten.Image) {

	if g.uiState != uiStatePlaying {
		g.drawFrontend(screen)
		g.drawFadeOverlay(screen)
		return
	}
	if g.debugGraphicsTest {
		g.drawGraphicsTest(screen)
		g.drawDebugOverlays(screen)
		g.drawScreenFlashOverlay(screen)
		g.drawDeathOverlay(screen)
		g.drawFadeOverlay(screen)
		return
	}
	if g.mode == modeMap {
		g.drawMap(screen)
		g.drawDebugOverlays(screen)
		g.drawScreenFlashOverlay(screen)
		g.drawDeathOverlay(screen)
		g.drawFadeOverlay(screen)
		return
	}
	g.drawRaycast(screen)
	g.drawDebugOverlays(screen)
	g.drawScreenFlashOverlay(screen)
	g.drawDeathOverlay(screen)
	g.drawFadeOverlay(screen)
}

func throttleDrawRate(drawStart time.Time) {
	frameBudget := time.Second / targetFrameRate
	remaining := frameBudget - time.Since(drawStart)
	if remaining > 0 {
		time.Sleep(remaining)
	}
}

func (g *game) startFadeTransition(frames int, action func() error) error {
	if frames < 1 {
		frames = 1
	}
	if g.fadePhase != 0 {
		return nil
	}
	g.fadeFrames = frames
	g.fadeFrame = 0
	g.fadePhase = fadePhaseOut
	g.fadeAction = action
	return nil
}

func (g *game) startFadeIn(frames int) {
	if frames < 1 {
		frames = 1
	}
	g.fadeFrames = frames
	g.fadeFrame = frames
	g.fadePhase = fadePhaseIn
	g.fadeAction = nil
}

func (g *game) updateFade() error {
	switch g.fadePhase {
	case fadePhaseOut:
		g.fadeFrame++
		if g.fadeFrame < g.fadeFrames {
			return nil
		}
		g.fadeFrame = g.fadeFrames
		if g.fadeAction != nil {
			if err := g.fadeAction(); err != nil {
				g.fadePhase = 0
				g.fadeAction = nil
				return err
			}
		}
		g.fadeAction = nil
		g.fadePhase = fadePhaseIn
	case fadePhaseIn:
		g.fadeFrame--
		if g.fadeFrame > 0 {
			return nil
		}
		g.fadeFrame = 0
		g.fadePhase = 0
	}
	return nil
}

func (g *game) drawFadeOverlay(screen *ebiten.Image) {
	if g.fadePhase == 0 || g.fadeFrames <= 0 || g.fadeFrame <= 0 {
		return
	}
	alpha := float32(g.fadeFrame) / float32(g.fadeFrames)
	vector.DrawFilledRect(screen, 0, 0, float32(g.viewWidth), float32(g.viewHeight), color.RGBA{A: byte(alpha * 255)}, false)
}

func (g *game) startBonusFlash() {
	g.bonusFlash = numWhiteShifts * whiteTics
}

func (g *game) startDamageFlash(damage int) {
	if damage <= 0 {
		return
	}
	g.damageFlash += damage
}

func (g *game) updateScreenFlashes(tics int) {
	if tics <= 0 {
		return
	}
	if g.bonusFlash > 0 {
		g.bonusFlash -= tics
		if g.bonusFlash < 0 {
			g.bonusFlash = 0
		}
	}
	if g.damageFlash > 0 {
		g.damageFlash -= tics
		if g.damageFlash < 0 {
			g.damageFlash = 0
		}
	}
}

func (g *game) currentScreenFlash() (color.RGBA, bool) {
	white := 0
	if g.bonusFlash > 0 {
		white = g.bonusFlash/whiteTics + 1
		if white > numWhiteShifts {
			white = numWhiteShifts
		}
	}
	red := 0
	if g.damageFlash > 0 {
		red = g.damageFlash/10 + 1
		if red > numRedShifts {
			red = numRedShifts
		}
	}
	if red > 0 {
		return color.RGBA{R: 255, G: 0, B: 0, A: uint8((255 * red) / (redSteps * 3))}, true
	}
	if white > 0 {
		return color.RGBA{R: 255, G: 247, B: 0, A: uint8((255 * white) / (whiteSteps * 3))}, true
	}
	return color.RGBA{}, false
}

func (g *game) drawScreenFlashOverlay(screen *ebiten.Image) {
	clr, ok := g.currentScreenFlash()
	if !ok {
		return
	}
	if flashOverlayPixel == nil {
		flashOverlayPixel = ebiten.NewImage(1, 1)
		flashOverlayPixel.Fill(color.White)
	}
	alpha := float32(clr.A) / 255
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(g.viewWidth), float64(g.viewHeight))
	op.ColorScale.Scale(
		(float32(clr.R)/255)*alpha,
		(float32(clr.G)/255)*alpha,
		(float32(clr.B)/255)*alpha,
		alpha,
	)
	screen.DrawImage(flashOverlayPixel, op)
}

func (g *game) deathOverlayAlpha() float32 {
	if !g.playerDying || g.deathPhase == deathPhaseNone || g.deathPhase == deathPhaseRotate {
		return 0
	}
	return 1
}

func (g *game) deathFizzleRevealCount() int {
	left, right, top, bottom, _, _ := g.gameplayRenderArea()
	width := maxInt(0, right-left)
	height := maxInt(0, bottom-top)
	total := width * height
	if total == 0 {
		return 0
	}
	switch g.deathPhase {
	case deathPhaseFizzle:
		progress := deathFadeTics - g.deathTimer
		if progress <= 0 {
			return 0
		}
		if progress >= deathFadeTics {
			return total
		}
		return (total * progress) / deathFadeTics
	case deathPhaseHold, deathPhaseWaitSound:
		return total
	default:
		return 0
	}
}

func (g *game) deathFillColor() color.RGBA {
	if g.walls != nil && len(g.walls.Palette) > 4 {
		return g.paletteColor(4)
	}
	return color.RGBA{R: 170, G: 0, B: 0, A: 255}
}

func (g *game) ensureDeathFizzleImage(width, height int) {
	if width <= 0 || height <= 0 {
		g.deathFizzleImage = nil
		g.deathFizzleOrder = nil
		g.deathFizzlePixels = nil
		g.deathFizzleFilled = 0
		return
	}
	if g.deathFizzleImage != nil {
		bounds := g.deathFizzleImage.Bounds()
		if bounds.Dx() == width && bounds.Dy() == height {
			return
		}
	}
	total := width * height
	g.deathFizzleImage = ebiten.NewImage(width, height)
	g.deathFizzleOrder = make([]int, total)
	for i := range g.deathFizzleOrder {
		g.deathFizzleOrder[i] = i
	}
	rng := rand.New(rand.NewSource(int64(width)<<32 | int64(height)))
	rng.Shuffle(total, func(i, j int) {
		g.deathFizzleOrder[i], g.deathFizzleOrder[j] = g.deathFizzleOrder[j], g.deathFizzleOrder[i]
	})
	g.deathFizzlePixels = make([]byte, total*4)
	g.deathFizzleFilled = 0
}

func (g *game) updateDeathFizzleImage(reveal int) {
	if g.deathFizzleImage == nil || len(g.deathFizzleOrder) == 0 {
		return
	}
	total := len(g.deathFizzleOrder)
	if reveal < 0 {
		reveal = 0
	}
	if reveal > total {
		reveal = total
	}
	if reveal < g.deathFizzleFilled {
		clear(g.deathFizzlePixels)
		g.deathFizzleFilled = 0
	}
	if reveal == g.deathFizzleFilled {
		return
	}
	fill := g.deathFillColor()
	for g.deathFizzleFilled < reveal {
		idx := g.deathFizzleOrder[g.deathFizzleFilled]
		offset := idx * 4
		g.deathFizzlePixels[offset] = fill.R
		g.deathFizzlePixels[offset+1] = fill.G
		g.deathFizzlePixels[offset+2] = fill.B
		g.deathFizzlePixels[offset+3] = fill.A
		g.deathFizzleFilled++
	}
	g.deathFizzleImage.WritePixels(g.deathFizzlePixels)
}

func (g *game) drawDeathOverlay(screen *ebiten.Image) {
	alpha := g.deathOverlayAlpha()
	if alpha <= 0 {
		return
	}
	left, right, top, bottom, _, _ := g.gameplayRenderArea()
	width := right - left
	height := bottom - top
	g.ensureDeathFizzleImage(width, height)
	g.updateDeathFizzleImage(g.deathFizzleRevealCount())
	if g.deathFizzleImage == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(left), float64(top))
	screen.DrawImage(g.deathFizzleImage, op)
}

func (g *game) isDebugChord(key ebiten.Key) bool {
	return ebiten.IsKeyPressed(ebiten.KeyTab) && inpututil.IsKeyJustPressed(key)
}

func (g *game) updateTabMapState() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.tabMapPending = true
		g.tabDebugChordUsed = false
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyTab) {
		shouldToggle := g.tabMapPending && !g.tabDebugChordUsed
		g.tabMapPending = false
		g.tabDebugChordUsed = false
		return shouldToggle
	}
	return false
}

func (g *game) handleDebugCheatChord() (bool, error) {
	switch {
	case g.isDebugChord(ebiten.KeyB):
		g.tabDebugChordUsed = true
		g.debugPrompt = debugPromptBorderColor
		g.debugPromptInput = ""
		g.setNotice("Border color 0-15")
	case g.isDebugChord(ebiten.KeyC):
		g.tabDebugChordUsed = true
		g.debugShowStats = !g.debugShowStats
		g.setNotice("Level statistics")
	case g.isDebugChord(ebiten.KeyE):
		g.tabDebugChordUsed = true
		if err := g.stepMap(1); err != nil {
			return true, err
		}
		g.setNotice("Warped to end of level")
	case g.isDebugChord(ebiten.KeyF):
		g.tabDebugChordUsed = true
		g.debugShowCoords = !g.debugShowCoords
		g.setNotice("Coordinates")
	case g.isDebugChord(ebiten.KeyG):
		g.tabDebugChordUsed = true
		g.godMode = !g.godMode
		if g.godMode {
			g.setNotice("God mode ON")
		} else {
			g.setNotice("God mode OFF")
		}
	case g.isDebugChord(ebiten.KeyH):
		g.tabDebugChordUsed = true
		g.takePlayerDamage(16)
	case g.isDebugChord(ebiten.KeyI):
		g.tabDebugChordUsed = true
		g.applyItemCheat()
	case g.isDebugChord(ebiten.KeyM):
		g.tabDebugChordUsed = true
		g.debugShowMemory = !g.debugShowMemory
		g.setNotice("Memory usage")
	case g.isDebugChord(ebiten.KeyP):
		g.tabDebugChordUsed = true
		if g.uiState == uiStatePlaying {
			g.openPauseMenu()
		} else if g.uiState == uiStatePauseMenu {
			g.resumePlaying()
		}
	case g.isDebugChord(ebiten.KeyQ):
		g.tabDebugChordUsed = true
		return true, g.requestQuit()
	case g.isDebugChord(ebiten.KeyS):
		g.tabDebugChordUsed = true
		g.debugSlowMotion = !g.debugSlowMotion
		g.debugSlowTick = 0
		if g.debugSlowMotion {
			g.setNotice("Slow motion ON")
		} else {
			g.setNotice("Slow motion OFF")
		}
	case g.isDebugChord(ebiten.KeyT):
		g.tabDebugChordUsed = true
		g.debugGraphicsTest = !g.debugGraphicsTest
		g.debugTexturePage = 0
		if g.debugGraphicsTest {
			g.setNotice("Graphics test")
		} else {
			g.setNotice("Graphics test closed")
		}
	case g.isDebugChord(ebiten.KeyV):
		g.tabDebugChordUsed = true
		g.debugPrompt = debugPromptExtraVBLs
		g.debugPromptInput = ""
		g.setNotice("Extra VBLs 0-8")
	case g.isDebugChord(ebiten.KeyW):
		g.tabDebugChordUsed = true
		g.debugPrompt = debugPromptWarpLevel
		g.debugPromptInput = ""
		g.setNotice("Warp to level")
	case g.isDebugChord(ebiten.KeyX):
		g.tabDebugChordUsed = true
		g.applyExtraStuffCheat()
	default:
		return false, nil
	}
	return true, nil
}

func (g *game) updateGraphicsTest() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || g.isDebugChord(ebiten.KeyT) {
		g.debugGraphicsTest = false
		return nil
	}
	if g.walls == nil || len(g.walls.AllPages) == 0 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.debugTexturePage--
		if g.debugTexturePage < 0 {
			g.debugTexturePage = len(g.walls.AllPages) - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		g.debugTexturePage++
		if g.debugTexturePage >= len(g.walls.AllPages) {
			g.debugTexturePage = 0
		}
	}
	return nil
}

func debugDigitFromKey(key ebiten.Key) (string, bool) {
	switch key {
	case ebiten.Key0, ebiten.KeyKP0:
		return "0", true
	case ebiten.Key1, ebiten.KeyKP1:
		return "1", true
	case ebiten.Key2, ebiten.KeyKP2:
		return "2", true
	case ebiten.Key3, ebiten.KeyKP3:
		return "3", true
	case ebiten.Key4, ebiten.KeyKP4:
		return "4", true
	case ebiten.Key5, ebiten.KeyKP5:
		return "5", true
	case ebiten.Key6, ebiten.KeyKP6:
		return "6", true
	case ebiten.Key7, ebiten.KeyKP7:
		return "7", true
	case ebiten.Key8, ebiten.KeyKP8:
		return "8", true
	case ebiten.Key9, ebiten.KeyKP9:
		return "9", true
	default:
		return "", false
	}
}

func (g *game) updateDebugPrompt() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.debugPrompt = debugPromptNone
		g.debugPromptInput = ""
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.debugPromptInput) > 0 {
		g.debugPromptInput = g.debugPromptInput[:len(g.debugPromptInput)-1]
		return nil
	}
	for _, key := range inpututil.AppendJustPressedKeys(nil) {
		digit, ok := debugDigitFromKey(key)
		if !ok {
			continue
		}
		limit := 2
		if g.debugPrompt == debugPromptWarpLevel {
			limit = 3
		}
		if len(g.debugPromptInput) < limit {
			g.debugPromptInput += digit
		}
		return nil
	}
	if !inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return nil
	}
	if g.debugPromptInput == "" {
		return nil
	}
	value, err := strconv.Atoi(g.debugPromptInput)
	if err != nil {
		return nil
	}
	switch g.debugPrompt {
	case debugPromptBorderColor:
		if value >= 0 && value <= 15 {
			g.debugBorderColor = byte(value)
			g.rebuildBackground()
			g.setNotice(fmt.Sprintf("Border color %d", value))
		} else {
			g.setNotice("Border color out of range")
		}
	case debugPromptExtraVBLs:
		if value >= 0 && value <= 8 {
			g.debugExtraVBLs = value
			g.setNotice(fmt.Sprintf("Extra VBLs %d", value))
		} else {
			g.setNotice("Extra VBLs out of range")
		}
	case debugPromptWarpLevel:
		if err := g.warpToLevelNumber(value); err != nil {
			return err
		}
	}
	g.debugPrompt = debugPromptNone
	g.debugPromptInput = ""
	return nil
}

func (g *game) updateFrontend() error {
	switch g.uiState {
	case uiStateTitle:
		if anyFrontendInput() {
			g.playSound(soundMenuConfirm)
			next := g.nextTitleInputState()
			return g.fadeToUIState(next, func() {
				if next == uiStateRenderModePrompt {
					g.menuIndex = renderModeMenuIndex(g.renderMode)
					return
				}
				g.menuIndex = 0
			})
		}
	case uiStateRenderModePrompt:
		items := renderModeMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateTitle, nil)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.setRenderMode(renderModeMenuChoice(g.menuIndex))
			g.renderModePrompted = true
			if err := g.savePersistentConfig(); err != nil {
				log.Printf("config save failed: %v", err)
			}
			g.playSound(soundMenuConfirm)
			return g.fadeToUIState(uiStateMainMenu, func() {
				g.menuIndex = 0
			})
		}
	case uiStateMainMenu:
		items := []string{"Start Game", "Load Game", "Level Select", "Options"}
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.playSound(soundMenuConfirm)
			switch g.menuIndex {
			case 0:
				g.selectedEpisode = 0
				if g.availableEpisodes() > 1 {
					return g.fadeToUIState(uiStateEpisodeSelect, nil)
				} else {
					g.pendingMap = 0
					g.startNewGame()
					return g.fadeToUIState(uiStateDifficultySelect, func() {
						g.menuReturn = uiStateMainMenu
						g.menuIndex = g.selectedDifficultyIndex()
					})
				}
			case 1:
				if err := g.reloadSaveSlots(); err != nil {
					log.Printf("save slot scan failed: %v", err)
				}
				return g.fadeToUIState(uiStateLoadGame, func() {
					g.menuReturn = uiStateMainMenu
					g.menuIndex = 0
				})
			case 2:
				return g.fadeToUIState(uiStateLevelSelect, func() {
					g.menuIndex = g.selectedLevel
				})
			case 3:
				return g.fadeToUIState(uiStateOptionsMenu, func() {
					g.menuReturn = uiStateMainMenu
					g.menuIndex = 0
				})
			}
		}
	case uiStatePauseMenu:
		items := []string{"Resume", "Save Game", "Load Game", "Options", "Main Menu", "Quit"}
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			g.resumePlaying()
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.playSound(soundMenuConfirm)
			switch g.menuIndex {
			case 0:
				g.resumePlaying()
				return nil
			case 1:
				if err := g.reloadSaveSlots(); err != nil {
					log.Printf("save slot scan failed: %v", err)
				}
				return g.fadeToUIState(uiStateSaveGame, func() {
					g.menuReturn = uiStatePauseMenu
					g.menuIndex = 0
					g.saveNameInput = ""
					g.saveNameActive = false
					g.saveStatusText = ""
				})
			case 2:
				if err := g.reloadSaveSlots(); err != nil {
					log.Printf("save slot scan failed: %v", err)
				}
				return g.fadeToUIState(uiStateLoadGame, func() {
					g.menuReturn = uiStatePauseMenu
					g.menuIndex = 0
				})
			case 3:
				return g.fadeToUIState(uiStateOptionsMenu, func() {
					g.menuReturn = uiStatePauseMenu
					g.menuIndex = 0
				})
			case 4:
				return g.returnToMainMenu()
			case 5:
				return g.requestQuit()
			}
		}
	case uiStateOptionsMenu:
		items := g.optionsMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			next := g.menuReturn
			return g.fadeToUIState(next, func() {
				switch next {
				case uiStatePauseMenu:
					g.menuIndex = 3
				default:
					g.menuIndex = 3
				}
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			switch g.menuIndex {
			case 0:
				g.playSound(soundMenuConfirm)
				return g.fadeToUIState(uiStateGraphicsMenu, func() {
					g.menuIndex = 0
				})
			case 1:
				g.playSound(soundMenuConfirm)
				return g.fadeToUIState(uiStateAudioMenu, func() {
					g.menuIndex = 0
				})
			case 2:
				g.playSound(soundMenuConfirm)
				return g.fadeToUIState(uiStateControlsMenu, func() {
					g.menuIndex = 0
				})
			case 3:
				g.playSound(soundMenuBack)
				next := g.menuReturn
				return g.fadeToUIState(next, func() {
					switch next {
					case uiStatePauseMenu:
						g.menuIndex = 3
					default:
						g.menuIndex = 3
					}
				})
			}
			g.playSound(soundMenuConfirm)
		}
	case uiStateAudioMenu:
		items := g.audioMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateOptionsMenu, func() {
				g.menuIndex = 1
			})
		}
		adjust := 0.0
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
			adjust = -0.1
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
			adjust = 0.1
		}
		if adjust != 0 {
			switch g.menuIndex {
			case 0:
				g.sfxVolume = clampUnit(g.sfxVolume + adjust)
				g.applySFXVolume()
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				g.playSound(soundMenuMove)
			case 1:
				g.musicVolume = clampUnit(g.musicVolume + adjust)
				g.applyMusicVolume()
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				g.playSound(soundMenuMove)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			switch g.menuIndex {
			case 2:
				g.playSound(soundMenuBack)
				return g.fadeToUIState(uiStateOptionsMenu, func() {
					g.menuIndex = 1
				})
			}
			g.playSound(soundMenuConfirm)
		}
	case uiStateControlsMenu:
		items := g.controlsMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateOptionsMenu, func() {
				g.menuIndex = 2
			})
		}
		adjust := 0.0
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
			adjust = -0.1
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
			adjust = 0.1
		}
		if adjust != 0 {
			switch g.menuIndex {
			case 0:
				g.mouseLook = clampMouseLook(g.mouseLook + adjust*defaultMouseLook)
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				g.playSound(soundMenuMove)
			case 1:
				g.turnSpeed = clampTurnSpeed(g.turnSpeed + adjust*defaultTurnSpeed)
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				g.playSound(soundMenuMove)
			case 2:
				g.mapMoveSpeed = clampMapMoveSpeed(g.mapMoveSpeed + adjust*defaultMapMoveSpeed)
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				g.playSound(soundMenuMove)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			switch g.menuIndex {
			case 3:
				g.playSound(soundMenuConfirm)
				return g.fadeToUIState(uiStateKeybindsMenu, func() {
					g.menuIndex = 0
				})
			case 4:
				g.playSound(soundMenuBack)
				return g.fadeToUIState(uiStateOptionsMenu, func() {
					g.menuIndex = 2
				})
			}
			g.playSound(soundMenuConfirm)
		}
	case uiStateGraphicsMenu:
		items := g.graphicsMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateOptionsMenu, func() {
				g.menuIndex = 0
			})
		}
		toggleSelection := func(confirm bool) {
			switch g.menuIndex {
			case 0:
				g.cycleRenderMode()
			case 1:
				if err := g.setHDTexturesEnabled(!g.hdTexturesEnabled); err != nil {
					log.Printf("toggle HD textures failed: %v", err)
				}
			case 2:
				g.vsyncEnabled = !g.vsyncEnabled
				ebiten.SetVsyncEnabled(g.vsyncEnabled)
			}
			if g.menuIndex <= 2 {
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				if confirm {
					g.playSound(soundMenuConfirm)
				} else {
					g.playSound(soundMenuMove)
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
			switch g.menuIndex {
			case 0:
				g.cycleRenderModeBackward()
				if err := g.savePersistentConfig(); err != nil {
					log.Printf("config save failed: %v", err)
				}
				g.playSound(soundMenuMove)
			case 1, 2:
				toggleSelection(false)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
			switch g.menuIndex {
			case 0, 1, 2:
				toggleSelection(false)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			switch g.menuIndex {
			case 0, 1, 2:
				toggleSelection(true)
			case 3:
				g.playSound(soundMenuBack)
				return g.fadeToUIState(uiStateOptionsMenu, func() {
					g.menuIndex = 0
				})
			}
		}
	case uiStateKeybindsMenu:
		if g.keybindCapture {
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
				g.keybindCapture = false
				g.keybindCaptureLabel = ""
				g.playSound(soundMenuBack)
				return nil
			}
			if keys := inpututil.AppendJustPressedKeys(nil); len(keys) > 0 {
				g.assignKeybind(g.menuIndex, g.keybindField, normalizeBindingKey(keys[0]))
				g.keybindCapture = false
				g.keybindCaptureLabel = ""
				g.playSound(soundMenuConfirm)
			}
			return nil
		}
		g.updateMenuSelection(len(g.keybinds))
		if _, wheelY := ebiten.Wheel(); wheelY != 0 {
			prev := g.menuIndex
			if wheelY > 0 {
				g.menuIndex--
			} else if wheelY < 0 {
				g.menuIndex++
			}
			if g.menuIndex < 0 {
				g.menuIndex = len(g.keybinds) - 1
			}
			if g.menuIndex >= len(g.keybinds) {
				g.menuIndex = 0
			}
			if g.menuIndex != prev {
				g.playSound(soundMenuMove)
			}
		}
		if g.keybindJustPressed(keybindPauseBack) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateControlsMenu, func() {
				g.menuIndex = 3
				g.keybindField = 0
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
			if g.keybindField > 0 {
				g.keybindField--
				g.playSound(soundMenuMove)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
			if g.keybindField < 1 {
				g.keybindField++
				g.playSound(soundMenuMove)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			g.clearKeybind(g.menuIndex, g.keybindField)
			g.playSound(soundMenuBack)
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.keybindCapture = true
			g.keybindCaptureLabel = g.keybinds[g.menuIndex].label
			g.playSound(soundMenuConfirm)
			return nil
		}
	case uiStateLoadGame:
		if g.handleSaveLoadActionButton(false) {
			return nil
		}
		g.updateMenuSelection(g.loadGameMenuCount())
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			next := g.menuReturn
			return g.fadeToUIState(next, func() {
				switch next {
				case uiStatePauseMenu:
					g.menuIndex = 2
				default:
					g.menuIndex = 1
				}
			})
		}
		if g.loadGameMenuCount() == 0 {
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			slot := g.loadMenuSelectionSlot()
			if slot < 0 || slot >= len(g.saveSlots) || !g.saveSlots[slot].Used {
				g.playSound(soundMenuBack)
				return nil
			}
			if err := g.loadGame(slot); err != nil {
				log.Printf("save load failed: %v", err)
				g.playSound(soundMenuBack)
				return nil
			}
			g.playSound(soundMenuConfirm)
			return g.fadeToPlaying()
		}
	case uiStateSaveGame:
		if g.saveNameActive {
			if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				g.saveNameActive = false
				g.saveNameInput = ""
				g.playSound(soundMenuBack)
				return nil
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.saveNameInput) > 0 {
				runes := []rune(g.saveNameInput)
				g.saveNameInput = string(runes[:len(runes)-1])
			}
			for _, r := range ebiten.AppendInputChars(nil) {
				if r < 32 || r > 126 {
					continue
				}
				if len([]rune(g.saveNameInput)) >= maxSaveNameLen {
					break
				}
				g.saveNameInput += string(r)
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				name := clampSaveName(g.saveNameInput)
				if name == "" {
					name = g.defaultSaveName()
				}
				if err := g.saveGame(g.saveMenuSelectionSlot(), name); err != nil {
					log.Printf("save failed: %v", err)
					g.playSound(soundMenuBack)
					g.saveNameActive = false
					g.saveNameInput = ""
					g.saveStatusText = "Save failed"
					return nil
				}
				g.playSound(soundMenuConfirm)
				g.saveNameActive = false
				g.saveNameInput = ""
				g.menuIndex = minInt(1, maxInt(0, g.saveGameMenuCount()-1))
				g.saveStatusText = fmt.Sprintf("Saved as \"%s\"", name)
				return nil
			}
			return nil
		}
		if g.handleSaveLoadActionButton(true) {
			return nil
		}
		g.updateMenuSelection(g.saveGameMenuCount())
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			g.saveStatusText = ""
			return g.fadeToUIState(uiStatePauseMenu, func() {
				g.menuIndex = 1
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.saveStatusText = ""
			g.saveNameActive = true
			g.saveNameInput = g.defaultSaveName()
			if summary := g.selectedSaveSlotSummary(true); summary != nil && summary.Used {
				g.saveNameInput = slotSummaryLabel(*summary)
			}
			g.playSound(soundMenuConfirm)
			return nil
		}
	case uiStateEpisodeSelect:
		items := g.episodeMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateMainMenu, func() {
				g.menuIndex = 0
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.selectedEpisode = g.menuIndex
			g.playSound(soundMenuConfirm)
			g.pendingMap = g.selectedEpisode * 10
			g.startNewGame()
			return g.fadeToUIState(uiStateDifficultySelect, func() {
				g.menuReturn = uiStateEpisodeSelect
				g.menuIndex = g.selectedDifficultyIndex()
			})
		}
	case uiStateDifficultySelect:
		items := difficultyMenuItems()
		g.updateMenuSelection(len(items))
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			next := g.menuReturn
			return g.fadeToUIState(next, func() {
				switch next {
				case uiStateEpisodeSelect:
					g.menuIndex = g.selectedEpisode
				case uiStateLevelSelect:
					g.menuIndex = g.selectedLevel
				default:
					g.menuIndex = 0
				}
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.difficulty = menuDifficulty(g.menuIndex)
			g.playSound(soundMenuConfirm)
			return g.startFadeTransition(12, func() error {
				return g.beginGetPsyched(g.pendingMap)
			})
		}
	case uiStateLevelSelect:
		prev := g.menuIndex
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.playSound(soundMenuBack)
			return g.fadeToUIState(uiStateMainMenu, func() {
				g.menuIndex = 1
			})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			g.menuIndex--
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.menuIndex++
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
			g.menuIndex -= 10
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
			g.menuIndex += 10
		}
		if g.menuIndex < 0 {
			g.menuIndex = 0
		}
		if g.menuIndex >= len(g.summaries) {
			g.menuIndex = len(g.summaries) - 1
		}
		if g.menuIndex != prev {
			g.playSound(soundMenuMove)
		}
		g.selectedLevel = g.menuIndex
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.pendingMap = g.summaries[g.selectedLevel].Index
			g.playSound(soundMenuConfirm)
			g.startNewGame()
			return g.fadeToUIState(uiStateDifficultySelect, func() {
				g.menuReturn = uiStateLevelSelect
				g.menuIndex = g.selectedDifficultyIndex()
			})
		}
	case uiStateGetPsyched:
		if time.Since(g.psychedStart) >= time.Second {
			return g.fadeToPlaying()
		}
	case uiStateVictoryIntermission:
		if anyFrontendInput() {
			g.playSound(soundMenuConfirm)
			return g.returnToMainMenu()
		}
	}
	g.syncMusicTrack()
	return nil
}

func (g *game) updateMenuSelection(count int) {
	if count <= 0 {
		g.menuIndex = 0
		return
	}
	prev := g.menuIndex
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.menuIndex--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.menuIndex++
	}
	if g.menuIndex < 0 {
		g.menuIndex = count - 1
	}
	if g.menuIndex >= count {
		g.menuIndex = 0
	}
	if g.menuIndex != prev {
		g.playSound(soundMenuMove)
	}
}

func (g *game) loadGameMenuCount() int {
	return len(g.visibleLoadSlotIndices())
}

func (g *game) saveGameMenuCount() int {
	return g.manualSaveSlotCount() + 1
}

func (g *game) visibleLoadSlotIndices() []int {
	indices := make([]int, 0, len(g.saveSlots))
	for i := range g.saveSlots {
		if g.saveSlots[i].IsAutosave && !g.saveSlots[i].Used {
			continue
		}
		indices = append(indices, i)
	}
	return indices
}

func (g *game) loadMenuSelectionSlot() int {
	indices := g.visibleLoadSlotIndices()
	if g.menuIndex < 0 || g.menuIndex >= len(indices) {
		return -1
	}
	return indices[g.menuIndex]
}

func (g *game) selectLoadSlotByPath(path string) {
	if path == "" {
		return
	}
	indices := g.visibleLoadSlotIndices()
	for i, slot := range indices {
		if slot < 0 || slot >= len(g.saveSlots) {
			continue
		}
		if g.saveSlots[slot].Path == path {
			g.menuIndex = i
			return
		}
	}
}

func (g *game) saveSlotIndexByPath(path string) int {
	if path == "" {
		return -1
	}
	for i := range g.saveSlots {
		if g.saveSlots[i].Path == path {
			return i
		}
	}
	return -1
}

func (g *game) saveMenuSelectionSlot() int {
	if g.menuIndex <= 0 {
		return -1
	}
	return g.manualSaveSlotIndex(g.menuIndex - 1)
}

func (g *game) hasAutosaveSlot() bool {
	return g.autosaveSlotPrefixCount() > 0
}

func (g *game) autosaveSlotPrefixCount() int {
	count := 0
	for i := range g.saveSlots {
		if !g.saveSlots[i].IsAutosave {
			break
		}
		count++
	}
	return count
}

func (g *game) manualSaveSlotCount() int {
	count := len(g.saveSlots)
	count -= g.autosaveSlotPrefixCount()
	return maxInt(0, count)
}

func (g *game) manualSaveSlotIndex(menuSlot int) int {
	if menuSlot < 0 || menuSlot >= g.manualSaveSlotCount() {
		return -1
	}
	return menuSlot + g.autosaveSlotPrefixCount()
}

func (g *game) availableEpisodes() int {
	count := g.files.Variant.EpisodeCount
	if count < 1 {
		count = 1
	}
	maxByMaps := len(g.summaries) / 10
	if maxByMaps < 1 {
		maxByMaps = 1
	}
	if count > maxByMaps {
		count = maxByMaps
	}
	return count
}

func (g *game) keybindPressed(action keybindAction) bool {
	idx := int(action)
	if idx < 0 || idx >= len(g.keybinds) {
		return false
	}
	binding := g.keybinds[idx]
	return keyPressed(binding.primary) || keyPressed(binding.secondary)
}

func (g *game) keybindJustPressed(action keybindAction) bool {
	idx := int(action)
	if idx < 0 || idx >= len(g.keybinds) {
		return false
	}
	binding := g.keybinds[idx]
	return keyJustPressed(binding.primary) || keyJustPressed(binding.secondary)
}

func (g *game) assignKeybind(actionIndex, field int, key ebiten.Key) {
	if actionIndex < 0 || actionIndex >= len(g.keybinds) {
		return
	}
	key = normalizeBindingKey(key)
	if key == keyUnbound {
		return
	}
	for i := range g.keybinds {
		if g.keybinds[i].primary == key {
			g.keybinds[i].primary = keyUnbound
		}
		if g.keybinds[i].secondary == key {
			g.keybinds[i].secondary = keyUnbound
		}
	}
	binding := &g.keybinds[actionIndex]
	if field == 0 {
		binding.primary = key
		if binding.secondary == key {
			binding.secondary = keyUnbound
		}
		if err := g.savePersistentConfig(); err != nil {
			log.Printf("config save failed: %v", err)
		}
		return
	}
	binding.secondary = key
	if binding.primary == key {
		binding.primary = keyUnbound
	}
	if err := g.savePersistentConfig(); err != nil {
		log.Printf("config save failed: %v", err)
	}
}

func (g *game) clearKeybind(actionIndex, field int) {
	if actionIndex < 0 || actionIndex >= len(g.keybinds) {
		return
	}
	if field == 0 {
		g.keybinds[actionIndex].primary = keyUnbound
	} else {
		g.keybinds[actionIndex].secondary = keyUnbound
	}
	if err := g.savePersistentConfig(); err != nil {
		log.Printf("config save failed: %v", err)
	}
}

func (g *game) episodeMenuItems() []string {
	items := []string{
		"Episode 1\nEscape from Wolfenstein",
		"Episode 2\nOperation: Eisenfaust",
		"Episode 3\nDie, Fuhrer, Die!",
		"Episode 4\nA Dark Secret",
		"Episode 5\nTrail of the Madman",
		"Episode 6\nConfrontation",
	}
	count := g.availableEpisodes()
	if count > len(items) {
		count = len(items)
	}
	return items[:count]
}

func difficultyMenuItems() []string {
	return []string{
		"Can I Play, Daddy?",
		"Don't Hurt Me.",
		"Bring 'Em On!",
		"I Am Death Incarnate!",
	}
}

func (g *game) optionsMenuItems() []string {
	return []string{
		submenuLabel("Graphics"),
		submenuLabel("Audio"),
		submenuLabel("Controls"),
		"Back",
	}
}

func (g *game) audioMenuItems() []string {
	return []string{
		fmt.Sprintf("Sound Volume  %3d%%", int(g.sfxVolume*100+0.5)),
		fmt.Sprintf("Music Volume  %3d%%", int(g.musicVolume*100+0.5)),
		"Back",
	}
}

func (g *game) controlsMenuItems() []string {
	return []string{
		fmt.Sprintf("Mouse Sens.   %3d%%", int(g.mouseLook/defaultMouseLook*100+0.5)),
		fmt.Sprintf("Turn Speed    %3d%%", int(g.turnSpeed/defaultTurnSpeed*100+0.5)),
		fmt.Sprintf("Map Move      %3d%%", int(g.mapMoveSpeed/defaultMapMoveSpeed*100+0.5)),
		submenuLabel("Keybinds"),
		"Back",
	}
}

func (g *game) graphicsMenuItems() []string {
	return []string{
		fmt.Sprintf("Render Mode   %s", g.renderMode.label()),
		fmt.Sprintf("HD Textures   %s", onOffLabel(g.hdTexturesEnabled)),
		fmt.Sprintf("VSync         %s", onOffLabel(g.vsyncEnabled)),
		"Back",
	}
}

func submenuLabel(label string) string {
	return label + " >"
}

func (m renderMode) label() string {
	switch m {
	case renderModeDOS:
		return "DOS"
	case renderModeHQ:
		return "HQ"
	default:
		return "ULTRA"
	}
}

func parseRenderMode(text string) renderMode {
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "DOS":
		return renderModeDOS
	case "HQ":
		return renderModeHQ
	default:
		return renderModeUltra
	}
}

func renderModeMenuItems() []string {
	return []string{"DOS", "HQ", "ULTRA"}
}

func renderModeMenuIndex(mode renderMode) int {
	switch mode {
	case renderModeDOS:
		return 0
	case renderModeHQ:
		return 1
	default:
		return 2
	}
}

func renderModeMenuChoice(index int) renderMode {
	switch index {
	case 0:
		return renderModeDOS
	case 1:
		return renderModeHQ
	default:
		return renderModeUltra
	}
}

func (g *game) nextTitleInputState() uiState {
	if g.renderModePrompted {
		return uiStateMainMenu
	}
	return uiStateRenderModePrompt
}

func renderModeDescriptionLines(mode renderMode) []string {
	switch mode {
	case renderModeDOS:
		return []string{
			"320x160 render.",
			"Closest to the original look.",
		}
	case renderModeHQ:
		return []string{
			"640x320 render.",
			"Sharper, still retro.",
		}
	default:
		return []string{
			"Native-size render.",
			"Sharpest image.",
		}
	}
}

func (g *game) setRenderMode(mode renderMode) {
	g.renderMode = mode
	if err := g.refreshRuntimeImageAssets(); err != nil {
		log.Printf("refresh runtime image assets: %v", err)
	}
	g.ensureFrame(g.viewWidth, g.viewHeight)
}

func (g *game) cycleRenderMode() {
	switch g.renderMode {
	case renderModeDOS:
		g.setRenderMode(renderModeHQ)
	case renderModeHQ:
		g.setRenderMode(renderModeUltra)
	default:
		g.setRenderMode(renderModeDOS)
	}
}

func (g *game) cycleRenderModeBackward() {
	switch g.renderMode {
	case renderModeDOS:
		g.setRenderMode(renderModeUltra)
	case renderModeHQ:
		g.setRenderMode(renderModeDOS)
	default:
		g.setRenderMode(renderModeHQ)
	}
}

func (g *game) setHDTexturesEnabled(enabled bool) error {
	g.hdTexturesEnabled = enabled
	if err := g.refreshRuntimeImageAssets(); err != nil {
		return err
	}
	g.ensureFrame(g.viewWidth, g.viewHeight)
	return nil
}

func clampMouseLook(v float64) float64 {
	if v < defaultMouseLook*0.25 {
		return defaultMouseLook * 0.25
	}
	if v > defaultMouseLook*4 {
		return defaultMouseLook * 4
	}
	return v
}

func clampTurnSpeed(v float64) float64 {
	if v < defaultTurnSpeed*0.25 {
		return defaultTurnSpeed * 0.25
	}
	if v > defaultTurnSpeed*4 {
		return defaultTurnSpeed * 4
	}
	return v
}

func clampMapMoveSpeed(v float64) float64 {
	if v < defaultMapMoveSpeed*0.25 {
		return defaultMapMoveSpeed * 0.25
	}
	if v > defaultMapMoveSpeed*4 {
		return defaultMapMoveSpeed * 4
	}
	return v
}

func onOffLabel(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}

func menuDifficulty(index int) gameDifficulty {
	switch index {
	case 0:
		return difficultyEasy
	case 1:
		return difficultyMedium
	default:
		return difficultyHard
	}
}

func (g *game) selectedDifficultyIndex() int {
	switch g.difficulty {
	case difficultyEasy:
		return 0
	case difficultyHard:
		return 2
	default:
		return 1
	}
}

func (g *game) beginGetPsyched(mapIndex int) error {
	if err := g.setMap(mapIndex); err != nil {
		return err
	}
	g.pendingMap = mapIndex
	g.uiState = uiStateGetPsyched
	g.psychedStart = time.Now()
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.mousePrimed = false
	return nil
}

func (g *game) fadeToUIState(next uiState, setup func()) error {
	return g.startFadeTransition(10, func() error {
		if setup != nil {
			setup()
		}
		g.uiState = next
		return nil
	})
}

func (g *game) fadeToPlaying() error {
	return g.startFadeTransition(12, func() error {
		g.ensureFrame(g.viewWidth, g.viewHeight)
		g.uiState = uiStatePlaying
		g.mode = modeRaycast
		g.paused = false
		g.mousePrimed = false
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		return nil
	})
}

func (g *game) loadWolfFonts() {
	g.fonts = make(map[wolfFontKind]*wolfFontRenderer, 2)
	for kind, idx := range map[wolfFontKind]int{
		wolfFontSmall: 0,
		wolfFontLarge: 1,
	} {
		font, raw, err := g.files.LoadRawFont(idx)
		if err != nil {
			continue
		}
		glyphs := make(map[uint32]*ebiten.Image)
		for ch := 0; ch < 256; ch++ {
			if font.Width[ch] == 0 {
				continue
			}
			for _, rgba := range []color.RGBA{
				{0, 0, 0, 255},
				{255, 255, 255, 255},
				{208, 208, 0, 255},
				{255, 176, 64, 255},
				{152, 152, 152, 255},
			} {
				key := uint32(ch)<<24 | uint32(rgba.R)<<16 | uint32(rgba.G)<<8 | uint32(rgba.B)
				img, err := font.GlyphImage(raw, byte(ch), rgba)
				if err != nil {
					continue
				}
				glyphs[key] = ebiten.NewImageFromImage(img)
			}
		}
		g.fonts[kind] = &wolfFontRenderer{font: font, raw: raw, glyphs: glyphs}
	}
}

func (g *game) drawWolfText(canvas *ebiten.Image, x, y int, text string, style wolfTextStyle) {
	renderer := g.fonts[style.font]
	if renderer == nil || renderer.font == nil {
		ebitenutil.DebugPrintAt(canvas, text, x, y)
		return
	}
	scale := style.scale
	if scale <= 0 {
		scale = 1
	}
	if style.shadow != nil {
		g.drawWolfTextRun(canvas, x+scale, y+scale, text, style.font, style.shadow, scale)
	}
	g.drawWolfTextRun(canvas, x, y, text, style.font, style.color, scale)
}

func (g *game) drawWolfTextRun(canvas *ebiten.Image, x, y int, text string, kind wolfFontKind, clr color.Color, scale int) {
	renderer := g.fonts[kind]
	if renderer == nil || renderer.font == nil {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	startX := x
	r, gc, b, _ := clr.RGBA()
	keyColor := uint32(uint8(r>>8))<<16 | uint32(uint8(gc>>8))<<8 | uint32(uint8(b>>8))
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\n' {
			x = startX
			y += renderer.font.Height * scale
			continue
		}
		width := int(renderer.font.Width[ch])
		if width <= 0 {
			continue
		}
		key := uint32(ch)<<24 | keyColor
		glyph := renderer.glyphs[key]
		if glyph == nil {
			img, err := renderer.font.GlyphImage(renderer.raw, ch, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(gc >> 8),
				B: uint8(b >> 8),
				A: 0xff,
			})
			if err != nil || img == nil {
				x += width * scale
				continue
			}
			glyph = ebiten.NewImageFromImage(img)
			renderer.glyphs[key] = glyph
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(scale), float64(scale))
		op.GeoM.Translate(float64(x), float64(y))
		canvas.DrawImage(glyph, op)
		x += width * scale
	}
}

func (g *game) measureWolfText(text string, kind wolfFontKind) (int, int) {
	renderer := g.fonts[kind]
	if renderer == nil || renderer.font == nil {
		return len(text) * 8, 8
	}
	return renderer.font.Measure(text)
}

func wolfTextNormalStyle() wolfTextStyle {
	return wolfTextStyle{
		font:   wolfFontSmall,
		color:  color.RGBA{R: 0xd0, G: 0xd0, B: 0x00, A: 0xff},
		shadow: color.RGBA{A: 0xff},
		scale:  1,
	}
}

func wolfTextLargeStyle() wolfTextStyle {
	return wolfTextStyle{
		font:   wolfFontLarge,
		color:  color.RGBA{R: 0xff, G: 0xb0, B: 0x40, A: 0xff},
		shadow: color.RGBA{A: 0xff},
		scale:  1,
	}
}

func wolfTextMutedStyle() wolfTextStyle {
	return wolfTextStyle{
		font:   wolfFontSmall,
		color:  color.RGBA{R: 0x98, G: 0x98, B: 0x98, A: 0xff},
		shadow: color.RGBA{A: 0xff},
		scale:  1,
	}
}

func wolfTextHintStyle() wolfTextStyle {
	return wolfTextStyle{
		font:   wolfFontSmall,
		color:  color.RGBA{R: 0xe8, G: 0xd8, B: 0xb8, A: 0xff},
		shadow: color.RGBA{R: 0x38, G: 0x18, B: 0x00, A: 0xff},
		scale:  1,
	}
}

func wolfTextSelectedStyle() wolfTextStyle {
	return wolfTextStyle{
		font:   wolfFontSmall,
		color:  color.RGBA{R: 0xff, G: 0xd8, B: 0x70, A: 0xff},
		shadow: color.RGBA{A: 0xff},
		scale:  1,
	}
}

func wolfTextAlertStyle() wolfTextStyle {
	return wolfTextStyle{
		font:   wolfFontSmall,
		color:  color.RGBA{R: 0xff, G: 0x48, B: 0x30, A: 0xff},
		shadow: color.RGBA{R: 0x40, G: 0x00, B: 0x00, A: 0xff},
		scale:  1,
	}
}

func (g *game) drawFrontend(screen *ebiten.Image) {
	canvas := g.frontendCanvas
	if canvas == nil {
		canvas = ebiten.NewImage(wolfScreenWidth, wolfScreenHeight)
		g.frontendCanvas = canvas
	}
	canvas.Fill(color.Black)

	switch g.uiState {
	case uiStateTitle:
		if g.shouldDrawTitleDirectToScreen() {
			screen.Fill(color.Black)
			drawImageFit(screen, g.titlePic, 0, 0, screen.Bounds().Dx(), screen.Bounds().Dy())
			canvas.Fill(color.RGBA{})
		} else {
			g.drawTitleScreen(canvas)
		}
	case uiStateRenderModePrompt:
		g.drawRenderModePrompt(canvas)
	case uiStateMainMenu:
		g.drawMainMenu(canvas)
	case uiStatePauseMenu:
		g.drawPauseMenu(canvas)
	case uiStateOptionsMenu:
		g.drawOptionsMenu(canvas)
	case uiStateAudioMenu:
		g.drawAudioMenu(canvas)
	case uiStateControlsMenu:
		g.drawControlsMenu(canvas)
	case uiStateGraphicsMenu:
		g.drawGraphicsMenu(canvas)
	case uiStateKeybindsMenu:
		g.drawKeybindsMenu(canvas)
	case uiStateLoadGame:
		g.drawSaveLoadMenu(canvas, false)
	case uiStateSaveGame:
		g.drawSaveLoadMenu(canvas, true)
	case uiStateEpisodeSelect:
		g.drawEpisodeSelect(canvas)
	case uiStateDifficultySelect:
		g.drawDifficultySelect(canvas)
	case uiStateLevelSelect:
		g.drawLevelSelect(canvas)
	case uiStateGetPsyched:
		g.drawGetPsyched(canvas)
	case uiStateVictoryIntermission:
		g.drawVictoryIntermission(canvas)
	}
	if g.hudNotice != "" {
		g.drawHudNotice(canvas)
	}

	drawFullscreenImage(screen, canvas)
	g.drawFrontendDirectOverlays(screen)
	switch g.uiState {
	case uiStateLoadGame:
		g.drawSaveSlotPreviewOverlay(screen, false)
	case uiStateSaveGame:
		g.drawSaveSlotPreviewOverlay(screen, true)
	}
}

func (g *game) shouldDrawTitleDirectToScreen() bool {
	if g == nil || g.titlePic == nil {
		return false
	}
	bounds := g.titlePic.Bounds()
	return bounds.Dx() > wolfScreenWidth || bounds.Dy() > wolfScreenHeight
}

func (g *game) shouldDrawPictureChunkDirectToScreen(chunk int) bool {
	if g == nil {
		return false
	}
	img, ok := g.pictureImage(chunk)
	if !ok {
		return false
	}
	width, height := g.pictureChunkSourceSize(chunk)
	if width <= 0 || height <= 0 {
		return false
	}
	bounds := img.Bounds()
	return bounds.Dx() > width || bounds.Dy() > height
}

func (g *game) shouldDrawGetPsychedDirectToScreen() bool {
	if g == nil || g.getPsychedPic == nil {
		return false
	}
	bounds := g.getPsychedPic.Bounds()
	return bounds.Dx() > 224 || bounds.Dy() > 48
}

func (g *game) drawFrontendDirectOverlays(screen *ebiten.Image) {
	switch g.uiState {
	case uiStateMainMenu, uiStateRenderModePrompt, uiStateOptionsMenu, uiStateAudioMenu, uiStateGraphicsMenu:
		g.drawPictureChunkDirectToScreen(screen, g.files.Variant.OptionsPicChunk, 84, 0)
	case uiStateGetPsyched:
		g.drawGetPsychedDirectToScreen(screen)
	}
}

func (g *game) drawPictureChunkDirectToScreen(screen *ebiten.Image, chunk, x, y int) {
	if !g.shouldDrawPictureChunkDirectToScreen(chunk) {
		return
	}
	img, ok := g.pictureImage(chunk)
	if !ok {
		return
	}
	srcW, srcH := g.pictureChunkSourceSize(chunk)
	if srcW <= 0 || srcH <= 0 {
		return
	}
	offsetX, offsetY, scaleX, scaleY := fullscreenImageTransform(screen, wolfScreenWidth)
	dstX := offsetX + float64(x)*scaleX
	dstY := offsetY + float64(y)*scaleY
	dstW := float64(srcW) * scaleX
	dstH := float64(srcH) * scaleY
	drawX, drawY, scale := fitImageRect(img.Bounds().Dx(), img.Bounds().Dy(), dstX, dstY, dstW, dstH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(drawX, drawY)
	screen.DrawImage(img, op)
}

func (g *game) drawGetPsychedDirectToScreen(screen *ebiten.Image) {
	if !g.shouldDrawGetPsychedDirectToScreen() {
		return
	}
	bounds := g.getPsychedPic.Bounds()
	windowW := 224.0
	windowH := 48.0
	windowX := (float64(wolfScreenWidth) - windowW) / 2
	windowY := (float64(wolfGameplayLines) - windowH) / 2
	offsetX, offsetY, scaleX, scaleY := fullscreenImageTransform(screen, wolfScreenWidth)
	dstX := offsetX + windowX*scaleX
	dstY := offsetY + windowY*scaleY
	dstW := windowW * scaleX
	dstH := windowH * scaleY
	drawX, drawY, scale := fitImageRect(bounds.Dx(), bounds.Dy(), dstX, dstY, dstW, dstH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(drawX, drawY)
	screen.DrawImage(g.getPsychedPic, op)

	progress := time.Since(g.psychedStart).Seconds()
	if progress > 1 {
		progress = 1
	}
	barX := drawX + 5
	barY := drawY + float64(bounds.Dy())*scale - 3
	barW := float64(bounds.Dx())*scale - 10
	vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(barW), 2, color.Black, false)
	fillW := math.Floor(barW * progress)
	if fillW > 0 {
		vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(fillW), 2, g.paletteColor(0x37), false)
		if fillW > 1 {
			vector.DrawFilledRect(screen, float32(barX), float32(barY), float32(fillW-1), 1, g.paletteColor(0x32), false)
		}
	}
}

func (g *game) drawTitleScreen(canvas *ebiten.Image) {
	if g.titlePic != nil {
		canvas.Fill(color.Black)
		drawImageFit(canvas, g.titlePic, 0, 0, wolfScreenWidth, wolfScreenHeight)
	} else {
		canvas.Fill(color.RGBA{R: 24, G: 18, B: 18, A: 255})
	}
	if time.Now().UnixMilli()/350%2 == 0 {
		w, _ := g.measureWolfText("PRESS ANY KEY", wolfFontSmall)
		g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 182, "PRESS ANY KEY", wolfTextNormalStyle())
	}
}

func (g *game) drawMainMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.OptionsPicChunk, true)
	g.drawMenuWindow(canvas, mainMenuWindowX, mainMenuWindowY, mainMenuWindowW, mainMenuWindowH)
	g.drawTextLines(canvas, 100, 55, []string{"Start Game", "Load Game", "Level Select", submenuLabel("Options")}, g.menuIndex, 76)
}

func (g *game) drawRenderModePrompt(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.OptionsPicChunk, true)
	g.drawMenuWindow(canvas, 20, 24, 280, 160)
	w, _ := g.measureWolfText("Choose Render Mode", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 28, "Choose Render Mode", wolfTextLargeStyle())
	g.drawWolfText(canvas, 34, 46, "You can change this later in options.", wolfTextHintStyle())

	items := renderModeMenuItems()
	g.drawTextLines(canvas, 58, 66, items, g.menuIndex, 28)

	mode := renderModeMenuChoice(g.menuIndex)
	g.drawWolfText(canvas, 34, 112, fmt.Sprintf("%s:", mode.label()), wolfTextSelectedStyle())
	descY := 124
	for _, line := range renderModeDescriptionLines(mode) {
		for _, wrapped := range g.wrapWolfText(line, 248, wolfFontSmall) {
			g.drawWolfText(canvas, 34, descY, wrapped, wolfTextMutedStyle())
			descY += 10
		}
	}
	g.drawWolfText(canvas, 34, 176, "Enter select  Esc back", wolfTextHintStyle())
}

func (g *game) drawVictoryIntermission(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.ControlPicChunk, true)
	g.drawMenuWindow(canvas, 18, 18, 284, 164)

	title := "Victory!"
	if g.mapIndex >= 0 {
		title = fmt.Sprintf("Floor %02d Clear", g.mapIndex+1)
	}
	w, _ := g.measureWolfText(title, wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 24, title, wolfTextLargeStyle())

	y := 52
	lines := []string{
		fmt.Sprintf("Score     %d", g.score),
		fmt.Sprintf("Secrets   %3d%%", percentInt(g.secretCount, g.secretTotal)),
		fmt.Sprintf("Treasure  %3d%%", percentInt(g.treasureCount, g.treasureTotal)),
		fmt.Sprintf("Health    %3d%%", maxInt(0, g.health)),
	}
	for _, line := range lines {
		g.drawWolfText(canvas, 62, y, line, wolfTextNormalStyle())
		y += 18
	}

	g.drawWolfText(canvas, 34, 136, "BJ made it out.", wolfTextHintStyle())
	g.drawWolfText(canvas, 34, 148, "Press any key.", wolfTextHintStyle())
}

func (g *game) drawPauseMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.ControlPicChunk, true)
	g.drawMenuWindow(canvas, 82, 44, 156, 140)
	g.drawTextLines(canvas, 102, 50, []string{"Resume", "Save Game", "Load Game", submenuLabel("Options"), "Main Menu", "Quit"}, g.menuIndex, 76)
}

func (g *game) drawOptionsMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.OptionsPicChunk, true)
	g.drawMenuWindow(canvas, mainMenuWindowX, mainMenuWindowY, mainMenuWindowW, mainMenuWindowH)
	w, _ := g.measureWolfText("Options", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 58, "Options", wolfTextLargeStyle())
	g.drawTextLines(canvas, 84, 78, g.optionsMenuItems(), g.menuIndex, 60)
}

func (g *game) drawAudioMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.OptionsPicChunk, true)
	g.drawMenuWindow(canvas, mainMenuWindowX, mainMenuWindowY, mainMenuWindowW, mainMenuWindowH)
	w, _ := g.measureWolfText("Audio", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 58, "Audio", wolfTextLargeStyle())
	g.drawTextLines(canvas, 84, 78, g.audioMenuItems(), g.menuIndex, 60)
}

func (g *game) drawControlsMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.ControlPicChunk, true)
	g.drawMenuWindow(canvas, mainMenuWindowX, mainMenuWindowY, mainMenuWindowW, mainMenuWindowH)
	w, _ := g.measureWolfText("Controls", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 58, "Controls", wolfTextLargeStyle())
	g.drawTextLines(canvas, 84, 78, g.controlsMenuItems(), g.menuIndex, 60)
}

func (g *game) drawGraphicsMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.OptionsPicChunk, true)
	g.drawMenuWindow(canvas, mainMenuWindowX, mainMenuWindowY, mainMenuWindowW, mainMenuWindowH)
	w, _ := g.measureWolfText("Graphics", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 58, "Graphics", wolfTextLargeStyle())
	g.drawTextLines(canvas, 84, 78, g.graphicsMenuItems(), g.menuIndex, 60)
}

func (g *game) drawKeybindsMenu(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, g.files.Variant.ControlPicChunk, true)
	g.drawMenuWindow(canvas, 18, 30, 284, 156)
	w, _ := g.measureWolfText("Keybinds", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 36, "Keybinds", wolfTextLargeStyle())

	const (
		actionX     = 34
		primaryX    = 168
		secondaryX  = 226
		rowY        = 64
		rowStep     = 8
		visibleRows = 13
	)
	g.drawWolfText(canvas, actionX, 55, "Action", wolfTextMutedStyle())
	g.drawWolfText(canvas, primaryX, 55, "Primary", wolfTextMutedStyle())
	g.drawWolfText(canvas, secondaryX, 55, "Alt", wolfTextMutedStyle())

	start, end := keybindMenuWindow(len(g.keybinds), g.menuIndex, visibleRows)
	for i := start; i < end; i++ {
		binding := g.keybinds[i]
		y := rowY + (i-start)*rowStep
		labelStyle := wolfTextNormalStyle()
		primaryStyle := wolfTextMutedStyle()
		secondaryStyle := wolfTextMutedStyle()
		if i == g.menuIndex {
			labelStyle = wolfTextSelectedStyle()
			if g.keybindField == 0 {
				primaryStyle = wolfTextSelectedStyle()
			} else {
				secondaryStyle = wolfTextSelectedStyle()
			}
		}
		g.drawWolfText(canvas, actionX, y, binding.label, labelStyle)
		g.drawWolfText(canvas, primaryX, y, keyName(binding.primary), primaryStyle)
		g.drawWolfText(canvas, secondaryX, y, keyName(binding.secondary), secondaryStyle)
	}
	if start > 0 {
		g.drawWolfText(canvas, 280, 55, "^", wolfTextMutedStyle())
	}
	if end < len(g.keybinds) {
		g.drawWolfText(canvas, 280, rowY+(visibleRows-1)*rowStep, "v", wolfTextMutedStyle())
	}

	footer := "Up/Down scroll  Enter change  Del clear  Esc back"
	if g.keybindCapture {
		footer = fmt.Sprintf("Press key for %s  Backspace cancel", g.keybindCaptureLabel)
	}
	g.drawWolfText(canvas, 28, 184, footer, wolfTextMutedStyle())
}

func keybindMenuWindow(count, selected, visibleRows int) (start, end int) {
	if count <= 0 {
		return 0, 0
	}
	if visibleRows <= 0 || count <= visibleRows {
		return 0, count
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= count {
		selected = count - 1
	}
	start = selected - visibleRows/2
	if start < 0 {
		start = 0
	}
	end = start + visibleRows
	if end > count {
		end = count
		start = end - visibleRows
	}
	return start, end
}

func (g *game) drawSaveLoadMenu(canvas *ebiten.Image, saving bool) {
	g.drawMenuBackground(canvas, g.files.Variant.ControlPicChunk, true)
	g.drawMenuWindow(canvas, 8, 20, 304, 168)
	title := "Load Game"
	subtitle := "Choose a save to load."
	footer := "Enter load  Esc back"
	if saving {
		title = "Save Game"
		subtitle = "Choose a slot"
		footer = "Enter rename/save  Esc back"
		if g.saveStatusText != "" {
			footer = g.saveStatusText + "  Esc back"
		}
		if g.saveNameActive {
			subtitle = "Type a save name, or press enter."
		}
	}
	w, _ := g.measureWolfText(title, wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 28, title, wolfTextLargeStyle())
	g.drawWolfText(canvas, 18, 42, subtitle, wolfTextHintStyle())
	g.drawSaveLoadActionButton(canvas, saving)

	listWindowX := 18
	listWindowY := 52
	listWindowW := 126
	listWindowH := 124
	footerY := 170
	g.drawMenuWindow(canvas, listWindowX, listWindowY, listWindowW, listWindowH)

	previewCardX, previewCardY, previewCardW, previewCardH, _, _, _, _ := saveSlotPreviewLayout()
	g.drawSaveSlotPreview(canvas, previewCardX, previewCardY, previewCardW, previewCardH, saving)

	listStartY := listWindowY + 10
	if saving && g.saveNameActive {
		listStartY = listWindowY + 24
	}

	var count int
	if saving {
		count = g.saveGameMenuCount()
	} else {
		count = g.loadGameMenuCount()
	}
	if count == 0 {
		g.drawWolfText(canvas, listWindowX+18, listWindowY+56, "No saved games", wolfTextHintStyle())
		g.drawWolfText(canvas, 18, footerY, footer, wolfTextHintStyle())
		return
	}
	const visibleRows = 9
	start, end := keybindMenuWindow(count, g.menuIndex, visibleRows)
	loadSlots := g.visibleLoadSlotIndices()
	for i := start; i < end; i++ {
		y := listStartY + (i-start)*12
		style := wolfTextNormalStyle()
		label := ""
		selected := i == g.menuIndex
		if saving && i == 0 {
			label = "New Save Slot"
		} else {
			slot := i
			if saving {
				slot = g.manualSaveSlotIndex(i - 1)
			} else if i >= 0 && i < len(loadSlots) {
				slot = loadSlots[i]
			} else {
				slot = -1
			}
			if slot >= 0 && slot < len(g.saveSlots) {
				label = slotSummaryLabel(g.saveSlots[slot])
			}
		}
		label = g.fitWolfText(label, 88, wolfFontSmall)
		if selected {
			style = wolfTextSelectedStyle()
			if saving && g.saveNameActive {
				label = g.saveNameInput
				if time.Now().UnixMilli()/300%2 == 0 {
					label += "_"
				}
				label = g.fitWolfText(label, 88, wolfFontSmall)
			}
		}
		indexLabel := fmt.Sprintf("%d.", i+1)
		slot := -1
		if saving && i > 0 {
			slot = g.manualSaveSlotIndex(i - 1)
		} else if !saving && i >= 0 && i < len(loadSlots) {
			slot = loadSlots[i]
		}
		if saving && i == 0 {
			indexLabel = "+"
		} else if !saving && slot >= 0 && slot < len(g.saveSlots) && g.saveSlots[slot].IsAutosave {
			indexLabel = fmt.Sprintf("A%d", g.saveSlots[slot].AutosaveOrdinal)
		}
		if !selected {
			g.drawWolfText(canvas, listWindowX+12, y, indexLabel, wolfTextHintStyle())
		}
		g.drawWolfText(canvas, listWindowX+32, y, label, style)
	}
	g.drawMenuCursor(canvas, listWindowX+2, listStartY, g.menuIndex-start, 12)
	g.drawWolfText(canvas, 18, footerY, footer, wolfTextHintStyle())
	if saving && g.saveNameActive {
		g.drawWolfText(canvas, listWindowX+10, listWindowY+10, "Enter to save", wolfTextAlertStyle())
	}
}

func (g *game) saveLoadActionButton(saving bool) (menuActionButton, bool) {
	if g == nil || !g.supportsBrowserSaveActions() {
		return menuActionButton{}, false
	}
	button := menuActionButton{
		x:       228,
		y:       28,
		w:       72,
		h:       16,
		label:   "Import",
		enabled: true,
	}
	if saving {
		button.label = "Export"
		button.enabled = !g.saveNameActive
		if g.menuIndex == 0 {
			button.label = "Export All"
			return button, true
		}
		summary := g.selectedSaveSlotSummary(true)
		button.enabled = summary != nil && summary.Used && summary.Path != ""
	}
	return button, true
}

func (g *game) frontendCursorPosition() (x, y float64, ok bool) {
	if g == nil {
		return 0, 0, false
	}
	mouseX, mouseY := ebiten.CursorPosition()
	offsetX, offsetY, scaleX, scaleY := g.wolfDisplayTransform()
	if scaleX <= 0 || scaleY <= 0 {
		return 0, 0, false
	}
	x = (float64(mouseX) - offsetX) / scaleX
	y = (float64(mouseY) - offsetY) / scaleY
	if x < 0 || y < 0 || x >= wolfScreenWidth || y >= wolfScreenHeight {
		return x, y, false
	}
	return x, y, true
}

func (g *game) saveLoadActionButtonHovered(saving bool) bool {
	button, ok := g.saveLoadActionButton(saving)
	if !ok || !button.enabled {
		return false
	}
	mouseX, mouseY, ok := g.frontendCursorPosition()
	if !ok {
		return false
	}
	return mouseX >= float64(button.x) &&
		mouseX < float64(button.x+button.w) &&
		mouseY >= float64(button.y) &&
		mouseY < float64(button.y+button.h)
}

func (g *game) drawSaveLoadActionButton(canvas *ebiten.Image, saving bool) {
	button, ok := g.saveLoadActionButton(saving)
	if !ok {
		return
	}
	fill := color.RGBA{R: 22, G: 18, B: 18, A: 236}
	border := color.RGBA{R: 134, G: 110, B: 62, A: 255}
	textStyle := wolfTextMutedStyle()
	if button.enabled {
		fill = color.RGBA{R: 28, G: 22, B: 12, A: 236}
		border = color.RGBA{R: 188, G: 152, B: 82, A: 255}
		textStyle = wolfTextHintStyle()
	}
	if g.saveLoadActionButtonHovered(saving) {
		fill = color.RGBA{R: 66, G: 44, B: 16, A: 244}
		border = color.RGBA{R: 235, G: 202, B: 116, A: 255}
		textStyle = wolfTextSelectedStyle()
	}
	vector.DrawFilledRect(canvas, float32(button.x), float32(button.y), float32(button.w), float32(button.h), fill, false)
	vector.StrokeRect(canvas, float32(button.x), float32(button.y), float32(button.w), float32(button.h), 1, border, false)
	labelWidth, _ := g.measureWolfText(button.label, wolfFontSmall)
	textX := button.x + (button.w-labelWidth)/2
	textY := button.y + 4
	g.drawWolfText(canvas, textX, textY, button.label, textStyle)
}

func (g *game) handleSaveLoadActionButton(saving bool) bool {
	button, ok := g.saveLoadActionButton(saving)
	if !ok || !button.enabled || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || !g.saveLoadActionButtonHovered(saving) {
		return false
	}
	if saving {
		if g.menuIndex == 0 {
			if err := g.triggerBrowserSaveExportAll(); err != nil {
				g.playSound(soundMenuBack)
				g.setNotice("Export unavailable")
				return true
			}
			g.playSound(soundMenuConfirm)
			g.setNotice("All saves exported")
			return true
		}
		summary := g.selectedSaveSlotSummary(true)
		if summary == nil || !summary.Used || summary.Path == "" {
			g.playSound(soundMenuBack)
			g.setNotice("Select a saved slot to export")
			return true
		}
		if err := g.triggerBrowserSaveExport(summary.Path); err != nil {
			g.playSound(soundMenuBack)
			g.setNotice("Export unavailable")
			return true
		}
		g.playSound(soundMenuConfirm)
		g.setNotice("Save exported")
		return true
	}
	if err := g.triggerBrowserSaveImport(); err != nil {
		g.playSound(soundMenuBack)
		g.setNotice("Import unavailable")
		return true
	}
	g.playSound(soundMenuConfirm)
	return true
}

func (g *game) fitWolfText(text string, maxWidth int, kind wolfFontKind) string {
	if text == "" || maxWidth <= 0 {
		return ""
	}
	if w, _ := g.measureWolfText(text, kind); w <= maxWidth {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		candidate := string(runes) + "..."
		if w, _ := g.measureWolfText(candidate, kind); w <= maxWidth {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "..."
}

func (g *game) wrapWolfText(text string, maxWidth int, kind wolfFontKind) []string {
	if text == "" || maxWidth <= 0 {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, len(words))
	line := words[0]
	for _, word := range words[1:] {
		candidate := line + " " + word
		if w, _ := g.measureWolfText(candidate, kind); w <= maxWidth {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = word
	}
	lines = append(lines, line)
	return lines
}

func (g *game) selectedSaveSlotSummary(saving bool) *saveSlotSummary {
	slot := g.menuIndex
	if saving {
		slot = g.saveMenuSelectionSlot()
	} else {
		slot = g.loadMenuSelectionSlot()
	}
	if slot < 0 || slot >= len(g.saveSlots) {
		return nil
	}
	return &g.saveSlots[slot]
}

func (g *game) drawSaveSlotPreview(canvas *ebiten.Image, x, y, w, h int, saving bool) {
	g.drawMenuWindow(canvas, x, y, w, h)

	_, _, _, _, previewX, previewY, _, _ := saveSlotPreviewLayout()

	summary := g.selectedSaveSlotSummary(saving)
	if summary != nil {
		if summary.thumbnailImageForMenu() == nil {
			g.drawWolfText(canvas, previewX+7, previewY+18, "No preview", wolfTextMutedStyle())
		}
		name := g.fitWolfText(slotSummaryLabel(*summary), w-16, wolfFontSmall)
		g.drawWolfText(canvas, x+8, y+86, name, wolfTextNormalStyle())
		meta := fmt.Sprintf("Map %02d", summary.MapIndex+1)
		if !summary.Timestamp.IsZero() {
			meta += "  " + summary.Timestamp.Format("2006-01-02 15:04")
		}
		g.drawWolfText(canvas, x+8, y+96, g.fitWolfText(meta, w-16, wolfFontSmall), wolfTextMutedStyle())
		return
	}

	g.drawWolfText(canvas, x+8, y+86, "New save", wolfTextHintStyle())
	g.drawWolfText(canvas, x+8, y+96, g.fitWolfText(g.defaultSaveName(), w-16, wolfFontSmall), wolfTextMutedStyle())
}

func saveSlotPreviewLayout() (cardX, cardY, cardW, cardH, previewX, previewY, previewW, previewH int) {
	cardX = 154
	cardY = 52
	cardW = 146
	cardH = 124
	previewW = 96
	previewH = 72
	previewX = cardX + (cardW-previewW)/2
	previewY = cardY + 8
	return
}

func (g *game) drawSaveSlotPreviewOverlay(screen *ebiten.Image, saving bool) {
	summary := g.selectedSaveSlotSummary(saving)
	if summary == nil {
		return
	}
	img := summary.thumbnailImageForMenu()
	if img == nil {
		return
	}

	_, _, _, _, previewX, previewY, previewW, previewH := saveSlotPreviewLayout()
	offsetX, offsetY, scaleX, scaleY := fullscreenImageTransform(screen, wolfScreenWidth)
	dstX := offsetX + float64(previewX)*scaleX
	dstY := offsetY + float64(previewY)*scaleY
	dstW := float64(previewW) * scaleX
	dstH := float64(previewH) * scaleY

	bounds := img.Bounds()
	scale := min(dstW/float64(bounds.Dx()), dstH/float64(bounds.Dy()))
	drawW := float64(bounds.Dx()) * scale
	drawH := float64(bounds.Dy()) * scale

	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(dstX+(dstW-drawW)/2, dstY+(dstH-drawH)/2)
	screen.DrawImage(img, op)
}

func (g *game) drawEpisodeSelect(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, 0, false)
	g.drawMenuWindow(canvas, 6, 19, 308, 162)
	w, _ := g.measureWolfText("Which episode to play?", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 6, "Which episode to play?", wolfTextLargeStyle())
	items := g.episodeMenuItems()
	for i := 0; i < len(items); i++ {
		g.drawPictureChunk(canvas, g.files.Variant.Episode1PicChunk+i, 42, 23+i*26)
		lines := strings.Split(items[i], "\n")
		for lineIdx, line := range lines {
			g.drawWolfText(canvas, 96, 24+i*26+lineIdx*10, line, wolfTextNormalStyle())
		}
	}
	g.drawMenuCursor(canvas, 10, 23, g.menuIndex, 26)
}

func (g *game) drawDifficultySelect(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, 0, false)
	g.drawMenuWindow(canvas, 45, 90, 225, 67)
	w, _ := g.measureWolfText("How tough are you?", wolfFontLarge)
	g.drawWolfText(canvas, (wolfScreenWidth-w)/2, 68, "How tough are you?", wolfTextLargeStyle())
	items := difficultyMenuItems()
	g.drawTextLines(canvas, 74, 100, items, g.menuIndex, 50)
	g.drawPictureChunk(canvas, g.files.Variant.BabyModePicChunk+g.menuIndex, 235, 107)
}

func (g *game) drawLevelSelect(canvas *ebiten.Image) {
	g.drawMenuBackground(canvas, 0, true)
	g.drawPictureChunk(canvas, g.files.Variant.LevelPicChunk, 136, 22)
	g.drawMenuWindow(canvas, 75, 50, 175, 140)
	items := make([]string, len(g.summaries))
	for i, s := range g.summaries {
		items[i] = fmt.Sprintf("%02d  %s", s.Index, s.Name)
	}
	start := g.menuIndex - 4
	if start < 0 {
		start = 0
	}
	end := start + 10
	if end > len(items) {
		end = len(items)
		start = maxInt(0, end-10)
	}
	g.drawTextLines(canvas, 109, 55, items[start:end], g.menuIndex-start, 85)
}

func (g *game) drawGetPsyched(canvas *ebiten.Image) {
	canvas.Fill(color.Black)
	vector.DrawFilledRect(canvas, 0, 0, wolfScreenWidth, wolfGameplayLines, g.paletteColor(127), false)
	windowW := 224.0
	windowH := 48.0
	windowX := (float64(wolfScreenWidth) - windowW) / 2
	windowY := (float64(wolfGameplayLines) - windowH) / 2
	barX := windowX + 5
	barY := windowY + windowH - 3
	barW := windowW - 10
	if g.getPsychedPic != nil && !g.shouldDrawGetPsychedDirectToScreen() {
		drawX, drawY, scale := fitImageRect(g.getPsychedPic.Bounds().Dx(), g.getPsychedPic.Bounds().Dy(), windowX, windowY, windowW, windowH)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(drawX, drawY)
		canvas.DrawImage(g.getPsychedPic, op)
		barX = drawX + 5
		barY = drawY + float64(g.getPsychedPic.Bounds().Dy())*scale - 3
		barW = float64(g.getPsychedPic.Bounds().Dx())*scale - 10
	}
	progress := time.Since(g.psychedStart).Seconds()
	if progress > 1 {
		progress = 1
	}
	vector.DrawFilledRect(canvas, float32(barX), float32(barY), float32(barW), 2, color.Black, false)
	fillW := math.Floor(barW * progress)
	if fillW > 0 {
		vector.DrawFilledRect(canvas, float32(barX), float32(barY), float32(fillW), 2, g.paletteColor(0x37), false)
		if fillW > 1 {
			vector.DrawFilledRect(canvas, float32(barX), float32(barY), float32(fillW-1), 1, g.paletteColor(0x32), false)
		}
	}
}

func (g *game) drawMenuBackground(canvas *ebiten.Image, headerChunk int, stripes bool) {
	canvas.Fill(g.paletteColor(0x2d))
	if stripes {
		vector.DrawFilledRect(canvas, 0, 10, wolfScreenWidth, 24, color.Black, false)
		vector.DrawFilledRect(canvas, 0, 32, wolfScreenWidth, 1, g.paletteColor(0x2c), false)
	}
	if headerChunk > 0 {
		g.drawPictureChunk(canvas, headerChunk, 84, 0)
	}
	g.drawPictureChunk(canvas, g.files.Variant.MouseBackPicChunk, 112, 184)
}

func (g *game) drawMenuWindow(canvas *ebiten.Image, x, y, w, h int) {
	fill := g.paletteColor(0x2d)
	edge := g.paletteColor(0x17)
	shadow := g.paletteColor(0x15)
	vector.DrawFilledRect(canvas, float32(x), float32(y), float32(w), float32(h), fill, false)
	vector.StrokeRect(canvas, float32(x), float32(y), float32(w), float32(h), 1, edge, false)
	vector.StrokeLine(canvas, float32(x+1), float32(y+h), float32(x+w), float32(y+h), 1, shadow, false)
	vector.StrokeLine(canvas, float32(x+w), float32(y+1), float32(x+w), float32(y+h), 1, shadow, false)
}

func (g *game) drawTextLines(canvas *ebiten.Image, x, y int, items []string, selected, cursorX int) {
	for i, item := range items {
		style := wolfTextNormalStyle()
		if i == selected {
			style = wolfTextLargeStyle()
			style.font = wolfFontSmall
		}
		g.drawWolfText(canvas, x, y+i*13, item, style)
	}
	if selected >= 0 && selected < len(items) {
		g.drawMenuCursor(canvas, cursorX, y, selected, 13)
	}
}

func (g *game) drawMenuCursor(canvas *ebiten.Image, x, y, selected, rowHeight int) {
	cursorChunk := g.files.Variant.Cursor1PicChunk
	if time.Now().UnixMilli()/200%2 == 1 {
		cursorChunk = g.files.Variant.Cursor2PicChunk
	}
	g.drawPictureChunk(canvas, cursorChunk, x, y+selected*rowHeight-2)
}

func (g *game) drawPictureChunk(dst *ebiten.Image, chunk, x, y int) {
	if g.shouldDrawPictureChunkDirectToScreen(chunk) {
		return
	}
	img, ok := g.pictureImage(chunk)
	if !ok {
		return
	}
	drawW, drawH := g.pictureChunkSourceSize(chunk)
	if drawW <= 0 || drawH <= 0 {
		drawW = img.Bounds().Dx()
		drawH = img.Bounds().Dy()
	}
	op := &ebiten.DrawImageOptions{}
	if img.Bounds().Dx() != drawW || img.Bounds().Dy() != drawH {
		drawX, drawY, scale := fitImageRect(img.Bounds().Dx(), img.Bounds().Dy(), float64(x), float64(y), float64(drawW), float64(drawH))
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(drawX, drawY)
	} else {
		op.GeoM.Translate(float64(x), float64(y))
	}
	dst.DrawImage(img, op)
}

func (g *game) pictureChunkSourceSize(chunk int) (int, int) {
	if chunk <= 0 || g == nil || g.files == nil {
		return 0, 0
	}
	if g.pictureSizeCache == nil {
		g.pictureSizeCache = make(map[int]pictureChunkSize)
	}
	if size, ok := g.pictureSizeCache[chunk]; ok {
		return size.width, size.height
	}
	pic, err := g.files.LoadPicture(chunk)
	if err != nil || pic == nil {
		g.pictureSizeCache[chunk] = pictureChunkSize{}
		return 0, 0
	}
	size := pictureChunkSize{width: pic.Width, height: pic.Height}
	g.pictureSizeCache[chunk] = size
	return size.width, size.height
}

func drawFullscreenImage(screen, img *ebiten.Image) {
	if img == nil {
		return
	}
	offsetX, offsetY, scale, scaleY := fullscreenImageTransform(screen, img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scaleY)
	op.GeoM.Translate(offsetX, offsetY)
	screen.DrawImage(img, op)
}

func drawImageFit(dst, src *ebiten.Image, x, y, width, height int) {
	if dst == nil || src == nil || width <= 0 || height <= 0 {
		return
	}
	drawX, drawY, scale := fitImageRect(src.Bounds().Dx(), src.Bounds().Dy(), float64(x), float64(y), float64(width), float64(height))
	if scale <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(drawX, drawY)
	dst.DrawImage(src, op)
}

func fitImageRect(srcWidth, srcHeight int, dstX, dstY, dstWidth, dstHeight float64) (drawX, drawY, scale float64) {
	if srcWidth <= 0 || srcHeight <= 0 || dstWidth <= 0 || dstHeight <= 0 {
		return dstX, dstY, 0
	}
	scale = math.Min(dstWidth/float64(srcWidth), dstHeight/float64(srcHeight))
	drawWidth := float64(srcWidth) * scale
	drawHeight := float64(srcHeight) * scale
	drawX = dstX + (dstWidth-drawWidth)/2
	drawY = dstY + (dstHeight-drawHeight)/2
	return drawX, drawY, scale
}

func fullscreenImageTransform(screen *ebiten.Image, imageWidth int) (offsetX, offsetY, scaleX, scaleY float64) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	scaleX = math.Min(float64(sw)/float64(imageWidth), float64(sh)/float64(wolfDisplayHeight))
	scaleY = scaleX * float64(wolfDisplayHeight) / float64(wolfScreenHeight)
	offsetX = (float64(sw) - float64(imageWidth)*scaleX) / 2
	offsetY = (float64(sh) - float64(wolfDisplayHeight)*scaleX) / 2
	return offsetX, offsetY, scaleX, scaleY
}

func anyFrontendInput() bool {
	return len(inpututil.AppendJustPressedKeys(nil)) > 0 ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
}

func (g *game) drawMap(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 18, G: 20, B: 24, A: 255})

	if g.mapData == nil {
		ebitenutil.DebugPrint(screen, "no map loaded")
		return
	}

	tileSize := float32(baseTileSize * g.zoom)
	offsetX := float32(g.viewWidth)/2 - float32(g.cameraX)*tileSize
	offsetY := float32(g.viewHeight)/2 - float32(g.cameraY)*tileSize

	width := g.levelWidth
	height := g.levelHeight
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			tile := g.level.Tile(x, y)
			if !g.mapTileVisible(x, y, tile) {
				continue
			}
			px := offsetX + float32(x)*tileSize
			py := offsetY + float32(y)*tileSize
			if px+tileSize < 0 || py+tileSize < 0 || px > float32(g.viewWidth) || py > float32(g.viewHeight) {
				continue
			}

			g.drawMapTile(screen, tile, float64(px), float64(py), float64(tileSize))
		}
	}

	g.drawMapSprites(screen, tileSize, offsetX, offsetY)
	g.drawMapActors(screen, tileSize, offsetX, offsetY)
	g.drawMapPlayer(screen, tileSize, offsetX, offsetY)
}

func (g *game) drawMapTile(screen *ebiten.Image, tile wl6.Tile, px, py, tileSize float64) {
	fill := color.RGBA{R: 36, G: 40, B: 44, A: 255}
	if !tile.Solid {
		fill = color.RGBA{R: 70, G: 74, B: 60, A: 255}
	}
	vector.DrawFilledRect(screen, float32(px), float32(py), float32(tileSize), float32(tileSize), fill, false)

	if img, ok := g.mapTileTexture(tile); ok {
		op := &ebiten.DrawImageOptions{}
		bounds := img.Bounds()
		op.Filter = mapTextureFilter(tileSize, bounds.Dx(), bounds.Dy())
		op.GeoM.Scale(tileSize/float64(bounds.Dx()), tileSize/float64(bounds.Dy()))
		op.GeoM.Translate(px, py)
		screen.DrawImage(img, op)
	}

	if tileSize >= 8 {
		stroke := color.RGBA{R: 20, G: 22, B: 24, A: 160}
		vector.StrokeRect(screen, float32(px), float32(py), float32(tileSize), float32(tileSize), 1, stroke, false)
	}
}

func mapTextureFilter(tileSize float64, texWidth, texHeight int) ebiten.Filter {
	if texWidth <= 0 || texHeight <= 0 {
		return ebiten.FilterNearest
	}
	if tileSize < float64(texWidth) || tileSize < float64(texHeight) {
		return ebiten.FilterLinear
	}
	return ebiten.FilterNearest
}

func (g *game) drawMapSprites(screen *ebiten.Image, tileSize, offsetX, offsetY float32) {
	if tileSize < 3 {
		return
	}

	size := maxf(8, tileSize*0.8)
	for _, spr := range g.staticSprites {
		if !spr.alive {
			continue
		}
		if !g.isReachableTile(int(spr.x), int(spr.y)) {
			continue
		}
		px := offsetX + float32(spr.x)*tileSize
		py := offsetY + float32(spr.y)*tileSize
		if px+size/2 < 0 || py+size/2 < 0 || px-size/2 > float32(g.viewWidth) || py-size/2 > float32(g.viewHeight) {
			continue
		}
		if !g.drawMapSpriteShape(screen, spr.shapenum, px, py, size) {
			fill := color.RGBA{R: 214, G: 182, B: 86, A: 220}
			if spr.blocking {
				fill = color.RGBA{R: 170, G: 120, B: 72, A: 220}
			}
			half := size / 2
			vector.DrawFilledRect(screen, px-half, py-half, size, size, fill, false)
		}
	}
	if g.victoryActive && g.victoryBJ.alive {
		px := offsetX + float32(g.victoryBJ.x)*tileSize
		py := offsetY + float32(g.victoryBJ.y)*tileSize
		if px+size/2 >= 0 && py+size/2 >= 0 && px-size/2 <= float32(g.viewWidth) && py-size/2 <= float32(g.viewHeight) {
			g.drawMapSpriteShape(screen, g.victoryBJ.shapenum, px, py, size)
		}
	}
}

func (g *game) drawMapActors(screen *ebiten.Image, tileSize, offsetX, offsetY float32) {
	if tileSize < 3 {
		return
	}

	size := maxf(10, tileSize*0.92)
	for _, actor := range g.actors {
		if !actor.alive {
			continue
		}
		if !g.isReachableTile(actor.tileX, actor.tileY) {
			continue
		}
		px := offsetX + float32(actor.x)*tileSize
		py := offsetY + float32(actor.y)*tileSize
		if px+size/2 < 0 || py+size/2 < 0 || px-size/2 > float32(g.viewWidth) || py-size/2 > float32(g.viewHeight) {
			continue
		}

		if !g.drawMapSpriteShape(screen, actor.shapenum, px, py, size) {
			fill, outline := mapActorColors(actor)
			radius := maxf(4, tileSize*0.24)
			vector.DrawFilledCircle(screen, px, py, radius, fill, false)
			vector.StrokeCircle(screen, px, py, radius+1, 1.5, outline, false)

			dirX, dirY := actorMapDirection(actor)
			if dirX != 0 || dirY != 0 {
				vector.StrokeLine(
					screen,
					px,
					py,
					px+dirX*tileSize*0.55,
					py+dirY*tileSize*0.55,
					maxf(1.5, tileSize*0.08),
					outline,
					false,
				)
			}
			continue
		}
		if actor.alerted {
			half := size / 2
			vector.StrokeRect(screen, px-half, py-half, size, size, maxf(1.5, tileSize*0.08), color.RGBA{R: 255, G: 230, B: 120, A: 255}, false)
		}
	}
}

func (g *game) drawMapPlayer(screen *ebiten.Image, tileSize, offsetX, offsetY float32) {
	playerPX := offsetX + float32(g.playerX)*tileSize
	playerPY := offsetY + float32(g.playerY)*tileSize
	size := maxf(12, tileSize*1.05)
	radius := maxf(5, tileSize*0.24)
	headingLen := maxf(radius+2, tileSize*0.75)
	fill := color.RGBA{R: 240, G: 56, B: 36, A: 255}
	outline := color.RGBA{R: 52, G: 12, B: 8, A: 255}

	if !g.drawMapSpriteShape(screen, mapPlayerShape, playerPX, playerPY, size) {
		vector.DrawFilledCircle(screen, playerPX, playerPY, radius, fill, false)
	}
	vector.StrokeCircle(screen, playerPX, playerPY, radius+1, 2, color.RGBA{R: 255, G: 225, B: 170, A: 255}, false)
	vector.StrokeLine(
		screen,
		playerPX,
		playerPY,
		playerPX+float32(math.Cos(g.playerA))*headingLen,
		playerPY+float32(math.Sin(g.playerA))*headingLen,
		maxf(2, tileSize*0.1),
		outline,
		false,
	)
}

func actorMapDirection(actor actorInstance) (float32, float32) {
	if actor.hasGoal {
		dx := float32(float64(actor.tileX) + 0.5 - actor.x)
		dy := float32(float64(actor.tileY) + 0.5 - actor.y)
		if dx != 0 || dy != 0 {
			length := float32(math.Hypot(float64(dx), float64(dy)))
			if length > 0 {
				return dx / length, dy / length
			}
		}
	}

	angle := facingDirAngle(actor.facingDir)
	return float32(math.Cos(angle)), float32(-math.Sin(angle))
}

func mapActorColors(actor actorInstance) (fill, outline color.RGBA) {
	switch actor.kind {
	case actorKindDog:
		fill = color.RGBA{R: 176, G: 118, B: 70, A: 235}
		outline = color.RGBA{R: 72, G: 38, B: 16, A: 255}
	case actorKindOfficer:
		fill = color.RGBA{R: 86, G: 152, B: 226, A: 235}
		outline = color.RGBA{R: 24, G: 48, B: 80, A: 255}
	case actorKindSS:
		fill = color.RGBA{R: 214, G: 214, B: 214, A: 235}
		outline = color.RGBA{R: 60, G: 60, B: 60, A: 255}
	case actorKindBoss:
		fill = color.RGBA{R: 240, G: 188, B: 56, A: 235}
		outline = color.RGBA{R: 110, G: 58, B: 12, A: 255}
	case actorKindMutant:
		fill = color.RGBA{R: 168, G: 90, B: 190, A: 235}
		outline = color.RGBA{R: 72, G: 24, B: 88, A: 255}
	default:
		fill = color.RGBA{R: 184, G: 64, B: 48, A: 235}
		outline = color.RGBA{R: 76, G: 18, B: 12, A: 255}
	}
	if actor.alerted {
		outline = color.RGBA{R: 255, G: 230, B: 120, A: 255}
	}
	return fill, outline
}

func (g *game) drawMapSpriteShape(screen *ebiten.Image, shape int, centerX, centerY, size float32) bool {
	img, ok := g.spriteImageByShape(shape)
	if !ok {
		return false
	}
	return drawMapImageCentered(screen, img, centerX, centerY, size)
}

func drawMapImageCentered(screen, img *ebiten.Image, centerX, centerY, size float32) bool {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return false
	}
	scale := float64(size) / math.Max(float64(bounds.Dx()), float64(bounds.Dy()))
	drawW := float32(float64(bounds.Dx()) * scale)
	drawH := float32(float64(bounds.Dy()) * scale)
	op := &ebiten.DrawImageOptions{}
	op.Filter = mapTextureFilter(float64(size), bounds.Dx(), bounds.Dy())
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(centerX-drawW/2), float64(centerY-drawH/2))
	screen.DrawImage(img, op)
	return true
}

func (g *game) spriteImageByShape(shape int) (*ebiten.Image, bool) {
	if g.sprites == nil {
		return nil, false
	}
	if img, ok := g.spriteImageCache[shape]; ok {
		return img, true
	}
	return nil, false
}

func (g *game) buildSpriteImage(sprite *wl6.Sprite) (*ebiten.Image, bool) {
	if sprite == nil || sprite.Width <= 0 || sprite.Height <= 0 || (!sprite.IndexedFormat && len(sprite.Pixels) == 0) {
		return nil, false
	}
	pix := make([]byte, sprite.Width*sprite.Height*4)
	if sprite.IndexedFormat && g.walls != nil {
		for x, column := range sprite.Columns {
			for _, post := range column.Posts {
				for y, colorIndex := range post.Indexed {
					i := ((post.StartY+y)*sprite.Width + x) * 4
					rgba := g.walls.Palette[colorIndex]
					pix[i] = byte(rgba >> 24)
					pix[i+1] = byte(rgba >> 16)
					pix[i+2] = byte(rgba >> 8)
					pix[i+3] = byte(rgba)
				}
			}
		}
	} else {
		for y := 0; y < sprite.Height; y++ {
			for x := 0; x < sprite.Width; x++ {
				rgba := sprite.Pixels[y*sprite.Width+x]
				i := (y*sprite.Width + x) * 4
				pix[i] = byte(rgba >> 24)
				pix[i+1] = byte(rgba >> 16)
				pix[i+2] = byte(rgba >> 8)
				pix[i+3] = byte(rgba)
			}
		}
	}
	img := ebiten.NewImage(sprite.Width, sprite.Height)
	img.WritePixels(pix)
	return img, true
}

func (g *game) rebuildSpriteImageCache() {
	if g == nil || g.sprites == nil {
		g.spriteImageCache = nil
		return
	}
	cache := make(map[int]*ebiten.Image, len(g.sprites.Pages))
	for shape := range g.sprites.Pages {
		sprite, ok := g.sprites.SpriteByShape(shape)
		if !ok {
			continue
		}
		img, ok := g.buildSpriteImage(sprite)
		if !ok {
			continue
		}
		cache[shape] = img
	}
	g.spriteImageCache = cache
}

func drawIndexedSpritePostInto(dst []uint32, stride, startIndex int, texPos, texStep uint32, yStart, yEnd int, post wl6.SpritePost, renderLUT *[2][256]uint32, lutDim int) {
	i := startIndex
	for y := yStart; y < yEnd; y++ {
		texY := int(texPos >> 16)
		if texY >= post.StartY && texY < post.EndY {
			dst[i] = renderLUT[lutDim][post.Indexed[texY-post.StartY]]
		}
		texPos += texStep
		i += stride
	}
}

func scaledPostBounds(spriteTop, spriteSize int, post wl6.SpritePost) (top, bottom int) {
	top = spriteTop + int((post.StartT*uint32(spriteSize))>>16)
	bottom = spriteTop + int((post.EndT*uint32(spriteSize))>>16)
	return top, bottom
}

func drawRGBASpritePostInto(dst []uint32, stride, startIndex int, texPos, texStep uint32, yStart, yEnd int, post wl6.SpritePost) {
	i := startIndex
	for y := yStart; y < yEnd; y++ {
		texY := int(texPos >> 16)
		if texY >= post.StartY && texY < post.EndY {
			writeRGBAOverBuffer32(dst, i, post.Pixels[texY-post.StartY])
		}
		texPos += texStep
		i += stride
	}
}

func clampPostDrawRange(spriteTop int, texStep uint32, yStart, yEnd int, post wl6.SpritePost) (int, int) {
	for yStart < yEnd {
		texY := int((uint32(yStart-spriteTop) * texStep) >> 16)
		if texY >= post.StartY && texY < post.EndY {
			break
		}
		yStart++
	}
	for yEnd > yStart {
		texY := int((uint32(yEnd-1-spriteTop) * texStep) >> 16)
		if texY >= post.StartY && texY < post.EndY {
			break
		}
		yEnd--
	}
	return yStart, yEnd
}

func (g *game) mapTileVisible(x, y int, tile wl6.Tile) bool {
	if tile.RenderWall {
		return g.isRenderableWall(x, y)
	}
	return g.isReachableTile(x, y)
}

func (g *game) mapTileTexture(tile wl6.Tile) (*ebiten.Image, bool) {
	if g.walls == nil {
		return nil, false
	}
	if tile.Door != nil {
		texture := g.pickDoorTexture(tile.RawWall, 0)
		return g.wallTextureImage(texture)
	}
	if !tile.RenderWall {
		return nil, false
	}
	texture := g.pickWallTexture(renderWallTile(tile), 0, -1)
	return g.wallTextureImage(texture)
}

func buildWeaponOverlayGeometry(cache *weaponOverlayGeometry, screenW, screenH, left, top int) weaponOverlayGeometry {
	if cache.valid && cache.screenW == screenW && cache.screenH == screenH && cache.left == left && cache.top == top {
		return *cache
	}
	spriteSize := int(math.Round(float64(screenH) * float64(wolfGameplayLines) / float64(wolfScreenHeight)))
	if spriteSize < 1 {
		spriteSize = 1
	}
	spriteTop := top + screenH - spriteSize
	spriteScreenX := left + screenW/2
	drawTop := maxInt(top, spriteTop)
	drawBottom := minInt(top+screenH, spriteTop+spriteSize)
	startX := maxInt(left, spriteScreenX-spriteSize/2)
	endX := minInt(left+screenW, spriteScreenX+spriteSize/2)
	*cache = weaponOverlayGeometry{
		valid:         true,
		screenW:       screenW,
		screenH:       screenH,
		left:          left,
		top:           top,
		startX:        startX,
		endX:          endX,
		drawTop:       drawTop,
		drawBottom:    drawBottom,
		spriteTop:     spriteTop,
		spriteScreenX: spriteScreenX,
		spriteSize:    spriteSize,
	}
	return *cache
}

func (g *game) wallTextureImage(texture *wl6.WallTexture) (*ebiten.Image, bool) {
	if texture == nil || texture.Empty() {
		return nil, false
	}
	if g.textureCache == nil {
		g.textureCache = make(map[*wl6.WallTexture]*ebiten.Image)
	}
	if img, ok := g.textureCache[texture]; ok {
		return img, true
	}
	width, height := texture.Size()
	pix := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rgba := texture.Pixel(x, y)
			i := (y*width + x) * 4
			pix[i] = byte(rgba >> 24)
			pix[i+1] = byte(rgba >> 16)
			pix[i+2] = byte(rgba >> 8)
			pix[i+3] = byte(rgba)
		}
	}
	img := ebiten.NewImage(width, height)
	img.WritePixels(pix)
	g.textureCache[texture] = img
	return img, true
}

func (g *game) drawRaycast(screen *ebiten.Image) {
	g.drawRaycastScaled(screen)
}

func (g *game) resetUltraWallSpans() {
	if g == nil || g.renderMode != renderModeUltra {
		return
	}
	renderLeft, renderRight, renderTop, renderBottom, _, _ := g.gameplayRenderArea()
	if renderRight <= renderLeft {
		return
	}
	if renderLeft < 0 {
		renderLeft = 0
	}
	if renderRight > len(g.prevWallTops) {
		renderRight = len(g.prevWallTops)
	}
	for i := renderLeft; i < renderRight; i++ {
		g.prevWallTops[i] = renderBottom
		g.prevWallBottoms[i] = renderTop
	}
}

func (g *game) drawRaycastScaled(screen *ebiten.Image) {
	if g.mapData == nil {
		ebitenutil.DebugPrint(screen, "no map loaded")
		return
	}
	if len(g.gameplayFrame) == 0 || len(g.gameplayBackground) != len(g.gameplayFrame) || g.gameplayImage == nil {
		g.ensureRenderBuffers()
	}
	if len(g.gameplayFrame) == 0 || g.layout.bufferWidth <= 0 || g.layout.bufferHeight <= 0 {
		return
	}

	copy(g.gameplayFrame, g.gameplayBackground)
	forwardX := math.Cos(g.playerA)
	forwardY := math.Sin(g.playerA)
	bufferWidth := g.layout.bufferWidth
	planeScale := math.Tan(fov / 2)
	planeX := -forwardY * planeScale
	planeY := forwardX * planeScale
	projPlaneDist := float64(bufferWidth) / (2 * planeScale)
	g.prepareRaycastDirections(0, bufferWidth, forwardX, forwardY, planeX, planeY)
	g.renderRaycastColumns(true, 0, bufferWidth, forwardX, forwardY, planeX, planeY, projPlaneDist)
	for i := 0; i < bufferWidth && i < len(g.columnCoverage); i++ {
		g.columnCoverage[i].clear()
	}
	g.drawWeaponOverlayScaled()
	g.drawSpritesScaled(forwardX, forwardY, planeX, planeY, projPlaneDist)
	g.drawWallColumnsScaled()

	if g.backgroundImage != nil {
		screen.DrawImage(g.backgroundImage, nil)
	} else {
		screen.WritePixels(g.background)
	}
	g.presentGameplayBuffer(screen)
	g.drawStatusBar(screen)
	if g.paused {
		g.drawPausedOverlay(screen)
	}
}

func (g *game) presentGameplayBuffer(screen *ebiten.Image) {
	if g.gameplayImage == nil || len(g.gameplayFrame) == 0 || g.layout.bufferWidth <= 0 || g.layout.bufferHeight <= 0 {
		return
	}
	g.gameplayImage.WritePixels(g.gameplayFrame)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(g.layout.renderWidth)/float64(g.layout.bufferWidth), float64(g.layout.renderHeight)/float64(g.layout.bufferHeight))
	op.GeoM.Translate(float64(g.layout.renderLeft), float64(g.layout.renderTop))
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.gameplayImage, op)
}

func (g *game) renderRaycastColumns(scaled bool, startX, endX int, forwardX, forwardY, planeX, planeY, projPlaneDist float64) {
	if endX <= startX {
		return
	}
	workers := g.effectiveRenderWorkers()
	jobTemplate := raycastJob{
		scaled:        scaled,
		forwardX:      forwardX,
		forwardY:      forwardY,
		planeX:        planeX,
		planeY:        planeY,
		projPlaneDist: projPlaneDist,
		rayDirX:       g.rayDirXColumns,
		rayDirY:       g.rayDirYColumns,
		zbuffer:       g.zbuffer,
	}
	if scaled {
		jobTemplate.dst = g.gameplayFrame32
		jobTemplate.stride = g.layout.bufferWidth
		jobTemplate.bufferHeight = g.layout.bufferHeight
	} else {
		_, _, renderTop, renderBottom, _, renderHeight := g.gameplayRenderArea()
		jobTemplate.dst = g.frame32
		jobTemplate.stride = g.viewWidth
		jobTemplate.renderTop = renderTop
		jobTemplate.renderBottom = renderBottom
		jobTemplate.renderHeight = renderHeight
		jobTemplate.prevWallTops = g.prevWallTops
		jobTemplate.prevWallBottoms = g.prevWallBottoms
	}
	maxEndX := endX
	if maxEndX > len(jobTemplate.rayDirX) {
		maxEndX = len(jobTemplate.rayDirX)
	}
	if maxEndX > len(jobTemplate.rayDirY) {
		maxEndX = len(jobTemplate.rayDirY)
	}
	if maxEndX > len(jobTemplate.zbuffer) {
		maxEndX = len(jobTemplate.zbuffer)
	}
	if jobTemplate.stride > 0 {
		if dstWidth := jobTemplate.stride; maxEndX > dstWidth {
			maxEndX = dstWidth
		}
	}
	if startX < 0 {
		startX = 0
	}
	if maxEndX <= startX {
		return
	}
	if workers <= 1 {
		job := jobTemplate
		job.startX = startX
		job.endX = maxEndX
		g.renderRaycastStripe(job)
		return
	}
	g.ensureRaycastWorkers(workers)
	stripeWidth := maxInt(16, (maxEndX-startX+workers*4-1)/(workers*4))
	var wg sync.WaitGroup
	for stripeStart := startX; stripeStart < maxEndX; stripeStart += stripeWidth {
		stripeEnd := minInt(stripeStart+stripeWidth, maxEndX)
		wg.Add(1)
		job := jobTemplate
		job.startX = stripeStart
		job.endX = stripeEnd
		job.wg = &wg
		g.raycastJobs <- job
	}
	wg.Wait()
}

func (g *game) prepareRaycastDirections(startX, endX int, forwardX, forwardY, planeX, planeY float64) {
	if endX <= startX {
		return
	}
	if startX < 0 {
		startX = 0
	}
	if endX > len(g.cameraColumns) {
		endX = len(g.cameraColumns)
	}
	if endX > len(g.rayDirXColumns) {
		endX = len(g.rayDirXColumns)
	}
	if endX > len(g.rayDirYColumns) {
		endX = len(g.rayDirYColumns)
	}
	for x := startX; x < endX; x++ {
		cameraX := g.cameraColumns[x]
		g.rayDirXColumns[x] = forwardX + planeX*cameraX
		g.rayDirYColumns[x] = forwardY + planeY*cameraX
	}
}

func (g *game) effectiveRenderWorkers() int {
	workers := g.renderThreads
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

func (g *game) ensureRaycastWorkers(workers int) {
	if workers < 1 {
		workers = 1
	}
	if g.raycastJobs != nil && g.raycastWorkerCount == workers {
		return
	}
	if g.raycastJobs != nil {
		close(g.raycastJobs)
	}
	g.raycastJobs = make(chan raycastJob, workers*4)
	g.raycastWorkerCount = workers
	for range workers {
		go func(jobs <-chan raycastJob) {
			for job := range jobs {
				g.renderRaycastStripe(job)
				job.wg.Done()
			}
		}(g.raycastJobs)
	}
}

func (g *game) renderRaycastStripe(job raycastJob) {
	if job.scaled {
		bufferHeight := job.bufferHeight
		viewHeight := float64(bufferHeight)
		for x := job.startX; x < job.endX; x++ {
			rayDirX := job.rayDirX[x]
			rayDirY := job.rayDirY[x]
			hitDist, wallID, side, texU, texOverride := g.castRay(rayDirX, rayDirY)
			if wallID == 0 {
				job.zbuffer[x] = maxRayDepth
				if x >= 0 && x < len(g.wallColumns) {
					g.wallColumns[x] = wallColumn{}
				}
				continue
			}
			if hitDist < 0.0001 {
				hitDist = 0.0001
			}
			job.zbuffer[x] = hitDist
			lineHeight := job.projPlaneDist / hitDist
			origTop := int((viewHeight - lineHeight) / 2)
			origBottom := int((viewHeight + lineHeight) / 2)
			drawTop := maxInt(0, origTop)
			drawBottom := minInt(bufferHeight, origBottom)
			if drawBottom <= drawTop {
				if x >= 0 && x < len(g.wallColumns) {
					g.wallColumns[x] = wallColumn{}
				}
				continue
			}
			if x >= 0 && x < len(g.wallColumns) {
				g.wallColumns[x] = wallColumn{
					hit:         true,
					dist:        hitDist,
					wallID:      wallID,
					side:        side,
					texU:        texU,
					texOverride: texOverride,
					origTop:     origTop,
					origBottom:  origBottom,
					drawTop:     drawTop,
					drawBottom:  drawBottom,
				}
			}
		}
		return
	}

	renderTop := job.renderTop
	renderBottom := job.renderBottom
	viewHeight := float64(job.renderHeight)
	for x := job.startX; x < job.endX; x++ {
		rayDirX := job.rayDirX[x]
		rayDirY := job.rayDirY[x]
		hitDist, wallID, side, texU, texOverride := g.castRay(rayDirX, rayDirY)
		if wallID == 0 {
			job.zbuffer[x] = maxRayDepth
			continue
		}
		if hitDist < 0.0001 {
			hitDist = 0.0001
		}
		job.zbuffer[x] = hitDist
		lineHeight := job.projPlaneDist / hitDist
		origTop := renderTop + int((viewHeight-lineHeight)/2)
		origBottom := renderTop + int((viewHeight+lineHeight)/2)
		drawTop := origTop
		drawBottom := origBottom
		if drawTop < renderTop {
			drawTop = renderTop
		}
		if drawBottom > renderBottom {
			drawBottom = renderBottom
		}
		if drawBottom <= drawTop {
			continue
		}
		texture := g.pickWallTexture(wallID, side, texOverride)
		if texture == nil || texture.Empty() {
			continue
		}
		if len(texture.Indices) > 0 {
			lutDim := 0
			if texture.UseDimPalette {
				lutDim = 1
			}
			drawIndexedTexturedColumnIntoTracked(job.dst, job.stride, x, drawTop, drawBottom, origTop, origBottom, texture, texU, &g.walls.RenderLUT[lutDim], job.prevWallTops, job.prevWallBottoms)
		} else {
			drawRGBATexturedColumnIntoTracked(job.dst, job.stride, x, drawTop, drawBottom, origTop, origBottom, texture, texU, job.prevWallTops, job.prevWallBottoms)
		}
	}
}

func (g *game) drawWeaponOverlay() {
	if g.playerDying {
		return
	}
	shapenum, ok := g.currentWeaponShape()
	if !ok {
		return
	}
	sprite, ok := g.sprites.SpriteByShape(shapenum)
	if !ok || len(sprite.Columns) == 0 {
		return
	}

	renderLeft, _, renderTop, _, renderWidth, renderHeight := g.gameplayRenderArea()
	geom := buildWeaponOverlayGeometry(&g.weaponOverlayScreen, renderWidth, renderHeight, renderLeft, renderTop)
	if geom.endX <= geom.startX || geom.drawBottom <= geom.drawTop {
		return
	}

	texStep := uint32(sprite.Height<<16) / uint32(geom.spriteSize)
	texXStep := uint32(sprite.Width<<16) / uint32(geom.spriteSize)
	spriteLeft := geom.spriteScreenX - geom.spriteSize/2
	texXPos := uint32(geom.startX-spriteLeft) * texXStep
	stride := g.viewWidth
	dst := g.frame32
	drawIndexed := sprite.IndexedFormat
	for x := geom.startX; x < geom.endX; x, texXPos = x+1, texXPos+texXStep {
		texX := int(texXPos >> 16)
		if texX < 0 || texX >= sprite.Width || texX >= len(sprite.Columns) {
			continue
		}
		column := sprite.Columns[texX]
		for _, post := range column.Posts {
			postDrawTop, postDrawBottom := scaledPostBounds(geom.spriteTop, geom.spriteSize, post)
			if postDrawBottom <= geom.drawTop || postDrawTop >= geom.drawBottom {
				continue
			}
			yStart := maxInt(geom.drawTop, postDrawTop)
			yEnd := minInt(geom.drawBottom, postDrawBottom)
			yStart, yEnd = clampPostDrawRange(geom.spriteTop, texStep, yStart, yEnd, post)
			if yEnd <= yStart {
				continue
			}
			texPos := uint32(yStart-geom.spriteTop) * texStep
			i := yStart*g.viewWidth + x
			if drawIndexed {
				drawIndexedSpritePostInto(dst, stride, i, texPos, texStep, yStart, yEnd, post, &g.walls.RenderLUT, 0)
			} else {
				drawRGBASpritePostInto(dst, stride, i, texPos, texStep, yStart, yEnd, post)
			}
		}
	}
}

func (g *game) drawWeaponOverlayScaled() {
	if g.playerDying {
		return
	}
	shapenum, ok := g.currentWeaponShape()
	if !ok {
		return
	}
	sprite, ok := g.sprites.SpriteByShape(shapenum)
	if !ok || len(sprite.Columns) == 0 || g.layout.bufferWidth <= 0 || g.layout.bufferHeight <= 0 {
		return
	}

	bufferWidth := g.layout.bufferWidth
	bufferHeight := g.layout.bufferHeight
	geom := buildWeaponOverlayGeometry(&g.weaponOverlayScaled, bufferWidth, bufferHeight, 0, 0)
	if geom.endX <= geom.startX || geom.drawBottom <= geom.drawTop {
		return
	}

	texStep := uint32(sprite.Height<<16) / uint32(geom.spriteSize)
	texXStep := uint32(sprite.Width<<16) / uint32(geom.spriteSize)
	spriteLeft := geom.spriteScreenX - geom.spriteSize/2
	texXPos := uint32(geom.startX-spriteLeft) * texXStep
	stride := bufferWidth
	dst := g.gameplayFrame32
	drawIndexed := sprite.IndexedFormat
	for x := geom.startX; x < geom.endX; x, texXPos = x+1, texXPos+texXStep {
		texX := int(texXPos >> 16)
		if texX < 0 || texX >= sprite.Width || texX >= len(sprite.Columns) {
			continue
		}
		column := sprite.Columns[texX]
		for _, post := range column.Posts {
			postDrawTop, postDrawBottom := scaledPostBounds(geom.spriteTop, geom.spriteSize, post)
			if postDrawBottom <= geom.drawTop || postDrawTop >= geom.drawBottom {
				continue
			}
			yStart := maxInt(geom.drawTop, postDrawTop)
			yEnd := minInt(geom.drawBottom, postDrawBottom)
			yStart, yEnd = clampPostDrawRange(geom.spriteTop, texStep, yStart, yEnd, post)
			if yEnd <= yStart {
				continue
			}
			texPos := uint32(yStart-geom.spriteTop) * texStep
			var visibleSpans [maxColumnSpanCount]columnSpan
			spanCount := g.columnCoverage[x].subtract(yStart, yEnd, &visibleSpans)
			for spanIdx := 0; spanIdx < spanCount; spanIdx++ {
				span := visibleSpans[spanIdx]
				i := span.start*bufferWidth + x
				spanTexPos := texPos + uint32(span.start-yStart)*texStep
				if drawIndexed {
					drawIndexedSpritePostInto(dst, stride, i, spanTexPos, texStep, span.start, span.end, post, &g.walls.RenderLUT, 0)
				} else {
					drawRGBASpritePostInto(dst, stride, i, spanTexPos, texStep, span.start, span.end, post)
				}
				g.columnCoverage[x].reserve(span.start, span.end)
			}
		}
	}
}

func (g *game) drawPausedOverlay(screen *ebiten.Image) {
	if g.pausedPic != nil {
		if g.pauseShade != nil {
			screen.DrawImage(g.pauseShade, nil)
		}
		gameX, gameY, scaleX, scaleY := g.gameplayOverlayTransform()

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(scaleX, scaleY)
		op.GeoM.Translate(gameX+128*scaleX, gameY+64*scaleY)
		screen.DrawImage(g.pausedPic, op)
		return
	}

	const label = "PAUSED"
	const charWidth = 7
	const charHeight = 16
	const paddingX = 14
	const paddingY = 10

	boxW := float32(len(label)*charWidth + paddingX*2)
	boxH := float32(charHeight + paddingY*2)
	boxX := float32(g.viewWidth)/2 - boxW/2
	boxY := float32(g.viewHeight)/2 - boxH/2

	if g.pauseShade != nil {
		screen.DrawImage(g.pauseShade, nil)
	}
	vector.DrawFilledRect(screen, boxX, boxY, boxW, boxH, color.RGBA{R: 24, G: 18, B: 18, A: 228}, false)
	vector.StrokeRect(screen, boxX, boxY, boxW, boxH, 2, color.RGBA{R: 185, G: 150, B: 86, A: 255}, false)
	ebitenutil.DebugPrintAt(screen, label, int(boxX)+paddingX, int(boxY)+paddingY)
}

func (g *game) wolfDisplayTransform() (offsetX, offsetY, scaleX, scaleY float64) {
	scaleX = math.Min(float64(g.viewWidth)/wolfScreenWidth, float64(g.viewHeight)/wolfDisplayHeight)
	scaleY = scaleX * float64(wolfDisplayHeight) / float64(wolfScreenHeight)
	offsetX = (float64(g.viewWidth) - wolfScreenWidth*scaleX) / 2
	offsetY = (float64(g.viewHeight) - wolfScreenHeight*scaleY) / 2
	return offsetX, offsetY, scaleX, scaleY
}

func (g *game) gameplayOverlayTransform() (offsetX, offsetY, scaleX, scaleY float64) {
	return g.layout.screenOffsetX, g.layout.screenOffsetY, g.layout.screenScaleX, g.layout.screenScaleY
}

func (g *game) gameplayRenderArea() (left, right, top, bottom, width, height int) {
	return g.layout.renderLeft, g.layout.renderRight, g.layout.renderTop, g.layout.renderBottom, g.layout.renderWidth, g.layout.renderHeight
}

func (g *game) gameplayViewport() (top, bottom, height int) {
	_, _, top, bottom, _, height = g.gameplayRenderArea()
	return top, bottom, height
}

func (g *game) statusBarTransform() (offsetX, offsetY, scaleX, scaleY float64) {
	return g.layout.statusX, g.layout.statusY, g.layout.statusScaleX, g.layout.statusScaleY
}

func (g *game) drawStatusBar(screen *ebiten.Image) {
	if !g.ensureStatusBar() || g.statusBarPic == nil {
		return
	}
	x, y, scaleX, scaleY := g.statusBarTransform()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, scaleY)
	op.GeoM.Translate(x, y)
	screen.DrawImage(g.statusBarPic, op)
}

func (g *game) ensureStatusBar() bool {
	next := g.currentStatusBarSnapshot()
	if g.statusBarPic != nil && next == g.statusBarState {
		return true
	}
	if g.statusBarPic == nil {
		g.statusBarPic = ebiten.NewImage(wolfScreenWidth, wolfStatusLines)
	}
	if !g.rebuildStatusBar(next) {
		return false
	}
	g.statusBarState = next
	return true
}

func (g *game) currentStatusBarSnapshot() statusBarSnapshot {
	return statusBarSnapshot{
		level:  g.mapIndex + 1,
		score:  maxInt(0, g.score),
		lives:  maxInt(0, g.lives),
		health: maxInt(0, minInt(100, g.health)),
		ammo:   maxInt(0, minInt(99, g.ammo)),
		weapon: maxInt(0, minInt(3, g.weapon)),
		keys:   g.keys,
		face:   g.statusFaceChunk(),
	}
}

func (g *game) rebuildStatusBar(state statusBarSnapshot) bool {
	base, ok := g.pictureImage(g.files.Variant.StatusBarPicChunk)
	if !ok {
		return false
	}
	g.statusBarPic.Clear()
	g.statusBarPic.DrawImage(base, nil)
	g.drawStatusNumber(6, 16, 6, state.score)
	g.drawStatusNumber(14, 16, 1, state.lives)
	g.drawStatusNumber(2, 16, 2, state.level)
	g.drawStatusNumber(21, 16, 3, state.health)
	g.drawStatusNumber(27, 16, 2, state.ammo)
	g.drawStatusChunk(17, 4, state.face)
	g.drawStatusChunk(32, 8, g.files.Variant.KnifePicChunk+state.weapon)
	if state.keys&1 != 0 {
		g.drawStatusChunk(30, 4, g.files.Variant.GoldKeyPicChunk)
	} else {
		g.drawStatusChunk(30, 4, g.files.Variant.NoKeyPicChunk)
	}
	if state.keys&2 != 0 {
		g.drawStatusChunk(30, 20, g.files.Variant.SilverKeyPicChunk)
	} else {
		g.drawStatusChunk(30, 20, g.files.Variant.NoKeyPicChunk)
	}
	return true
}

func (g *game) drawStatusNumber(x, y, width, number int) {
	if width <= 0 {
		return
	}
	if number < 0 {
		number = 0
	}
	str := fmt.Sprintf("%d", number)
	if len(str) > width {
		str = str[len(str)-width:]
	}
	for len(str) < width {
		g.drawStatusChunk(x, y, g.files.Variant.NumberBlankChunk)
		x++
		width--
	}
	for _, ch := range str {
		if ch < '0' || ch > '9' {
			g.drawStatusChunk(x, y, g.files.Variant.NumberBlankChunk)
		} else {
			g.drawStatusChunk(x, y, g.files.Variant.NumberZeroChunk+int(ch-'0'))
		}
		x++
	}
}

func (g *game) drawStatusChunk(x, y, chunk int) {
	img, ok := g.pictureImage(chunk)
	if !ok || g.statusBarPic == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x*8), float64(y))
	g.statusBarPic.DrawImage(img, op)
}

func (g *game) pictureImage(chunk int) (*ebiten.Image, bool) {
	if chunk <= 0 || g.files == nil || g.walls == nil {
		return nil, false
	}
	if g.pictureCache == nil {
		g.pictureCache = make(map[int]*ebiten.Image)
	}
	if img, ok := g.pictureCache[chunk]; ok {
		return img, true
	}
	if img, ok := g.loadHDPictureImage(chunk); ok {
		g.pictureCache[chunk] = img
		return img, true
	}
	pic, err := g.files.LoadPicture(chunk)
	if err != nil {
		return nil, false
	}
	rgba := pic.RGBA(g.walls.Palette)
	if g.shouldColorKeyPictureChunk(chunk) {
		applyTopLeftColorKey(rgba)
	}
	img := ebiten.NewImageFromImage(rgba)
	g.pictureCache[chunk] = img
	return img, true
}

func (g *game) shouldColorKeyPictureChunk(chunk int) bool {
	if g == nil || g.files == nil {
		return false
	}
	return chunk == g.files.Variant.MouseBackPicChunk ||
		chunk == g.files.Variant.Cursor1PicChunk ||
		chunk == g.files.Variant.Cursor2PicChunk
}

func (g *game) refreshRuntimeImageAssets() error {
	if g == nil || g.files == nil {
		return nil
	}

	walls, err := g.files.LoadWallSet()
	if err != nil {
		return err
	}
	g.walls = walls
	if g.shouldUseHDAssets() {
		g.applyHDWallOverrides()
	}

	sprites, err := g.files.LoadSpriteSet()
	if err != nil {
		g.sprites = nil
	} else {
		g.sprites = sprites
		if g.shouldUseHDAssets() {
			g.applyHDSpriteOverrides()
		}
	}

	g.pictureCache = nil
	g.pictureSizeCache = nil
	g.textureCache = nil
	g.spriteImageCache = nil
	g.statusBarPic = nil
	g.titlePic = nil
	g.getPsychedPic = nil
	g.pausedPic = nil

	if img, ok := g.pictureImage(g.files.Variant.TitlePicChunk); ok {
		g.titlePic = img
	}
	if img, ok := g.pictureImage(g.files.Variant.GetPsychedPicChunk); ok {
		g.getPsychedPic = img
	}
	if img, ok := g.pictureImage(g.files.Variant.PausedPicChunk); ok {
		g.pausedPic = img
	}
	g.rebuildSpriteImageCache()

	return nil
}

func (g *game) statusFaceChunk() int {
	if g.health <= 0 {
		return g.files.Variant.Face8APicChunk
	}
	band := (100 - minInt(100, maxInt(0, g.health))) / 16
	if band < 0 {
		band = 0
	}
	if band > 6 {
		band = 6
	}
	return g.files.Variant.Face1APicChunk + band*3
}

func (g *game) castRay(dirX, dirY float64) (dist float64, wallID uint16, side int, texU float64, texOverride int) {
	posX := g.playerX
	posY := g.playerY
	texOverride = -1

	mapX := int(math.Floor(posX))
	mapY := int(math.Floor(posY))

	deltaDistX := huge
	invDirX := 0.0
	if dirX != 0 {
		invDirX = 1 / dirX
		deltaDistX = math.Abs(invDirX)
	}
	deltaDistY := huge
	invDirY := 0.0
	if dirY != 0 {
		invDirY = 1 / dirY
		deltaDistY = math.Abs(invDirY)
	}

	var (
		stepX, stepY         int
		sideDistX, sideDistY float64
	)

	if dirX < 0 {
		stepX = -1
		sideDistX = (posX - float64(mapX)) * deltaDistX
	} else {
		stepX = 1
		sideDistX = (float64(mapX+1) - posX) * deltaDistX
	}
	if dirY < 0 {
		stepY = -1
		sideDistY = (posY - float64(mapY)) * deltaDistY
	} else {
		stepY = 1
		sideDistY = (float64(mapY+1) - posY) * deltaDistY
	}

	for {
		if sideDistX < sideDistY {
			mapX += stepX
			sideDistX += deltaDistX
			side = 1
		} else {
			mapY += stepY
			sideDistY += deltaDistY
			side = 2
		}

		if mapX < 0 || mapY < 0 || mapX >= g.levelWidth || mapY >= g.levelHeight {
			return maxRayDepth, 0, side, 0, -1
		}

		tileIndex := mapY*g.levelWidth + mapX
		if g.tileIntersectsPushWall(mapX, mapY) {
			if hit, pushDist, pushSide, pushTexU := g.castPushWall(mapX, mapY, dirX, dirY); hit {
				return pushDist, renderWallTile(g.pushWall.wall), pushSide, pushTexU, -1
			}
		}
		if doorFlags := g.cachedDoorFlags[tileIndex]; doorFlags != 0 {
			if (doorFlags&2 != 0 && dirX != 0) || (doorFlags&2 == 0 && dirY != 0) {
				if hit, doorDist, doorSide, doorTexU := g.castDoor(tileIndex, mapX, mapY, dirX, dirY, doorFlags); hit {
					return doorDist, g.cachedWallIDs[tileIndex], doorSide, doorTexU, -1
				}
			}
			continue
		}
		wallID = g.cachedWallIDs[tileIndex]
		if wallID != 0 {
			if !g.renderableWalls[tileIndex] {
				continue
			}
			if side == 1 {
				dist = (float64(mapX) - posX + float64(1-stepX)/2) * invDirX
			} else {
				dist = (float64(mapY) - posY + float64(1-stepY)/2) * invDirY
			}
			hitX := posX + dist*dirX
			hitY := posY + dist*dirY
			var wallFrac float64
			if side == 1 {
				wallFrac = hitY - float64(int(hitY))
				if dirX < 0 {
					wallFrac = 1 - wallFrac
				}
			} else {
				wallFrac = hitX - float64(int(hitX))
				if dirY > 0 {
					wallFrac = 1 - wallFrac
				}
			}
			texU = clampTextureU(wallFrac)
			texOverride = g.wallDoorSideTexture(tileIndex, side, stepX, stepY)
			return dist, wallID, side, texU, texOverride
		}
	}
}

func (g *game) castPushWall(mapX, mapY int, dirX, dirY float64) (hit bool, dist float64, side int, texU float64) {
	if !g.tileIntersectsPushWall(mapX, mapY) {
		return false, 0, 0, 0
	}
	minX, maxX, minY, maxY, ok := g.pushWallBounds()
	if !ok {
		return false, 0, 0, 0
	}

	txMin := math.Inf(-1)
	txMax := math.Inf(1)
	if dirX == 0 {
		if g.playerX <= minX || g.playerX >= maxX {
			return false, 0, 0, 0
		}
	} else {
		tx1 := (minX - g.playerX) / dirX
		tx2 := (maxX - g.playerX) / dirX
		txMin = math.Min(tx1, tx2)
		txMax = math.Max(tx1, tx2)
	}

	tyMin := math.Inf(-1)
	tyMax := math.Inf(1)
	if dirY == 0 {
		if g.playerY <= minY || g.playerY >= maxY {
			return false, 0, 0, 0
		}
	} else {
		ty1 := (minY - g.playerY) / dirY
		ty2 := (maxY - g.playerY) / dirY
		tyMin = math.Min(ty1, ty2)
		tyMax = math.Max(ty1, ty2)
	}

	enter := math.Max(txMin, tyMin)
	exit := math.Min(txMax, tyMax)
	if exit <= 0 || enter > exit || enter <= 0 {
		return false, 0, 0, 0
	}
	dist = enter

	hitX := g.playerX + dist*dirX
	hitY := g.playerY + dist*dirY
	if txMin > tyMin {
		side = 1
		frac := hitY - minY
		if dirX < 0 {
			frac = 1 - frac
		}
		texU = clampTextureU(frac)
	} else {
		side = 2
		frac := hitX - minX
		if dirY > 0 {
			frac = 1 - frac
		}
		texU = clampTextureU(frac)
	}

	return true, dist, side, texU
}

func (g *game) castDoor(tileIndex, mapX, mapY int, dirX, dirY float64, doorFlags byte) (hit bool, dist float64, side int, texU float64) {
	open := g.doorOpen[tileIndex]
	if open >= 0.999 {
		return false, 0, 0, 0
	}

	if doorFlags == 0 {
		return false, 0, 0, 0
	}

	if doorFlags&2 != 0 {
		planeX := float64(mapX) + 0.5
		dist = (planeX - g.playerX) / dirX
		if dist <= 0 {
			return false, 0, 0, 0
		}
		hitY := g.playerY + dist*dirY
		if hitY < float64(mapY) || hitY >= float64(mapY+1) {
			return false, 0, 0, 0
		}
		frac := hitY - float64(mapY)
		if frac < open {
			return false, 0, 0, 0
		}
		side = 1
		texU = clampTextureU(frac - open)
	} else {
		planeY := float64(mapY) + 0.5
		dist = (planeY - g.playerY) / dirY
		if dist <= 0 {
			return false, 0, 0, 0
		}
		hitX := g.playerX + dist*dirX
		if hitX < float64(mapX) || hitX >= float64(mapX+1) {
			return false, 0, 0, 0
		}
		frac := hitX - float64(mapX)
		if frac < open {
			return false, 0, 0, 0
		}
		side = 2
		texU = clampTextureU(frac - open)
	}

	return true, dist, side, texU
}

func renderWallTile(tile wl6.Tile) uint16 {
	if !tile.RenderWall {
		return 0
	}
	return tile.RawWall
}

func (g *game) clearColumnRange(x, startY, endY int) {
}

func (g *game) resetLoadout() {
	g.weapon = 1
	g.bestWeapon = 1
	g.chosenWeapon = 1
	g.ammo = startAmmo
	g.attacking = false
	g.weaponSequence = ""
	g.weaponFrameIdx = 0
	g.weaponFrameTics = 0
}

func (g *game) startNewGame() {
	g.resetLoadout()
	g.health = 100
	g.lives = 3
	g.keys = 0
	g.score = 0
	g.secretCount = 0
	g.treasureCount = 0
	g.hudNotice = ""
	g.hudNoticeTimer = 0
	g.madeNoise = false
	g.playerMovingFast = false
	g.playerDying = false
	g.victoryActive = false
	g.victoryPhase = victoryPhaseNone
	g.victoryBJ = staticSprite{}
	g.victoryRunDistance = 0
	g.deathPhase = deathPhaseNone
	g.deathTimer = 0
	g.gameplayTickAccum = 0
	g.autosaveTickAccum = 0
	g.deathHasKiller = false
	g.deathFizzleOrder = nil
	g.deathFizzlePixels = nil
	g.deathFizzleFilled = 0
	g.deathFizzleImage = nil
}

func (g *game) resetLevelState() {
	g.keys = 0
	g.secretCount = 0
	g.treasureCount = 0
	g.hudNotice = ""
	g.hudNoticeTimer = 0
	g.madeNoise = false
	g.playerMovingFast = false
	g.playerDying = false
	g.victoryActive = false
	g.victoryPhase = victoryPhaseNone
	g.victoryBJ = staticSprite{}
	g.victoryRunDistance = 0
	g.deathPhase = deathPhaseNone
	g.deathTimer = 0
	g.deathHasKiller = false
	g.deathFizzleOrder = nil
	g.deathFizzlePixels = nil
	g.deathFizzleFilled = 0
	g.deathFizzleImage = nil
}

func (g *game) reloadCurrentLevelAfterDeath() error {
	if g.files != nil {
		if err := g.setMap(g.mapIndex); err != nil {
			return err
		}
	} else {
		g.resetLevelState()
		g.resetPlayer()
	}
	g.resetLoadout()
	g.health = 100
	g.keys = 0
	return nil
}

func (g *game) handleGameOver() {
	g.startNewGame()
	g.paused = false
	g.uiState = uiStateMainMenu
	g.menuReturn = uiStateMainMenu
	g.menuIndex = 0
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.mousePrimed = false
}

func normalizeAngle(angle float64) float64 {
	for angle < 0 {
		angle += 2 * math.Pi
	}
	for angle >= 2*math.Pi {
		angle -= 2 * math.Pi
	}
	return angle
}

func shortestAngleDelta(current, target float64) float64 {
	delta := normalizeAngle(target) - normalizeAngle(current)
	if delta > math.Pi {
		delta -= 2 * math.Pi
	}
	if delta < -math.Pi {
		delta += 2 * math.Pi
	}
	return delta
}

func (g *game) startDeathFizzle() {
	g.deathPhase = deathPhaseFizzle
	g.deathTimer = deathFadeTics
	g.damageFlash = 0
	g.bonusFlash = 0
	g.deathFizzleFilled = 0
	if g.deathFizzleImage != nil && len(g.deathFizzlePixels) > 0 {
		clear(g.deathFizzlePixels)
		g.deathFizzleImage.WritePixels(g.deathFizzlePixels)
	}
}

func (g *game) startDeathHold() {
	g.deathPhase = deathPhaseHold
	g.deathTimer = deathHoldTics
}

func (g *game) startDeathWaitSound() {
	g.deathPhase = deathPhaseWaitSound
	g.deathTimer = 0
}

func (g *game) beginPlayerDeath(killerX, killerY float64, hasKiller bool) {
	g.health = 0
	g.playerDying = true
	g.deathPhase = deathPhaseRotate
	g.deathTimer = 0
	g.deathKillerX = killerX
	g.deathKillerY = killerY
	g.deathHasKiller = hasKiller
	g.deathFizzleOrder = nil
	g.deathFizzlePixels = nil
	g.deathFizzleFilled = 0
	g.deathFizzleImage = nil
	g.hudNotice = ""
	g.hudNoticeTimer = 0
	g.attacking = false
	g.weaponSequence = ""
	g.weaponFrameIdx = 0
	g.weaponFrameTics = 0
	g.mode = modeRaycast
	g.paused = false
	g.mousePrimed = false
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.playSound(soundPlayerDeath)
	if !hasKiller || math.Hypot(killerX-g.playerX, killerY-g.playerY) < 0.001 {
		g.startDeathFizzle()
	}
}

func (g *game) finishPlayerDeath() {
	g.playerDying = false
	g.deathPhase = deathPhaseNone
	g.deathTimer = 0
	g.deathHasKiller = false
	g.deathFizzleOrder = nil
	g.deathFizzlePixels = nil
	g.deathFizzleFilled = 0
	g.deathFizzleImage = nil
	g.lives--
	if g.lives < 0 {
		g.handleGameOver()
		return
	}
	if err := g.reloadCurrentLevelAfterDeath(); err != nil {
		g.resetLoadout()
		g.health = 100
		g.keys = 0
		g.resetPlayer()
	}
	if g.uiState == uiStatePlaying && g.mode == modeRaycast {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	}
}

func (g *game) isSoundPlaying(id soundID) bool {
	for _, voice := range g.soundBanks[id] {
		if voice.player.IsPlaying() {
			return true
		}
	}
	return false
}

func (g *game) advancePlayerDeathTics(tics int) error {
	for tics > 0 {
		switch g.deathPhase {
		case deathPhaseRotate:
			if !g.deathHasKiller {
				g.startDeathFizzle()
				continue
			}
			target := math.Atan2(g.deathKillerY-g.playerY, g.deathKillerX-g.playerX)
			delta := shortestAngleDelta(g.playerA, target)
			step := float64(tics*deathRotateDegrees) * math.Pi / 180
			if math.Abs(delta) <= step {
				g.playerA = normalizeAngle(target)
				g.startDeathFizzle()
				tics = 0
				continue
			}
			if delta < 0 {
				g.playerA = normalizeAngle(g.playerA - step)
			} else {
				g.playerA = normalizeAngle(g.playerA + step)
			}
			tics = 0
		case deathPhaseFizzle:
			consume := minInt(tics, g.deathTimer)
			g.deathTimer -= consume
			tics -= consume
			if g.deathTimer == 0 {
				g.startDeathHold()
			}
		case deathPhaseHold:
			consume := minInt(tics, g.deathTimer)
			g.deathTimer -= consume
			tics -= consume
			if g.deathTimer == 0 {
				g.startDeathWaitSound()
			}
		case deathPhaseWaitSound:
			if g.isSoundPlaying(soundPlayerDeath) {
				tics = 0
				break
			}
			g.finishPlayerDeath()
			tics = 0
		default:
			g.finishPlayerDeath()
			tics = 0
		}
	}
	g.rebuildHUDText()
	return nil
}

func (g *game) checkVictoryTile() bool {
	if g.victoryActive || g.level == nil {
		return false
	}
	tileX := int(math.Floor(g.playerX))
	tileY := int(math.Floor(g.playerY))
	if tileX < 0 || tileY < 0 || tileX >= g.levelWidth || tileY >= g.levelHeight {
		return false
	}
	if g.level.Tile(tileX, tileY).RawInfo != wolfExitTile {
		return false
	}
	g.startVictorySequence()
	return true
}

func (g *game) startVictorySequence() {
	g.victoryActive = true
	g.victoryPhase = victoryPhaseRun
	g.victoryRunDistance = 6
	g.attacking = false
	g.weaponSequence = ""
	g.weaponFrameIdx = 0
	g.weaponFrameTics = 0
	g.victoryBJ = staticSprite{
		x:        g.playerX,
		y:        g.playerY,
		shapenum: activeVictoryBJWalk1,
		alive:    true,
	}
	g.startSpriteSequence(&g.victoryBJ, seqVictoryBJRun)
}

func (g *game) advanceVictoryPlayer(tics int) {
	target := 3 * math.Pi / 2
	delta := shortestAngleDelta(g.playerA, target)
	step := float64(tics*3) * math.Pi / 180
	if math.Abs(delta) <= step {
		g.playerA = normalizeAngle(target)
	} else if delta < 0 {
		g.playerA = normalizeAngle(g.playerA - step)
	} else {
		g.playerA = normalizeAngle(g.playerA + step)
	}

	destY := float64(int(math.Floor(g.playerY))-5) + 0.8125
	move := 0.0625 * float64(tics)
	if g.playerY > destY {
		g.playerY -= move
		if g.playerY < destY {
			g.playerY = destY
		}
	}
}

func (g *game) finishVictorySequence() error {
	g.victoryActive = false
	g.victoryPhase = victoryPhaseNone
	g.victoryRunDistance = 0
	g.victoryBJ = staticSprite{}
	return g.fadeToUIState(uiStateVictoryIntermission, func() {
		g.paused = false
		g.menuReturn = uiStateMainMenu
		g.menuIndex = 0
		g.mode = modeRaycast
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		g.mousePrimed = false
	})
}

func (g *game) updateVictorySequence(tics int) error {
	g.advanceVictoryPlayer(tics)
	switch g.victoryPhase {
	case victoryPhaseRun:
		move := bjRunSpeedPerTic * float64(tics)
		if move > g.victoryRunDistance {
			move = g.victoryRunDistance
		}
		g.victoryBJ.y -= move
		g.victoryRunDistance -= move
		g.advanceStaticSpriteSequence(&g.victoryBJ, tics)
		if g.victoryRunDistance <= 0 {
			g.victoryPhase = victoryPhaseJump
			g.startSpriteSequence(&g.victoryBJ, seqVictoryBJJump)
		}
	case victoryPhaseJump:
		if g.victoryBJ.frameIndex < 3 {
			g.victoryBJ.y -= bjJumpSpeedPerTic * float64(tics)
		}
		g.advanceStaticSpriteSequence(&g.victoryBJ, tics)
		if g.spriteSequenceDone(g.victoryBJ) {
			return g.finishVictorySequence()
		}
	}
	g.rebuildHUDText()
	return nil
}

func (g *game) updatePlayerDeath(tics int) error {
	return g.advancePlayerDeathTics(tics)
}

func (g *game) giveAmmo(ammo int) {
	if ammo <= 0 {
		return
	}
	if g.ammo <= 0 && !g.attacking {
		g.weapon = g.chosenWeapon
	}
	g.ammo = minInt(99, g.ammo+ammo)
}

func (g *game) giveWeapon(weapon int) {
	g.giveAmmo(6)
	if g.bestWeapon < weapon {
		g.bestWeapon = weapon
		g.weapon = weapon
		g.chosenWeapon = weapon
	}
}

func pickupLabel(pickup pickupType) string {
	switch pickup {
	case pickupFood:
		return "Food"
	case pickupFirstAid:
		return "First Aid"
	case pickupClip:
		return "Ammo Clip"
	case pickupMachineGun:
		return "Machine Gun"
	case pickupChaingun:
		return "Chaingun"
	case pickupCross:
		return "Cross"
	case pickupChalice:
		return "Chalice"
	case pickupBible:
		return "Bible"
	case pickupCrown:
		return "Crown"
	case pickupFullHeal:
		return "1-Up"
	case pickupGibs:
		return "Gibs"
	case pickupClip2:
		return "Dropped Clip"
	case pickupAlpo:
		return "Dog Food"
	case pickupKey1:
		return "Gold Key"
	case pickupKey2:
		return "Silver Key"
	case pickupKey3:
		return "Key 3"
	case pickupKey4:
		return "Key 4"
	default:
		return ""
	}
}

func pickupTreasureValue(pickup pickupType) (score int, counts bool) {
	switch pickup {
	case pickupCross:
		return 100, true
	case pickupChalice:
		return 500, true
	case pickupBible:
		return 1000, true
	case pickupCrown:
		return 5000, true
	case pickupFullHeal:
		return 0, true
	default:
		return 0, false
	}
}

func canOpenDoorLock(lock int, keys byte) bool {
	if lock < 1 || lock > 4 {
		return true
	}
	return keys&(1<<uint(lock-1)) != 0
}

func (g *game) buildStaticSprites() ([]staticSprite, int) {
	if g.level == nil {
		return nil, 0
	}

	sprites := make([]staticSprite, 0, 96)
	treasureTotal := 0
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			info := g.level.Tile(x, y).RawInfo
			if def, ok := LookupStatic(info); ok {
				if _, counts := pickupTreasureValue(def.Pickup); counts {
					treasureTotal++
				}
				sprites = append(sprites, staticSprite{
					x:        float64(x) + 0.5,
					y:        float64(y) + 0.5,
					shapenum: def.Shape,
					blocking: def.Blocking,
					alive:    true,
					pickup:   def.Pickup,
				})
				continue
			}
			def, ok := LookupActorSpawn(info, g.difficulty)
			if !ok {
				continue
			}
			if isActiveGameplayActorKind(def.Kind) {
				continue
			}
			dropPickup, scoreValue := def.ResolveDrop(g.bestWeapon)
			sprites = append(sprites, staticSprite{
				x:             float64(x) + 0.5,
				y:             float64(y) + 0.5,
				shapenum:      def.SpawnShape(info),
				rotate:        def.Rotate,
				facingDir:     def.FacingDir(info),
				blocking:      def.Blocking,
				shootable:     def.Shootable,
				alive:         true,
				dropPickup:    dropPickup,
				scoreValue:    scoreValue,
				deathSequence: def.DeathSequence,
			})
		}
	}
	return sprites, treasureTotal
}

func soundForWeapon(weapon int) soundID {
	switch weapon {
	case 0:
		return soundKnife
	case 1:
		return soundPistol
	case 2:
		return soundMachineGun
	case 3:
		return soundChainGun
	default:
		return soundPistol
	}
}

func (g *game) updateWeaponSelection() {
	for _, entry := range []struct {
		key    ebiten.Key
		weapon int
	}{
		{ebiten.Key1, 0},
		{ebiten.Key2, 1},
		{ebiten.Key3, 2},
		{ebiten.Key4, 3},
	} {
		if !inpututil.IsKeyJustPressed(entry.key) {
			continue
		}
		if entry.weapon == 0 || entry.weapon <= g.bestWeapon {
			g.chosenWeapon = entry.weapon
			if !g.attacking && (entry.weapon == 0 || g.ammo > 0) {
				g.weapon = entry.weapon
			}
		}
	}
}

func (g *game) updateWeaponAttack(tics int) {
	attackPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControlRight)

	if !g.attacking {
		if !attackPressed {
			return
		}
		if g.weapon > 0 && g.ammo <= 0 {
			g.weapon = 0
			g.chosenWeapon = 0
		}
		if !g.startWeaponSequence() {
			return
		}
	}

	if tics <= 0 {
		return
	}
	g.weaponFrameTics -= tics
	for g.attacking && g.weaponFrameTics <= 0 {
		if !g.advanceWeaponSequence(attackPressed) {
			return
		}
	}
}

func (g *game) currentWeaponShape() (int, bool) {
	if g.sprites == nil {
		return 0, false
	}
	def, ok := WeaponAnim(g.weapon)
	if !ok {
		return 0, false
	}
	if !g.attacking {
		return def.ReadyShape, true
	}
	seq, ok := LookupAnimSequence(g.weaponSequence)
	if !ok || g.weaponFrameIdx < 0 || g.weaponFrameIdx >= len(seq.Frames) {
		return def.ReadyShape, true
	}
	return seq.Frames[g.weaponFrameIdx].Shape, true
}

func (g *game) startWeaponSequence() bool {
	def, ok := WeaponAnim(g.weapon)
	if !ok {
		return false
	}
	seq, ok := LookupAnimSequence(def.AttackSequence)
	if !ok || len(seq.Frames) == 0 {
		return false
	}
	g.attacking = true
	g.weaponSequence = def.AttackSequence
	g.weaponFrameIdx = 0
	g.weaponFrameTics = seq.Frames[0].Tics
	return true
}

func (g *game) advanceWeaponSequence(attackPressed bool) bool {
	seq, ok := LookupAnimSequence(g.weaponSequence)
	if !ok || len(seq.Frames) == 0 {
		g.finishWeaponSequence()
		return false
	}
	if g.weaponFrameIdx < 0 || g.weaponFrameIdx >= len(seq.Frames) {
		g.finishWeaponSequence()
		return false
	}

	cur := seq.Frames[g.weaponFrameIdx]
	switch cur.Action {
	case animActionFireGun:
		if g.ammo > 0 {
			g.playSound(soundForWeapon(g.weapon))
			g.shootAhead()
			g.ammo--
		}
	case animActionFireKnife:
		g.playSound(soundForWeapon(g.weapon))
		g.shootAhead()
	case animActionLoopIfPressed:
		if g.ammo > 0 && attackPressed {
			g.weaponFrameIdx -= 2
		}
	case animActionLoopFireIfPressed:
		if g.ammo > 0 && attackPressed {
			g.weaponFrameIdx -= 2
		}
		if g.ammo > 0 {
			g.playSound(soundForWeapon(g.weapon))
			g.shootAhead()
			g.ammo--
		}
	case animActionEnd:
		g.finishWeaponSequence()
		return false
	}

	g.weaponFrameIdx++
	if g.weaponFrameIdx >= len(seq.Frames) {
		g.weaponFrameIdx = len(seq.Frames) - 1
	}
	g.weaponFrameTics += seq.Frames[g.weaponFrameIdx].Tics
	return true
}

func (g *game) finishWeaponSequence() {
	g.attacking = false
	g.weaponSequence = ""
	g.weaponFrameIdx = 0
	g.weaponFrameTics = 0
	if g.ammo <= 0 {
		g.weapon = 0
		g.chosenWeapon = 0
	} else if g.weapon != g.chosenWeapon {
		g.weapon = g.chosenWeapon
	}
}

func (g *game) startSpriteSequence(spr *staticSprite, id AnimSequenceID) bool {
	seq, ok := LookupAnimSequence(id)
	if !ok || len(seq.Frames) == 0 {
		return false
	}
	spr.sequenceID = id
	spr.frameIndex = 0
	spr.frameTimer = 0
	spr.shapenum = seq.Frames[0].Shape
	return true
}

func (g *game) advanceStaticSpriteSequence(spr *staticSprite, tics int) {
	if spr == nil || spr.sequenceID == "" || tics <= 0 {
		return
	}
	seq, ok := LookupAnimSequence(spr.sequenceID)
	if !ok || len(seq.Frames) == 0 {
		return
	}
	cur := seq.Frames[spr.frameIndex]
	frameTics := cur.Tics
	if frameTics <= 0 {
		return
	}

	spr.frameTimer += tics
	for spr.frameTimer >= frameTics {
		spr.frameTimer -= frameTics
		spr.frameIndex++
		if spr.frameIndex >= len(seq.Frames) {
			if !seq.Loop {
				spr.frameIndex = len(seq.Frames) - 1
				spr.frameTimer = 0
				return
			}
			spr.frameIndex = 0
		}
		cur = seq.Frames[spr.frameIndex]
		spr.shapenum = cur.Shape
		frameTics = cur.Tics
		if frameTics <= 0 {
			return
		}
	}
}

func (g *game) spriteSequenceDone(spr staticSprite) bool {
	seq, ok := LookupAnimSequence(spr.sequenceID)
	if !ok || len(seq.Frames) == 0 {
		return true
	}
	return !seq.Loop && spr.frameIndex >= len(seq.Frames)-1 && (seq.Frames[spr.frameIndex].Tics <= 0 || spr.frameTimer == 0)
}

func mapInfoFacing(v uint16) int {
	// WOLFSRC actor spawn dirs are passed as 0..3 and then multiplied by 2
	// into dirtype, yielding east,north,west,south.
	switch v {
	case 0: // EAST
		return 0
	case 1: // NORTH
		return 2
	case 2: // WEST
		return 4
	case 3: // SOUTH
		return 6
	default:
		return 0
	}
}

func (g *game) ceilingColorIndex() byte {
	if g.mapIndex < 0 || g.mapIndex >= len(vgaCeiling) {
		return 0x1d
	}
	return vgaCeiling[g.mapIndex]
}

func (g *game) paletteColor(index byte) color.RGBA {
	if g.walls == nil {
		return color.RGBA{A: 255}
	}
	rgba := g.walls.Palette[index]
	return color.RGBA{
		R: byte(rgba >> 24),
		G: byte(rgba >> 16),
		B: byte(rgba >> 8),
		A: 255,
	}
}

func (g *game) drawTexturedColumnScreen(screenX, drawTop, drawBottom, origTop, origBottom int, texture *wl6.WallTexture, texU float64) {
	if screenX < 0 || screenX >= g.viewWidth {
		return
	}
	if len(texture.Indices) > 0 {
		lutDim := 0
		if texture.UseDimPalette {
			lutDim = 1
		}
		drawIndexedTexturedColumnIntoTracked(g.frame32, g.viewWidth, screenX, drawTop, drawBottom, origTop, origBottom, texture, texU, &g.walls.RenderLUT[lutDim], g.prevWallTops, g.prevWallBottoms)
		return
	}
	drawRGBATexturedColumnIntoTracked(g.frame32, g.viewWidth, screenX, drawTop, drawBottom, origTop, origBottom, texture, texU, g.prevWallTops, g.prevWallBottoms)
}

func drawIndexedTexturedColumnInto(dst []uint32, stride, screenX, drawTop, drawBottom, texXBase int, texPos, texStep uint32, indexed []byte, renderLUT *[256]uint32) {
	i := drawTop*stride + screenX
	for rows := drawBottom - drawTop; rows > 0; rows-- {
		texY := int(texPos >> 16)
		dst[i] = renderLUT[indexed[texXBase+texY]]
		texPos += texStep
		i += stride
	}
}

func drawRGBATexturedColumnInto(dst []uint32, stride, screenX, drawTop, drawBottom, texXBase int, texPos, texStep uint32, pixels []uint32) {
	i := drawTop*stride + screenX
	for rows := drawBottom - drawTop; rows > 0; rows-- {
		texY := int(texPos >> 16)
		dst[i] = pixels[texXBase+texY]
		texPos += texStep
		i += stride
	}
}

func drawIndexedTexturedColumnIntoTracked(dst []uint32, stride, screenX, drawTop, drawBottom, origTop, origBottom int, texture *wl6.WallTexture, texU float64, renderLUT *[256]uint32, prevWallTops, prevWallBottoms []int) {
	fullHeight := origBottom - origTop
	if fullHeight <= 0 {
		return
	}

	textureWidth, textureHeight := wallTextureDimensions(texture)
	if textureWidth <= 0 || textureHeight <= 0 {
		return
	}
	texX := textureXFromU(texU, textureWidth)
	texXBase := texX * textureHeight
	texStep := uint32(textureHeight<<16) / uint32(fullHeight)
	texPos := uint32(drawTop-origTop) * texStep
	drawIndexedTexturedColumnInto(dst, stride, screenX, drawTop, drawBottom, texXBase, texPos, texStep, texture.Indices, renderLUT)
	if screenX >= 0 && screenX < len(prevWallTops) {
		prevWallTops[screenX] = drawTop
	}
	if screenX >= 0 && screenX < len(prevWallBottoms) {
		prevWallBottoms[screenX] = drawBottom
	}
}

func drawRGBATexturedColumnIntoTracked(dst []uint32, stride, screenX, drawTop, drawBottom, origTop, origBottom int, texture *wl6.WallTexture, texU float64, prevWallTops, prevWallBottoms []int) {
	fullHeight := origBottom - origTop
	if fullHeight <= 0 {
		return
	}
	textureWidth, textureHeight := wallTextureDimensions(texture)
	if textureWidth <= 0 || textureHeight <= 0 {
		return
	}
	texX := textureXFromU(texU, textureWidth)
	texXBase := texX * textureHeight
	texStep := uint32(textureHeight<<16) / uint32(fullHeight)
	texPos := uint32(drawTop-origTop) * texStep
	drawRGBATexturedColumnInto(dst, stride, screenX, drawTop, drawBottom, texXBase, texPos, texStep, texture.Pixels)
	if screenX >= 0 && screenX < len(prevWallTops) {
		prevWallTops[screenX] = drawTop
	}
	if screenX >= 0 && screenX < len(prevWallBottoms) {
		prevWallBottoms[screenX] = drawBottom
	}
}

func (g *game) drawTexturedColumnGameplay(screenX, drawTop, drawBottom, origTop, origBottom int, texture *wl6.WallTexture, texU float64) {
	if screenX < 0 || screenX >= g.layout.bufferWidth {
		return
	}
	if len(texture.Indices) > 0 {
		lutDim := 0
		if texture.UseDimPalette {
			lutDim = 1
		}
		drawIndexedTexturedColumnIntoGameplay(g.gameplayFrame32, g.layout.bufferWidth, screenX, drawTop, drawBottom, origTop, origBottom, texture, texU, &g.walls.RenderLUT[lutDim])
		return
	}
	drawRGBATexturedColumnIntoGameplay(g.gameplayFrame32, g.layout.bufferWidth, screenX, drawTop, drawBottom, origTop, origBottom, texture, texU)
}

func drawIndexedTexturedColumnIntoGameplay(dst []uint32, stride, screenX, drawTop, drawBottom, origTop, origBottom int, texture *wl6.WallTexture, texU float64, renderLUT *[256]uint32) {
	fullHeight := origBottom - origTop
	if fullHeight <= 0 {
		return
	}

	textureWidth, textureHeight := wallTextureDimensions(texture)
	if textureWidth <= 0 || textureHeight <= 0 {
		return
	}
	texX := textureXFromU(texU, textureWidth)
	texXBase := texX * textureHeight
	texStep := uint32(textureHeight<<16) / uint32(fullHeight)
	texPos := uint32(drawTop-origTop) * texStep
	drawIndexedTexturedColumnInto(dst, stride, screenX, drawTop, drawBottom, texXBase, texPos, texStep, texture.Indices, renderLUT)
}

func drawRGBATexturedColumnIntoGameplay(dst []uint32, stride, screenX, drawTop, drawBottom, origTop, origBottom int, texture *wl6.WallTexture, texU float64) {
	fullHeight := origBottom - origTop
	if fullHeight <= 0 {
		return
	}
	textureWidth, textureHeight := wallTextureDimensions(texture)
	if textureWidth <= 0 || textureHeight <= 0 {
		return
	}
	texX := textureXFromU(texU, textureWidth)
	texXBase := texX * textureHeight
	texStep := uint32(textureHeight<<16) / uint32(fullHeight)
	texPos := uint32(drawTop-origTop) * texStep
	drawRGBATexturedColumnInto(dst, stride, screenX, drawTop, drawBottom, texXBase, texPos, texStep, texture.Pixels)
}

func clampTextureU(u float64) float64 {
	if u < 0 {
		return 0
	}
	if u >= 1 {
		return math.Nextafter(1, 0)
	}
	return u
}

func textureXFromU(texU float64, textureWidth int) int {
	if textureWidth <= 1 {
		return 0
	}
	texU = clampTextureU(texU)
	texX := int(texU * float64(textureWidth))
	if texX >= textureWidth {
		texX = textureWidth - 1
	}
	return texX
}

func wallTextureDimensions(texture *wl6.WallTexture) (width, height int) {
	if texture == nil {
		return 0, 0
	}
	width = texture.Width
	height = texture.Height
	if width <= 0 {
		width = wl6.WallTextureSize()
	}
	if height <= 0 {
		height = wl6.WallTextureSize()
	}
	return width, height
}

func writeRGBAFast(dst []byte, i int, rgba uint32) {
	dst[i] = byte(rgba >> 24)
	dst[i+1] = byte(rgba >> 16)
	dst[i+2] = byte(rgba >> 8)
	dst[i+3] = byte(rgba)
}

func writeRGBAFast32(dst []uint32, i int, rgba uint32) {
	dst[i] = bits.ReverseBytes32(rgba)
}

func blendRGBAOverBuffer32(dst []uint32, i int, rgba uint32) {
	srcA := int(byte(rgba))
	if srcA <= 0 {
		return
	}
	if srcA >= 255 {
		dst[i] = bits.ReverseBytes32(rgba)
		return
	}
	dstRGBA := bits.ReverseBytes32(dst[i])
	srcR := int(byte(rgba >> 24))
	srcG := int(byte(rgba >> 16))
	srcB := int(byte(rgba >> 8))
	invA := 255 - srcA
	dstR := int(byte(dstRGBA >> 24))
	dstG := int(byte(dstRGBA >> 16))
	dstB := int(byte(dstRGBA >> 8))
	out := uint32((srcR*srcA+dstR*invA)/255)<<24 |
		uint32((srcG*srcA+dstG*invA)/255)<<16 |
		uint32((srcB*srcA+dstB*invA)/255)<<8 |
		255
	dst[i] = bits.ReverseBytes32(out)
}

func writeRGBAOverBuffer32(dst []uint32, i int, rgba uint32) {
	switch byte(rgba) {
	case 0:
		return
	case 255:
		dst[i] = bits.ReverseBytes32(rgba)
	default:
		blendRGBAOverBuffer32(dst, i, rgba)
	}
}

func byteViewFromU32(src []uint32) []byte {
	if len(src) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&src[0])), len(src)*4)
}

func blendRGBAOverBuffer(dst []byte, i int, rgba uint32) {
	srcA := int(byte(rgba))
	if srcA <= 0 {
		return
	}
	if srcA >= 255 {
		dst[i] = byte(rgba >> 24)
		dst[i+1] = byte(rgba >> 16)
		dst[i+2] = byte(rgba >> 8)
		dst[i+3] = 255
		return
	}

	srcR := int(byte(rgba >> 24))
	srcG := int(byte(rgba >> 16))
	srcB := int(byte(rgba >> 8))
	invA := 255 - srcA

	dst[i] = byte((srcR*srcA + int(dst[i])*invA) / 255)
	dst[i+1] = byte((srcG*srcA + int(dst[i+1])*invA) / 255)
	dst[i+2] = byte((srcB*srcA + int(dst[i+2])*invA) / 255)
	dst[i+3] = 255
}

func writeRGBAOverBuffer(dst []byte, i int, rgba uint32) {
	switch byte(rgba) {
	case 0:
		return
	case 255:
		dst[i] = byte(rgba >> 24)
		dst[i+1] = byte(rgba >> 16)
		dst[i+2] = byte(rgba >> 8)
		dst[i+3] = 255
	default:
		blendRGBAOverBuffer(dst, i, rgba)
	}
}

func (g *game) visibleSprites() []spriteVis {
	g.spriteVisBuf = g.spriteVisBuf[:0]
	if cap(g.spriteVisBuf) < len(g.staticSprites)+len(g.actors)+1 {
		g.spriteVisBuf = make([]spriteVis, 0, len(g.staticSprites)+len(g.actors)+1)
	}
	for i, spr := range g.staticSprites {
		if !spr.alive {
			continue
		}
		dx := spr.x - g.playerX
		dy := spr.y - g.playerY
		g.spriteVisBuf = append(g.spriteVisBuf, spriteVis{
			index:     i,
			x:         dx,
			y:         dy,
			shapenum:  spr.shapenum,
			rotate:    spr.rotate,
			facingDir: spr.facingDir,
			tileDist:  g.playerAttackTileDistance(spr.x, spr.y),
			dist2:     dx*dx + dy*dy,
		})
	}
	if g.victoryActive && g.victoryBJ.alive {
		dx := g.victoryBJ.x - g.playerX
		dy := g.victoryBJ.y - g.playerY
		g.spriteVisBuf = append(g.spriteVisBuf, spriteVis{
			index:    -1,
			x:        dx,
			y:        dy,
			shapenum: g.victoryBJ.shapenum,
			tileDist: g.playerAttackTileDistance(g.victoryBJ.x, g.victoryBJ.y),
			dist2:    dx*dx + dy*dy,
		})
	}
	for _, actor := range g.actors {
		if !shouldDrawActorSprite(actor) {
			continue
		}
		dx := actor.x - g.playerX
		dy := actor.y - g.playerY
		g.spriteVisBuf = append(g.spriteVisBuf, spriteVis{
			x:         dx,
			y:         dy,
			shapenum:  actor.shapenum,
			rotate:    actor.rotate,
			facingDir: actor.facingDir,
			tileDist:  g.playerAttackTileDistance(actor.x, actor.y),
			dist2:     dx*dx + dy*dy,
		})
	}
	if len(g.spriteVisBuf) < 2 {
		return g.spriteVisBuf
	}
	sort.Sort(spriteVisByDist(g.spriteVisBuf))
	return g.spriteVisBuf
}

func (g *game) drawSprites(forwardX, forwardY, planeX, planeY, projPlaneDist float64) {
	if g.sprites == nil || len(g.zbuffer) != g.viewWidth {
		return
	}

	invDet := 1.0 / (planeX*forwardY - forwardX*planeY)
	visible := g.visibleSprites()
	if len(visible) == 0 {
		return
	}

	renderLeft, renderRight, renderTop, renderBottom, renderWidth, renderHeight := g.gameplayRenderArea()
	viewHeight := float64(renderHeight)
	for _, spr := range visible {
		transformX := invDet * (forwardY*spr.x - forwardX*spr.y)
		transformY := invDet * (-planeY*spr.x + planeX*spr.y)
		if transformY <= 0.01 || transformY >= maxSpriteDrawDepth {
			continue
		}

		spriteScreenX := renderLeft + int((float64(renderWidth)/2)*(1+transformX/transformY))
		shapenum := spr.shapenum
		if spr.rotate {
			shapenum += g.spriteRotateOffset(spr.x, spr.y, spr.facingDir)
		}
		spriteSize := int(projPlaneDist / transformY)
		if spriteSize <= 0 {
			continue
		}

		spriteTop := renderTop + int((viewHeight-float64(spriteSize))/2)
		drawTop := spriteTop
		drawBottom := spriteTop + spriteSize
		startX := spriteScreenX - spriteSize/2
		endX := startX + spriteSize
		if startX < renderLeft {
			startX = renderLeft
		}
		if endX > renderRight {
			endX = renderRight
		}
		if drawTop < renderTop {
			drawTop = renderTop
		}
		if drawBottom > renderBottom {
			drawBottom = renderBottom
		}
		if endX <= startX || drawBottom <= drawTop {
			continue
		}
		sprite, ok := g.sprites.SpriteByShape(shapenum)
		if !ok || len(sprite.Columns) == 0 {
			continue
		}
		texStep := uint32(sprite.Height<<16) / uint32(spriteSize)
		texXStep := uint32(sprite.Width<<16) / uint32(spriteSize)
		spriteLeft := spriteScreenX - spriteSize/2
		texXPos := uint32(startX-spriteLeft) * texXStep
		stride := g.viewWidth
		dst := g.frame32
		drawIndexed := sprite.IndexedFormat
		for x := startX; x < endX; x, texXPos = x+1, texXPos+texXStep {
			if transformY >= g.zbuffer[x] {
				continue
			}
			texX := int(texXPos >> 16)
			if texX < 0 || texX >= sprite.Width || texX >= len(sprite.Columns) {
				continue
			}
			column := sprite.Columns[texX]
			if len(column.Posts) == 0 {
				continue
			}
			for _, post := range column.Posts {
				postDrawTop, postDrawBottom := scaledPostBounds(spriteTop, spriteSize, post)
				if postDrawBottom <= drawTop || postDrawTop >= drawBottom {
					continue
				}
				yStart := maxInt(drawTop, postDrawTop)
				yEnd := minInt(drawBottom, postDrawBottom)
				if yEnd <= yStart {
					continue
				}
				texPos := uint32(yStart-spriteTop) * texStep
				i := yStart*g.viewWidth + x
				if drawIndexed {
					drawIndexedSpritePostInto(dst, stride, i, texPos, texStep, yStart, yEnd, post, &g.walls.RenderLUT, 0)
				} else {
					drawRGBASpritePostInto(dst, stride, i, texPos, texStep, yStart, yEnd, post)
				}
			}
		}
	}
}

func (g *game) drawSpritesScaled(forwardX, forwardY, planeX, planeY, projPlaneDist float64) {
	if g.sprites == nil || len(g.zbuffer) < g.layout.bufferWidth || g.layout.bufferWidth <= 0 || g.layout.bufferHeight <= 0 {
		return
	}

	invDet := 1.0 / (planeX*forwardY - forwardX*planeY)
	visible := g.visibleSprites()
	if len(visible) == 0 {
		return
	}

	bufferWidth := g.layout.bufferWidth
	bufferHeight := g.layout.bufferHeight
	viewHeight := float64(bufferHeight)
	for visIdx := len(visible) - 1; visIdx >= 0; visIdx-- {
		spr := visible[visIdx]
		transformX := invDet * (forwardY*spr.x - forwardX*spr.y)
		transformY := invDet * (-planeY*spr.x + planeX*spr.y)
		if transformY <= 0.01 || transformY >= maxSpriteDrawDepth {
			continue
		}

		spriteScreenX := int((float64(bufferWidth) / 2) * (1 + transformX/transformY))
		shapenum := spr.shapenum
		if spr.rotate {
			shapenum += g.spriteRotateOffset(spr.x, spr.y, spr.facingDir)
		}
		spriteSize := int(projPlaneDist / transformY)
		if spriteSize <= 0 {
			continue
		}

		spriteTop := int((viewHeight - float64(spriteSize)) / 2)
		drawTop := maxInt(0, spriteTop)
		drawBottom := minInt(bufferHeight, spriteTop+spriteSize)
		startX := spriteScreenX - spriteSize/2
		endX := startX + spriteSize
		if startX < 0 {
			startX = 0
		}
		if endX > bufferWidth {
			endX = bufferWidth
		}
		if endX <= startX || drawBottom <= drawTop {
			continue
		}
		sprite, ok := g.sprites.SpriteByShape(shapenum)
		if !ok || len(sprite.Columns) == 0 {
			continue
		}
		texStep := uint32(sprite.Height<<16) / uint32(spriteSize)
		texXStep := uint32(sprite.Width<<16) / uint32(spriteSize)
		spriteLeft := spriteScreenX - spriteSize/2
		texXPos := uint32(startX-spriteLeft) * texXStep
		stride := bufferWidth
		dst := g.gameplayFrame32
		drawIndexed := sprite.IndexedFormat
		for x := startX; x < endX; x, texXPos = x+1, texXPos+texXStep {
			if transformY >= g.zbuffer[x] {
				continue
			}
			texX := int(texXPos >> 16)
			if texX < 0 || texX >= sprite.Width || texX >= len(sprite.Columns) {
				continue
			}
			column := sprite.Columns[texX]
			if len(column.Posts) == 0 {
				continue
			}
			for _, post := range column.Posts {
				postDrawTop, postDrawBottom := scaledPostBounds(spriteTop, spriteSize, post)
				if postDrawBottom <= drawTop || postDrawTop >= drawBottom {
					continue
				}
				yStart := maxInt(drawTop, postDrawTop)
				yEnd := minInt(drawBottom, postDrawBottom)
				if yEnd <= yStart {
					continue
				}
				texPos := uint32(yStart-spriteTop) * texStep
				var visibleSpans [maxColumnSpanCount]columnSpan
				spanCount := g.columnCoverage[x].subtract(yStart, yEnd, &visibleSpans)
				for spanIdx := 0; spanIdx < spanCount; spanIdx++ {
					span := visibleSpans[spanIdx]
					i := span.start*bufferWidth + x
					spanTexPos := texPos + uint32(span.start-yStart)*texStep
					if drawIndexed {
						drawIndexedSpritePostInto(dst, stride, i, spanTexPos, texStep, span.start, span.end, post, &g.walls.RenderLUT, 0)
					} else {
						drawRGBASpritePostInto(dst, stride, i, spanTexPos, texStep, span.start, span.end, post)
					}
					g.columnCoverage[x].reserve(span.start, span.end)
				}
			}
		}
	}
}

func (g *game) drawWallColumnsScaled() {
	if len(g.wallColumns) < g.layout.bufferWidth || len(g.columnCoverage) < g.layout.bufferWidth {
		return
	}
	for x := 0; x < g.layout.bufferWidth; x++ {
		col := g.wallColumns[x]
		if !col.hit || col.drawBottom <= col.drawTop {
			continue
		}
		texture := g.pickWallTexture(col.wallID, col.side, col.texOverride)
		if texture == nil || texture.Empty() {
			continue
		}
		var visibleSpans [maxColumnSpanCount]columnSpan
		spanCount := g.columnCoverage[x].subtract(col.drawTop, col.drawBottom, &visibleSpans)
		for spanIdx := 0; spanIdx < spanCount; spanIdx++ {
			span := visibleSpans[spanIdx]
			if len(texture.Indices) > 0 {
				lutDim := 0
				if texture.UseDimPalette {
					lutDim = 1
				}
				drawIndexedTexturedColumnIntoGameplay(g.gameplayFrame32, g.layout.bufferWidth, x, span.start, span.end, col.origTop, col.origBottom, texture, col.texU, &g.walls.RenderLUT[lutDim])
			} else {
				drawRGBATexturedColumnIntoGameplay(g.gameplayFrame32, g.layout.bufferWidth, x, span.start, span.end, col.origTop, col.origBottom, texture, col.texU)
			}
		}
	}
}

func shouldDrawActorSprite(actor actorInstance) bool {
	return actor.alive || actor.aiState == actorStateDead
}

func (c *columnCoverage) clear() {
	c.count = 0
}

func (c *columnCoverage) reserve(start, end int) {
	if end <= start {
		return
	}
	newStart := start
	newEnd := end
	insertAt := c.count
	for i := 0; i < c.count; i++ {
		span := c.spans[i]
		if newEnd < span.start {
			insertAt = i
			break
		}
		if newStart > span.end {
			continue
		}
		if span.start < newStart {
			newStart = span.start
		}
		if span.end > newEnd {
			newEnd = span.end
		}
		if insertAt > i {
			insertAt = i
		}
		copy(c.spans[i:c.count-1], c.spans[i+1:c.count])
		c.count--
		i--
	}
	if c.count >= len(c.spans) {
		c.spans[c.count-1] = columnSpan{start: newStart, end: newEnd}
		return
	}
	copy(c.spans[insertAt+1:c.count+1], c.spans[insertAt:c.count])
	c.spans[insertAt] = columnSpan{start: newStart, end: newEnd}
	c.count++
}

func (c *columnCoverage) subtract(start, end int, out *[maxColumnSpanCount]columnSpan) int {
	if end <= start {
		return 0
	}
	cur := start
	outCount := 0
	for i := 0; i < c.count; i++ {
		span := c.spans[i]
		if span.end <= cur {
			continue
		}
		if span.start >= end {
			break
		}
		if cur < span.start {
			out[outCount] = columnSpan{start: cur, end: minInt(end, span.start)}
			outCount++
			if outCount >= len(out) {
				return outCount
			}
		}
		if span.end > cur {
			cur = span.end
		}
		if cur >= end {
			return outCount
		}
	}
	if cur < end && outCount < len(out) {
		out[outCount] = columnSpan{start: cur, end: end}
		outCount++
	}
	return outCount
}

func (g *game) spriteRotateOffset(spriteDX, spriteDY float64, facingDir int) int {
	playerOctant := vectorOctant(-spriteDX, -spriteDY)
	facingOctant := (8 - (facingDir & 7)) & 7
	offset := facingOctant - playerOctant
	if offset < 0 {
		offset += 8
	}
	return offset
}

func vectorOctant(dx, dy float64) int {
	const tan22p5 = 0.41421356237309503
	ax := math.Abs(dx)
	ay := math.Abs(dy)

	if ay < ax*tan22p5 {
		if dx < 0 {
			return 4
		}
		return 0
	}
	if ax <= ay*tan22p5 {
		if dy < 0 {
			return 6
		}
		return 2
	}

	switch {
	case dx >= 0 && dy >= 0:
		return 1
	case dx < 0 && dy >= 0:
		return 3
	case dx < 0 && dy < 0:
		return 5
	default:
		return 7
	}
}

func facingDirAngle(facingDir int) float64 {
	angle := float64(facingDir&7) * (math.Pi / 4)
	return math.Mod(2*math.Pi-angle, 2*math.Pi)
}

func (g *game) pickWallTexture(wallID uint16, side int, texOverride int) *wl6.WallTexture {
	if texOverride >= 0 {
		if g.walls == nil || texOverride >= len(g.walls.AllPages) {
			return nil
		}
		return &g.walls.AllPages[texOverride]
	}
	if wallID >= 90 && wallID <= 101 {
		return g.pickDoorTexture(wallID, side)
	}
	if int(wallID) >= len(g.walls.Horizontal) {
		return nil
	}
	if side == 1 {
		if g.walls.Vertical[wallID].Empty() {
			return nil
		}
		return &g.walls.Vertical[wallID]
	}
	if g.walls.Horizontal[wallID].Empty() {
		return nil
	}
	return &g.walls.Horizontal[wallID]
}

func (g *game) wallDoorSideTexture(tileIndex, side, stepX, stepY int) int {
	if g.level == nil || g.walls == nil || len(g.walls.AllPages) < 102 || tileIndex < 0 || tileIndex >= len(g.cachedDoorSides) {
		return -1
	}

	doorSides := g.cachedDoorSides[tileIndex]
	if side == 1 {
		if stepX < 0 && doorSides&2 != 0 {
			return 101 // DOORWALL+3
		}
		if stepX > 0 && doorSides&1 != 0 {
			return 101 // DOORWALL+3
		}
		return -1
	}

	if stepY < 0 && doorSides&8 != 0 {
		return 100 // DOORWALL+2
	}
	if stepY > 0 && doorSides&4 != 0 {
		return 100 // DOORWALL+2
	}
	return -1
}

func (g *game) pickDoorTexture(wallID uint16, side int) *wl6.WallTexture {
	if g.walls == nil || len(g.walls.AllPages) < 8 {
		return nil
	}

	base := len(g.walls.AllPages) - 8 // DOORWALL = PMSpriteStart-8 across Wolf data sets.
	lock := 0
	if wallID%2 == 0 {
		lock = int((wallID - 90) / 2)
	} else {
		lock = int((wallID - 91) / 2)
	}

	switch {
	case lock >= 1 && lock <= 4:
		base += 6
	case lock == 5:
		base += 4
	}
	if side == 1 {
		base++
	}

	if base < 0 || base >= len(g.walls.AllPages) {
		return nil
	}
	return &g.walls.AllPages[base]
}

func (g *game) tryMove(dx, dy float64) {
	if g.level == nil {
		return
	}

	nextX := g.playerX + dx
	nextY := g.playerY + dy
	if g.collidesDoor(nextX, g.playerY) || g.collidesDoor(g.playerX, nextY) || g.collidesDoor(nextX, nextY) {
		if !g.collides(nextX, nextY) {
			g.playerX = nextX
			g.playerY = nextY
		}
		return
	}

	if !g.collides(nextX, g.playerY) {
		g.playerX = nextX
	}

	if !g.collides(g.playerX, nextY) {
		g.playerY = nextY
	}
}

func playerBounds(x, y float64) (left, right, top, bottom float64) {
	return x - playerRadius, x + playerRadius, y - playerRadius, y + playerRadius
}

func playerTileSpan(x, y float64) (xl, xh, yl, yh int) {
	left, right, top, bottom := playerBounds(x, y)
	return int(math.Floor(left)), int(math.Floor(right)), int(math.Floor(top)), int(math.Floor(bottom))
}

func playerOverlapsTile(x, y float64, tileX, tileY int) bool {
	left, right, top, bottom := playerBounds(x, y)
	return right > float64(tileX) &&
		left < float64(tileX+1) &&
		bottom > float64(tileY) &&
		top < float64(tileY+1)
}

func boundsOverlap(x1, y1, r1, x2, y2, r2 float64) bool {
	return math.Abs(x1-x2) < r1+r2 && math.Abs(y1-y2) < r1+r2
}

func (g *game) pushWallBounds() (minX, maxX, minY, maxY float64, ok bool) {
	if !g.pushWall.active {
		return 0, 0, 0, 0, false
	}
	progress := float64(g.pushWall.tics) / float64(pushWallStepTics)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	minX = float64(g.pushWall.x)
	minY = float64(g.pushWall.y)
	if g.pushWall.dx > 0 {
		minX += progress
	} else if g.pushWall.dx < 0 {
		minX -= progress
	}
	if g.pushWall.dy > 0 {
		minY += progress
	} else if g.pushWall.dy < 0 {
		minY -= progress
	}
	return minX, minX + 1, minY, minY + 1, true
}

func (g *game) tileIntersectsPushWall(x, y int) bool {
	minX, maxX, minY, maxY, ok := g.pushWallBounds()
	if !ok {
		return false
	}
	return float64(x+1) > minX &&
		float64(x) < maxX &&
		float64(y+1) > minY &&
		float64(y) < maxY
}

func (g *game) collides(x, y float64) bool {
	left, right, top, bottom := playerBounds(x, y)
	xl, xh, yl, yh := playerTileSpan(x, y)
	for ty := yl; ty <= yh; ty++ {
		for tx := xl; tx <= xh; tx++ {
			if g.doorCollisionAt(tx, ty, left, right, top, bottom) {
				return true
			}
			if g.isBlockingTile(tx, ty) {
				return true
			}
		}
	}
	for _, spr := range g.staticSprites {
		if !spr.alive || !spr.blocking {
			continue
		}
		if boundsOverlap(x, y, playerRadius, spr.x, spr.y, staticBlockRadius) {
			return true
		}
	}
	for i := range g.actors {
		actor := &g.actors[i]
		if !actor.alive || !actor.blocking {
			continue
		}
		if math.Abs(x-actor.x) < playerBlockDist && math.Abs(y-actor.y) < playerBlockDist {
			return true
		}
	}
	if minX, maxX, minY, maxY, ok := g.pushWallBounds(); ok {
		if right > minX && left < maxX && bottom > minY && top < maxY {
			return true
		}
	}
	return false
}

func (g *game) collidesDoor(x, y float64) bool {
	left, right, top, bottom := playerBounds(x, y)
	xl, xh, yl, yh := playerTileSpan(x, y)
	for ty := yl; ty <= yh; ty++ {
		for tx := xl; tx <= xh; tx++ {
			if g.doorCollisionAt(tx, ty, left, right, top, bottom) {
				return true
			}
		}
	}
	return false
}

func (g *game) doorCollisionAt(tileX, tileY int, left, right, top, bottom float64) bool {
	if g.level == nil || tileX < 0 || tileY < 0 || tileX >= g.level.Width || tileY >= g.level.Height {
		return false
	}
	tile := g.level.Tile(tileX, tileY)
	if tile.Door == nil {
		return false
	}
	thickness := doorCollisionThickness * (1 - clampUnit(g.doorOpenness(tileX, tileY)))
	if thickness <= 0 {
		return false
	}

	halfThickness := thickness / 2
	if tile.Door.Vertical {
		doorLeft := float64(tileX) + 0.5 - halfThickness
		doorRight := float64(tileX) + 0.5 + halfThickness
		return right > doorLeft && left < doorRight &&
			bottom > float64(tileY) && top < float64(tileY+1)
	}

	doorTop := float64(tileY) + 0.5 - halfThickness
	doorBottom := float64(tileY) + 0.5 + halfThickness
	return right > float64(tileX) && left < float64(tileX+1) &&
		bottom > doorTop && top < doorBottom
}

func (g *game) isBlockingTile(x, y int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.level.Width || y >= g.level.Height {
		return true
	}
	if g.pushWall.active && x == g.pushWall.x && y == g.pushWall.y {
		return true
	}
	tile := g.level.Tile(x, y)
	if tile.Door != nil {
		return false
	}
	return tile.Solid
}

func (g *game) isDoorOpen(x, y int) bool {
	if g.level == nil || len(g.doorOpen) != g.level.Width*g.level.Height {
		return false
	}
	i := y*g.level.Width + x
	return g.doorState[i] == 2
}

func (g *game) doorOpenness(x, y int) float64 {
	if g.level == nil || len(g.doorOpen) != g.level.Width*g.level.Height {
		return 0
	}
	return g.doorOpen[y*g.level.Width+x]
}

func (g *game) doorBlockedByPlayer(tileX, tileY int, door *wl6.Door) bool {
	if door == nil {
		return false
	}
	left, right, top, bottom := playerBounds(g.playerX, g.playerY)
	if door.Vertical {
		doorLeft := float64(tileX) + 0.5 - doorCollisionThickness/2
		doorRight := float64(tileX) + 0.5 + doorCollisionThickness/2
		return right > doorLeft && left < doorRight &&
			bottom > float64(tileY) && top < float64(tileY+1)
	}
	doorTop := float64(tileY) + 0.5 - doorCollisionThickness/2
	doorBottom := float64(tileY) + 0.5 + doorCollisionThickness/2
	return right > float64(tileX) && left < float64(tileX+1) &&
		bottom > doorTop && top < doorBottom
}

func (g *game) doorBlockedByActors(tileX, tileY int, door *wl6.Door) bool {
	if door == nil {
		return false
	}
	if g.blockingActorAt(nil, tileX, tileY) != nil {
		return true
	}
	if door.Vertical {
		if actor := g.blockingActorAt(nil, tileX-1, tileY); actor != nil && int(math.Floor(actor.x+playerRadius)) == tileX {
			return true
		}
		if actor := g.blockingActorAt(nil, tileX+1, tileY); actor != nil && int(math.Floor(actor.x-playerRadius)) == tileX {
			return true
		}
		return false
	}
	if actor := g.blockingActorAt(nil, tileX, tileY-1); actor != nil && int(math.Floor(actor.y+playerRadius)) == tileY {
		return true
	}
	if actor := g.blockingActorAt(nil, tileX, tileY+1); actor != nil && int(math.Floor(actor.y-playerRadius)) == tileY {
		return true
	}
	return false
}

func (g *game) updateDoors(tics int) {
	if tics <= 0 {
		return
	}
	if g.level == nil || len(g.doorOpen) != g.level.Width*g.level.Height || len(g.doorState) != len(g.doorOpen) || len(g.doorTimer) != len(g.doorOpen) {
		return
	}
	for i, state := range g.doorState {
		switch state {
		case 1: // opening
			g.doorOpen[i] += doorOpenRatePerTic * float64(tics)
			if g.doorOpen[i] >= 1 {
				g.doorOpen[i] = 1
				g.doorState[i] = 2
				g.doorTimer[i] = 0
			}
		case 2: // open
			g.doorTimer[i] += tics
			if g.doorTimer[i] >= doorOpenHoldTics {
				x := i % g.level.Width
				y := i / g.level.Width
				door := g.level.Tile(x, y).Door
				if g.doorBlockedByPlayer(x, y, door) || g.doorBlockedByActors(x, y, door) {
					continue
				}
				g.doorState[i] = 3
				g.playWorldSound(soundDoorClose, float64(x)+0.5, float64(y)+0.5)
			}
		case 3: // closing
			x := i % g.level.Width
			y := i / g.level.Width
			door := g.level.Tile(x, y).Door
			if g.doorBlockedByPlayer(x, y, door) || g.doorBlockedByActors(x, y, door) || playerOverlapsTile(g.playerX, g.playerY, x, y) {
				g.doorState[i] = 1
				g.doorTimer[i] = 0
				g.doorOpen[i] = max(g.doorOpen[i], 0.01)
				continue
			}
			g.doorOpen[i] -= doorOpenRatePerTic * float64(tics)
			if g.doorOpen[i] <= 0 {
				g.doorOpen[i] = 0
				g.doorState[i] = 0
				g.doorTimer[i] = 0
			}
		default:
			g.doorOpen[i] = max(0, g.doorOpen[i])
		}
	}
}

func (g *game) useDoorAhead() {
	if g.level == nil || len(g.doorOpen) != g.level.Width*g.level.Height || len(g.doorState) != len(g.doorOpen) || len(g.doorTimer) != len(g.doorOpen) {
		return
	}

	x, y := g.cardinalUseTile()
	if x < 0 || y < 0 || x >= g.level.Width || y >= g.level.Height {
		return
	}
	tile := g.level.Tile(x, y)
	if tile.RawInfo == pushableTile {
		dx, dy := g.cardinalUseVector()
		if !g.startPushWall(x, y, dx, dy) {
			g.playSound(soundNoWay)
			g.setNotice("No secret push")
		}
		return
	}
	if tile.RawWall == wolfElevatorTile && g.canUseElevatorSwitch() {
		if err := g.useElevator(); err != nil {
			g.setNotice("Elevator failed")
		}
		return
	}
	if tile.Door == nil {
		return
	}
	if !canOpenDoorLock(tile.Door.Lock, g.keys) {
		g.playSound(soundNoWay)
		g.setNotice("Locked door")
		return
	}
	i := y*g.level.Width + x
	switch g.doorState[i] {
	case 2, 1:
		g.doorState[i] = 2
		g.doorTimer[i] = 0
	default:
		g.doorState[i] = 1
		g.doorOpen[i] = max(g.doorOpen[i], 0.01)
		g.doorTimer[i] = 0
		g.playWorldSound(soundDoorOpen, float64(x)+0.5, float64(y)+0.5)
	}
}

func (g *game) canUseElevatorSwitch() bool {
	dirX := math.Cos(g.playerA)
	dirY := math.Sin(g.playerA)
	return math.Abs(dirX) >= math.Abs(dirY)
}

func (g *game) currentEpisodeIndex() int {
	episode := g.mapIndex / 10
	if episode < 0 {
		return 0
	}
	if episode >= len(wolfElevatorBackTo) {
		return len(wolfElevatorBackTo) - 1
	}
	return episode
}

func (g *game) nextElevatorMap(secret bool) int {
	episode := g.currentEpisodeIndex()
	local := g.mapIndex % 10
	if local == 9 {
		return episode*10 + wolfElevatorBackTo[episode]
	}
	if secret {
		return episode*10 + 9
	}
	return g.mapIndex + 1
}

func (g *game) useElevator() error {
	switchX, switchY := g.cardinalUseTile()
	playerTileX := int(math.Floor(g.playerX))
	playerTileY := int(math.Floor(g.playerY))
	secret := false
	if g.level != nil {
		secret = g.level.Tile(playerTileX, playerTileY).RawWall == wolfAltElevatorTile
	}
	g.flipElevatorSwitch(switchX, switchY)
	nextMap := g.nextElevatorMap(secret)
	g.playSound(soundMenuConfirm)
	if g.mapData != nil && g.level != nil {
		if err := g.triggerAutosave("level-transition"); err != nil && !errors.Is(err, errSaveUnsupported) {
			log.Printf("level-transition autosave failed: %v", err)
		}
	}
	return g.startFadeTransition(14, func() error {
		return g.beginGetPsyched(nextMap)
	})
}

func (g *game) flipElevatorSwitch(x, y int) {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return
	}
	tile := g.level.Tile(x, y)
	if tile.RawWall != wolfElevatorTile {
		return
	}
	tile.RawWall = wolfElevatorUsedTile
	g.setLevelTile(x, y, tile)
	g.refreshLevelGeometry()
}

func (g *game) cardinalUseTile() (int, int) {
	tileX := int(math.Floor(g.playerX))
	tileY := int(math.Floor(g.playerY))
	dx, dy := g.cardinalUseVector()
	return tileX + dx, tileY + dy
}

func (g *game) cardinalUseVector() (int, int) {
	dirX := math.Cos(g.playerA)
	dirY := math.Sin(g.playerA)
	if math.Abs(dirX) >= math.Abs(dirY) {
		if dirX >= 0 {
			return 1, 0
		}
		return -1, 0
	}
	if dirY >= 0 {
		return 0, 1
	}
	return 0, -1
}

func (g *game) setLevelTile(x, y int, tile wl6.Tile) {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return
	}
	g.level.Tiles[y*g.levelWidth+x] = tile
}

func (g *game) floorTileFor(x, y int) wl6.Tile {
	area := -1
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || ny < 0 || nx >= g.levelWidth || ny >= g.levelHeight {
			continue
		}
		neighbor := g.level.Tile(nx, ny)
		if neighbor.Solid || neighbor.Door != nil {
			continue
		}
		area = neighbor.Area
		if area >= 0 {
			break
		}
	}
	return wl6.Tile{Area: area}
}

func (g *game) pushWallDestinationClear(x, y int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false
	}
	tile := g.level.Tile(x, y)
	if tile.Solid || tile.Door != nil {
		return false
	}
	if playerOverlapsTile(g.playerX, g.playerY, x, y) {
		return false
	}
	for _, spr := range g.staticSprites {
		if !spr.alive || !spr.blocking {
			continue
		}
		if int(spr.x) == x && int(spr.y) == y {
			return false
		}
	}
	if g.blockingActorAt(nil, x, y) != nil {
		return false
	}
	return true
}

func (g *game) startPushWall(x, y, dx, dy int) bool {
	if g.level == nil || g.pushWall.active || (dx == 0 && dy == 0) {
		return false
	}
	tile := g.level.Tile(x, y)
	if tile.RawInfo != pushableTile || !tile.Solid || tile.Door != nil {
		return false
	}
	if !g.pushWallDestinationClear(x+dx, y+dy) {
		return false
	}
	tile.RawInfo = 0
	g.setLevelTile(x, y, g.floorTileFor(x, y))
	g.secretCount++
	g.pushWall = pushWallState{
		active: true,
		x:      x,
		y:      y,
		dx:     dx,
		dy:     dy,
		steps:  0,
		tics:   0,
		wall:   tile,
	}
	g.refreshLevelGeometry()
	g.playWorldSound(soundPushWall, float64(x)+0.5, float64(y)+0.5)
	g.setNotice("Secret wall")
	return true
}

func (g *game) finishPushWall() {
	if g.pushWall.active && g.level != nil {
		tile := g.pushWall.wall
		tile.RawInfo = 0
		g.setLevelTile(g.pushWall.x, g.pushWall.y, tile)
		g.refreshLevelGeometry()
	}
	g.pushWall = pushWallState{}
}

func (g *game) updatePushWall(tics int) {
	if !g.pushWall.active || tics <= 0 {
		return
	}
	g.pushWall.tics += tics
	for g.pushWall.active && g.pushWall.tics >= pushWallStepTics {
		g.pushWall.tics -= pushWallStepTics

		curX := g.pushWall.x
		curY := g.pushWall.y
		nextX := curX + g.pushWall.dx
		nextY := curY + g.pushWall.dy
		if !g.pushWallDestinationClear(nextX, nextY) {
			g.finishPushWall()
			return
		}

		g.setLevelTile(curX, curY, g.floorTileFor(curX, curY))
		g.pushWall.x = nextX
		g.pushWall.y = nextY
		g.pushWall.steps++
		g.refreshLevelGeometry()

		if g.pushWall.steps >= pushWallMaxSteps || !g.pushWallDestinationClear(nextX+g.pushWall.dx, nextY+g.pushWall.dy) {
			g.finishPushWall()
		}
	}
}

func (g *game) updateSpriteAnimations(tics int) {
	if tics <= 0 {
		return
	}
	for i := range g.staticSprites {
		spr := &g.staticSprites[i]
		if spr.sequenceID == "" {
			continue
		}
		g.advanceStaticSpriteSequence(spr, tics)
	}
}

func (g *game) applyItemCheat() {
	g.score += 100000
	g.health = 100
	if g.bestWeapon < 3 {
		g.giveWeapon(g.bestWeapon + 1)
	}
	g.giveAmmo(50)
	g.setNotice("Free items!")
}

func (g *game) applyExtraStuffCheat() {
	g.keys = 0x0f
	g.setNotice("Extra stuff!")
}

func (g *game) warpToLevelNumber(level int) error {
	if level < 1 || level > len(g.summaries) {
		g.setNotice("Invalid level")
		return nil
	}
	index := g.summaries[level-1].Index
	if err := g.setMap(index); err != nil {
		return err
	}
	g.selectedLevel = level - 1
	g.pendingMap = index
	g.setNotice(fmt.Sprintf("Warped to level %d", level))
	return nil
}

func (g *game) debugStatsLines() []string {
	doors := 0
	for _, flags := range g.cachedDoorFlags {
		if flags != 0 {
			doors++
		}
	}
	staticsAlive := 0
	for _, spr := range g.staticSprites {
		if spr.alive {
			staticsAlive++
		}
	}
	actorsAlive := 0
	for _, actor := range g.actors {
		if actor.alive {
			actorsAlive++
		}
	}
	return []string{
		"Level Stats",
		fmt.Sprintf("Statics %d/%d", staticsAlive, len(g.staticSprites)),
		fmt.Sprintf("Doors %d", doors),
		fmt.Sprintf("Actors %d/%d", actorsAlive, len(g.actors)),
		fmt.Sprintf("Treasure %d/%d", g.treasureCount, g.treasureTotal),
	}
}

func (g *game) debugCoordsLines() []string {
	angle := math.Mod(g.playerA*180/math.Pi+360, 360)
	return []string{
		"Coordinates",
		fmt.Sprintf("X %.2f", g.playerX),
		fmt.Sprintf("Y %.2f", g.playerY),
		fmt.Sprintf("A %.1f", angle),
	}
}

func (g *game) debugMemoryLines() []string {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return []string{
		"Memory Usage",
		fmt.Sprintf("Alloc %.1f MB", float64(stats.Alloc)/(1024*1024)),
		fmt.Sprintf("Sys %.1f MB", float64(stats.Sys)/(1024*1024)),
		fmt.Sprintf("Heap %.1f MB", float64(stats.HeapAlloc)/(1024*1024)),
		fmt.Sprintf("GC %d", stats.NumGC),
	}
}

func (g *game) drawTextPanel(screen *ebiten.Image, x, y, w, h int, lines []string) {
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{R: 12, G: 10, B: 10, A: 220}, false)
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, color.RGBA{R: 180, G: 148, B: 84, A: 255}, false)
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, x+10, y+10+i*14)
	}
}

func (g *game) drawDebugOverlays(screen *ebiten.Image) {
	left := 8
	top := 8
	if g.debugShowStats {
		g.drawTextPanel(screen, left, top, 184, 86, g.debugStatsLines())
		top += 94
	}
	if g.debugShowCoords {
		g.drawTextPanel(screen, left, top, 184, 72, g.debugCoordsLines())
		top += 80
	}
	if g.debugShowMemory {
		g.drawTextPanel(screen, left, top, 184, 86, g.debugMemoryLines())
	}
	if g.debugPrompt != debugPromptNone {
		title := "Cheat"
		rangeText := ""
		switch g.debugPrompt {
		case debugPromptBorderColor:
			title = "Border color"
			rangeText = "0-15"
		case debugPromptExtraVBLs:
			title = "Extra VBLs"
			rangeText = "0-8"
		case debugPromptWarpLevel:
			title = "Warp level"
			rangeText = fmt.Sprintf("1-%d", len(g.summaries))
		}
		lines := []string{
			title,
			"Enter value: " + g.debugPromptInput,
			"Range " + rangeText,
			"Enter accept  Esc cancel",
		}
		w := 240
		h := 72
		x := (g.viewWidth - w) / 2
		y := (g.viewHeight - h) / 2
		g.drawTextPanel(screen, x, y, w, h, lines)
	}
	if g.hudNotice != "" {
		g.drawHudNotice(screen)
	}
	if g.fpsText != "" {
		ebitenutil.DebugPrintAt(screen, g.fpsText, g.viewWidth-len(g.fpsText)*7-4, 4)
	}
}

func (g *game) drawHudNotice(screen *ebiten.Image) {
	style := wolfTextNormalStyle()
	style.scale = 2
	g.drawWolfText(screen, 8, 8, g.hudNotice, style)
}

func (g *game) drawGraphicsTest(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 10, G: 10, B: 14, A: 255})
	if g.walls == nil || len(g.walls.AllPages) == 0 {
		ebitenutil.DebugPrint(screen, "no textures loaded")
		return
	}
	page := g.debugTexturePage
	if page < 0 || page >= len(g.walls.AllPages) {
		page = 0
	}
	texture := &g.walls.AllPages[page]
	img, ok := g.wallTextureImage(texture)
	if !ok {
		ebitenutil.DebugPrint(screen, "texture unavailable")
		return
	}
	scale := math.Min(float64(g.viewWidth)/float64(img.Bounds().Dx())*0.7, float64(g.viewHeight)/float64(img.Bounds().Dy())*0.7)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((float64(g.viewWidth)-float64(img.Bounds().Dx())*scale)/2, (float64(g.viewHeight)-float64(img.Bounds().Dy())*scale)/2)
	screen.DrawImage(img, op)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Graphics Test  Page %d/%d  Texture %d", page+1, len(g.walls.AllPages), texture.Page), 16, 16)
	ebitenutil.DebugPrintAt(screen, "Left/Right browse  Enter/Esc/TAB+T close", 16, 32)
}

func (g *game) setNotice(msg string) {
	if msg == "" {
		return
	}
	g.hudNotice = msg
	g.hudNoticeTimer = 180
}

func (g *game) closeSoundBanks() {
	for _, players := range g.soundBanks {
		for _, voice := range players {
			if voice == nil || voice.player == nil {
				continue
			}
			voice.player.Pause()
			_ = voice.player.Close()
		}
	}
	g.soundBanks = nil
	g.soundBankIndex = nil
}

func (g *game) rebuildSoundBanks() error {
	g.closeSoundBanks()
	if g.audioContext == nil || len(g.soundData) == 0 {
		return nil
	}

	g.soundBanks = make(map[soundID][]*soundVoice, len(g.soundData))
	g.soundBankIndex = make(map[soundID]int, len(g.soundData))
	for id, data := range g.soundData {
		if len(data) == 0 {
			continue
		}
		players := make([]*soundVoice, 0, soundVoiceCount(id))
		for range soundVoiceCount(id) {
			base := make([]byte, len(data))
			copy(base, data)
			src := &pcmBufferSource{buf: base}
			player, err := g.audioContext.NewPlayer(src)
			if err != nil {
				return err
			}
			player.SetVolume(g.sfxVolume)
			players = append(players, &soundVoice{player: player, src: src, base: base})
		}
		g.soundBanks[id] = players
	}
	return nil
}

func (g *game) applySFXVolume() {
	for _, players := range g.soundBanks {
		for _, voice := range players {
			if voice == nil || voice.player == nil {
				continue
			}
			voice.player.SetVolume(g.sfxVolume)
		}
	}
}

func (g *game) playSound(id soundID) {
	g.lastPlayedSound = id
	voices := g.soundBanks[id]
	if len(voices) == 0 {
		return
	}

	start := g.soundBankIndex[id]
	chosen := voices[start%len(voices)]
	for i := range voices {
		idx := (start + i) % len(voices)
		if !voices[idx].player.IsPlaying() {
			chosen = voices[idx]
			g.soundBankIndex[id] = (idx + 1) % len(voices)
			chosen.src.buf = chosen.base
			chosen.src.Reset()
			_ = chosen.player.Rewind()
			chosen.player.SetVolume(g.sfxVolume)
			chosen.player.Play()
			return
		}
	}
	g.soundBankIndex[id] = (start + 1) % len(voices)
	chosen.player.Pause()
	chosen.src.buf = chosen.base
	chosen.src.Reset()
	_ = chosen.player.Rewind()
	chosen.player.SetVolume(g.sfxVolume)
	chosen.player.Play()
}

func (g *game) collectPickups() {
	if len(g.staticSprites) == 0 {
		return
	}

	playerTileX := int(g.playerX)
	playerTileY := int(g.playerY)
	for i := range g.staticSprites {
		spr := &g.staticSprites[i]
		if !spr.alive || spr.pickup == pickupNone {
			continue
		}
		if int(spr.x) != playerTileX || int(spr.y) != playerTileY {
			continue
		}
		if !g.applyPickup(spr.pickup) {
			continue
		}
		spr.alive = false
		spr.blocking = false
		g.setNotice("Picked up " + pickupLabel(spr.pickup))
	}
}

func (g *game) applyPickup(pickup pickupType) bool {
	switch pickup {
	case pickupFood:
		if g.health >= 100 {
			return false
		}
		g.health = minInt(100, g.health+10)
		g.playSound(soundPickupHealth1)
	case pickupFirstAid:
		if g.health >= 100 {
			return false
		}
		g.health = minInt(100, g.health+25)
		g.playSound(soundPickupHealth2)
	case pickupGibs:
		if g.health > 10 {
			return false
		}
		g.health = minInt(100, g.health+1)
		g.playSound(soundPickupGibs)
	case pickupAlpo:
		if g.health >= 100 {
			return false
		}
		g.health = minInt(100, g.health+4)
		g.playSound(soundPickupHealth1)
	case pickupClip:
		if g.ammo >= 99 {
			return false
		}
		g.giveAmmo(8)
		g.playSound(soundPickupAmmo)
	case pickupClip2:
		if g.ammo >= 99 {
			return false
		}
		g.giveAmmo(4)
		g.playSound(soundPickupAmmo)
	case pickupMachineGun:
		g.giveWeapon(2)
		g.playSound(soundPickupMachineGun)
	case pickupChaingun:
		g.giveWeapon(3)
		g.playSound(soundPickupChaingun)
	case pickupKey1:
		g.keys |= 1 << 0
		g.playSound(soundPickupKey)
	case pickupKey2:
		g.keys |= 1 << 1
		g.playSound(soundPickupKey)
	case pickupKey3:
		g.keys |= 1 << 2
		g.playSound(soundPickupKey)
	case pickupKey4:
		g.keys |= 1 << 3
		g.playSound(soundPickupKey)
	case pickupCross, pickupChalice, pickupBible, pickupCrown:
		score, _ := pickupTreasureValue(pickup)
		g.score += score
		g.treasureCount++
		switch pickup {
		case pickupCross:
			g.playSound(soundPickupTreasure1)
		case pickupChalice:
			g.playSound(soundPickupTreasure2)
		case pickupBible:
			g.playSound(soundPickupTreasure3)
		case pickupCrown:
			g.playSound(soundPickupTreasure4)
		}
	case pickupFullHeal:
		g.health = 100
		g.giveAmmo(25)
		_, counts := pickupTreasureValue(pickup)
		if counts {
			g.treasureCount++
		}
		g.playSound(soundPickupOneUp)
	default:
		return false
	}
	g.startBonusFlash()
	return true
}

func (g *game) spawnDroppedPickup(x, y float64, pickup pickupType) {
	def, ok := StaticDefForPickup(pickup)
	if !ok {
		return
	}
	dropX := math.Floor(x) + 0.5
	dropY := math.Floor(y) + 0.5
	g.staticSprites = append(g.staticSprites, staticSprite{
		x:        dropX,
		y:        dropY,
		shapenum: def.Shape,
		alive:    true,
		pickup:   pickup,
	})
}

func (g *game) shootAhead() {
	g.madeNoise = true
	if len(g.staticSprites) == 0 && len(g.actors) == 0 {
		return
	}

	forwardX := math.Cos(g.playerA)
	forwardY := math.Sin(g.playerA)
	planeScale := math.Tan(fov / 2)
	planeX := -forwardY * planeScale
	planeY := forwardX * planeScale
	invDet := 1.0 / (planeX*forwardY - forwardX*planeY)
	centerX := g.viewWidth / 2
	shootDelta := g.viewWidth / 10

	type shootTarget struct {
		isActor bool
		index   int
	}
	remaining := make([]shootTarget, 0, len(g.actors)+len(g.staticSprites))
	for i, actor := range g.actors {
		if actor.alive && actor.shootable {
			remaining = append(remaining, shootTarget{isActor: true, index: i})
		}
	}
	for i, spr := range g.staticSprites {
		if spr.alive && spr.shootable {
			remaining = append(remaining, shootTarget{index: i})
		}
	}

	for len(remaining) > 0 {
		bestIndex := -1
		bestDist := math.MaxFloat64
		for i, target := range remaining {
			targetX := 0.0
			targetY := 0.0
			if target.isActor {
				targetX = g.actors[target.index].x
				targetY = g.actors[target.index].y
			} else {
				targetX = g.staticSprites[target.index].x
				targetY = g.staticSprites[target.index].y
			}
			dx := targetX - g.playerX
			dy := targetY - g.playerY
			transformX := invDet * (forwardY*dx - forwardX*dy)
			transformY := invDet * (-planeY*dx + planeX*dy)
			if transformY <= 0.01 || (g.weapon == 0 && transformY > knifeRange) {
				continue
			}
			screenX := int((float64(g.viewWidth) / 2) * (1 + transformX/transformY))
			if absInt(screenX-centerX) >= shootDelta {
				continue
			}
			if transformY < bestDist {
				bestDist = transformY
				bestIndex = i
			}
		}

		if bestIndex < 0 {
			return
		}

		target := remaining[bestIndex]
		targetX := 0.0
		targetY := 0.0
		if target.isActor {
			targetX = g.actors[target.index].x
			targetY = g.actors[target.index].y
		} else {
			targetX = g.staticSprites[target.index].x
			targetY = g.staticSprites[target.index].y
		}
		dirX := targetX - g.playerX
		dirY := targetY - g.playerY
		dist := math.Hypot(dirX, dirY)
		if dist <= 0 {
			remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
			continue
		}
		dirX /= dist
		dirY /= dist
		wallDist, _, _, _, _ := g.castRay(dirX, dirY)
		if wallDist+enemyRadius < dist {
			remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
			continue
		}

		if target.isActor {
			g.damageActor(&g.actors[target.index], g.playerAttackDamage(g.playerAttackTileDistance(targetX, targetY)))
			return
		}

		spr := &g.staticSprites[target.index]
		if !g.startSpriteSequence(spr, spr.deathSequence) {
			spr.alive = false
		}
		spr.rotate = false
		spr.blocking = false
		spr.shootable = false
		g.score += spr.scoreValue
		if spr.dropPickup != pickupNone {
			g.spawnDroppedPickup(spr.x, spr.y, spr.dropPickup)
			spr.dropPickup = pickupNone
		}
		return
	}
}

func (g *game) playerAttackTileDistance(targetX, targetY float64) int {
	playerTileX := int(math.Floor(g.playerX))
	playerTileY := int(math.Floor(g.playerY))
	targetTileX := int(math.Floor(targetX))
	targetTileY := int(math.Floor(targetY))
	dx := absInt(targetTileX - playerTileX)
	dy := absInt(targetTileY - playerTileY)
	if dx > dy {
		return dx
	}
	return dy
}

func (g *game) playerAttackDamage(tileDist int) int {
	if g.rng == nil {
		g.rng = defaultRNG()
	}
	if g.weapon == 0 {
		return g.rng.Intn(256) >> 4
	}
	if tileDist < 2 {
		return g.rng.Intn(256) / 4
	}
	if tileDist < 4 {
		return g.rng.Intn(256) / 6
	}
	if (g.rng.Intn(256) / 12) < tileDist {
		return 0
	}
	return g.rng.Intn(256) / 6
}

func (g *game) isRenderableWall(x, y int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.levelWidth || y >= g.levelHeight {
		return false
	}
	if len(g.renderableWalls) != g.levelWidth*g.levelHeight {
		return false
	}
	return g.renderableWalls[y*g.levelWidth+x]
}

func (g *game) isReachableTile(x, y int) bool {
	if g.level == nil || x < 0 || y < 0 || x >= g.level.Width || y >= g.level.Height {
		return false
	}
	if len(g.reachable) != g.level.Width*g.level.Height {
		return false
	}
	return g.reachable[y*g.level.Width+x]
}

func (g *game) ensureFrame(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if width == g.viewWidth && height == g.viewHeight && len(g.frame) == width*height*4 {
		if len(g.background) == width*height*4 &&
			g.layout.renderWidth > 0 &&
			g.layout.renderHeight > 0 {
			g.rebuildLayout()
			g.ensureRenderBuffers()
			return
		}
		g.background32 = make([]uint32, width*height)
		g.background = byteViewFromU32(g.background32)
		g.prevWallTops = make([]int, width)
		g.prevWallBottoms = make([]int, width)
		g.rebuildLayout()
		g.rebuildBackground()
		g.ensureRenderBuffers()
		return
	}

	g.viewWidth = width
	g.viewHeight = height
	g.frame32 = make([]uint32, width*height)
	g.frame = byteViewFromU32(g.frame32)
	g.background32 = make([]uint32, width*height)
	g.background = byteViewFromU32(g.background32)
	g.prevWallTops = make([]int, width)
	g.prevWallBottoms = make([]int, width)
	g.rebuildLayout()
	g.rebuildBackground()
	g.ensureRenderBuffers()
}

func (g *game) rebuildLayout() {
	g.weaponOverlayScreen.valid = false
	g.weaponOverlayScaled.valid = false
	offsetX, offsetY, scaleX, scaleY := g.wolfDisplayTransform()
	renderLeft := int(math.Round(offsetX))
	renderTop := int(math.Round(offsetY))
	renderWidth := int(math.Round(wolfScreenWidth * scaleX))
	renderHeight := int(math.Round(wolfGameplayLines * scaleY))
	if renderWidth < 1 {
		renderWidth = 1
	}
	if renderHeight < 1 {
		renderHeight = 1
	}
	if renderLeft < 0 {
		renderLeft = 0
	}
	renderRight := renderLeft + renderWidth
	if renderRight > g.viewWidth {
		renderRight = g.viewWidth
	}
	if renderRight <= renderLeft {
		renderRight = minInt(g.viewWidth, renderLeft+1)
	}
	renderWidth = renderRight - renderLeft
	if renderTop < 0 {
		renderTop = 0
	}
	renderBottom := renderTop + renderHeight
	if renderBottom > g.viewHeight {
		renderBottom = g.viewHeight
	}
	if renderBottom <= renderTop {
		renderBottom = minInt(g.viewHeight, renderTop+1)
	}
	bufferWidth := renderWidth
	bufferHeight := renderBottom - renderTop
	switch g.renderMode {
	case renderModeDOS:
		bufferWidth = wolfScreenWidth
		bufferHeight = wolfGameplayLines
	case renderModeHQ:
		bufferWidth = hqGameplayWidth
		bufferHeight = hqGameplayHeight
	}
	g.layout = wolfRenderLayout{
		screenOffsetX: offsetX,
		screenOffsetY: offsetY,
		screenScaleX:  scaleX,
		screenScaleY:  scaleY,
		renderLeft:    renderLeft,
		renderRight:   renderRight,
		renderTop:     renderTop,
		renderBottom:  renderBottom,
		renderWidth:   renderWidth,
		renderHeight:  renderBottom - renderTop,
		bufferWidth:   bufferWidth,
		bufferHeight:  bufferHeight,
		statusX:       offsetX,
		statusY:       offsetY + wolfGameplayLines*scaleY,
		statusScaleX:  scaleX,
		statusScaleY:  scaleY,
	}
}

func (g *game) ensureRenderBuffers() {
	if g.renderMode == renderModeUltra {
		g.ensureUltraRenderBuffers()
		return
	}
	g.ensureScaledRenderBuffers()
}

func (g *game) rebuildCameraColumns() {
	g.ensureRenderBuffers()
}

func (g *game) ensureUltraRenderBuffers() {
	width := g.layout.bufferWidth
	height := g.layout.bufferHeight
	if width <= 0 || height <= 0 {
		g.cameraColumns = nil
		g.rayDirXColumns = nil
		g.rayDirYColumns = nil
		g.zbuffer = nil
		g.gameplayFrame32 = nil
		g.gameplayFrame = nil
		g.gameplayBackground32 = nil
		g.gameplayBackground = nil
		g.gameplayImage = nil
		return
	}
	if len(g.zbuffer) != width {
		g.zbuffer = make([]float64, width)
	}
	if len(g.wallColumns) != width {
		g.wallColumns = make([]wallColumn, width)
	}
	if len(g.columnCoverage) != width {
		g.columnCoverage = make([]columnCoverage, width)
	}
	if len(g.cameraColumns) != width {
		g.cameraColumns = make([]float64, width)
	}
	if len(g.rayDirXColumns) != width {
		g.rayDirXColumns = make([]float64, width)
	}
	if len(g.rayDirYColumns) != width {
		g.rayDirYColumns = make([]float64, width)
	}
	invWidth := 1.0 / float64(width)
	for x := 0; x < width; x++ {
		g.cameraColumns[x] = (2*(float64(x)+0.5))*invWidth - 1
	}
	if len(g.gameplayFrame32) != width*height {
		g.gameplayFrame32 = make([]uint32, width*height)
		g.gameplayFrame = byteViewFromU32(g.gameplayFrame32)
	}
	if len(g.gameplayBackground32) != width*height {
		g.gameplayBackground32 = make([]uint32, width*height)
		g.gameplayBackground = byteViewFromU32(g.gameplayBackground32)
	}
	if g.gameplayImage == nil || g.gameplayImage.Bounds().Dx() != width || g.gameplayImage.Bounds().Dy() != height {
		g.gameplayImage = ebiten.NewImage(width, height)
	}
	g.rebuildGameplayBackground()
}

func (g *game) ensureScaledRenderBuffers() {
	width := g.layout.bufferWidth
	height := g.layout.bufferHeight
	if width <= 0 || height <= 0 {
		g.cameraColumns = nil
		g.rayDirXColumns = nil
		g.rayDirYColumns = nil
		g.zbuffer = nil
		g.gameplayFrame32 = nil
		g.gameplayFrame = nil
		g.gameplayBackground32 = nil
		g.gameplayBackground = nil
		g.gameplayImage = nil
		return
	}
	if len(g.zbuffer) != width {
		g.zbuffer = make([]float64, width)
	}
	if len(g.wallColumns) != width {
		g.wallColumns = make([]wallColumn, width)
	}
	if len(g.columnCoverage) != width {
		g.columnCoverage = make([]columnCoverage, width)
	}
	if len(g.cameraColumns) != width {
		g.cameraColumns = make([]float64, width)
	}
	if len(g.rayDirXColumns) != width {
		g.rayDirXColumns = make([]float64, width)
	}
	if len(g.rayDirYColumns) != width {
		g.rayDirYColumns = make([]float64, width)
	}
	invWidth := 1.0 / float64(width)
	for x := 0; x < width; x++ {
		g.cameraColumns[x] = (2*(float64(x)+0.5))*invWidth - 1
	}
	if len(g.gameplayFrame32) != width*height {
		g.gameplayFrame32 = make([]uint32, width*height)
		g.gameplayFrame = byteViewFromU32(g.gameplayFrame32)
	}
	if len(g.gameplayBackground32) != width*height {
		g.gameplayBackground32 = make([]uint32, width*height)
		g.gameplayBackground = byteViewFromU32(g.gameplayBackground32)
	}
	if g.gameplayImage == nil || g.gameplayImage.Bounds().Dx() != width || g.gameplayImage.Bounds().Dy() != height {
		g.gameplayImage = ebiten.NewImage(width, height)
	}
	g.rebuildGameplayBackground()
}

func (g *game) rebuildBackground() {
	if g.viewWidth <= 0 || g.viewHeight <= 0 {
		return
	}
	border := g.paletteColor(g.debugBorderColor)
	for i := 0; i < len(g.background); i += 4 {
		g.background[i] = border.R
		g.background[i+1] = border.G
		g.background[i+2] = border.B
		g.background[i+3] = 255
	}
	g.ceilingColor = g.paletteColor(g.ceilingColorIndex())
	g.floorColor = g.paletteColor(0x19)
	renderLeft, renderRight, renderTop, renderBottom, renderHeight := 0, 0, 0, 0, 0
	renderLeft, renderRight, renderTop, renderBottom, _, renderHeight = g.gameplayRenderArea()
	half := renderTop + renderHeight/2

	stride := g.viewWidth * 4
	for y := renderTop; y < half; y++ {
		row := g.background[y*stride : (y+1)*stride]
		for x := renderLeft; x < renderRight; x++ {
			i := x * 4
			clr := gradientDitherColor(g.ceilingColor, x-renderLeft, y-renderTop, y-renderTop, maxInt(1, half-renderTop), 0.88, false)
			row[i] = clr.R
			row[i+1] = clr.G
			row[i+2] = clr.B
			row[i+3] = clr.A
		}
	}
	for y := half; y < renderBottom; y++ {
		row := g.background[y*stride : (y+1)*stride]
		for x := renderLeft; x < renderRight; x++ {
			i := x * 4
			clr := gradientDitherColor(g.floorColor, x-renderLeft, y-half, y-half, maxInt(1, renderBottom-half), 0.82, true)
			row[i] = clr.R
			row[i+1] = clr.G
			row[i+2] = clr.B
			row[i+3] = clr.A
		}
	}
	copy(g.frame, g.background)
	for i := range g.prevWallTops {
		g.prevWallTops[i] = 0
	}
	for i := range g.prevWallBottoms {
		g.prevWallBottoms[i] = 0
	}
	if g.backgroundImage == nil || g.backgroundImage.Bounds().Dx() != g.viewWidth || g.backgroundImage.Bounds().Dy() != g.viewHeight {
		g.backgroundImage = ebiten.NewImage(g.viewWidth, g.viewHeight)
	}
	g.backgroundImage.WritePixels(g.background)
	g.rebuildPauseShade()
}

func (g *game) rebuildGameplayBackground() {
	if g.renderMode == renderModeUltra || len(g.gameplayBackground) == 0 || g.layout.bufferWidth <= 0 || g.layout.bufferHeight <= 0 {
		return
	}
	half := g.layout.bufferHeight / 2
	stride := g.layout.bufferWidth * 4
	for y := 0; y < g.layout.bufferHeight; y++ {
		row := g.gameplayBackground[y*stride : (y+1)*stride]
		for x := 0; x < stride; x += 4 {
			pixelX := x / 4
			clr := gradientDitherColor(g.floorColor, pixelX, y-half, y-half, maxInt(1, g.layout.bufferHeight-half), 0.82, true)
			if y < half {
				clr = gradientDitherColor(g.ceilingColor, pixelX, y, y, maxInt(1, half), 0.88, false)
			}
			row[x] = clr.R
			row[x+1] = clr.G
			row[x+2] = clr.B
			row[x+3] = clr.A
		}
	}
	copy(g.gameplayFrame, g.gameplayBackground)
}

func gradientDitherColor(base color.RGBA, patternX, patternY, index, span int, horizonShade float64, reverse bool) color.RGBA {
	if span <= 1 {
		return base
	}
	t := float64(index) / float64(span-1)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	if !reverse {
		t = 1 - t
	}
	shade := horizonShade + (1-horizonShade)*t
	shade = orderedDitherShade(shade, patternX, patternY)
	return color.RGBA{
		R: uint8(float64(base.R) * shade),
		G: uint8(float64(base.G) * shade),
		B: uint8(float64(base.B) * shade),
		A: base.A,
	}
}

func orderedDitherShade(shade float64, x, y int) float64 {
	const levels = 24.0
	var bayer4 = [4][4]float64{
		{0, 8, 2, 10},
		{12, 4, 14, 6},
		{3, 11, 1, 9},
		{15, 7, 13, 5},
	}
	if shade < 0 {
		shade = 0
	} else if shade > 1 {
		shade = 1
	}
	scaled := shade * (levels - 1)
	lo := math.Floor(scaled)
	hi := math.Ceil(scaled)
	if hi <= lo {
		return lo / (levels - 1)
	}
	frac := scaled - lo
	threshold := (bayer4[y&3][x&3] + 0.5) / 16.0
	if frac >= threshold {
		return hi / (levels - 1)
	}
	return lo / (levels - 1)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth < 320 {
		outsideWidth = 320
	}
	if outsideHeight < wolfDisplayHeight {
		outsideHeight = wolfDisplayHeight
	}
	if outsideWidth != g.viewWidth || outsideHeight != g.viewHeight {
		g.ensureFrame(outsideWidth, outsideHeight)
	}
	return outsideWidth, outsideHeight
}

func (g *game) stepMap(delta int) error {
	if len(g.summaries) == 0 {
		return nil
	}

	next := g.mapIndex + delta
	if next < 0 {
		next = g.summaries[len(g.summaries)-1].Index
	}
	if next > g.summaries[len(g.summaries)-1].Index {
		next = g.summaries[0].Index
	}
	return g.setMap(next)
}

func (g *game) setMap(index int) error {
	data, err := g.files.LoadMap(index)
	if err != nil {
		return err
	}
	g.mapIndex = index
	g.mapData = data
	g.level = data.Level()
	g.levelWidth = g.level.Width
	g.levelHeight = g.level.Height
	g.cachedWallIDs = g.computeWallIDs()
	g.cachedDoorFlags = g.computeDoorFlags()
	g.reachable = g.computeReachableTiles()
	g.renderableWalls = g.computeRenderableWalls()
	g.cachedDoorSides = g.computeDoorSides()
	g.secretTotal = g.countSecretWalls()
	g.staticSprites, g.treasureTotal = g.buildStaticSprites()
	g.actors = g.buildActors()
	g.doorOpen = make([]float64, g.levelWidth*g.levelHeight)
	g.doorState = make([]byte, g.levelWidth*g.levelHeight)
	g.doorTimer = make([]int, g.levelWidth*g.levelHeight)
	g.pushWall = pushWallState{}
	g.playerAreas = g.computePlayerAreas()
	g.resetLevelState()
	g.centerMap()
	g.resetPlayer()
	g.rebuildPlayerAreas()
	g.rebuildBackground()
	g.rebuildHUDText()
	return nil
}

func (g *game) rebuildHUDText() {
	if g.mapData == nil {
		g.hudText = ""
		return
	}
	g.hudText = fmt.Sprintf(
		"MAP %d: %s  HP %d  AMMO %d  WEAPON %d  KEYS %04b  SECRET %d/%d  TREASURE %d/%d  SCORE %d\nmouse look/fire  1-4 weapon  WASD run  shift walk  F use door  Q/E alt-turn  esc pause  tab/enter/m map",
		g.mapIndex,
		g.mapData.Name(),
		g.health,
		g.ammo,
		g.weapon+1,
		g.keys,
		g.secretCount,
		g.secretTotal,
		g.treasureCount,
		g.treasureTotal,
		g.score,
	)
	if g.hudNotice != "" {
		g.hudText += "\n" + g.hudNotice
	}
}

func (g *game) countSecretWalls() int {
	if g.level == nil {
		return 0
	}
	total := 0
	for _, tile := range g.level.Tiles {
		if tile.RawInfo == pushableTile {
			total++
		}
	}
	return total
}

func (g *game) refreshLevelGeometry() {
	g.cachedWallIDs = g.computeWallIDs()
	g.reachable = g.computeReachableTiles()
	g.renderableWalls = g.computeRenderableWalls()
	g.rebuildPlayerAreas()
}

func (g *game) updateFPS() {
	now := time.Now()
	if !g.lastFPSUpdate.IsZero() && now.Sub(g.lastFPSUpdate) < time.Second {
		return
	}
	g.lastFPSUpdate = now
	g.fpsText = fmt.Sprintf("FPS %.1f", ebiten.ActualFPS())
}

func (g *game) computeReachableTiles() []bool {
	if g.level == nil {
		return nil
	}

	width := g.levelWidth
	height := g.levelHeight
	reachable := make([]bool, width*height)

	startX, startY := g.reachableStart()
	if startX < 0 || startY < 0 {
		return reachable
	}

	queue := [][2]int{{startX, startY}}
	reachable[startY*width+startX] = true
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]

		for _, d := range dirs {
			nx, ny := p[0]+d[0], p[1]+d[1]
			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}
			i := ny*width + nx
			if reachable[i] {
				continue
			}

			tile := g.level.Tile(nx, ny)
			if tile.Solid && tile.Door == nil {
				continue
			}

			reachable[i] = true
			queue = append(queue, [2]int{nx, ny})
		}
	}

	return reachable
}

func (g *game) computeRenderableWalls() []bool {
	if g.level == nil {
		return nil
	}

	width := g.levelWidth
	height := g.levelHeight
	renderable := make([]bool, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			tile := g.level.Tile(x, y)
			if !tile.RenderWall {
				continue
			}

			if x == 0 || y == 0 || x == width-1 || y == height-1 {
				renderable[y*width+x] = true
				continue
			}

			if g.isReachableTile(x-1, y) || g.isReachableTile(x+1, y) || g.isReachableTile(x, y-1) || g.isReachableTile(x, y+1) {
				renderable[y*width+x] = true
			}
		}
	}

	return renderable
}

func (g *game) computeWallIDs() []uint16 {
	if g.level == nil {
		return nil
	}
	wallIDs := make([]uint16, g.levelWidth*g.levelHeight)
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			wallIDs[y*g.levelWidth+x] = renderWallTile(g.level.Tile(x, y))
		}
	}
	return wallIDs
}

func (g *game) computeDoorFlags() []byte {
	if g.level == nil {
		return nil
	}
	flags := make([]byte, g.levelWidth*g.levelHeight)
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			tile := g.level.Tile(x, y)
			if tile.Door == nil {
				continue
			}
			if tile.Door.Vertical {
				flags[y*g.levelWidth+x] = 2
			} else {
				flags[y*g.levelWidth+x] = 1
			}
		}
	}
	return flags
}

func (g *game) computeDoorSides() []byte {
	if g.level == nil {
		return nil
	}
	sides := make([]byte, g.levelWidth*g.levelHeight)
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			i := y*g.levelWidth + x
			if x > 0 && g.cachedDoorFlags[i-1] != 0 {
				sides[i] |= 1
			}
			if x+1 < g.levelWidth && g.cachedDoorFlags[i+1] != 0 {
				sides[i] |= 2
			}
			if y > 0 && g.cachedDoorFlags[i-g.levelWidth] != 0 {
				sides[i] |= 4
			}
			if y+1 < g.levelHeight && g.cachedDoorFlags[i+g.levelWidth] != 0 {
				sides[i] |= 8
			}
		}
	}
	return sides
}

func (g *game) computePlayerAreas() []bool {
	if g.level == nil {
		return nil
	}
	maxArea := -1
	for _, tile := range g.level.Tiles {
		if tile.Area > maxArea {
			maxArea = tile.Area
		}
	}
	if maxArea < 0 {
		return nil
	}
	return make([]bool, maxArea+1)
}

func (g *game) rebuildPlayerAreas() {
	if len(g.playerAreas) == 0 {
		return
	}
	for i := range g.playerAreas {
		g.playerAreas[i] = false
	}
	startArea := g.playerArea()
	if startArea < 0 || startArea >= len(g.playerAreas) {
		return
	}
	queue := []int{startArea}
	g.playerAreas[startArea] = true
	for len(queue) > 0 {
		area := queue[0]
		queue = queue[1:]
		for y := 0; y < g.levelHeight; y++ {
			for x := 0; x < g.levelWidth; x++ {
				tile := g.level.Tile(x, y)
				if tile.Door == nil {
					continue
				}
				i := y*g.levelWidth + x
				if g.doorState[i] == 0 {
					continue
				}
				a, b, ok := g.doorAreas(x, y, tile.Door.Vertical)
				if !ok {
					continue
				}
				if a == area && b >= 0 && b < len(g.playerAreas) && !g.playerAreas[b] {
					g.playerAreas[b] = true
					queue = append(queue, b)
				}
				if b == area && a >= 0 && a < len(g.playerAreas) && !g.playerAreas[a] {
					g.playerAreas[a] = true
					queue = append(queue, a)
				}
			}
		}
	}
}

func (g *game) doorAreas(x, y int, vertical bool) (int, int, bool) {
	if g.level == nil {
		return -1, -1, false
	}
	if vertical {
		left := g.actorAreaAt(x-1, y)
		right := g.actorAreaAt(x+1, y)
		if left >= 0 && right >= 0 {
			return left, right, true
		}
		return -1, -1, false
	}
	top := g.actorAreaAt(x, y-1)
	bottom := g.actorAreaAt(x, y+1)
	if top >= 0 && bottom >= 0 {
		return top, bottom, true
	}
	return -1, -1, false
}

func (g *game) rebuildPauseShade() {
	if g.viewWidth <= 0 || g.viewHeight <= 0 {
		g.pauseShade = nil
		return
	}
	img := ebiten.NewImage(g.viewWidth, g.viewHeight)
	img.Fill(color.RGBA{A: 72})
	g.pauseShade = img
}

func (g *game) reachableStart() (int, int) {
	if g.level == nil {
		return -1, -1
	}
	if len(g.level.PlayerStarts) > 0 {
		start := g.level.PlayerStarts[0]
		return start.X, start.Y
	}

	x, y, _ := g.findSpawn()
	return int(x), int(y)
}

func (g *game) centerMap() {
	if g.mapData == nil {
		g.cameraX = 0
		g.cameraY = 0
		return
	}
	g.cameraX = float64(g.mapData.Width()) / 2
	g.cameraY = float64(g.mapData.Height()) / 2
}

func (g *game) centerMapOnPlayer() {
	if g.mapData == nil {
		g.cameraX = 0
		g.cameraY = 0
		return
	}
	g.cameraX = g.playerX
	g.cameraY = g.playerY
}

func (g *game) resetPlayer() {
	x, y, a := g.findSpawn()
	g.playerX = x
	g.playerY = y
	g.playerA = a
}

func (g *game) findSpawn() (float64, float64, float64) {
	if g.mapData == nil {
		return 1.5, 1.5, 0
	}

	width := g.mapData.Width()
	height := g.mapData.Height()

	if len(g.level.PlayerStarts) > 0 {
		start := g.level.PlayerStarts[0]
		return float64(start.X) + 0.5, float64(start.Y) + 0.5, directionAngle(start.Direction)
	}

	// Start from the map center and search outward for a walkable cell with
	// some breathing room so movement is immediately visible in raycast mode.
	centerX := width / 2
	centerY := height / 2
	maxRadius := maxInt(width, height)
	for radius := 0; radius < maxRadius; radius++ {
		for y := maxInt(1, centerY-radius); y <= minInt(height-2, centerY+radius); y++ {
			for x := maxInt(1, centerX-radius); x <= minInt(width-2, centerX+radius); x++ {
				if !g.isWalkableSpawn(x, y) {
					continue
				}
				return float64(x) + 0.5, float64(y) + 0.5, 0
			}
		}
	}

	return 1.5, 1.5, 0
}

func (g *game) isWalkableSpawn(x, y int) bool {
	if g.level.Tile(x, y).Solid {
		return false
	}
	for _, spr := range g.staticSprites {
		if spr.alive && spr.blocking && int(spr.x) == x && int(spr.y) == y {
			return false
		}
	}
	for _, actor := range g.actors {
		if actor.alive && actor.blocking && actor.tileX == x && actor.tileY == y {
			return false
		}
	}

	// Require a small clear neighborhood so the player doesn't begin wedged
	// against a wall or object marker.
	for yy := y - 1; yy <= y+1; yy++ {
		for xx := x - 1; xx <= x+1; xx++ {
			if g.level.Tile(xx, yy).Solid {
				return false
			}
		}
	}

	return true
}

func directionAngle(dir wl6.Direction) float64 {
	switch dir {
	case wl6.North:
		return -math.Pi / 2
	case wl6.East:
		return 0
	case wl6.South:
		return math.Pi / 2
	case wl6.West:
		return math.Pi
	default:
		return 0
	}
}

func tileColors(wall, obj uint16) (color.Color, color.Color) {
	base := color.RGBA{R: 34, G: 40, B: 46, A: 255}
	stroke := color.RGBA{R: 24, G: 28, B: 32, A: 255}

	switch {
	case wall == 0:
		base = color.RGBA{R: 222, G: 214, B: 182, A: 255}
		stroke = color.RGBA{R: 196, G: 182, B: 143, A: 255}
	case wall >= 1 && wall <= 63:
		base = hueBandColor(wall, 0.52, 0.45)
		stroke = hueBandColor(wall, 0.58, 0.22)
	default:
		base = hueBandColor(wall, 0.42, 0.55)
		stroke = hueBandColor(wall, 0.48, 0.26)
	}

	if obj != 0 {
		base = blend(base, color.RGBA{R: 225, G: 88, B: 41, A: 255}, 0.35)
		stroke = color.RGBA{R: 104, G: 28, B: 10, A: 255}
	}

	return base, stroke
}

func hueBandColor(id uint16, saturation, value float64) color.RGBA {
	h := math.Mod(float64(id)*37.0, 360.0)
	return hsvToRGB(h, saturation, value)
}

func hsvToRGB(h, s, v float64) color.RGBA {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := v - c

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return color.RGBA{
		R: uint8((r + m) * 255),
		G: uint8((g + m) * 255),
		B: uint8((b + m) * 255),
		A: 255,
	}
}

func blend(a color.Color, b color.Color, alpha float64) color.RGBA {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()

	mix := func(x, y uint32) uint8 {
		return uint8((1-alpha)*float64(x>>8) + alpha*float64(y>>8))
	}

	return color.RGBA{
		R: mix(ar, br),
		G: mix(ag, bg),
		B: mix(ab, bb),
		A: 255,
	}
}

func isJustPressed(keys ...ebiten.Key) bool {
	for _, key := range keys {
		if inpututil.IsKeyJustPressed(key) {
			return true
		}
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func percentInt(count, total int) int {
	if total <= 0 {
		if count > 0 {
			return 100
		}
		return 0
	}
	if count <= 0 {
		return 0
	}
	if count >= total {
		return 100
	}
	return (count * 100) / total
}

const huge = 1e30
