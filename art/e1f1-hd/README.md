# First-level HD sprite pack

The complete sprite set used by the embedded WL1 demo's first level (map index 0): **136 shapes with higher-resolution, alpha-transparent PNG copies**.

| Group | Shapes | Count | Canvas |
| --- | --- | ---: | --- |
| Scenery and pickups | Listed in `requirements.json` | 27 | 256 × 256; existing lamp 1024 × 1024 |
| Guard animations | 050–098 | 49 | 256 × 256 |
| Dog animations | 099–137 | 39 | 256 × 256 |
| Map player marker | 408 | 1 | 256 × 256 |
| Player weapons | 416–435 | 20 | 512 × 512 |

Coverage includes every difficulty, all directional standing/movement poses, pain and death frames, enemy drops, and all weapons the player can carry. The first level has guards and dogs; its map does not spawn other enemy types or the victory sequence. HUD pictures and wall textures are separate asset types.

`sprites/` contains runtime filenames. Guard and weapon frames are copied from the approved experiments in `art/guard-hd/v1` and `art/player-weapons-hd/v1`, including the corrected machine-gun and chaingun anatomy. Shape 016 preserves the existing hanging-lamp artwork from `hd-assets/sprites/shape-016.png` byte for byte. The other 66 frames were generated with the **built-in image_gen tool**; `prompts.json` records the complete prompt set, including correction attempts. The selected first scenery atlas is the initial generation, which preserves the detached chandelier shadow; its surrounding color haze is stored in alpha-zero pixels and is invisible when composited.

`atlases/` retains the selected generated sheets. `references/sprites/` contains decoded original 64 × 64 frames. `register_sprite_frames.go` separates the sheets at transparent gutters and registers their occupied bounds to the original logical canvas, retaining the generated color and alpha. The chandelier and its detached shadow are registered separately. The extra-life medallion keeps its original medallion bounds; its new artwork omits the original detached floor line. `registrations.json` records every new frame's source and target rectangles.

Regenerate the new frame files from the saved atlases:

```sh
go run ./art/e1f1-hd/register_sprite_frames.go
```

All 136 frames have been installed locally in `hd-assets/sprites/`. The existing lamp is unchanged. Native **ULTRA** mode uses the overrides when HD textures are enabled. To install this saved pack in another checkout without replacing any custom sprite files:

```sh
mkdir -p hd-assets/sprites
cp --update=none art/e1f1-hd/sprites/*.png hd-assets/sprites/
```

The pack is preserved under `art/` because the local `hd-assets/` directory is ignored by Git. The browser game does not currently package filesystem HD overrides.

`TestFirstLevelHDSpriteCoverage` derives required shapes from the embedded demo map and current runtime catalog, checks every PNG's size and transparency, and loads the complete pack through the actual ULTRA override loader. Run it with a display available (or Xvfb on Linux):

```sh
xvfb-run -a go test . -run TestFirstLevelHDSpriteCoverage -count=1
```

To refresh the map-derived inventory:

```sh
GDWOLF_SPRITE_INVENTORY=art/e1f1-hd/requirements.json xvfb-run -a go test . -run TestFirstLevelHDSpriteCoverage -count=1
```

Open `preview.html` through a local server to compare every frame and play dog animations. It also links to the guard and player-weapon animation previews. These are initial HD art copies with the approved smooth illustrated look; some generated edges retain faint color fringes that are easiest to inspect against the preview's light background.
