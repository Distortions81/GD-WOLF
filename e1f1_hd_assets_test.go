package main

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gd-wolf/internal/wl6"
)

type firstLevelSpriteRequirement struct {
	Shape int      `json:"shape"`
	Uses  []string `json:"uses"`
}

// Derive coverage from the shipped demo map and the runtime catalog. Include all
// difficulty levels, directional frames, both pain poses, conditional drops,
// carried weapons, and the map's player marker.
func firstLevelSpriteRequirements(t *testing.T) []firstLevelSpriteRequirement {
	t.Helper()
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	setSpriteCatalogVariant(files.Variant)
	t.Cleanup(func() { setSpriteCatalogVariant(wl6.VariantSpec{Ext: "WL6"}) })
	level, err := files.LoadMap(0)
	if err != nil {
		t.Fatal(err)
	}
	uses := map[int]map[string]bool{}
	add := func(shape int, use string) {
		if shape < 0 {
			return
		}
		if uses[shape] == nil {
			uses[shape] = map[string]bool{}
		}
		uses[shape][use] = true
	}
	addSequence := func(id AnimSequenceID, rotate bool) {
		if id == "" {
			return
		}
		seq, ok := LookupAnimSequence(id)
		if !ok {
			t.Fatalf("missing runtime sequence %s", id)
		}
		for _, frame := range seq.Frames {
			count := 1
			if rotate {
				count = 8
			}
			for direction := 0; direction < count; direction++ {
				add(frame.Shape+direction, string(id))
			}
		}
	}
	for _, info := range level.Planes[1] {
		if def, ok := LookupStatic(info); ok {
			add(def.Shape, fmt.Sprintf("map info %d", info))
			continue
		}
		if info == wolfExitTile {
			addSequence(seqVictoryBJRun, false)
			addSequence(seqVictoryBJJump, false)
		}
		def, ok := LookupActorSpawn(info, difficultyHard)
		if !ok {
			continue
		}
		addSequence(def.StandSequence, def.Rotate)
		addSequence(def.PatrolSequence, def.Rotate)
		addSequence(def.ChaseSequence, def.Rotate)
		for _, id := range []AnimSequenceID{def.PainSequence, def.ShootSequence, def.JumpSequence, def.DeathSequence} {
			addSequence(id, false)
		}
		g := &game{}
		for health := 1; health <= 2; health++ {
			addSequence(g.actorPainSequence(&actorInstance{kind: def.Kind, health: health, painSeq: def.PainSequence}), false)
		}
		for best := 0; best <= 3; best++ {
			pickup, _ := def.ResolveDrop(best)
			if drop, ok := StaticDefForPickup(pickup); ok {
				add(drop.Shape, "enemy drop")
			}
		}
	}
	for weapon := 0; weapon < len(activeWeaponAnimDefs); weapon++ {
		def, ok := WeaponAnim(weapon)
		if !ok {
			t.Fatalf("missing weapon %d", weapon)
		}
		add(def.ReadyShape, fmt.Sprintf("weapon %d ready", weapon))
		addSequence(def.AttackSequence, false)
	}
	add(mapPlayerShape, "map player marker")
	var requirements []firstLevelSpriteRequirement
	for shape, shapeUses := range uses {
		entry := firstLevelSpriteRequirement{Shape: shape}
		for use := range shapeUses {
			entry.Uses = append(entry.Uses, use)
		}
		sort.Strings(entry.Uses)
		requirements = append(requirements, entry)
	}
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].Shape < requirements[j].Shape })
	return requirements
}

func TestFirstLevelHDSpriteCoverage(t *testing.T) {
	requirements := firstLevelSpriteRequirements(t)
	if output := os.Getenv("GDWOLF_SPRITE_INVENTORY"); output != "" {
		data, err := json.MarshalIndent(requirements, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("inventoried %d first-level sprite shapes", len(requirements))
		return
	}
	files, err := wl6.OpenEmbeddedShareware()
	if err != nil {
		t.Fatal(err)
	}
	sprites, err := files.LoadSpriteSet()
	if err != nil {
		t.Fatal(err)
	}
	g := &game{sprites: sprites, hdAssetRoot: filepath.Join("art", "e1f1-hd"), hdTexturesEnabled: true, renderMode: renderModeUltra}
	g.applyHDSpriteOverrides()
	for _, requirement := range requirements {
		path := filepath.Join("art", "e1f1-hd", "sprites", fmt.Sprintf("shape-%03d.png", requirement.Shape))
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("shape %d (%v): %v", requirement.Shape, requirement.Uses, err)
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if img.Bounds().Dx() < 512 || img.Bounds().Dy() < 512 {
			t.Errorf("%s is an older undersized replacement: %v", path, img.Bounds())
		}
		visible, clear := 0, 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				_, _, _, alpha := img.At(x, y).RGBA()
				if alpha == 0 {
					clear++
				}
				if alpha >= 32768 {
					visible++
				}
			}
		}
		if visible == 0 || clear == 0 {
			t.Errorf("%s lacks visible sprite content or transparency", path)
		}
		// Exercise the runtime's sprite conversion, not just PNG decoding.
		sprite := buildSpriteFromImage(img)
		if len(sprite.Columns) != img.Bounds().Dx() {
			t.Errorf("%s: invalid runtime column count", path)
		}
		loaded, ok := g.sprites.SpriteByShape(requirement.Shape)
		if !ok || loaded.IndexedFormat || loaded.Width != img.Bounds().Dx() || loaded.Height != img.Bounds().Dy() {
			t.Errorf("%s: HD override did not load into the runtime sprite set", path)
		}
	}
}
