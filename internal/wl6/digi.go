package wl6

import (
	"encoding/binary"
	"fmt"
)

const (
	digiPageSize   = 4096
	DigiSampleRate = 7000
)

type DigitizedSoundSet struct {
	SampleRate int
	Samples    [][]byte
}

func (f *Files) LoadDigitizedSounds() (*DigitizedSoundSet, error) {
	chunkCount, _, soundStart, offsets, lengths, err := parseVSwapHeader(f.VSwap)
	if err != nil {
		return nil, err
	}
	if soundStart <= 0 || soundStart >= chunkCount {
		return nil, fmt.Errorf("invalid VSWAP sound start %d of %d", soundStart, chunkCount)
	}

	listChunk := chunkCount - 1
	listRaw, err := loadVSwapChunk(f.VSwap, offsets, lengths, listChunk)
	if err != nil {
		return nil, fmt.Errorf("load digilist chunk: %w", err)
	}
	if len(listRaw)%4 != 0 {
		return nil, fmt.Errorf("invalid digilist size %d", len(listRaw))
	}

	sounds := make([][]byte, 0, len(listRaw)/4)
	for i := 0; i+3 < len(listRaw); i += 4 {
		idx := i / 4
		start := int(wordLE(listRaw[i:]))
		length := int(wordLE(listRaw[i+2:]))
		if soundStart+start >= listChunk {
			break
		}

		lastPage := listChunk
		if i+7 < len(listRaw) {
			nextStart := int(wordLE(listRaw[i+4:]))
			if nextStart > 0 && soundStart+nextStart < listChunk {
				lastPage = soundStart + nextStart
			}
		}

		raw, err := collectDigiPages(f.VSwap, offsets, lengths, soundStart+start, lastPage, length)
		if err != nil {
			return nil, fmt.Errorf("load digitized sound %d: %w", idx, err)
		}
		sounds = append(sounds, convertUnsigned8MonoToStereo16(raw))
	}

	return &DigitizedSoundSet{
		SampleRate: DigiSampleRate,
		Samples:    sounds,
	}, nil
}

func (f *Files) LoadDigitizedSoundsResampled(dstRate int) (*DigitizedSoundSet, error) {
	set, err := f.LoadDigitizedSounds()
	if err != nil {
		return nil, err
	}
	if dstRate <= 0 || dstRate == set.SampleRate {
		return set, nil
	}

	resampled := make([][]byte, len(set.Samples))
	for i, sample := range set.Samples {
		resampled[i] = resampleStereo16(sample, set.SampleRate, dstRate)
	}
	return &DigitizedSoundSet{
		SampleRate: dstRate,
		Samples:    resampled,
	}, nil
}

func collectDigiPages(vswap []byte, offsets []uint32, lengths []uint16, startPage, endPage, length int) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("negative digitized sound length %d", length)
	}
	if length == 0 {
		return nil, nil
	}
	if startPage < 0 || startPage >= len(offsets) {
		return nil, fmt.Errorf("start page %d out of range", startPage)
	}
	if endPage <= startPage || endPage > len(offsets) {
		return nil, fmt.Errorf("end page %d out of range for start %d", endPage, startPage)
	}

	out := make([]byte, 0, (endPage-startPage)*digiPageSize)
	for page := startPage; page < endPage; page++ {
		chunk, err := loadVSwapChunk(vswap, offsets, lengths, page)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	if len(out) < length {
		buf := make([]byte, len(out))
		copy(buf, out)
		return buf, nil
	}
	return out[:length], nil
}

func loadVSwapChunk(vswap []byte, offsets []uint32, lengths []uint16, page int) ([]byte, error) {
	if page < 0 || page >= len(offsets) {
		return nil, fmt.Errorf("page %d out of range", page)
	}
	offset := offsets[page]
	if offset == 0 {
		return nil, nil
	}

	length := int(lengths[page])
	if page+1 < len(offsets) && offsets[page+1] != 0 {
		length = int(offsets[page+1] - offset)
	}
	start := int(offset)
	end := start + length
	if start < 0 || end < start || end > len(vswap) {
		return nil, fmt.Errorf("VSWAP chunk [%d:%d] out of bounds", start, end)
	}
	buf := make([]byte, length)
	copy(buf, vswap[start:end])
	return buf, nil
}

func convertUnsigned8MonoToStereo16(src []byte) []byte {
	dst := make([]byte, len(src)*4)
	for i, b := range src {
		v := int16(int(b)-128) << 8
		j := i * 4
		dst[j] = byte(v)
		dst[j+1] = byte(v >> 8)
		dst[j+2] = byte(v)
		dst[j+3] = byte(v >> 8)
	}
	return dst
}

func resampleStereo16(src []byte, srcRate, dstRate int) []byte {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		buf := make([]byte, len(src))
		copy(buf, src)
		return buf
	}

	srcFrames := len(src) / 4
	if srcFrames == 0 {
		return nil
	}
	dstFrames := (srcFrames*dstRate + srcRate - 1) / srcRate
	if dstFrames < 1 {
		dstFrames = 1
	}
	dst := make([]byte, dstFrames*4)
	last := srcFrames - 1
	step := (int64(srcRate) << 16) / int64(dstRate)
	pos := int64(0)
	for i := 0; i < dstFrames; i++ {
		idx := int(pos >> 16)
		frac := int(pos & 0xffff)
		if idx < 0 {
			idx = 0
			frac = 0
		}
		if idx >= last {
			copy(dst[i*4:(i+1)*4], src[last*4:(last+1)*4])
			pos += step
			continue
		}

		aBase := idx * 4
		bBase := (idx + 1) * 4
		for ch := 0; ch < 2; ch++ {
			a := int32(int16(binary.LittleEndian.Uint16(src[aBase+ch*2:])))
			b := int32(int16(binary.LittleEndian.Uint16(src[bBase+ch*2:])))
			sample := (a*int32(0x10000-frac) + b*int32(frac)) >> 16
			binary.LittleEndian.PutUint16(dst[i*4+ch*2:], uint16(int16(sample)))
		}
		pos += step
	}
	return dst
}

func wordLE(buf []byte) uint16 {
	return uint16(buf[0]) | uint16(buf[1])<<8
}
