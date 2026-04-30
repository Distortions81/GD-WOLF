package wl6

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/Distortions81/impsynth"
)

const adLibEffectTickRate = 140
const adLibEffectGain = 3.0

const (
	adLibModulator0 = 0x00
	adLibCarrier0   = 0x03
	adLibChar       = 0x20
	adLibScale      = 0x40
	adLibAttack     = 0x60
	adLibSustain    = 0x80
	adLibFreqL      = 0xa0
	adLibFreqH      = 0xb0
	adLibFeedCon    = 0xc0
	adLibWave       = 0xe0
)

type AdLibInstrument struct {
	MChar   uint8
	CChar   uint8
	MScale  uint8
	CScale  uint8
	MAttack uint8
	CAttack uint8
	MSus    uint8
	CSus    uint8
	MWave   uint8
	CWave   uint8
	NConn   uint8
	Voice   uint8
	Mode    uint8
	Unused  [3]uint8
}

type AdLibSound struct {
	Length     int
	Priority   uint16
	Instrument AdLibInstrument
	Block      uint8
	Data       []byte
}

func (f *Files) LoadAdLibSound(sound int) (*AdLibSound, error) {
	if f == nil {
		return nil, fmt.Errorf("nil files")
	}
	soundCount := f.Variant.StartMusicChunk / 3
	if soundCount <= 0 {
		return nil, fmt.Errorf("invalid start music chunk %d", f.Variant.StartMusicChunk)
	}
	if sound < 0 || sound >= soundCount {
		return nil, fmt.Errorf("adlib sound %d out of range", sound)
	}
	raw, err := f.loadAudioChunk(soundCount + sound)
	if err != nil {
		return nil, err
	}
	if len(raw) < 23 {
		return nil, fmt.Errorf("adlib sound %d too small: %d", sound, len(raw))
	}

	length := int(binary.LittleEndian.Uint32(raw[0:4]))
	if length < 0 {
		return nil, fmt.Errorf("adlib sound %d invalid length %d", sound, length)
	}

	dataStart := 23
	dataEnd := dataStart + length
	if dataEnd > len(raw) {
		return nil, fmt.Errorf("adlib sound %d truncated: want %d bytes, have %d", sound, dataEnd, len(raw))
	}

	inst := AdLibInstrument{
		MChar:   raw[6],
		CChar:   raw[7],
		MScale:  raw[8],
		CScale:  raw[9],
		MAttack: raw[10],
		CAttack: raw[11],
		MSus:    raw[12],
		CSus:    raw[13],
		MWave:   raw[14],
		CWave:   raw[15],
		NConn:   raw[16],
		Voice:   raw[17],
		Mode:    raw[18],
		Unused:  [3]uint8{raw[19], raw[20], raw[21]},
	}

	data := make([]byte, length)
	copy(data, raw[dataStart:dataEnd])

	return &AdLibSound{
		Length:     length,
		Priority:   binary.LittleEndian.Uint16(raw[4:6]),
		Instrument: inst,
		Block:      raw[22],
		Data:       data,
	}, nil
}

func (f *Files) RenderAdLibSoundPCM(sound, sampleRate int) ([]byte, error) {
	effect, err := f.LoadAdLibSound(sound)
	if err != nil {
		return nil, err
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate %d", sampleRate)
	}
	if len(effect.Data) == 0 {
		return nil, nil
	}

	framesPerTick := sampleRate / adLibEffectTickRate
	if framesPerTick < 1 {
		framesPerTick = 1
	}

	synth := impsynth.New(sampleRate)
	synth.Reset()
	writeAdLibFXInstrument(synth, effect.Instrument)

	block := ((effect.Block & 7) << 2) | 0x20
	out := make([]byte, 0, (len(effect.Data)+1)*framesPerTick*4)

	for _, note := range effect.Data {
		if note == 0 {
			synth.WriteReg(adLibFreqH, 0)
		} else {
			synth.WriteReg(adLibFreqL, note)
			synth.WriteReg(adLibFreqH, block)
		}
		pcm := synth.GenerateStereoS16(framesPerTick)
		applyAdLibEffectGain(pcm, adLibEffectGain)
		appendPCM16(&out, pcm)
	}

	// Match WOLFSRC's explicit key-off after the last AdLib step.
	synth.WriteReg(adLibFreqH, 0)
	pcm := synth.GenerateStereoS16(framesPerTick)
	applyAdLibEffectGain(pcm, adLibEffectGain)
	appendPCM16(&out, pcm)
	return out, nil
}

func writeAdLibFXInstrument(synth *impsynth.Synth, inst AdLibInstrument) {
	synth.WriteReg(adLibModulator0+adLibChar, inst.MChar)
	synth.WriteReg(adLibModulator0+adLibScale, inst.MScale)
	synth.WriteReg(adLibModulator0+adLibAttack, inst.MAttack)
	synth.WriteReg(adLibModulator0+adLibSustain, inst.MSus)
	synth.WriteReg(adLibModulator0+adLibWave, inst.MWave)

	synth.WriteReg(adLibCarrier0+adLibChar, inst.CChar)
	synth.WriteReg(adLibCarrier0+adLibScale, inst.CScale)
	synth.WriteReg(adLibCarrier0+adLibAttack, inst.CAttack)
	synth.WriteReg(adLibCarrier0+adLibSustain, inst.CSus)
	synth.WriteReg(adLibCarrier0+adLibWave, inst.CWave)

	// WOLFSRC's SDL_AlSetFXInst writes zero here for old MUSE compatibility.
	synth.WriteReg(adLibFeedCon, 0)
}

func appendPCM16(dst *[]byte, samples []int16) {
	start := len(*dst)
	*dst = append(*dst, make([]byte, len(samples)*2)...)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16((*dst)[start+i*2:], uint16(sample))
	}
}

func applyAdLibEffectGain(samples []int16, gain float64) {
	if gain <= 0 || gain == 1 {
		return
	}
	for i, sample := range samples {
		v := float64(sample) * gain
		if v > math.MaxInt16 {
			v = math.MaxInt16
		}
		if v < math.MinInt16 {
			v = math.MinInt16
		}
		samples[i] = int16(v)
	}
}
