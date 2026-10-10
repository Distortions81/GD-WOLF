package main

import (
	"math"

	"gd-wolf/internal/wl6"
)

const (
	demoButtonAttack = 1 << iota
	demoButtonStrafe
	demoButtonRun
	demoButtonUse
)

type wolfDemoPlayback struct {
	sound        *wolfDemoSound
	demo         *wl6.Demo
	command      int
	levelExit    int // Original playstate: 1 = demo complete/normal exit, 6 = victory, 9 = secret exit.
	usedExit     bool
	timeCount    int
	victoryTileY int // VictorySpin keeps the tile from the final Thrust.
	victoryEntry bool
	deathCam     bool
	bossKillX    float64
	bossKillY    float64
	tileMap      []byte
	staticSlots  []int // Original statobjlist order; excludes SpawnDeadGuard actors.
	staticInfo   []demoStaticInfo
	memory       *wolfDemoMemory
	areaPlane    []uint16
	spawnAreas   []byte
	doors        []demoDoor
	doorIndices  []int
	ticAccum     int
	buttons      byte
	inputButtons byte // T_Attack can suppress new use/attack presses this frame.
	angle        int  // Original counterclockwise integer degrees.
	angleFrac    int
	faceCount    int
	faceFrame    int
	projections  []demoActorProjection
	hits         []demoActorHit
	shots        int
	visibleTiles []bool

	visibilityScratch []bool // Owned raycast storage; probe-supplied masks remain separate.
}

// WOLFSRC BuildTables accumulates a float (32-bit) angle and stores sine
// fractions in signed-magnitude format. FixedByFrac uses only the low word.
var wolfDemoTrigTable = func() [451]uint32 {
	var table [451]uint32
	angle := float32(0)
	step := float32(3.141592657 / 2 / 90)
	for i := 0; i <= 90; i++ {
		value := uint32(65536 * math.Sin(float64(angle)))
		table[i], table[i+360], table[180-i] = value, value, value
		table[360-i], table[180+i] = value|0x80000000, value|0x80000000
		angle += step
	}
	return table
}()

func wolfDemoFixedByFrac(speed int, fraction uint32) int {
	result := int((int64(speed) * int64(fraction&0xffff)) >> 16)
	if fraction&0x80000000 != 0 {
		return -result
	}
	return result
}

func (g *game) startDemo(demo *wl6.Demo) error {
	g.startNewGame()
	g.difficulty = difficultyHard
	g.rng = newWolfRNG(0)
	if err := g.setMap(demo.Map); err != nil {
		return err
	}
	g.demoPlayback = &wolfDemoPlayback{demo: demo, angle: wolfDemoAngle(g.playerA)}
	g.initializeDemoTileMap()
	var err error
	g.demoPlayback.sound, err = newWolfDemoSound(g.files, "adlib-digi")
	if err != nil {
		return err
	}
	g.initializeDemoActorTimers()
	g.initializeDemoActorPool()
	g.fadePhase = 0
	g.uiState = uiStatePlaying
	g.mode = modeRaycast
	return nil
}

// SpawnNewObj consumes one shared random byte for each timed initial state.
// A random zero ticcount is special: DoActor thinks without advancing states
// until another state is entered. Stand states consume no random bytes.
func (g *game) initializeDemoActorTimers() {
	g.initializeDemoActorAreas()
	for i := range g.actors {
		a := &g.actors[i]
		// Interactive actor speeds historically used a 60 Hz conversion.
		// Recorded commands pass Wolf tics directly, as original speed*tics.
		a.patrolSpeed = math.Round(a.patrolSpeed*65536*60/70) / 65536
		a.chaseSpeed = math.Round(a.chaseSpeed*65536*60/70) / 65536
		seq, ok := LookupAnimSequence(a.sequenceID)
		if !ok || len(seq.Frames) == 0 || seq.Frames[0].Tics <= 0 {
			continue
		}
		tics := seq.Frames[0].Tics
		remaining := g.rng.Intn(tics)
		a.frameTimer = tics - remaining
		a.spawnAnimationFrozen = remaining == 0

	}
}

// UpdateFace uses the gameplay RNG, even if the face does not change. Sound
// suppression is supplied explicitly so comparisons can isolate its timing.
func (g *game) updateDemoFace(tics int, gatlingSound bool) {
	if gatlingSound {
		return
	}
	d := g.demoPlayback
	d.faceCount += tics
	if d.faceCount > g.rng.Intn(256) {
		d.faceFrame = g.rng.Intn(256) >> 6
		if d.faceFrame == 3 {
			d.faceFrame = 1
		}
		d.faceCount = 0
	}
}

func wolfDemoAngle(radians float64) int {
	return (int(math.Round(-radians*180/math.Pi))%360 + 360) % 360
}

// updateDemoPlayback uses the same 70 Hz tic accumulator as interactive play,
// consuming a whole recorded command only when four tics are available.
func (g *game) updateDemoPlayback(tics int) {
	d := g.demoPlayback
	d.ticAccum += tics
	for d.ticAccum >= wl6.DemoTics && d.command < len(d.demo.Commands) {
		d.ticAccum -= wl6.DemoTics
		if g.playerDying || d.levelExit != 0 {
			break
		}
		g.stepDemoCommand(d.demo.Commands[d.command])
		d.command++
	}
}

// stepDemoCommand advances actual port doors, weapons, actors and pickups.
// Those systems are not yet independently verified by the demo player harness.
func (g *game) stepDemoCommand(command wl6.DemoCommand) {
	wasAttacking := g.prepareDemoCommand(command)
	g.moveDemoPlayer(command)
	g.finishDemoCommand(command, wasAttacking)
}

