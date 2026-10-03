# First-level HD sprite pack

The complete sprite set used by the embedded WL1 demo's first level (map index 0): **136 shapes with higher-resolution, alpha-transparent PNG copies**.

| Group | Shapes | Count | Canvas |
| --- | --- | ---: | --- |
| Scenery and pickups | Listed in `requirements.json` | 27 | 512 × 512; hanging fixtures 1024 × 1024 |
| Guard animations | 050–098 | 49 | 512 × 512 |
| Dog animations | 099–137 | 39 | 512 × 512 |
| Map player marker | 408 | 1 | 512 × 512 |
| Player weapons | 416–435 | 20 | 512 × 512 |

Coverage includes every difficulty, all directional standing/movement poses, pain and death frames, enemy drops, and all weapons the player can carry. The first level has guards and dogs; its map does not spawn other enemy types or the victory sequence. HUD pictures and wall textures are separate asset types.

`sprites/` contains the selected runtime filenames. The accuracy audit replaced the older guard, dog, scenery and map-marker sheets with transparent revisions in `revision-2/atlases/`. Guard corrections preserve the approved compact character while restoring boot/corpse contours; dog revisions restore lean canine anatomy and original directional/gait references. Scenery cutouts omit opaque gray floor patches and external lighting halos, and the standing flag is plain red. `revision-2/prompts.json` records exact built-in image-generation requests, selected source paths and rejected draft attempts. The older `atlases/` sheets remain historical references, not extraction inputs.

Weapon artwork keeps the approved arm/hand corrections from `art/player-weapons-hd/v1`; it has been registered again with smooth alpha-aware filtering and copied into this pack.
Hanging fixtures 006 and 016 now use transparent fixture-only edits saved in `edits/`. Their old baked floor patches and light cones have been removed. The hanging lights and standing lamp use a subtle warm glow projected onto the actual floor, with camera perspective and wall occlusion. `edits/prompts.json` records the two edit prompts; `edits/references/` preserves the earlier sprites.

`revision-2/atlases/` retains the selected generated sheets. `references/sprites/` contains decoded original 64 × 64 frames. `register_sprite_frames.go` separates the sheets at transparent gutters and registers their occupied bounds to the original logical canvas. It uses alpha-aware premultiplied filtering for every cutout, with fixture-only edits for both hanging lights. Physical scenery bounds and dog paws are registered without stretching the removed original shadow into the body. The extra-life medallion keeps its original medallion bounds; its new artwork omits the original detached floor line. `registrations.json` records the source and target rectangles.

Regenerate the 116 scenery, actor and marker frames, then register and copy the 20 approved weapon frames:

```sh
go run ./art/e1f1-hd/register_sprite_frames.go
go run ./art/player-weapons-hd/v1/register_weapon_frames.go
cp art/player-weapons-hd/v1/sprites/*.png art/e1f1-hd/sprites/
```

All 136 frames have been installed locally in `hd-assets/sprites/`. Native **ULTRA** mode uses the overrides when HD textures are enabled. To install the selected revisions, replacing older copies with these same filenames:

```sh
mkdir -p hd-assets/sprites
cp art/e1f1-hd/sprites/*.png hd-assets/sprites/
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

Open `preview.html` through a local server to compare every frame and play dog animations. It also links to the guard and player-weapon animation previews. The renderer composites translucent edges over the actual scene, draws overlapping sprites by camera depth, and uploads premultiplied colors to Ebiten.
