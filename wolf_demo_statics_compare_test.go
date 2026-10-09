package main

import (
	"os"
	"testing"
)

type wolfDemoRuntimeStatic struct {
	X          int `json:"x"`
	Y          int `json:"y"`
	Shape      int `json:"shape"`
	Flags      int `json:"flags"`
	Item       int `json:"item"`
	Visibility int `json:"visibility"`
}

func captureDemoRuntimeStatics(g *game) []wolfDemoRuntimeStatic {
	if os.Getenv("GDWOLF_DEMO_STATICS") != "1" {
		return nil
	}
	g.initializeDemoStaticSlots()
	result := make([]wolfDemoRuntimeStatic, 0, len(g.demoPlayback.staticSlots))
	for slot, index := range g.demoPlayback.staticSlots {
		sprite := &g.staticSprites[index]
		info := g.demoPlayback.staticInfo[slot]
		shape := sprite.shapenum
		if !sprite.alive {
			shape = -1
		}
		result = append(result, wolfDemoRuntimeStatic{int(sprite.x), int(sprite.y), shape, int(info.flags), int(info.item), int(info.visibility) - demoDOSVisibilityBase})
	}
	return result
}

func compareDemoStatics(t *testing.T, command int, want, got []wolfDemoRuntimeStatic) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("command %d static slots: original=%d port=%d", command, len(want), len(got))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("command %d static slot %d: original=%+v port=%+v", command, i, want[i], got[i])
		}
	}
}
