package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"gd-wolf/internal/wl6"
)

type wolfDemoRuntimeActor struct {
	PoolSlot  int   `json:"pool_slot"`
	Kind      int   `json:"kind"`
	X         int64 `json:"x"`
	Y         int64 `json:"y"`
	TileX     int   `json:"tile_x"`
	TileY     int   `json:"tile_y"`
	Dir       int   `json:"dir"`
	Area      int   `json:"area"`
	Distance  int64 `json:"distance"`
	Reaction  int   `json:"reaction"`
	Health    int   `json:"health"`
	Flags     int   `json:"flags"`
	Shape     int   `json:"shape"`
	TicCount  int   `json:"tic_count"`
	Active    bool  `json:"active"`
	State     int   `json:"state"`
	FrameTics int   `json:"frame_tics"`
	Angle     int   `json:"angle"`
	Speed     int   `json:"speed"`
}

type wolfDemoRuntimeState struct {
	Statics       []wolfDemoRuntimeStatic `json:"statics,omitempty"`
	MemoryProfile string                  `json:"memory_profile"`
	Area255Word   int                     `json:"area_255_word"`
	AreaPlane     []uint16                `json:"area_plane,omitempty"`
	ActorAt       []int                   `json:"actorat,omitempty"`
	RNGIndex      int                     `json:"rng_index"`
	Damage        int                     `json:"damage"`
	Actors        []wolfDemoRuntimeActor  `json:"actors"`
	Doors         []wolfDemoRuntimeDoor   `json:"doors"`
	Weapon        wolfDemoRuntimeWeapon   `json:"weapon"`
	Walls         []int                   `json:"walls"`
	Player        wolfDemoPlayerState     `json:"player"`
	Stats         wolfDemoRuntimeStats    `json:"stats"`
	Face          wolfDemoRuntimeFace     `json:"face"`
	PushWall      wolfDemoRuntimePushWall `json:"pushwall"`
	Areas         [64]bool                `json:"areas"`
	Sound         wolfDemoRuntimeSound    `json:"sound"`
	Victory       wolfDemoRuntimeVictory  `json:"victory"`
	PlayerState   int                     `json:"player_state"`
	Terminal      int                     `json:"terminal"`
}

type wolfDemoRuntimeVictory struct {
	Active bool                          `json:"active"`
	Actors []wolfDemoRuntimeVictoryActor `json:"actors"`
}

type wolfDemoRuntimeVictoryActor struct {
	X              int64 `json:"x"`
	Y              int64 `json:"y"`
	TileX          int   `json:"tile_x"`
	TileY          int   `json:"tile_y"`
	Dir            int   `json:"dir"`
	Distance       int64 `json:"distance"`
	RemainingTiles int   `json:"remaining_tiles"`
	Shape          int   `json:"shape"`
	TicCount       int   `json:"tic_count"`
	FrameTics      int   `json:"frame_tics"`
	State          int   `json:"state"`
	ActorActive    bool  `json:"actor_active"`
}

func captureDemoRuntimeVictory(g *game) wolfDemoRuntimeVictory {
	v := wolfDemoRuntimeVictory{Active: g.victoryActive, Actors: []wolfDemoRuntimeVictoryActor{}}
	for _, a := range g.actors {
		if a.kind != actorKindVictoryBJ || a.removed {
			continue
		}
		seq, _ := LookupAnimSequence(a.sequenceID)
		tics := seq.Frames[a.frameIndex].Tics
		count := tics - a.frameTimer
		if a.spawnAnimationFrozen {
			count = 0
		}
		v.Actors = append(v.Actors, wolfDemoRuntimeVictoryActor{
			X: int64(math.Round(a.x * 65536)), Y: int64(math.Round(a.y * 65536)),
			TileX: a.tileX, TileY: a.tileY, Dir: a.dir, Distance: int64(math.Round(a.moveDistance * 65536)),
			RemainingTiles: a.reactionTimer, Shape: a.shapenum, TicCount: count, FrameTics: tics,
			State: int(a.aiState), ActorActive: a.demoActive,
		})
	}
	return v
}

func captureDemoRuntimePlayerState(g *game) int {
	if g.demoPlayback.deathCam {
		return 2
	}
	return boolInt(g.attacking)
}

func compareDemoActorGrid(t *testing.T, command int, want, got []int) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("command %d actorat size: original=%d port=%d", command, len(want), len(got))
	}
	for i, tag := range want {
		if tag != got[i] {
			t.Fatalf("command %d actorat (%d,%d): original=%d port=%d", command, i%64, i/64, tag, got[i])
		}
	}
}

