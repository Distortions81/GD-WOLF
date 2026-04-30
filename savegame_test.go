package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"reflect"
	"testing"
	"time"

	"gd-wolf/internal/wl6"
)

func TestCaptureSaveGameRoundTrip(t *testing.T) {
	level := blankLevel(4, 4)
	level.Tiles[5] = wl6.Tile{
		RawWall:    98,
		RawInfo:    0,
		Solid:      true,
		RenderWall: true,
		Area:       -1,
	}
	g := testGameWithLevel(level)
	g.mapIndex = 7
	g.playerX = 1.5
	g.playerY = 2.5
	g.playerA = 1.25
	g.cameraX = 1.5
	g.cameraY = 2.5
	g.zoom = 2.0
	g.health = 73
	g.ammo = 18
	g.lives = 2
	g.keys = 3
	g.score = 4400
	g.secretTotal = 2
	g.secretCount = 1
	g.treasureTotal = 4
	g.treasureCount = 2
	g.weapon = 2
	g.bestWeapon = 3
	g.chosenWeapon = 2
	g.attacking = true
	g.weaponSequence = seqWeaponMachineGun
	g.weaponFrameIdx = 1
	g.weaponFrameTics = 6
	g.damageFlash = 3
	g.bonusFlash = 2
	g.godMode = true
	g.doorOpen[1] = 0.5
	g.doorState[1] = 2
	g.doorTimer[1] = 12
	g.pushWall = pushWallState{
		active: true,
		x:      1,
		y:      1,
		dx:     1,
		dy:     0,
		steps:  1,
		tics:   8,
		wall: wl6.Tile{
			RawWall:    98,
			Solid:      true,
			RenderWall: true,
		},
	}
	g.staticSprites = []staticSprite{{
		x:          1.5,
		y:          1.5,
		shapenum:   42,
		rotate:     true,
		facingDir:  2,
		blocking:   true,
		shootable:  true,
		alive:      true,
		pickup:     pickupMachineGun,
		dropPickup: pickupClip2,
		scoreValue: 500,
		sequenceID: seqActorGuardDeath,
		frameIndex: 2,
		frameTimer: 9,
	}}
	g.actors = []actorInstance{{
		kind:            actorKindGuard,
		shapenum:        11,
		x:               2.5,
		y:               1.5,
		tileX:           2,
		tileY:           1,
		goalX:           3,
		goalY:           1,
		hasGoal:         true,
		dir:             1,
		facingDir:       1,
		rotate:          true,
		blocking:        true,
		shootable:       true,
		alive:           true,
		alerted:         true,
		health:          25,
		patrolSpeed:     0.1,
		chaseSpeed:      0.2,
		scoreValue:      100,
		dropPickup:      pickupClip2,
		standSeq:        seqActorGuardStand,
		patrolSeq:       seqActorGuardPatrol,
		chaseSeq:        seqActorGuardChase,
		painSeq:         seqActorGuardPain,
		shootSeq:        seqActorGuardShoot,
		deathSeq:        seqActorGuardDeath,
		aiState:         actorStateChase,
		spawnMode:       actorSpawnPatrol,
		reactionTimer:   7,
		sequenceID:      seqActorGuardChase,
		sequenceLoop:    true,
		frameIndex:      1,
		frameTimer:      4,
		frameActionDone: true,
	}}

	save := g.captureSaveGame("Test Slot")
	data, err := marshalSaveGame(save)
	if err != nil {
		t.Fatalf("marshalSaveGame failed: %v", err)
	}
	parsed, err := parseSaveGame(data)
	if err != nil {
		t.Fatalf("parseSaveGame failed: %v", err)
	}

	if parsed.Name != "Test Slot" || parsed.MapIndex != 7 {
		t.Fatalf("parsed summary = (%q,%d), want (%q,%d)", parsed.Name, parsed.MapIndex, "Test Slot", 7)
	}
	if json.Valid(data) {
		t.Fatal("save payload is still JSON, want binary save format")
	}
	if len(data) <= len(saveFileMagic)+len(saveFooterMagic)+saveChecksumSize {
		t.Fatalf("save payload too short to contain checksum footer: %d bytes", len(data))
	}
	footerStart := len(data) - len(saveFooterMagic) - saveChecksumSize
	if got := string(data[footerStart : footerStart+len(saveFooterMagic)]); got != saveFooterMagic {
		t.Fatalf("save footer magic = %q, want %q", got, saveFooterMagic)
	}
	if got := len(parsed.LevelState.TileOverrides); got != len(level.Tiles) {
		t.Fatalf("parsed tile override count = %d, want %d for synthetic test level", got, len(level.Tiles))
	}
	foundWall98 := false
	for _, override := range parsed.LevelState.TileOverrides {
		if override.Tile.RawWall == 98 {
			foundWall98 = true
			break
		}
	}
	if !foundWall98 {
		t.Fatalf("parsed tile overrides = %+v, want one with wall 98", parsed.LevelState.TileOverrides)
	}
	if got := len(parsed.LevelState.Doors); got != 1 {
		t.Fatalf("parsed door runtime count = %d, want 1", got)
	}
	if !parsed.PushWall.Active || parsed.PushWall.Wall.RawWall != 98 {
		t.Fatalf("parsed push wall = %+v, want active wall 98", parsed.PushWall)
	}
	if len(parsed.StaticSprites) != 1 || parsed.StaticSprites[0].ShapeNum != 42 {
		t.Fatalf("parsed static sprites = %+v, want one shape 42", parsed.StaticSprites)
	}
	if len(parsed.Actors) != 1 || parsed.Actors[0].SequenceID != seqActorGuardChase {
		t.Fatalf("parsed actors = %+v, want one guard in chase sequence", parsed.Actors)
	}
}

