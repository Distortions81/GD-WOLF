package wl6

import (
	"encoding/binary"
	"testing"
)

func TestLoadDigitizedSounds(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	set, err := files.LoadDigitizedSounds()
	if err != nil {
		t.Fatalf("load digitized sounds: %v", err)
	}
	if set.SampleRate != DigiSampleRate {
		t.Fatalf("sample rate = %d, want %d", set.SampleRate, DigiSampleRate)
	}
	if got := len(set.Samples); got != 46 {
		t.Fatalf("sample count = %d, want 46", got)
	}

	for _, idx := range []int{4, 5, 6} {
		if idx >= len(set.Samples) {
			t.Fatalf("sample %d missing", idx)
		}
		sample := set.Samples[idx]
		if len(sample) == 0 {
			t.Fatalf("sample %d empty", idx)
		}
		if len(sample)%4 != 0 {
			t.Fatalf("sample %d len = %d, want stereo 16-bit frames", idx, len(sample))
		}
	}
}

func TestConvertUnsigned8MonoToStereo16(t *testing.T) {
	got := convertUnsigned8MonoToStereo16([]byte{0x00, 0x80, 0xff})
	wantLen := 12
	if len(got) != wantLen {
		t.Fatalf("len = %d, want %d", len(got), wantLen)
	}
	if got[0] != 0x00 || got[1] != 0x80 {
		t.Fatalf("first sample = %02x %02x, want 00 80", got[0], got[1])
	}
	if got[4] != 0x00 || got[5] != 0x00 {
		t.Fatalf("middle sample = %02x %02x, want 00 00", got[4], got[5])
	}
	if got[8] != 0x00 || got[9] != 0x7f {
		t.Fatalf("last sample = %02x %02x, want 00 7f", got[8], got[9])
	}
}

func TestLoadDigitizedSoundsResampled(t *testing.T) {
	files, _, err := OpenDefault()
	if err != nil {
		t.Fatalf("open fixture data: %v", err)
	}

	set, err := files.LoadDigitizedSoundsResampled(44100)
	if err != nil {
		t.Fatalf("load resampled digitized sounds: %v", err)
	}
	if set.SampleRate != 44100 {
		t.Fatalf("sample rate = %d, want 44100", set.SampleRate)
	}
	if got := len(set.Samples); got != 46 {
		t.Fatalf("sample count = %d, want 46", got)
	}
	if len(set.Samples[5]) <= 5410*4 {
		t.Fatalf("resampled pistol sample len = %d, want > %d", len(set.Samples[5]), 5410*4)
	}
}

func TestResampleStereo16Interpolates(t *testing.T) {
	src := []byte{
		0x00, 0x00, 0x00, 0x00,
		0xe8, 0x03, 0xe8, 0x03,
	}
	got := resampleStereo16(src, 1, 2)
	if len(got) != 16 {
		t.Fatalf("len = %d, want 16", len(got))
	}
	left0 := int16(binary.LittleEndian.Uint16(got[0:2]))
	left1 := int16(binary.LittleEndian.Uint16(got[4:6]))
	left2 := int16(binary.LittleEndian.Uint16(got[8:10]))
	if left0 != 0 {
		t.Fatalf("first sample = %d, want 0", left0)
	}
	if left1 <= 0 || left1 >= 1000 {
		t.Fatalf("interpolated sample = %d, want between 0 and 1000", left1)
	}
	if left2 != 1000 {
		t.Fatalf("last source sample = %d, want 1000", left2)
	}
}