func (g *game) prepareDemoCommand(command wl6.DemoCommand) bool {
	d := g.demoPlayback
	// PollControls marks completion before this command's gameplay. A death,
	// elevator or victory later in the command can replace that playstate.
	if d.demo != nil && d.command+1 == len(d.demo.Commands) {
		d.levelExit = 1
	}
	d.hits = nil
	d.shots = 0
	g.madeNoise = false
	d.sound.advance(wl6.DemoTics)
	g.updateDoors(wl6.DemoTics)
	g.updatePushWall(wl6.DemoTics)
	g.updateSpriteAnimations(wl6.DemoTics)
	g.updateScreenFlashes(wl6.DemoTics)
	wasAttacking := g.attacking
	d.victoryEntry = g.victoryActive
	d.inputButtons = command.Buttons
	if d.deathCam {
		return wasAttacking
	}
	// T_Attack updates the face before its victory check; T_Player checks
	// first. The attack state and its counters survive VictoryTile.
	if !g.victoryActive || wasAttacking {
		g.updateDemoFace(wl6.DemoTics, g.isSoundPlaying(soundPickupChaingun))
	}
	if g.victoryActive {
		return wasAttacking
	}
	if wasAttacking {
		for _, button := range []byte{demoButtonAttack, demoButtonUse} {
			if command.Buttons&button != 0 && d.buttons&button == 0 {
				d.inputButtons &^= button
			}
		}
	}
	if !wasAttacking {
		if command.Buttons&demoButtonUse != 0 {
			g.useDoorAhead()
		}
		if g.ammo > 0 {
			for weapon := 0; weapon <= g.bestWeapon; weapon++ {
				if command.Buttons&(1<<uint(weapon+4)) != 0 {
					g.weapon, g.chosenWeapon = weapon, weapon
					break
				}
			}
		}
		// Cmd_Fire enters s_attack; T_Attack starts counting on the next command.
		if command.Buttons&demoButtonAttack != 0 && d.buttons&demoButtonAttack == 0 {
			g.updateWeaponAttackWithInput(0, true)
		}
	}
	return wasAttacking
}

func (g *game) finishDemoCommand(command wl6.DemoCommand, wasAttacking bool) {
	if wasAttacking && !g.victoryActive {
		g.updateWeaponAttackWithInput(wl6.DemoTics, g.demoPlayback.inputButtons&demoButtonAttack != 0)
	}
	g.updateDemoActors(wl6.DemoTics)
	g.refreshDemoActorProjections()
	g.collectPickups()
	g.demoPlayback.timeCount += wl6.DemoTics
	g.demoPlayback.buttons = g.demoPlayback.inputButtons
}

func (g *game) moveDemoPlayer(command wl6.DemoCommand) {
	d := g.demoPlayback
	if d.deathCam {
		return
	}
	if d.victoryEntry {
		g.advanceDemoVictoryPlayer(wl6.DemoTics)
		return
	}
	cx, cy := int(command.ControlX)*wl6.DemoTics, int(command.ControlY)*wl6.DemoTics
	thrust := func(angle, speed int) {
		if speed >= 0xb000 {
			speed = 0xafff
		}
		dx := wolfDemoFixedByFrac(speed, wolfDemoTrigTable[angle+90])
		dy := -wolfDemoFixedByFrac(speed, wolfDemoTrigTable[angle])
		g.clipDemoPlayer(float64(dx)/65536, float64(dy)/65536)
		d.victoryTileY = int(g.playerY)
		// Original Thrust checks EXITTILE after every movement, including
		// the strafe leg of a combined strafe/forward command.
		g.checkVictoryTile()
	}
	if command.Buttons&demoButtonStrafe != 0 {
		if cx > 0 {
			thrust((d.angle+270)%360, cx*150)
		}
		if cx < 0 {
			thrust((d.angle+90)%360, -cx*150)
		}
	} else {
		d.angleFrac += cx
		units := d.angleFrac / 20
		d.angleFrac -= units * 20
		d.angle = (d.angle - units + 360) % 360
	}
	g.playerA = -float64(d.angle) * math.Pi / 180
	if cy < 0 {
		thrust(d.angle, -cy*150)
	}
	if cy > 0 {
		thrust((d.angle+180)%360, cy*100)
	}
	speed := 0
	if command.Buttons&demoButtonStrafe != 0 {
		speed += absInt(cx) * 150
	}
	if cy < 0 {
		speed += -cy * 150
	} else {
		speed += cy * 100
	}
	g.playerMovingFast = speed >= 6000
}

// Recorded demos need original tile-solid doors and axis sliding to follow
// their route. Interactive movement retains the documented thin door slab.
func (g *game) clipDemoPlayer(dx, dy float64) {
	x, y := g.playerX, g.playerY
	if g.demoPlayerPositionClear(x+dx, y+dy) {
		g.playerX, g.playerY = x+dx, y+dy
		return
	}
	if g.demoPlayback.sound.playingSound() == 0 {
		g.playSound(soundHitWall)
	}
	if g.demoPlayerPositionClear(x+dx, y) {
		g.playerX = x + dx
		return
	}
	if g.demoPlayerPositionClear(x, y+dy) {
		g.playerY = y + dy
	}
}

func (g *game) demoPlayerPositionClear(x, y float64) bool {
	return g.demoActorGridPlayerPositionClear(x, y)
}

func (g *game) demoSolidTile(x, y int) bool {
	if g.isBlockingTile(x, y) || g.blockingStaticAt(x, y) {
		return true
	}
	return g.level.Tile(x, y).Door != nil && !g.isDoorOpen(x, y)
}
