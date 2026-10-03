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
	demo      *wl6.Demo
	command   int
	ticAccum  int
	buttons   byte
	angle     int // Original counterclockwise integer degrees.
	angleFrac int
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
	g.fadePhase = 0
	g.uiState = uiStatePlaying
	g.mode = modeRaycast
	return nil
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
		if g.playerDying || g.victoryActive {
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
	g.madeNoise = false
	g.updateDoors(wl6.DemoTics)
	g.updatePushWall(wl6.DemoTics)
	g.updateSpriteAnimations(wl6.DemoTics)
	g.updateScreenFlashes(wl6.DemoTics)
	wasAttacking := g.attacking
	if !wasAttacking {
		if command.Buttons&demoButtonUse != 0 && d.buttons&demoButtonUse == 0 {
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
	if wasAttacking {
		g.updateWeaponAttackWithInput(wl6.DemoTics, command.Buttons&demoButtonAttack != 0)
	}
	g.updateActors(wl6.DemoTics)
	g.collectPickups()
	g.demoPlayback.buttons = command.Buttons
}

func (g *game) moveDemoPlayer(command wl6.DemoCommand) {
	d := g.demoPlayback
	cx, cy := int(command.ControlX)*wl6.DemoTics, int(command.ControlY)*wl6.DemoTics
	thrust := func(angle, speed int) {
		if speed >= 0xb000 {
			speed = 0xafff
		}
		dx := wolfDemoFixedByFrac(speed, wolfDemoTrigTable[angle+90])
		dy := -wolfDemoFixedByFrac(speed, wolfDemoTrigTable[angle])
		g.clipDemoPlayer(float64(dx)/65536, float64(dy)/65536)
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
	if g.demoPlayerPositionClear(x+dx, y) {
		g.playerX = x + dx
		return
	}
	if g.demoPlayerPositionClear(x, y+dy) {
		g.playerY = y + dy
	}
}

func (g *game) demoPlayerPositionClear(x, y float64) bool {
	xl, xh, yl, yh := playerTileSpan(x, y)
	for ty := yl; ty <= yh; ty++ {
		for tx := xl; tx <= xh; tx++ {
			if g.demoBlockingActorAt(tx, ty) != nil {
				continue
			}
			if g.demoSolidTile(tx, ty) {
				return false
			}
		}
	}
	// TryMove expands the tile box by one before checking shootable actors.
	for i := range g.actors {
		a := &g.actors[i]
		if !a.alive || !a.blocking || !a.shootable || a.tileX < xl-1 || a.tileX > xh+1 || a.tileY < yl-1 || a.tileY > yh+1 {
			continue
		}
		if math.Abs(x-a.x) <= 1 && math.Abs(y-a.y) <= 1 {
			return false
		}
	}
	return true
}

func (g *game) demoBlockingActorAt(x, y int) *actorInstance {
	for i := range g.actors {
		a := &g.actors[i]
		if a.alive && a.blocking && a.shootable && a.tileX == x && a.tileY == y {
			return a
		}
	}
	return nil
}

func (g *game) demoSolidTile(x, y int) bool {
	if g.isBlockingTile(x, y) || g.blockingStaticAt(x, y) {
		return true
	}
	return g.level.Tile(x, y).Door != nil && !g.isDoorOpen(x, y)
}