func TestActorFromSaveCanonicalizesReservedGoalToTile(t *testing.T) {
	saved := saveActor{
		Kind:         actorKindGuard,
		X:            2.5,
		Y:            1.5,
		TileX:        2,
		TileY:        1,
		GoalX:        99,
		GoalY:        77,
		HasGoal:      true,
		Dir:          0,
		FacingDir:    0,
		Alive:        true,
		Blocking:     true,
		Shootable:    true,
		PatrolSpeed:  0.1,
		ChaseSpeed:   0.2,
		MoveDistance: 0.5,
	}

	actor := actorFromSave(saved)

	if !actor.hasGoal {
		t.Fatal("loaded actor should preserve hasGoal")
	}
	if actor.tileX != 2 || actor.tileY != 1 {
		t.Fatalf("reserved tile = (%d,%d), want (2,1)", actor.tileX, actor.tileY)
	}
	if actor.goalX != 2 || actor.goalY != 1 {
		t.Fatalf("goal mirror = (%d,%d), want canonicalized (2,1)", actor.goalX, actor.goalY)
	}
}

func TestAutosaveNameUsesCurrentFloor(t *testing.T) {
	g := &game{mapIndex: 1}
	if got := g.autosaveName(); got != "Autosave Floor 02" {
		t.Fatalf("autosaveName = %q, want %q", got, "Autosave Floor 02")
	}
}

func TestAutosavePlaceholderSummariesUseDedicatedSlots(t *testing.T) {
	summaries := autosavePlaceholderSummaries()
	if len(summaries) != autosaveSlotCount {
		t.Fatalf("autosave summary count = %d, want %d", len(summaries), autosaveSlotCount)
	}
	for i, summary := range summaries {
		if !summary.IsAutosave {
			t.Fatalf("autosave %d should be marked as autosave", i)
		}
		if summary.Used {
			t.Fatalf("autosave %d should start unused", i)
		}
		if summary.Path != autosaveSlotPath(i) {
			t.Fatalf("autosave %d path = %q, want %q", i, summary.Path, autosaveSlotPath(i))
		}
		wantLabel := fmt.Sprintf("%s %d", autosaveSlotName, i+1)
		if got := slotSummaryLabel(summary); got != wantLabel {
			t.Fatalf("autosave %d label = %q, want %q", i, got, wantLabel)
		}
	}
}