func compareDemoAreaPlane(t *testing.T, command int, want, got []uint16) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("command %d area-plane size: original=%d port=%d", command, len(want), len(got))
	}
	for i, word := range want {
		if word != got[i] {
			t.Fatalf("command %d area-plane (%d,%d): original=%d port=%d", command, i%64, i/64, word, got[i])
		}
	}
}

type wolfDemoRuntimeSound struct {
	Playing   int `json:"playing"`
	Priority  int `json:"priority"`
	Remaining int `json:"remaining"`
}

func captureDemoRuntimeSound(g *game) wolfDemoRuntimeSound {
	if s := g.demoPlayback.sound; s != nil {
		return wolfDemoRuntimeSound{s.playingSound(), s.priority, s.remaining}
	}
	return wolfDemoRuntimeSound{}
}

func configureWolfDemoReferenceSound(t *testing.T, files *wl6.Files, mode string, mapIndex int, dir string) {
	t.Helper()
	t.Setenv("GDWOLF_DEMO_SOUND_MODE", mode)
	t.Setenv("GDWOLF_DEMO_REGISTERED", strconv.Itoa(boolInt(files.Variant.EpisodeCount > 1)))
	t.Setenv("GDWOLF_DEMO_MAP_INDEX", strconv.Itoa(mapIndex))
	for _, entry := range []struct {
		env, name string
		data      []byte
	}{
		{"GDWOLF_DEMO_AUDIO_HEAD", "reference-AUDIOHED.bin", files.AudioHead},
		{"GDWOLF_DEMO_AUDIO_DATA", "reference-AUDIOT.bin", files.AudioT},
	} {
		path := filepath.Join(dir, entry.name)
		if err := os.WriteFile(path, entry.data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(entry.env, path)
	}
}

type wolfDemoRuntimeStats struct {
	Health        int `json:"health"`
	Ammo          int `json:"ammo"`
	Lives         int `json:"lives"`
	Keys          int `json:"keys"`
	Score         int `json:"score"`
	NextExtra     int `json:"next_extra"`
	Weapon        int `json:"weapon"`
	BestWeapon    int `json:"best_weapon"`
	ChosenWeapon  int `json:"chosen_weapon"`
	Secrets       int `json:"secrets"`
	SecretTotal   int `json:"secret_total"`
	Treasure      int `json:"treasure"`
	TreasureTotal int `json:"treasure_total"`
	Kills         int `json:"kills"`
	KillTotal     int `json:"kill_total"`
	TimeCount     int `json:"time_count"`
}

type wolfDemoRuntimeFace struct {
	Frame int `json:"frame"`
	Timer int `json:"timer"`
}

type wolfDemoRuntimePushWall struct {
	Active   bool `json:"active"`
	X        int  `json:"x"`
	Y        int  `json:"y"`
	Dir      int  `json:"dir"`
	State    int  `json:"state"`
	Position int  `json:"position"`
}

func captureDemoRuntimeStats(g *game) wolfDemoRuntimeStats {
	s := wolfDemoRuntimeStats{
		Health: g.health, Ammo: g.ammo, Lives: g.lives, Keys: int(g.keys), Score: g.score,
		NextExtra: g.nextExtra, Weapon: g.weapon, BestWeapon: g.bestWeapon, ChosenWeapon: g.chosenWeapon,
		Secrets: g.secretCount, SecretTotal: g.secretTotal, Treasure: g.treasureCount,
		TreasureTotal: g.treasureTotal, TimeCount: g.demoPlayback.timeCount,
	}
	for _, actor := range g.actors {
		if !isEnemyActorKind(actor.kind) {
			continue
		}
		if actor.kind != actorKindHitler {
			s.KillTotal++
		}
		if !actor.alive {
			s.Kills++
		}
	}
	return s
}

type wolfDemoRuntimeWeapon struct {
	Type      int  `json:"type"`
	Attacking bool `json:"attacking"`
	Frame     int  `json:"frame"`
	Timer     int  `json:"timer"`
	Ammo      int  `json:"ammo"`
	Shots     int  `json:"shots"`
}

type wolfDemoRuntimeDoor struct {
	Action   int `json:"action"`
	Position int `json:"position"`
	Timer    int `json:"timer"`
}

type wolfDemoRuntimeBonus struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Shape int `json:"shape"`
}

