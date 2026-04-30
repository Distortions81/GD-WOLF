//go:build !js

package main

import (
	"encoding/binary"
	"math"
)

const (
	soundMaxDistance = 16.0
	soundPanRange    = 6.0
)

func (g *game) playWorldSound(id soundID, x, y float64) {
	g.lastPlayedSound = id
	voices := g.soundBanks[id]
	if len(voices) == 0 {
		return
	}

	leftGain, rightGain, ok := g.worldSoundGains(x, y)
	if !ok {
		return
	}

	voice := g.chooseSoundVoice(id, voices)
	if voice == nil || voice.player == nil || voice.src == nil {
		return
	}
	voice.player.Pause()
	voice.buf = spatializeStereo16LE(voice.buf[:0], voice.base, leftGain, rightGain)
	voice.src.buf = voice.buf
	voice.src.Reset()
	_ = voice.player.Rewind()
	voice.player.SetVolume(g.sfxVolume)
	voice.player.Play()
}

func (g *game) chooseSoundVoice(id soundID, voices []*soundVoice) *soundVoice {
	start := g.soundBankIndex[id]
	if len(voices) == 0 {
		return nil
	}
	chosen := voices[start%len(voices)]
	for i := range voices {
		idx := (start + i) % len(voices)
		if !voices[idx].player.IsPlaying() {
			g.soundBankIndex[id] = (idx + 1) % len(voices)
			return voices[idx]
		}
	}
	g.soundBankIndex[id] = (start + 1) % len(voices)
	return chosen
}

func (g *game) worldSoundGains(x, y float64) (float64, float64, bool) {
	dx := x - g.playerX
	dy := y - g.playerY
	dist := math.Hypot(dx, dy)
	if dist > soundMaxDistance {
		return 0, 0, false
	}

	attenuation := 1.0
	if dist > 1 {
		attenuation = 1 - (dist-1)/(soundMaxDistance-1)
	}
	if attenuation <= 0 {
		return 0, 0, false
	}

	rightX := -math.Sin(g.playerA)
	rightY := math.Cos(g.playerA)
	pan := (dx*rightX + dy*rightY) / soundPanRange
	if pan < -1 {
		pan = -1
	} else if pan > 1 {
		pan = 1
	}

	left := attenuation
	right := attenuation
	if pan > 0 {
		left *= 1 - pan
	} else if pan < 0 {
		right *= 1 + pan
	}
	return left, right, true
}

func spatializeStereo16LE(dst, src []byte, leftGain, rightGain float64) []byte {
	if len(src) == 0 {
		return dst[:0]
	}
	if leftGain < 0 {
		leftGain = 0
	}
	if rightGain < 0 {
		rightGain = 0
	}
	dst = resizePCMBuffer(dst, len(src))
	for i := 0; i+3 < len(src); i += 4 {
		left := int16(binary.LittleEndian.Uint16(src[i:]))
		right := int16(binary.LittleEndian.Uint16(src[i+2:]))
		binary.LittleEndian.PutUint16(dst[i:], uint16(scalePCM16(left, leftGain)))
		binary.LittleEndian.PutUint16(dst[i+2:], uint16(scalePCM16(right, rightGain)))
	}
	return dst
}

func resizePCMBuffer(dst []byte, n int) []byte {
	if cap(dst) < n {
		return make([]byte, n)
	}
	return dst[:n]
}

func scalePCM16(sample int16, gain float64) int16 {
	if gain <= 0 {
		return 0
	}
	if gain >= 1 {
		return sample
	}
	value := int(math.Round(float64(sample) * gain))
	if value > math.MaxInt16 {
		return math.MaxInt16
	}
	if value < math.MinInt16 {
		return math.MinInt16
	}
	return int16(value)
}
