package main

import "testing"

func TestDemoSoundOffSkipsAudioVoices(t *testing.T) {
	for _, spatial := range []bool{false, true} {
		g := &game{
			demoPlayback: &wolfDemoPlayback{sound: &wolfDemoSound{mode: "off"}},
			// An unavailable device must be safe even when a bank has voices.
			// Any attempt to choose or start this voice would panic.
			soundBanks:     map[soundID][]*soundVoice{soundPistol: {nil}},
			soundBankIndex: map[soundID]int{soundPistol: 7},
		}
		if spatial {
			g.playWorldSound(soundPistol, 1, 1)
		} else {
			g.playSound(soundPistol)
		}
		if g.lastPlayedSound != soundPistol || g.soundBankIndex[soundPistol] != 7 || g.demoPlayback.sound.remaining != 0 {
			t.Fatalf("muted sound spatial=%v: diagnostic=%d voice=%d remaining=%d", spatial, g.lastPlayedSound, g.soundBankIndex[soundPistol], g.demoPlayback.sound.remaining)
		}
	}
}
