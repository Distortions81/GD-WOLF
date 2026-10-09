package main

import (
	"fmt"

	"gd-wolf/internal/wl6"
)

type wolfDemoSoundInfo struct {
	length    int // Original AdLib service steps, at 140 Hz.
	priority  int
	digitized bool
}

// The original synthesized effect channel affects UpdateFace's shared RNG.
// Keep its lifetime in simulation tics: PCM playback, volume and wall-clock
// scheduling must not change a recorded route. SoundBlaster effects use a
// separate channel, which SD_SoundPlaying does not report.
type wolfDemoSound struct {
	mode      string
	bank      []wolfDemoSoundInfo
	sound     int
	priority  int
	remaining int
}

func newWolfDemoSound(files *wl6.Files, mode string) (*wolfDemoSound, error) {
	if mode != "off" && mode != "adlib" && mode != "adlib-digi" {
		return nil, fmt.Errorf("invalid demo sound mode %q: expected off, adlib, or adlib-digi", mode)
	}
	s := &wolfDemoSound{mode: mode}
	if mode == "off" {
		return s, nil
	}
	if files == nil {
		return nil, fmt.Errorf("demo sound requires game data")
	}
	count := files.Variant.StartMusicChunk / 3
	s.bank = make([]wolfDemoSoundInfo, count)
	for index := range s.bank {
		effect, err := files.LoadAdLibSound(index)
		if err != nil {
			return nil, fmt.Errorf("demo sound metadata: %w", err)
		}
		_, digitized := wolfDigitizedSoundIndex(index, files.Variant.Ext == "WL1")
		s.bank[index] = wolfDemoSoundInfo{effect.Length, int(effect.Priority), digitized}
	}
	return s, nil
}

func (s *wolfDemoSound) playRaw(index int) {
	if s == nil || s.mode == "off" || index < 0 || index >= len(s.bank) {
		return
	}
	effect := s.bank[index]
	if s.mode == "adlib-digi" && effect.digitized {
		return
	}
	if effect.length == 0 || effect.priority < s.priority {
		return
	}
	s.sound, s.priority, s.remaining = index, effect.priority, effect.length
}

func (s *wolfDemoSound) service(steps int) {
	if s == nil || s.remaining == 0 || steps <= 0 {
		return
	}
	if steps >= s.remaining {
		s.sound, s.priority, s.remaining = 0, 0, 0
		return
	}
	s.remaining -= steps
}

func (s *wolfDemoSound) advance(tics int) {
	if tics > 0 {
		s.service(tics * 2)
	}
}

func (s *wolfDemoSound) playingSound() int {
	if s == nil || s.remaining == 0 {
		return 0
	}
	return s.sound
}

func (g *game) recordDemoSound(id soundID) {
	if g.demoPlayback != nil {
		if index, ok := wolfSoundIDs[id]; ok {
			g.demoPlayback.sound.playRaw(index)
		}
	}
}

// Original WL_MAIN.C wolfdigimap, expressed as sound number -> sample number.
// These mappings also select the audible PCM, so the simulated channel agrees
// with which effects actually use the SoundBlaster path.
var wolfDigitizedSounds = map[int]int{
	21: 0, 41: 1, 19: 2, 18: 3, 26: 4, 24: 5, 11: 6, 51: 7,
	55: 8, 50: 9, 59: 10, 60: 11, 29: 12, 22: 13, 25: 13,
	16: 14, 46: 15, 56: 20, 58: 21, 61: 22, 72: 32,
}

var wolfRegisteredDigitizedSounds = map[int]int{
	10: 16, 52: 17, 53: 18, 54: 19, 62: 23, 63: 24, 64: 25,
	65: 26, 66: 27, 67: 28, 68: 29, 40: 30, 70: 31, 57: 33,
	73: 34, 74: 35, 79: 36, 80: 37, 81: 38, 75: 39, 76: 40,
	77: 41, 78: 42, 82: 43, 83: 44, 84: 45,
}

func wolfDigitizedSoundIndex(sound int, shareware bool) (int, bool) {
	if index, ok := wolfDigitizedSounds[sound]; ok {
		return index, true
	}
	if !shareware {
		index, ok := wolfRegisteredDigitizedSounds[sound]
		return index, ok
	}
	return 0, false
}