func TestCaptureSaveGameRoundTripSharewareEnemyActors(t *testing.T) {
	level := blankLevel(6, 2)
	g := testGameWithLevel(level)
	g.mapIndex = 0
	g.playerX = 4.5
	g.playerY = 1.5
	g.staticSprites = []staticSprite{{
		x:        5.5,
		y:        0.5,
		shapenum: shapeGuardDead,
		alive:    true,
	}}
	g.actors = []actorInstance{
		{
			kind:            actorKindGuard,
			shapenum:        shapeGuardDie1,
			x:               0.5,
			y:               0.5,
			tileX:           0,
			tileY:           0,
			alive:           true,
			blocking:        true,
			shootable:       true,
			alerted:         true,
			firstAttack:     true,
			area:            0,
			health:          25,
			patrolSpeed:     0.1,
			chaseSpeed:      0.2,
			scoreValue:      100,
			dropPickup:      pickupClip2,
			standSeq:        seqActorGuardStand,
			patrolSeq:       seqActorGuardPatrol,
			chaseSeq:        seqActorGuardChase,
			painSeq:         seqActorGuardPain,
			shootSeq:        seqActorGuardShoot,
			deathSeq:        seqActorGuardDeath,
			aiState:         actorStateShoot,
			spawnMode:       actorSpawnStand,
			sequenceID:      seqActorGuardShoot,
			frameIndex:      1,
			frameTimer:      3,
			frameActionDone: true,
		},
		{
			kind:            actorKindDog,
			shapenum:        shapeDogJump2,
			x:               1.5,
			y:               0.5,
			tileX:           1,
			tileY:           0,
			alive:           true,
			blocking:        true,
			shootable:       true,
			alerted:         true,
			area:            0,
			health:          1,
			patrolSpeed:     0.1,
			chaseSpeed:      0.2,
			scoreValue:      200,
			standSeq:        seqActorDogStand,
			patrolSeq:       seqActorDogPatrol,
			chaseSeq:        seqActorDogChase,
			jumpSeq:         seqActorDogJump,
			deathSeq:        seqActorDogDeath,
			aiState:         actorStateJump,
			spawnMode:       actorSpawnStand,
			sequenceID:      seqActorDogJump,
			frameIndex:      1,
			frameTimer:      4,
			frameActionDone: true,
		},
		{
			kind:            actorKindOfficer,
			shapenum:        shapeOfficerShoot2,
			x:               2.5,
			y:               0.5,
			tileX:           2,
			tileY:           0,
			alive:           true,
			blocking:        true,
			shootable:       true,
			alerted:         true,
			area:            0,
			health:          50,
			patrolSpeed:     0.1,
			chaseSpeed:      0.3,
			scoreValue:      400,
			dropPickup:      pickupClip2,
			standSeq:        seqActorOfficerStand,
			patrolSeq:       seqActorOfficerPatrol,
			chaseSeq:        seqActorOfficerChase,
			painSeq:         seqActorOfficerPain,
			shootSeq:        seqActorOfficerShoot,
			deathSeq:        seqActorOfficerDeath,
			aiState:         actorStateShoot,
			spawnMode:       actorSpawnStand,
			sequenceID:      seqActorOfficerShoot,
			frameIndex:      1,
			frameTimer:      5,
			frameActionDone: true,
		},
		{
			kind:            actorKindSS,
			shapenum:        shapeSSShoot2,
			x:               3.5,
			y:               0.5,
			tileX:           3,
			tileY:           0,
			alive:           true,
			blocking:        true,
			shootable:       true,
			alerted:         true,
			area:            0,
			health:          100,
			patrolSpeed:     0.1,
			chaseSpeed:      0.25,
			scoreValue:      500,
			dropPickup:      pickupMachineGun,
			standSeq:        seqActorSSStand,
			patrolSeq:       seqActorSSPatrol,
			chaseSeq:        seqActorSSChase,
			painSeq:         seqActorSSPain,
			shootSeq:        seqActorSSShoot,
			deathSeq:        seqActorSSDeath,
			aiState:         actorStateShoot,
			spawnMode:       actorSpawnStand,
			sequenceID:      seqActorSSShoot,
			frameIndex:      3,
			frameTimer:      2,
			frameActionDone: true,
		},
		{
			kind:            actorKindBoss,
			shapenum:        shapeBossShoot2,
			x:               4.5,
			y:               0.5,
			tileX:           4,
			tileY:           0,
			alive:           true,
			blocking:        true,
			shootable:       true,
			alerted:         true,
			area:            0,
			health:          950,
			patrolSpeed:     0.1,
			chaseSpeed:      0.2,
			scoreValue:      5000,
			standSeq:        seqActorBossStand,
			chaseSeq:        seqActorBossChase,
			shootSeq:        seqActorBossShoot,
			deathSeq:        seqActorBossDeath,
			aiState:         actorStateShoot,
			spawnMode:       actorSpawnStand,
			sequenceID:      seqActorBossShoot,
			frameIndex:      1,
			frameTimer:      6,
			frameActionDone: true,
		},
	}

	save := g.captureSaveGame("Shareware Audit")
	data, err := marshalSaveGame(save)
	if err != nil {
		t.Fatalf("marshalSaveGame failed: %v", err)
	}
	parsed, err := parseSaveGame(data)
	if err != nil {
		t.Fatalf("parseSaveGame failed: %v", err)
	}
	if len(parsed.Actors) != len(g.actors) {
		t.Fatalf("parsed actor count = %d, want %d", len(parsed.Actors), len(g.actors))
	}
	if len(parsed.StaticSprites) != 1 || parsed.StaticSprites[0].ShapeNum != shapeGuardDead {
		t.Fatalf("parsed static sprites = %+v, want dead guard corpse", parsed.StaticSprites)
	}

	for i, orig := range g.actors {
		got := actorFromSave(parsed.Actors[i])
		if got.kind != orig.kind {
			t.Fatalf("actor %d kind = %v, want %v", i, got.kind, orig.kind)
		}
		if got.aiState != orig.aiState || got.sequenceID != orig.sequenceID {
			t.Fatalf("actor %d state/sequence = (%v,%q), want (%v,%q)", i, got.aiState, got.sequenceID, orig.aiState, orig.sequenceID)
		}
		if got.firstAttack != orig.firstAttack {
			t.Fatalf("actor %d firstAttack = %t, want %t", i, got.firstAttack, orig.firstAttack)
		}
		if got.frameIndex != orig.frameIndex || got.frameTimer != orig.frameTimer || got.frameActionDone != orig.frameActionDone {
			t.Fatalf("actor %d frame state = (%d,%d,%t), want (%d,%d,%t)", i, got.frameIndex, got.frameTimer, got.frameActionDone, orig.frameIndex, orig.frameTimer, orig.frameActionDone)
		}
		if got.jumpSeq != orig.jumpSeq || got.shootSeq != orig.shootSeq || got.deathSeq != orig.deathSeq {
			t.Fatalf("actor %d sequences changed: got jump/shoot/death (%q,%q,%q), want (%q,%q,%q)", i, got.jumpSeq, got.shootSeq, got.deathSeq, orig.jumpSeq, orig.shootSeq, orig.deathSeq)
		}
	}
}

