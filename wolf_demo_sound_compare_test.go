package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gd-wolf/internal/wl6"
)

func TestDemoFaceSoundUsesSimulationClock(t *testing.T) {
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	sound, err := newWolfDemoSound(files, "adlib-digi")
	if err != nil {
		t.Fatal(err)
	}
	g := &game{files: files, rng: newWolfRNG(0), demoPlayback: &wolfDemoPlayback{sound: sound}}
	// With no audio device or voices, the original synthesized channel still
	// suppresses face RNG. Digitized gunshots do not interrupt that channel.
	g.playSound(soundPickupChaingun)
	g.playSound(soundPistol)
	g.updateDemoFace(4, g.isSoundPlaying(soundPickupChaingun))
	if g.rng.index != 0 || g.demoPlayback.faceCount != 0 {
		t.Fatal("headless/digitized playback failed to suppress face RNG")
	}
	sound.service(sound.remaining - 1)
	if !g.isSoundPlaying(soundPickupChaingun) {
		t.Fatal("effect ended before its last 140 Hz service step")
	}
	sound.service(1)
	g.updateDemoFace(4, g.isSoundPlaying(soundPickupChaingun))
	if g.rng.index == 0 || sound.remaining != 0 {
		t.Fatal("face RNG did not resume exactly when the effect ended")
	}
}

func TestWolfDemoSoundCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_DEMO_RUNTIME_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_demo_runtime_compare.sh")
	}
	files, err := wl6.OpenEmbeddedShareware()
	if dir := os.Getenv("GDWOLF_DEMO_RUNTIME_DATA"); dir != "" {
		files, err = wl6.Open(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"off", "adlib", "adlib-digi"} {
		t.Run(mode, func(t *testing.T) {
			configureWolfDemoReferenceSound(t, files, mode, 0, t.TempDir())
			p := startWolfSourceBinary(t, path, "--sound-probe")
			sound, err := newWolfDemoSound(files, mode)
			if err != nil {
				t.Fatal(err)
			}
			g := &game{rng: newWolfRNG(0), demoPlayback: &wolfDemoPlayback{sound: sound}}
			checks := 0
			step := func(action string, amount int) {
				t.Helper()
				line := action
				if action != "reset" {
					line += fmt.Sprintf(" %d", amount)
				}
				if _, err := fmt.Fprintln(p.in, line); err != nil {
					t.Fatal(err)
				}
				if err := p.in.Flush(); err != nil {
					t.Fatal(err)
				}
				switch action {
				case "reset":
					sound.sound, sound.priority, sound.remaining = 0, 0, 0
					g.rng = newWolfRNG(0)
					g.demoPlayback.faceCount, g.demoPlayback.faceFrame = 0, 0
				case "play":
					sound.playRaw(amount)
				case "advance":
					sound.advance(amount)
				case "service":
					sound.service(amount)
				case "face":
					g.updateDemoFace(amount, g.isSoundPlaying(soundPickupChaingun))
				}
				if !p.out.Scan() {
					t.Fatalf("original sound reference stopped at %q: %v", line, p.out.Err())
				}
				var want struct {
					Sound     wolfDemoRuntimeSound `json:"sound"`
					RNG       int                  `json:"rng_index"`
					FaceTimer int                  `json:"face_timer"`
					FaceFrame int                  `json:"face_frame"`
				}
				if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
					t.Fatal(err)
				}
				if got := captureDemoRuntimeSound(g); got != want.Sound || int(g.rng.index) != want.RNG || g.demoPlayback.faceCount != want.FaceTimer || g.demoPlayback.faceFrame != want.FaceFrame {
					t.Fatalf("step %d %q: original=%+v port sound=%+v rng=%d face=%d/%d", checks, line, want, got, g.rng.index, g.demoPlayback.faceCount, g.demoPlayback.faceFrame)
				}
				checks++
			}
			// Every sound competing with GETGATLINGSND covers lower/equal/higher
			// priorities and digital routing. Test expiry on exact service edges.
			for raw := 0; raw < files.Variant.StartMusicChunk/3; raw++ {
				effect, err := files.LoadAdLibSound(raw)
				if err != nil {
					t.Fatal(err)
				}
				if effect.Length == 0 {
					continue // Original SD_PlaySound rejects uncached/empty effects.
				}
				step("reset", 0)
				step("play", 38)
				step("face", 4)
				step("advance", 1)
				step("play", raw)
				step("face", 4)
				step("service", maxInt(0, sound.remaining-1))
				step("face", 4)
				step("service", 1)
				step("face", 4)
				step("play", raw)
				step("advance", 2)
				step("play", raw) // Equal priority restarts the same sound.
				step("play", 38)
				step("advance", 4)
				step("face", 4)
			}
			t.Logf("matched %d original sound-priority, expiry and face-RNG states", checks)
		})
	}
}
