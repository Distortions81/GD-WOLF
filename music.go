package main

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/Distortions81/impsynth"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"gd-wolf/internal/wl6"
)

const musicTickRate = 700

const (
	musicOutputGain    = 2.0
	musicMaxGain       = 5.0
	musicSoftKneeStart = 0.85
)

var wolfMusicMapSongs = [...]int{
	3, 11, 9, 12, 3, 11, 9, 12, 2, 0,
	8, 18, 17, 4, 8, 18, 4, 17, 2, 1,
	6, 20, 22, 21, 6, 20, 22, 21, 19, 26,
	3, 11, 9, 12, 3, 11, 9, 12, 2, 0,
	8, 18, 17, 4, 8, 18, 4, 17, 2, 1,
	6, 20, 22, 21, 6, 20, 22, 21, 19, 15,
}

type musicSequencer struct {
	chunk           *wl6.MusicChunk
	eventIndex      int
	framesUntilNext int
	tickFrames      int
}

func newMusicSequencer(sampleRate int) *musicSequencer {
	tickFrames := 1
	if sampleRate > 0 {
		tickFrames = sampleRate / musicTickRate
		if tickFrames < 1 {
			tickFrames = 1
		}
	}
	return &musicSequencer{tickFrames: tickFrames}
}

func (s *musicSequencer) SetChunk(chunk *wl6.MusicChunk) {
	s.chunk = chunk
	s.eventIndex = 0
	s.framesUntilNext = 0
}

func (s *musicSequencer) advance(writeReg func(uint16, uint8)) int {
	if s.chunk == nil || len(s.chunk.Events) == 0 {
		return 0
	}
	for {
		if s.eventIndex >= len(s.chunk.Events) {
			s.eventIndex = 0
		}
		ev := s.chunk.Events[s.eventIndex]
		s.eventIndex++
		writeReg(ev.Reg, ev.Value)
		s.framesUntilNext = int(ev.Delay) * s.tickFrames
		if s.framesUntilNext > 0 {
			return s.framesUntilNext
		}
	}
}

type musicStreamSource struct {
	mu         sync.Mutex
	synth      *impsynth.Synth
	sequencer  *musicSequencer
	pending    []byte
	outputGain float64
}

func newMusicStreamSource(sampleRate int) *musicStreamSource {
	synth := impsynth.New(sampleRate)
	synth.Reset()
	return &musicStreamSource{
		synth:      synth,
		sequencer:  newMusicSequencer(sampleRate),
		outputGain: musicOutputGain,
	}
}

func (s *musicStreamSource) SetChunk(chunk *wl6.MusicChunk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.synth.Reset()
	s.sequencer.SetChunk(chunk)
	s.pending = s.pending[:0]
}

func (s *musicStreamSource) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0
	for n < len(p) {
		if len(s.pending) > 0 {
			copied := copy(p[n:], s.pending)
			s.pending = s.pending[copied:]
			n += copied
			continue
		}

		needFrames := (len(p) - n + 3) / 4
		if needFrames < 1 {
			needFrames = 1
		}
		s.pending = s.pending[:0]
		s.pending = append(s.pending, s.renderFrames(needFrames)...)
	}
	return n, nil
}

func (s *musicStreamSource) renderFrames(frames int) []byte {
	if frames <= 0 {
		return nil
	}
	out := make([]byte, 0, frames*4)
	for frames > 0 {
		if s.sequencer.chunk == nil {
			out = append(out, make([]byte, frames*4)...)
			break
		}
		if s.sequencer.framesUntilNext <= 0 {
			s.sequencer.advance(s.synth.WriteReg)
		}
		chunkFrames := frames
		if s.sequencer.framesUntilNext > 0 && chunkFrames > s.sequencer.framesUntilNext {
			chunkFrames = s.sequencer.framesUntilNext
		}
		if chunkFrames <= 0 {
			chunkFrames = 1
		}
		pcm := s.synth.GenerateStereoS16(chunkFrames)
		applyMusicOutputGainSoftKnee(pcm, s.outputGain)
		start := len(out)
		out = append(out, make([]byte, len(pcm)*2)...)
		for i, sample := range pcm {
			binary.LittleEndian.PutUint16(out[start+i*2:], uint16(sample))
		}
		frames -= chunkFrames
		s.sequencer.framesUntilNext -= chunkFrames
	}
	return out
}