func TestSaveGameThumbnailPNGRoundTrip(t *testing.T) {
	pixels := []byte{
		0xff, 0x00, 0x00, 0xff,
		0x00, 0xff, 0x00, 0xff,
		0x00, 0x00, 0xff, 0xff,
		0xff, 0xff, 0x00, 0xff,
	}
	thumbPNG, err := encodeThumbnailPNG(pixels, 2, 2)
	if err != nil {
		t.Fatalf("encodeThumbnailPNG failed: %v", err)
	}

	data, err := marshalSaveGame(saveGameData{
		Version:   saveGameVersion,
		Name:      "PNG Save",
		MapIndex:  3,
		Timestamp: time.Unix(123, 0).UTC(),
		Thumbnail: &saveThumbnailData{PNG: thumbPNG},
	})
	if err != nil {
		t.Fatalf("marshalSaveGame failed: %v", err)
	}

	parsed, err := parseSaveGame(data)
	if err != nil {
		t.Fatalf("parseSaveGame failed: %v", err)
	}
	if parsed.Thumbnail == nil || len(parsed.Thumbnail.PNG) == 0 {
		t.Fatal("parsed thumbnail missing PNG data")
	}

	img, err := png.Decode(bytes.NewReader(parsed.Thumbnail.PNG))
	if err != nil {
		t.Fatalf("png decode failed: %v", err)
	}
	if got := img.Bounds().Dx(); got != 2 {
		t.Fatalf("thumbnail width = %d, want 2", got)
	}
	if got := img.Bounds().Dy(); got != 2 {
		t.Fatalf("thumbnail height = %d, want 2", got)
	}
}

