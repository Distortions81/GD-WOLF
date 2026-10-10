package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	"gd-wolf/internal/wl6"
)

func musicStreamTestChunk(tb testing.TB, song string) *wl6.MusicChunk {
	tb.Helper()
	if song == "silence" {
		return nil
	}
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		tb.Fatal(err)
	}
	index := files.Variant.MenuSong
	if song == "intro" {
		index = files.Variant.IntroSong
	}
	chunk, err := files.LoadMusicChunk(index)
	if err != nil {
		tb.Fatal(err)
	}
	return chunk
}

func TestMusicStreamPCM(t *testing.T) {
	// Capture one second from the original stream implementation. These cover
	// real register changes, event boundaries and reads that split PCM frames.
	wantHashes := map[string]string{
		"silence": "fd6f479534cdd14635e88dfedf25c3859c01062b645f2a85570f20451b4a95bc",
		"menu":    "290c7d1d092c226e06ddb218b270228a6851e2ed635121350e22e73fcf6ac824",
		"intro":   "7e89f9385a7bf3d2e6525136dcb74f4d9213f29bc0dd65186d195b27945a6f02",
	}
	for _, song := range []string{"silence", "menu", "intro"} {
		t.Run(song, func(t *testing.T) {
			chunk := musicStreamTestChunk(t, song)
			for _, sizes := range [][]int{{8192}, {0, 1, 3, 5, 1023, 4096}} {
				src := newMusicStreamSource(audioSampleRate)
				src.SetChunk(chunk)
				out := make([]byte, audioSampleRate*4)
				for offset, step := 0, 0; offset < len(out); step++ {
					size := sizes[step%len(sizes)]
					if size > len(out)-offset {
						size = len(out) - offset
					}
					n, err := src.Read(out[offset : offset+size])
					if err != nil || n != size {
						t.Fatalf("Read: count=%d, error=%v", n, err)
					}
					offset += n
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(out)); got != wantHashes[song] {
					t.Fatalf("sizes=%v PCM hash=%s want %s", sizes, got, wantHashes[song])
				}
			}
		})
	}
}

func TestMusicStreamSetChunkDiscardsPartialFrame(t *testing.T) {
	for _, song := range []string{"silence", "intro"} {
		t.Run(song, func(t *testing.T) {
			src := newMusicStreamSource(audioSampleRate)
			src.SetChunk(musicStreamTestChunk(t, "menu"))
			// Leave three bytes of a partially consumed frame in the stream.
			if _, err := src.Read(make([]byte, 65537)); err != nil {
				t.Fatal(err)
			}
			chunk := musicStreamTestChunk(t, song)
			src.SetChunk(chunk)
			got := make([]byte, 8192)
			if _, err := src.Read(got); err != nil {
				t.Fatal(err)
			}
			fresh := newMusicStreamSource(audioSampleRate)
			fresh.SetChunk(chunk)
			want := make([]byte, len(got))
			if _, err := fresh.Read(want); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("switching chunks retained PCM from the previous song")
			}
		})
	}
}

func BenchmarkMusicStreamRead(b *testing.B) {
	for _, song := range []string{"silence", "menu", "intro"} {
		b.Run(song, func(b *testing.B) {
			chunk := musicStreamTestChunk(b, song)
			for _, size := range []int{8192, 8191} {
				b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
					src := newMusicStreamSource(audioSampleRate)
					src.SetChunk(chunk)
					out := make([]byte, size)
					if _, err := src.Read(out); err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.SetBytes(int64(size))
					b.ResetTimer()
					for b.Loop() {
						if _, err := src.Read(out); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
