# HD wall textures — first batch

Four selected textures at **1254 × 1254**, fully opaque:

| Tile | Design |
| --- | --- |
| 01 | Gray stone, original broad block arrangement with continuous curved outlines |
| 02 | Approved rough gray stone surface and strong chiseled bevels, with large stepped outline notches replaced by natural fracture edges |
| 03 | Plain red cloth banner on corrected gray stone, requested replacement |
| 12 | Warm brown vertical wood paneling |

Tile02 now follows the user's reference in `references/stone-user-style.png`: rough pitted faces, cracks and strong chiseled bevels are preserved, while large coarse staircase notches are reshaped into natural irregular fracture contours. The rounded-block revision is retained in `previous/tile-02-rounded-blocks.png`. The base stone wall (tile01) and banner background (tile03) still need to be brought into this newly approved direction. No low-resolution image was enlarged to make a final texture.

Final textures are in `walls/horizontal/` and are copied into `hd-assets/walls/horizontal/` for native ULTRA with HD textures enabled. The existing loader creates the second wall orientation at `wl6.WallShadeScale` (0.66 brightness). Existing blue HD walls are preserved. This batch does not complete all first-level wall designs.

Open `preview.html` to compare originals, previous drafts, corrected textures, horizontal repeats and shaded orientations. The HD previews use smooth browser sampling; only the original 64px references use nearest-neighbor sampling. Repetition is shown for inspection; these are individual wall textures, not a guarantee of identical pixels at every boundary.

`prompts.json` records the selected built-in image-generation prompts. `remake_stone.go` reproduces earlier 1024px contour guides. `remake_wood.go` reproduces a directly drawn wood alternative; the generated wood is selected for its more natural grain. Earlier wall drafts are in `previous/`.

Run the direct drawing tools from the repository root:

```sh
GOCACHE=/tmp/gdwolf-site-build-cache go run art/walls-hd/v1/remake_stone.go
GOCACHE=/tmp/gdwolf-site-build-cache go run art/walls-hd/v1/remake_wood.go
```

Validation: the approved tile02 revision is 1254 × 1254 with every pixel opaque and is installed in the native override directory. The prior batch passed the real native wall override loader checks and existing conversion, shading and override tests. The repeat preview was visually inspected for that batch. `git diff --check` passed.
