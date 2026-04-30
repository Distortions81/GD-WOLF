package main

import (
	"math"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestMusicSequencerLoops(t *testing.T) {
	seq := newMusicSequencer(audioSampleRate)
	chunk := &wl6.MusicChunk{
		Events: []wl6.MusicEvent{
			{Reg: 0x20, Value: 0x01, Delay: 1},
			{Reg: 0x40, Value: 0x10, Delay: 2},
		},
	}
	seq.SetChunk(chunk)

	var writes [][2]uint16
	writeReg := func(reg uint16, value uint8) {
		writes = append(writes, [2]uint16{reg, uint16(value)})
	}

	if got := seq.advance(writeReg); got != audioSampleRate/musicTickRate {
		t.Fatalf("first advance frames=%d want %d", got, audioSampleRate/musicTickRate)
	}
	seq.framesUntilNext = 0
	if got := seq.advance(writeReg); got != 2*(audioSampleRate/musicTickRate) {
		t.Fatalf("second advance frames=%d want %d", got, 2*(audioSampleRate/musicTickRate))
	}
	seq.framesUntilNext = 0
	if got := seq.advance(writeReg); got != audioSampleRate/musicTickRate {
		t.Fatalf("looped advance frames=%d want %d", got, audioSampleRate/musicTickRate)
	}
	if len(writes) != 3 {
		t.Fatalf("write count=%d want 3", len(writes))
	}
	if writes[0][0] != 0x20 || writes[1][0] != 0x40 || writes[2][0] != 0x20 {
		t.Fatalf("write sequence=%v want 0x20,0x40,0x20", writes)
	}
}

func TestDesiredMusicTrack(t *testing.T) {
	g := &game{
		files: &wl6.Files{
			Variant: wl6.VariantSpec{
				IntroSong: 7,
				MenuSong:  14,
			},
		},
		mapIndex:      8,
		pendingMap:    3,
		uiState:       uiStateTitle,
		selectedLevel: 0,
	}

	if got := g.desiredMusicTrack(); got != 7 {
		t.Fatalf("title track=%d want 7", got)
	}
	g.uiState = uiStateMainMenu
	if got := g.desiredMusicTrack(); got != 14 {
		t.Fatalf("menu track=%d want 14", got)
	}
	g.uiState = uiStateGetPsyched
	if got := g.desiredMusicTrack(); got != wolfMusicMapSongs[3] {
		t.Fatalf("get psyched track=%d want %d", got, wolfMusicMapSongs[3])
	}
	g.uiState = uiStatePlaying
	if got := g.desiredMusicTrack(); got != wolfMusicMapSongs[8] {
		t.Fatalf("playing track=%d want %d", got, wolfMusicMapSongs[8])
	}
}

func TestApplyMusicOutputGainSoftKnee(t *testing.T) {
	samples := []int16{1000, -1000, 12000, -12000}
	applyMusicOutputGainSoftKnee(samples, 2.0)
	if math.Abs(float64(samples[0])) <= 1000 {
		t.Fatalf("quiet sample not amplified: %v", samples[0])
	}
	for _, s := range samples {
		if s == math.MinInt16 || s == math.MaxInt16 {
			t.Fatalf("sample hard-clipped: %v", s)
		}
	}
}