func applyMusicOutputGainSoftKnee(samples []int16, gain float64) {
	if len(samples) == 0 {
		return
	}
	g := clampMusicOutputGain(gain)
	if g == 1 {
		return
	}
	const fullScale = 32767.0
	kneeStart := musicSoftKneeStart
	for i := range samples {
		x := float64(samples[i]) / fullScale
		if x > 1 {
			x = 1
		}
		if x < -1 {
			x = -1
		}
		y := x * g
		ay := math.Abs(y)
		if ay > kneeStart {
			over := (ay - kneeStart) / (1 - kneeStart)
			soft := kneeStart + (1-kneeStart)*(over/(1+over))
			if y < 0 {
				y = -soft
			} else {
				y = soft
			}
		}
		if y > 1 {
			y = 1
		}
		if y < -1 {
			y = -1
		}
		if y >= 0 {
			samples[i] = int16(math.Round(y * fullScale))
		} else {
			samples[i] = int16(math.Round(y * 32768.0))
		}
	}
}

func clampMusicOutputGain(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > musicMaxGain {
		return musicMaxGain
	}
	return v
}

type musicController struct {
	player      *audio.Player
	src         *musicStreamSource
	files       *wl6.Files
	currentSong int
	targetSong  int
}

func newMusicController(ctx *audio.Context, files *wl6.Files) (*musicController, error) {
	src := newMusicStreamSource(audioSampleRate)
	player, err := ctx.NewPlayer(src)
	if err != nil {
		return nil, err
	}
	player.Play()
	return &musicController{
		player:      player,
		src:         src,
		files:       files,
		currentSong: -1,
		targetSong:  -1,
	}, nil
}

func (m *musicController) Close() error {
	if m == nil || m.player == nil {
		return nil
	}
	m.player.Pause()
	return m.player.Close()
}

func (m *musicController) SetVolume(v float64) {
	if m == nil || m.player == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	m.player.SetVolume(v)
}

func (m *musicController) SetSong(song int) error {
	if m == nil || m.src == nil {
		return nil
	}
	if song == m.targetSong {
		return nil
	}
	m.targetSong = song
	chunk, err := m.files.LoadMusicChunk(song)
	if err != nil {
		return err
	}
	m.currentSong = song
	m.src.SetChunk(chunk)
	return nil
}

func (g *game) desiredMusicTrack() int {
	if g.files == nil {
		return -1
	}
	switch g.uiState {
	case uiStateTitle:
		return g.files.Variant.IntroSong
	case uiStateGetPsyched:
		if g.pendingMap >= 0 {
			return musicTrackForMap(g.pendingMap)
		}
		return musicTrackForMap(g.mapIndex)
	case uiStatePlaying:
		return musicTrackForMap(g.mapIndex)
	case uiStatePauseMenu, uiStateOptionsMenu, uiStateKeybindsMenu, uiStateSaveGame, uiStateLoadGame:
		if g.paused || g.menuReturn == uiStatePauseMenu {
			return musicTrackForMap(g.mapIndex)
		}
		return g.files.Variant.MenuSong
	default:
		return g.files.Variant.MenuSong
	}
}

func musicTrackForMap(mapIndex int) int {
	if mapIndex < 0 {
		return -1
	}
	if mapIndex < len(wolfMusicMapSongs) {
		return wolfMusicMapSongs[mapIndex]
	}
	return wolfMusicMapSongs[mapIndex%len(wolfMusicMapSongs)]
}

func (g *game) syncMusicTrack() {
	if g.music == nil {
		return
	}
	song := g.desiredMusicTrack()
	if song < 0 {
		return
	}
	if err := g.music.SetSong(song); err != nil {
		g.musicError = err.Error()
	}
}

func (g *game) applyMusicVolume() {
	if g.music == nil {
		return
	}
	g.music.SetVolume(g.musicVolume)
}
