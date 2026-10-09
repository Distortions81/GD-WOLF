package main

import (
	"testing"

	"gd-wolf/internal/wl6"
)

func TestDemoActorOpenDoorDefersSoundToMovement(t *testing.T) {
	for _, position := range []float64{0, 0.4} {
		level := blankLevel(4, 3)
		setLevelTile(level, 2, 1, wl6.Tile{Door: &wl6.Door{Vertical: true}})
		g := testGameWithLevel(level)
		g.demoPlayback = &wolfDemoPlayback{}
		g.lastPlayedSound = soundPickupChaingun
		i := level.Width + 2
		if position != 0 {
			g.doorState[i] = 3
		}
		g.doorOpen[i] = position
		g.openDoorAt(2, 1)
		if g.doorState[i] != 1 || g.doorOpen[i] != position || g.lastPlayedSound != soundPickupChaingun {
			t.Fatalf("OpenDoor at position%g: action=%d position=%g sound=%d", position, g.doorState[i], g.doorOpen[i], g.lastPlayedSound)
		}
	}
}

func TestDemoDoorSoundTimingAndReachability(t *testing.T) {
	for _, connected := range []bool{true, false} {
		level := blankLevel(5, 3)
		for y := 0; y < level.Height; y++ {
			for x := 0; x < level.Width; x++ {
				area := 1
				if x >= 2 {
					area = 2
				}
				setLevelTile(level, x, y, wl6.Tile{Area: area})
			}
		}
		setLevelTile(level, 2, 1, wl6.Tile{Area: 1, Door: &wl6.Door{Vertical: true}})
		g := testGameWithLevel(level)
		g.playerX, g.playerY = 1.5, 1.5
		if !connected {
			setLevelTile(level, 1, 1, wl6.Tile{Area: 0})
		}
		g.playerAreas = g.computePlayerAreas()
		g.demoPlayback = &wolfDemoPlayback{}
		g.rebuildPlayerAreas()
		g.lastPlayedSound = soundPickupChaingun
		g.useDoorAhead()
		if g.lastPlayedSound != soundPickupChaingun {
			t.Fatal("use emitted door sound before its movement tick")
		}
		// The disconnected test needs a different floor area on the left
		// of the door from the player's current area.
		if !connected {
			g.playerX, g.playerY = 0.5, 0.5
			setLevelTile(level, 0, 0, wl6.Tile{Area: 0})
			setLevelTile(level, 1, 1, wl6.Tile{Area: 1})
		}
		g.updateDoors(4)
		want := soundPickupChaingun
		if connected {
			want = soundDoorOpen
		}
		if g.lastPlayedSound != want {
			t.Fatalf("connected=%v: opening sound=%d want%d", connected, g.lastPlayedSound, want)
		}
		g.lastPlayedSound = soundPickupChaingun
		g.updateDoors(4)
		if g.lastPlayedSound != soundPickupChaingun {
			t.Fatal("door repeated opening sound during movement")
		}
		g.playerX, g.playerY = 0.5, 0.5
		g.rebuildPlayerAreas()
		i := level.Width + 2
		g.doorOpen[i], g.doorState[i], g.doorTimer[i] = 1, 2, doorOpenHoldTics
		g.updateDoors(4)
		if connected {
			want = soundDoorClose
		}
		if g.doorState[i] != 3 || g.lastPlayedSound != want {
			t.Fatalf("connected=%v: closing action=%d sound=%d want%d", connected, g.doorState[i], g.lastPlayedSound, want)
		}
	}
}

func TestDemoElevatorWaitsForSynthesizedSound(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"off", "adlib", "adlib-digi"} {
		sound, err := newWolfDemoSound(files, mode)
		if err != nil {
			t.Fatal(err)
		}
		// Force both synthesized and digitized LEVELDONE routing with the
		// same metadata to exercise waiting for an earlier pickup as well.
		if len(sound.bank) > 40 {
			sound.bank[40].digitized = true
		}
		g := testGameWithLevel(blankLevel(4, 3))
		g.playerX, g.playerY = 1.5, 1.5
		g.demoPlayback = &wolfDemoPlayback{sound: sound}
		g.playSound(soundPickupChaingun)
		if err := g.useElevator(); err != nil {
			t.Fatal(err)
		}
		if sound.remaining != 0 || g.demoPlayback.levelExit != 1 {
			t.Fatalf("%s: elevator left channel active: %+v", mode, sound)
		}
	}
}

func TestDemoClosedDoorDisconnectsBeforeLaterDoorSound(t *testing.T) {
	level := blankLevel(7, 3)
	for y := 0; y < level.Height; y++ {
		for x := 0; x < level.Width; x++ {
			area := 0
			if x >= 3 {
				area = 1
			}
			setLevelTile(level, x, y, wl6.Tile{Area: area})
		}
	}
	for _, x := range []int{2, 4} {
		setLevelTile(level, x, 1, wl6.Tile{Door: &wl6.Door{Vertical: true}})
	}
	g := testGameWithLevel(level)
	g.playerX, g.playerY = 0.5, 0.5
	g.demoPlayback = &wolfDemoPlayback{}
	g.playerAreas = g.computePlayerAreas()
	first, later := level.Width+2, level.Width+4
	g.doorOpen[first], g.doorState[first] = doorOpenRatePerTic*4, 3
	g.doorOpen[later], g.doorState[later], g.doorTimer[later] = 1, 2, doorOpenHoldTics
	g.rebuildPlayerAreas()
	if !g.isAreaConnectedToPlayer(1) {
		t.Fatal("test door did not initially connect area1")
	}
	g.lastPlayedSound = soundPickupChaingun
	g.updateDoors(4)
	if g.isAreaConnectedToPlayer(1) || g.doorState[later] != 3 || g.lastPlayedSound != soundPickupChaingun {
		t.Fatal("later door used connectivity from before the first door finished closing")
	}
}