func TestApplySaveGameRestoresEquivalentState(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("OpenEmbeddedShareware failed: %v", err)
	}

	src := &game{
		files:         files,
		difficulty:    difficultyHard,
		bestWeapon:    1,
		weapon:        1,
		chosenWeapon:  1,
		selectedLevel: 0,
		pendingMap:    0,
		rng:           newWolfRNG(37),
	}
	src.ensureFrame(320, 200)
	if err := src.setMap(0); err != nil {
		t.Fatalf("setMap failed: %v", err)
	}

	src.selectedEpisode = 0
	src.selectedLevel = 0
	src.pendingMap = 0
	src.playerX += 0.25
	src.playerY += 0.25
	src.playerA = 1.25
	src.cameraX = src.playerX
	src.cameraY = src.playerY
	src.zoom = 1.75
	src.weapon = 2
	src.bestWeapon = 3
	src.chosenWeapon = 2
	src.health = 73
	src.ammo = 18
	src.lives = 2
	src.keys = 3
	src.score = 4400
	src.secretCount = 1
	src.treasureCount = 2
	src.attacking = true
	src.weaponSequence = seqWeaponMachineGun
	src.weaponFrameIdx = 1
	src.weaponFrameTics = 6
	src.damageFlash = 3
	src.bonusFlash = 2
	src.godMode = true
	src.gameplayTickAccum = 11
	src.victoryActive = true
	src.victoryPhase = victoryPhaseJump
	src.victoryBJ = staticSprite{
		x:        src.playerX,
		y:        src.playerY - 1,
		shapenum: shapeBJJump2,
		alive:    true,
	}
	src.victoryRunDistance = 2.5
	src.rng.Intn(256)
	src.rng.Intn(256)

	for i, tile := range src.level.Tiles {
		if tile.Door != nil {
			src.doorOpen[i] = 0.5
			src.doorState[i] = 2
			src.doorTimer[i] = 12
			break
		}
	}
	for i, tile := range src.level.Tiles {
		if !tile.Solid && tile.Door == nil {
			tile.RawInfo++
			src.level.Tiles[i] = tile
			break
		}
	}
	if len(src.staticSprites) > 0 {
		src.staticSprites[0].frameTimer++
		src.staticSprites[0].frameIndex++
	}
	if len(src.actors) > 0 {
		src.actors[0].reactionTimer++
		src.actors[0].frameTimer++
		src.actors[0].frameActionDone = !src.actors[0].frameActionDone
	}

	save := src.captureSaveGame("Apply Round Trip")
	data, err := marshalSaveGame(save)
	if err != nil {
		t.Fatalf("marshalSaveGame failed: %v", err)
	}
	parsed, err := parseSaveGame(data)
	if err != nil {
		t.Fatalf("parseSaveGame failed: %v", err)
	}

	dst := &game{
		files:         files,
		difficulty:    difficultyEasy,
		bestWeapon:    1,
		weapon:        1,
		chosenWeapon:  1,
		selectedLevel: 0,
		pendingMap:    0,
		rng:           newWolfRNG(0),
	}
	dst.ensureFrame(320, 200)
	if err := dst.applySaveGame(parsed); err != nil {
		t.Fatalf("applySaveGame failed: %v", err)
	}

	got := dst.captureSaveGame("Apply Round Trip")
	normalizeSaveGameForCompare(&save)
	normalizeSaveGameForCompare(&got)
	if !reflect.DeepEqual(got, save) {
		t.Fatalf("save/apply/save mismatch:\n got: %+v\nwant: %+v", got, save)
	}

	for i := 0; i < 8; i++ {
		if gotRoll, wantRoll := dst.rng.Intn(256), src.rng.Intn(256); gotRoll != wantRoll {
			t.Fatalf("rng roll %d = %d, want %d", i, gotRoll, wantRoll)
		}
	}
}

func TestApplySaveGameRejectsMismatchedContentDigest(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("OpenEmbeddedShareware failed: %v", err)
	}

	g := &game{files: files}
	save := saveGameData{
		Version:       saveGameVersion,
		Name:          "Wrong Content",
		MapIndex:      0,
		Timestamp:     time.Unix(789, 0).UTC(),
		ContentTag:    files.Variant.Ext,
		ContentDigest: append([]byte(nil), g.saveContentDigest()...),
	}
	save.ContentDigest[0] ^= 0xff

	if err := g.applySaveGame(save); err == nil {
		t.Fatal("applySaveGame succeeded for mismatched content digest, want failure")
	}
}

func TestApplySaveGameRejectsMismatchedContentTag(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("OpenEmbeddedShareware failed: %v", err)
	}

	g := &game{files: files}
	save := saveGameData{
		Version:       saveGameVersion,
		Name:          "Wrong Variant",
		MapIndex:      0,
		Timestamp:     time.Unix(790, 0).UTC(),
		ContentTag:    "SOD",
		ContentDigest: g.saveContentDigest(),
	}

	if err := g.applySaveGame(save); err == nil {
		t.Fatal("applySaveGame succeeded for mismatched content tag, want failure")
	}
}

func TestParseSaveGameRejectsChecksumMismatch(t *testing.T) {
	data, err := marshalSaveGame(saveGameData{
		Version:   saveGameVersion,
		Name:      "Corrupt Save",
		MapIndex:  1,
		Timestamp: time.Unix(456, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("marshalSaveGame failed: %v", err)
	}

	corrupted := append([]byte(nil), data...)
	corrupted[len(saveFileMagic)] ^= 0x01

	if _, err := parseSaveGame(corrupted); err == nil {
		t.Fatal("parseSaveGame succeeded for corrupted save, want checksum failure")
	}
}

func normalizeSaveGameForCompare(save *saveGameData) {
	if save == nil {
		return
	}
	save.Timestamp = time.Time{}
	save.Thumbnail = nil
}
