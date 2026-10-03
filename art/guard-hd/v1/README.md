# Brown guard HD experiment — version 1

HD versions of the standard brown guard from E1F1, using the embedded shareware sprites as direct references. The intent is to preserve the original uniform, palette, compact proportions, and animation poses. Blood droplets use rounded contours rather than square pixels. The latest pass reduces the heavy black contours introduced by generation.

## Preview

Open `preview.html` in a browser for animated original/HD comparisons. It works directly from disk or through a local HTTP server. Select an animation and viewing direction; shooting, injury, and death use the original single-view frames.

## Contents

- `movement-no-outline.png`: latest 8-column × 5-row sheet; standing followed by four walking phases, each with eight directions.
- `front-actions-no-outline.png`: latest 4 × 4 sheet; five front-facing standing/walking references, three shooting frames, two hit reactions, four death frames, two empty cells.
- `movement.png` and `front-actions.png`: earlier versions retained for comparison.
- `sprites/shape-050.png` through `shape-098.png`: all 49 runtime override frames on consistent 256 × 256 canvases, registered to the original occupied bounds and foot baselines.
- `references/`: decoded original sprites and the reference sheets supplied to image generation.
- `frames.json`: exact atlas source regions, content bounds, target bounds, and output dimensions for every extracted frame.
- `prompts.json`: the prompt set and built-in generation mode used for this version.

The raw atlases provide roughly 198-pixel movement cells and 314-pixel action cells, compared with the original 64 × 64 canvases. Generated row spacing is uneven, so extraction uses transparent gaps between complete figures rather than a rigid grid. Occupied bounds (alpha ≥ 128) are mapped to four times the original occupied bounds with nearest-neighbor resampling. This keeps the generated colors and alpha values while aligning all frames on 256 × 256 canvases. The renderer normalizes each frame to the same world-space sprite size.

## Runtime mapping

| Shapes | Animation |
| --- | --- |
| 050–057 | Standing, eight directions |
| 058–065 | Walk phase 1, eight directions |
| 066–073 | Walk phase 2, eight directions |
| 074–081 | Walk phase 3, eight directions |
| 082–089 | Walk phase 4, eight directions |
| 090, 094 | Hit reactions |
| 091–093, 095 | Death and corpse |
| 096–098 | Shooting |

To try these in a native build, copy the PNGs from `sprites/` into the repo's ignored `hd-assets/sprites/` directory, enable HD textures, and select `ULTRA` render mode. The browser build does not currently package filesystem HD overrides.

These are experimental generated assets. Atlas layout and pose consistency were reviewed, and every extracted frame was checked for image content and transparent background. Colored edge fringes remain in the generated artwork, and the bottom walking row has some truncated boot detail. These need art cleanup before release; the frames are kept separate from the active `hd-assets` overrides. The preview retains these details for review.

The current preview reads the corrected 512 × 512 runtime frames from `art/e1f1-hd/sprites/`. The original version-1 sheets, frames and registration metadata here are archived experiments. Repeatable extraction of the selected guard restoration is now part of `art/e1f1-hd/register_sprite_frames.go`; its sources and prompts are under `art/e1f1-hd/revision-2/`.