func captureDemoRuntimeBonuses(g *game) []wolfDemoRuntimeBonus {
	bonuses := []wolfDemoRuntimeBonus{}
	g.initializeDemoStaticSlots()
	for slot, index := range g.demoPlayback.staticSlots {
		sprite := g.staticSprites[index]
		if sprite.alive && g.demoPlayback.staticInfo[slot].flags&2 != 0 {
			bonuses = append(bonuses, wolfDemoRuntimeBonus{int(sprite.x), int(sprite.y), sprite.shapenum})
		}
	}
	sort.Slice(bonuses, func(i, j int) bool {
		a, b := bonuses[i], bonuses[j]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Shape < b.Shape
	})
	return bonuses
}

func captureDemoRuntimeState(g *game) wolfDemoRuntimeState {
	g.ensureDemoActorPool()
	s := wolfDemoRuntimeState{RNGIndex: int(g.rng.index), Actors: []wolfDemoRuntimeActor{}, Doors: []wolfDemoRuntimeDoor{}}
	if g.demoPlayback.memory == nil {
		if err := g.configureDemoMemory(os.Getenv("GDWOLF_DEMO_MEMORY_PROFILE")); err != nil {
			panic(err)
		}
	}
	s.MemoryProfile, s.Area255Word = g.demoMemory().profile, g.demoMemory().area255Word()
	s.Statics = captureDemoRuntimeStatics(g)
	if os.Getenv("GDWOLF_DEMO_OCCUPANCY") == "1" {
		s.ActorAt = append([]int(nil), g.demoActorPool.tags...)
	}
	if os.Getenv("GDWOLF_DEMO_AREA_PLANE") == "1" {
		s.AreaPlane = append([]uint16(nil), g.demoPlayback.areaPlane...)
	}
	s.Player = captureDemoPlayer(g)
	s.Stats = captureDemoRuntimeStats(g)
	s.Face = wolfDemoRuntimeFace{g.demoPlayback.faceFrame, g.demoPlayback.faceCount}
	s.Sound = captureDemoRuntimeSound(g)
	s.Victory = captureDemoRuntimeVictory(g)
	s.PlayerState = boolInt(g.attacking)
	if g.demoPlayback.deathCam {
		s.PlayerState = 2
	}
	s.Terminal = g.demoPlayback.levelExit
	if g.playerDying {
		s.Terminal = 2
	}
	copy(s.Areas[:], g.playerAreas)
	if p := g.pushWall; p.active {
		dir := 0
		switch {
		case p.dx > 0:
			dir = 1
		case p.dy > 0:
			dir = 2
		case p.dx < 0:
			dir = 3
		}
		state := 1 + p.steps*pushWallStepTics + p.tics
		s.PushWall = wolfDemoRuntimePushWall{true, p.x, p.y, dir, state, (state / 2) & 63}
	}
	s.Weapon = wolfDemoRuntimeWeapon{g.weapon, g.attacking, g.weaponFrameIdx, g.weaponFrameTics, g.ammo, g.demoPlayback.shots}
	if g.demoPlayback.deathCam {
		s.Weapon.Timer = 0
	}
	for _, a := range g.actors {
		if a.removed || a.kind == actorKindVictoryBJ {
			continue
		}
		flags := boolInt(a.shootable) | boolInt(a.alerted)*16 | boolInt(a.firstAttack)*32 | boolInt(a.ambush)*64 | wolfActorMarkFlags(&a)
		seq, _ := LookupAnimSequence(a.sequenceID)
		count := g.demoActorFrameTics(&a, seq.Frames[a.frameIndex]) - a.frameTimer
		if a.spawnAnimationFrozen {
			count = 0
		}
		distance := int64(math.Round(a.moveDistance * 65536))
		if a.moveDistance < 0 {
			// Negative distance is a door-number sentinel, not fixed-point movement.
			distance = int64(a.moveDistance)
		}
		s.Actors = append(s.Actors, wolfDemoRuntimeActor{
			PoolSlot: a.poolSlot, Kind: int(a.kind), X: int64(math.Round(a.x * 65536)), Y: int64(math.Round(a.y * 65536)),
			TileX: a.tileX, TileY: a.tileY, Dir: a.dir, Area: a.area, Distance: distance,
			Reaction: a.reactionTimer, Health: a.health, Flags: flags, Shape: a.shapenum,
			TicCount: count, Active: a.demoActive, State: int(a.aiState), FrameTics: g.demoActorFrameTics(&a, seq.Frames[a.frameIndex]),
			Angle: a.angle, Speed: wolfActorOriginalSpeed(&a),
		})
	}
	for i := range g.level.Tiles {
		if g.doorDefinitionAt(i%g.levelWidth, i/g.levelWidth) == nil {
			continue
		}
		action := []int{1, 2, 0, 3}[g.doorState[i]]
		timer := 0
		if action == 0 {
			timer = g.doorTimer[i]
		}
		s.Doors = append(s.Doors, wolfDemoRuntimeDoor{action, int(math.Round(g.doorOpen[i] * 65535)), timer})
	}
	for y := 0; y < g.levelHeight; y++ {
		for x := 0; x < g.levelWidth; x++ {
			s.Walls = append(s.Walls, boolInt(g.demoHasRayWall(x, y)))
		}
	}
	return s
}

func TestWolfDemoRuntimeCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_runtime_compare.sh")
	}
	dataDir := os.Getenv("GDWOLF_DEMO_RUNTIME_DATA")
	var files *wl6.Files
	var err error
	if dataDir == "" {
		files, err = wl6.OpenEmbeddedShareware()
	} else {
		files, err = wl6.Open(dataDir)
	}
	if err != nil {
		t.Fatal(err)
	}
	demoIndex := 0
	if raw := os.Getenv("GDWOLF_DEMO_INDEX"); raw != "" {
		demoIndex, err = strconv.Atoi(raw)
		if err != nil || demoIndex < 0 || demoIndex > 3 {
			t.Fatalf("invalid demo index %q: expected 0-3", raw)
		}
	}
	demoFile := os.Getenv("GDWOLF_DEMO_FILE")
	demo, err := loadWolfDemoRuntimeInput(files, demoIndex, demoFile)
	if err != nil {
		t.Fatal(err)
	}
	if demoFile != "" {
		demoIndex = -1
	}
	extraFire := os.Getenv("GDWOLF_DEMO_EXTRA_FIRE") == "1"
	if extraFire {
		// Keep the opening route to reach its first fight, then preserve
		// steering and use inputs while changing the attack schedule.
		for i := range demo.Commands {
			if i < 200 {
				continue
			}
			if i%20 < 12 {
				demo.Commands[i].Buttons |= demoButtonAttack
			} else {
				demo.Commands[i].Buttons &^= demoButtonAttack
			}
		}
	}
	data, err := files.LoadMap(demo.Map)
	if err != nil {
		t.Fatal(err)
	}
	g, err := buildEnemyAIFuzzBaseline(files, demo.Map)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.startDemo(demo); err != nil {
		t.Fatal(err)
	}
	if err := g.configureDemoMemory(os.Getenv("GDWOLF_DEMO_MEMORY_PROFILE")); err != nil {
		t.Fatal(err)
	}
	soundMode := os.Getenv("GDWOLF_DEMO_SOUND_MODE")
	if soundMode == "" {
		soundMode = "adlib-digi"
	}
	g.demoPlayback.sound, err = newWolfDemoSound(files, soundMode)
	if err != nil {
		t.Fatal(err)
	}
	independentDoors, independentWorld := true, true
	out := os.Getenv("GDWOLF_DEMO_RUNTIME_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	configureWolfDemoReferenceSound(t, files, soundMode, demo.Map, out)
	t.Setenv("GDWOLF_DEMO_COMMAND_COUNT", strconv.Itoa(len(demo.Commands)))
	p := startWolfSourceBinary(t, path)
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		})
		return f
	}
	p.in = bufio.NewWriter(io.MultiWriter(p.input, open("reference-input.txt")))
	referenceTrace, portTrace := json.NewEncoder(open("reference.jsonl")), json.NewEncoder(open("port.jsonl"))
	renderTrace := json.NewEncoder(open("reference-render.jsonl"))
	portRenderTrace := json.NewEncoder(open("port-render.jsonl"))
	result := json.NewEncoder(open("result.json"))
	matched, status := 0, "mismatch"
	attemptedCommand := -1
	maxAlerted, kills := 0, 0
	terminal := ""
	defer func() {
		var mismatchCommand, mismatchTic any
		if status == "mismatch" {
			mismatchCommand, mismatchTic = attemptedCommand, (attemptedCommand+1)*wl6.DemoTics
		}
		var executableHash any
		if g.demoMemory().profile == wolfDemoRegisteredMemoryProfile {
			executableHash = wolfDemoRegisteredExecutableSHA256
		}
		memoryProfile := map[string]any{"name": g.demoMemory().profile, "executable_sha256": executableHash}
		if err := result.Encode(map[string]any{"demo_index": demoIndex, "demo_file": demoFile, "map": demo.Map, "sound_mode": soundMode, "memory_profile": memoryProfile, "registered": files.Variant.EpisodeCount > 1, "extra_fire": extraFire, "demo_commands": len(demo.Commands), "matched_commands": matched, "matched_tics": matched * wl6.DemoTics, "remaining_commands": len(demo.Commands) - matched, "mismatch_command": mismatchCommand, "mismatch_tic": mismatchTic, "max_simultaneous_alerted": maxAlerted, "kills": kills, "status": status, "terminal": terminal}); err != nil {
			t.Error(err)
		}
	}()
	if err := writeDemoStartMap(p.in, data); err != nil {
		t.Fatal(err)
	}
	if err := p.in.Flush(); err != nil {
		t.Fatal(err)
	}
	read := func() wolfDemoRuntimeState {
		if !p.out.Scan() {
			t.Fatalf("original actor reference returned no state: %v", p.out.Err())
		}
		var s wolfDemoRuntimeState
		if err := json.Unmarshal(p.out.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	compare := func(command int, want, got wolfDemoRuntimeState) {
		compareDemoStatics(t, command, want.Statics, got.Statics)
		if want.MemoryProfile != got.MemoryProfile || want.Area255Word != got.Area255Word {
			t.Errorf("command %d DOS memory profile: original=%s/%d port=%s/%d", command, want.MemoryProfile, want.Area255Word, got.MemoryProfile, got.Area255Word)
		}
		if err := referenceTrace.Encode(map[string]any{"command": command, "tic": (command + 1) * wl6.DemoTics, "state": want}); err != nil {
			t.Fatal(err)
		}
		if err := portTrace.Encode(map[string]any{"command": command, "tic": (command + 1) * wl6.DemoTics, "state": got}); err != nil {
			t.Fatal(err)
		}
		compareDemoActorGrid(t, command, want.ActorAt, got.ActorAt)
		compareDemoAreaPlane(t, command, want.AreaPlane, got.AreaPlane)
		if want.Weapon != got.Weapon {
			t.Errorf("command %d weapon: original=%+v port=%+v", command, want.Weapon, got.Weapon)
		}
		if want.RNGIndex != got.RNGIndex {
			t.Errorf("command %d RNG: original=%d port=%d", command, want.RNGIndex, got.RNGIndex)
		}
		if want.Stats != got.Stats {
			t.Errorf("command %d stats: original=%+v port=%+v", command, want.Stats, got.Stats)
		}
		if want.Face != got.Face {
			t.Errorf("command %d face: original=%+v port=%+v", command, want.Face, got.Face)
		}
		if want.Sound != got.Sound {
			t.Errorf("command %d synthesized sound: original=%+v port=%+v", command, want.Sound, got.Sound)
		}
		if !reflect.DeepEqual(want.Victory, got.Victory) || want.PlayerState != got.PlayerState || want.Terminal != got.Terminal {
			t.Errorf("command %d victory/player-state/terminal: original=%+v/%d/%d port=%+v/%d/%d", command, want.Victory, want.PlayerState, want.Terminal, got.Victory, got.PlayerState, got.Terminal)
		}
		if want.PushWall != got.PushWall {
			t.Errorf("command %d pushwall: original=%+v port=%+v", command, want.PushWall, got.PushWall)
		}
		if want.Areas != got.Areas {
			t.Errorf("command %d connected areas: original=%v port=%v", command, want.Areas, got.Areas)
		}
		if len(want.Actors) != len(got.Actors) {
			t.Fatalf("actor count differs: original=%d port=%d", len(want.Actors), len(got.Actors))
		}
		for i := range want.Actors {
			if want.Actors[i] != got.Actors[i] {
				t.Errorf("command %d actor %d: original=%+v port=%+v", command, i, want.Actors[i], got.Actors[i])
			}
		}
		if independentDoors {
			if len(want.Doors) != len(got.Doors) {
				t.Fatal("door count differs")
			}
			for i := range want.Doors {
				if want.Doors[i] != got.Doors[i] {
					t.Errorf("command %d door %d: original=%+v port=%+v", command, i, want.Doors[i], got.Doors[i])
				}
			}
		}
		if independentWorld {
			if want.Player != got.Player {
				t.Errorf("command %d player: original=%+v port=%+v", command, want.Player, got.Player)
			}
			if len(want.Walls) != len(got.Walls) {
				t.Fatal("wall count differs")
			}
			for i := range want.Walls {
				if want.Walls[i] != got.Walls[i] {
					t.Errorf("command %d wall (%d,%d): original=%d port=%d", command, i%64, i/64, want.Walls[i], got.Walls[i])
				}
			}
		}
		if t.Failed() {
			t.FailNow()
		}
	}
	compare(-1, read(), captureDemoRuntimeState(g))
	previousTiles := append([]wl6.Tile(nil), g.level.Tiles...)
	for i, command := range demo.Commands {
		attemptedCommand = i
		if g.playerDying || g.demoPlayback.levelExit != 0 {
			status = "unsupported_terminal_state"
			t.Fatalf("port stopped before command %d", i)
		}
		useDoor := -1
		pushX, pushY, pushDir := 0, 0, -1
		if !g.attacking && command.Buttons&demoButtonUse != 0 && g.demoPlayback.buttons&demoButtonUse == 0 {
			x, y := g.cardinalUseTile()
			if g.level.Tile(x, y).RawInfo == pushableTile {
				pushX, pushY = x, y
				dx, dy := g.cardinalUseVector()
				switch {
				case dx > 0:
					pushDir = 1
				case dx < 0:
					pushDir = 3
				case dy < 0:
					pushDir = 0
				default:
					pushDir = 2
				}
			}
			index := 0
			for tileIndex, tile := range g.level.Tiles {
				if tile.Door == nil {
					continue
				}
				if tileIndex == y*g.levelWidth+x {
					useDoor = index
					break
				}
				index++
			}
		}
		wasAttacking := g.prepareDemoCommand(command)
		g.moveDemoPlayer(command)
		entryRNG, weapon, ammo, chosenWeapon := g.rng.index, g.weapon, g.ammo, g.chosenWeapon
		if wasAttacking && !g.victoryActive {
			g.updateWeaponAttackWithInput(wl6.DemoTics, g.demoPlayback.inputButtons&demoButtonAttack != 0)
		}
		g.rebuildPlayerAreas()
		if _, err := fmt.Fprintf(p.in, "%d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d", int64(math.Round(g.playerX*65536)), int64(math.Round(g.playerY*65536)), entryRNG, boolInt(g.madeNoise), boolInt(g.playerMovingFast), g.bestWeapon, weapon, g.demoPlayback.shots, useDoor, command.Buttons, ammo, chosenWeapon, pushX, pushY, pushDir, command.ControlX, command.ControlY); err != nil {
			t.Fatal(err)
		}
		for area := 0; area < 64; area++ {
			connected := area < len(g.playerAreas) && g.playerAreas[area]
			if _, err := fmt.Fprintf(p.in, " %d", boolInt(connected)); err != nil {
				t.Fatal(err)
			}
		}
		// Changed wall tiles are diagnostic input; the original reference
		// evolves its own pushwalls and actor/static reservations.
		changed := []int{}
		for index, tile := range g.level.Tiles {
			if tile.Solid != previousTiles[index].Solid || tile.RawWall != previousTiles[index].RawWall || tile.Area != previousTiles[index].Area {
				changed = append(changed, index)
			}
		}
		if _, err := fmt.Fprintf(p.in, " %d", len(changed)); err != nil {
			t.Fatal(err)
		}
		for _, index := range changed {
			tile := g.level.Tiles[index]
			wall := 0
			if tile.Solid && tile.Door == nil {
				wall = int(tile.RawWall)
			}
			if _, err := fmt.Fprintf(p.in, " %d %d %d", index, wall, tile.Area); err != nil {
				t.Fatal(err)
			}
			previousTiles[index] = tile
		}
		for tileIndex := range g.level.Tiles {
			if g.doorDefinitionAt(tileIndex%g.levelWidth, tileIndex/g.levelWidth) == nil {
				continue
			}
			action := []int{1, 2, 0, 3}[g.doorState[tileIndex]]
			if _, err := fmt.Fprintf(p.in, " %d %d", action, int(math.Round(g.doorOpen[tileIndex]*65535))); err != nil {
				t.Fatal(err)
			}
		}
		for index := range g.actors {
			if g.actors[index].removed || g.actors[index].kind == actorKindVictoryBJ {
				continue
			}
			projection := demoActorProjection{}
			if index < len(g.demoPlayback.projections) {
				projection = g.demoPlayback.projections[index]
			}
			if _, err := fmt.Fprintf(p.in, " %d %d %d", boolInt(projection.visible), projection.viewX, projection.transX); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := fmt.Fprintln(p.in); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		want := read()
		beforeHealth := g.health
		g.updateDemoActors(wl6.DemoTics)
		got := captureDemoRuntimeState(g)
		alerted := 0
		for _, actor := range g.actors {
			if actor.alive && actor.alerted {
				alerted++
			}
		}
		if alerted > maxAlerted {
			maxAlerted = alerted
		}

		compare(i, want, got)
		if gotDamage := beforeHealth - g.health; gotDamage != minInt(beforeHealth, want.Damage) {
			t.Fatalf("command %d enemy damage: original=%d port=%d", i, want.Damage, gotDamage)
		}
		g.refreshDemoActorProjections()
		if _, err := fmt.Fprintf(p.in, "%d", g.demoPlayback.angle); err != nil {
			t.Fatal(err)
		}
		for _, visible := range g.demoPlayback.visibleTiles {
			if _, err := fmt.Fprintf(p.in, " %d", boolInt(visible)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := fmt.Fprintln(p.in); err != nil {
			t.Fatal(err)
		}
		if err := p.in.Flush(); err != nil {
			t.Fatal(err)
		}
		if !p.out.Scan() {
			t.Fatal("original renderer returned no projections")
		}
		var rendered struct {
			Statics     []wolfDemoRuntimeStatic `json:"statics,omitempty"`
			Stats       wolfDemoRuntimeStats    `json:"stats"`
			Face        wolfDemoRuntimeFace     `json:"face"`
			Health      int                     `json:"health"`
			Ammo        int                     `json:"ammo"`
			Died        bool                    `json:"died"`
			Terminal    int                     `json:"terminal"`
			Sound       wolfDemoRuntimeSound    `json:"sound"`
			Victory     wolfDemoRuntimeVictory  `json:"victory"`
			PlayerState int                     `json:"player_state"`
			Score       int                     `json:"score"`
			Kills       int                     `json:"kills"`
			Bonuses     []wolfDemoRuntimeBonus  `json:"bonuses"`
			Projections []struct {
				Visible bool `json:"visible"`
				ViewX   int  `json:"view_x"`
				TransX  int  `json:"trans_x"`
				Shape   int  `json:"shape"`
			} `json:"projections"`
		}
		if err := json.Unmarshal(p.out.Bytes(), &rendered); err != nil {
			t.Fatal(err)
		}
		if err := renderTrace.Encode(map[string]any{"command": i, "tic": (i + 1) * wl6.DemoTics, "render": rendered}); err != nil {
			t.Fatal(err)
		}
		projectionActors := []int{}
		for index, a := range g.actors {
			if !a.removed && a.kind != actorKindVictoryBJ {
				projectionActors = append(projectionActors, index)
			}
		}
		if len(rendered.Projections) != len(projectionActors) {
			t.Fatal("projection count differs")
		}
		for index, want := range rendered.Projections {
			got := g.demoPlayback.projections[projectionActors[index]]
			if got != (demoActorProjection{want.Visible, want.ViewX, want.TransX}) {
				t.Fatalf("command %d actor %d renderer: original=%+v port=%+v", i, index, want, got)
			}
			if shape := g.demoActorRenderShape(&g.actors[projectionActors[index]], got.viewX); shape != want.Shape {
				t.Fatalf("command %d actor %d rendered shape: original=%d port=%d", i, index, want.Shape, shape)
			}
		}
		g.collectPickups()
		g.demoPlayback.timeCount += wl6.DemoTics
		gotStats := captureDemoRuntimeStats(g)
		gotFace := wolfDemoRuntimeFace{g.demoPlayback.faceFrame, g.demoPlayback.faceCount}
		portBonuses := captureDemoRuntimeBonuses(g)
		compareDemoStatics(t, i, rendered.Statics, captureDemoRuntimeStatics(g))
		portTerminal := g.demoPlayback.levelExit
		if g.playerDying {
			portTerminal = 2
		}
		if err := portRenderTrace.Encode(map[string]any{"command": i, "tic": (i + 1) * wl6.DemoTics, "render": map[string]any{
			"stats": gotStats, "face": gotFace, "health": g.health, "ammo": g.ammo,
			"score": g.score, "kills": gotStats.Kills, "died": g.playerDying, "terminal": portTerminal, "bonuses": portBonuses,
			"sound": captureDemoRuntimeSound(g), "player_state": captureDemoRuntimePlayerState(g),
			"statics": captureDemoRuntimeStatics(g),
		}}); err != nil {
			t.Fatal(err)
		}
		if rendered.Stats != gotStats {
			t.Fatalf("command %d after pickups stats: original=%+v port=%+v", i, rendered.Stats, gotStats)
		}
		if rendered.Face != gotFace {
			t.Fatalf("command %d after pickups face: original=%+v port=%+v", i, rendered.Face, gotFace)
		}
		if gotSound := captureDemoRuntimeSound(g); rendered.Sound != gotSound {
			t.Fatalf("command %d after pickups synthesized sound: original=%+v port=%+v", i, rendered.Sound, gotSound)
		}
		if gotVictory := captureDemoRuntimeVictory(g); !reflect.DeepEqual(rendered.Victory, gotVictory) {
			t.Fatalf("command %d after rendering victory: original=%+v port=%+v", i, rendered.Victory, gotVictory)
		}
		portPlayerState := boolInt(g.attacking)
		if g.demoPlayback.deathCam {
			portPlayerState = 2
		}
		if rendered.PlayerState != portPlayerState {
			t.Fatalf("command %d after rendering player state: original=%d port=%d", i, rendered.PlayerState, portPlayerState)
		}
		portKills := gotStats.Kills
		if rendered.Score != g.score || rendered.Kills != portKills {
			t.Fatalf("command %d combat totals: original score/kills=%d/%d port=%d/%d", i, rendered.Score, rendered.Kills, g.score, portKills)
		}
		kills = rendered.Kills
		sort.Slice(rendered.Bonuses, func(i, j int) bool {
			a, b := rendered.Bonuses[i], rendered.Bonuses[j]
			if a.Y != b.Y {
				return a.Y < b.Y
			}
			if a.X != b.X {
				return a.X < b.X
			}
			return a.Shape < b.Shape
		})
		if len(rendered.Bonuses) != len(portBonuses) {
			t.Fatalf("command %d bonuses: original=%d port=%d", i, len(rendered.Bonuses), len(portBonuses))
		}
		for bonus := range rendered.Bonuses {
			if rendered.Bonuses[bonus] != portBonuses[bonus] {
				t.Fatalf("command %d bonus %d: original=%+v port=%+v", i, bonus, rendered.Bonuses[bonus], portBonuses[bonus])
			}
		}
		if rendered.Died != g.playerDying {
			t.Fatalf("command %d death: original=%v port=%v", i, rendered.Died, g.playerDying)
		}
		if rendered.Terminal != portTerminal {
			t.Fatalf("command %d terminal state: original=%d port=%d", i, rendered.Terminal, portTerminal)
		}
		if rendered.Health != g.health || rendered.Ammo != g.ammo {
			t.Fatalf("command %d after pickups: original health/ammo=%d/%d port=%d/%d", i, rendered.Health, rendered.Ammo, g.health, g.ammo)
		}
		g.demoPlayback.buttons = g.demoPlayback.inputButtons
		g.demoPlayback.command++
		matched++
		if rendered.Terminal != 0 {
			if extraFire && (maxAlerted < 2 || kills < 1) {
				t.Fatalf("extra-fire route did not exercise a multi-enemy fight: alerted=%d kills=%d", maxAlerted, kills)
			}
			switch rendered.Terminal {
			case 1:
				terminal = "level_exit"
				if !g.demoPlayback.usedExit && matched == len(demo.Commands) {
					terminal = "demo_completed"
				}
			case 2:
				terminal = "death"
			case 6:
				terminal = "victory"
			case 9:
				terminal = "secret_level_exit"
			default:
				status = "unsupported_terminal_state"
				t.Fatalf("command %d unsupported original terminal state %d", i, rendered.Terminal)
			}
			status = "matched_terminal_state"
			t.Logf("matched original %s after command %d, tic %d; original playback leaves %d recorded commands unread", terminal, i, matched*wl6.DemoTics, len(demo.Commands)-matched)
			return
		}
	}
	status = "success"
	if extraFire && (maxAlerted < 2 || kills < 1) {
		t.Fatalf("extra-fire route did not exercise a multi-enemy fight: alerted=%d kills=%d", maxAlerted, kills)
	}
	t.Logf("matched every demo %d runtime update with original C, sharing visible-floor masks", demoIndex)
}
