package wl6

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestLoadAdLibSound(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}

	// GETAMMOSND is present in the original Wolf sound enum set.
	snd, err := files.LoadAdLibSound(31)
	if err != nil {
		t.Fatalf("LoadAdLibSound: %v", err)
	}
	if snd.Length <= 0 {
		t.Fatalf("length = %d, want > 0", snd.Length)
	}
	if len(snd.Data) != snd.Length {
		t.Fatalf("data len = %d, want %d", len(snd.Data), snd.Length)
	}
}

func TestRenderAdLibSoundPCM(t *testing.T) {
	files, err := OpenEmbeddedShareware()
	if err != nil {
		t.Fatalf("open embedded shareware: %v", err)
	}

	pcm, err := files.RenderAdLibSoundPCM(31, 44100)
	if err != nil {
		t.Fatalf("RenderAdLibSoundPCM: %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("pcm is empty")
	}
	if len(pcm)%4 != 0 {
		t.Fatalf("pcm len = %d, want stereo 16-bit frames", len(pcm))
	}
	if peakPCM16(pcm) < 0.30 {
		t.Fatalf("pcm peak = %.3f, want >= 0.30 after gain", peakPCM16(pcm))
	}
}

func peakPCM16(b []byte) float64 {
	peak := 0.0
	for i := 0; i+1 < len(b); i += 2 {
		v := math.Abs(float64(int16(binary.LittleEndian.Uint16(b[i:])))) / 32767.0
		if v > peak {
			peak = v
		}
	}
	return peak
}
