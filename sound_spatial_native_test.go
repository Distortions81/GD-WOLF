//go:build !js

package main

import (
	"encoding/binary"
	"testing"
)

func TestSpatializeStereo16LEScalesChannels(t *testing.T) {
	src := make([]byte, 8)
	putPCM16(src[0:], 12000)
	putPCM16(src[2:], 12000)
	putPCM16(src[4:], -8000)
	putPCM16(src[6:], -8000)

	got := spatializeStereo16LE(nil, src, 0.25, 1.0)

	if left := int16(binary.LittleEndian.Uint16(got[0:])); left != 3000 {
		t.Fatalf("first left sample = %d, want 3000", left)
	}
	if right := int16(binary.LittleEndian.Uint16(got[2:])); right != 12000 {
		t.Fatalf("first right sample = %d, want 12000", right)
	}
	if left := int16(binary.LittleEndian.Uint16(got[4:])); left != -2000 {
		t.Fatalf("second left sample = %d, want -2000", left)
	}
	if right := int16(binary.LittleEndian.Uint16(got[6:])); right != -8000 {
		t.Fatalf("second right sample = %d, want -8000", right)
	}
}

func putPCM16(dst []byte, sample int16) {
	binary.LittleEndian.PutUint16(dst, uint16(sample))
}
