# Player arms and weapons HD experiment — version 1

Higher-resolution drafts of all four original player weapons: knife, pistol, machine gun, and chaingun. Each includes the ready pose and four attack poses, for 20 frames total. The source is the repo's embedded shareware artwork; weapon shape numbers also match the current registered-data catalog.

The intended look follows the original weapon silhouettes, warm skin, gray sleeves, grayscale pistol/machine gun, and the chaingun's cyan highlights. Contours use material shading rather than heavy black cartoon strokes. Existing muzzle flashes are retained in their corresponding firing frames.

The automatic-weapon revisions match the original limb visibility: the chaingun has no visible hands or arms, and the machine gun has only the small lower-left forearm/hand shown by the source. The initial atlases that added extra limbs are retained under `previous/` for comparison.

## Preview

Open `preview.html` in a browser, directly from disk or through a local HTTP server. Select a weapon, adjust zoom or playback speed, and step through individual frames to compare the original and HD hands, weapons, and recoil.

The 1× zoom uses the game's weapon-overlay size calculation (80% of the preview height). The default 2× zoom enlarges the artwork for detail inspection. The four attack frames use the game's six-tic timing at 70 Hz; a ready-pose hold is inserted between attacks for the preview.

## Files

- `knife.png`, `pistol.png`, `machinegun.png`, `chaingun.png`: generated five-frame horizontal atlases.
- `sprites/shape-416.png` through `shape-435.png`: transparent 512 × 512 frames registered to the original screen positions and occupied bounds.
- `references/`: original 64 × 64 frames and enlarged reference strips.
- `frames.json`: each shape's weapon, phase, source region, source content bounds, and target bounds.
- `prompts.json`: exact prompts, reference roles, and built-in generation mode.
- `register_weapon_frames.go`: repeatable extraction and registration, with image-content/transparency validation.
- `previous/`: initial machine gun and chaingun atlases before the limb corrections.

| Weapon | Ready | Attack frames |
| --- | --- | --- |
| Knife | 416 | 417–420 |
| Pistol | 421 | 422–425 |
| Machine gun | 426 | 427–430 |
| Chaingun | 431 | 432–435 |

## Frame preparation

Run from the repo root:

```sh
go run ./art/player-weapons-hd/v1/register_weapon_frames.go
```

The registration step finds transparent gaps between the five figures in each atlas, then maps each figure's occupied bounds to eight times the corresponding original bounds on a 512 × 512 canvas. It uses nearest-neighbor sampling to retain the generated color/alpha values. Generated background margins are discarded; the original arm cropping, muzzle height, recoil shifts, and screen alignment are restored. This is a canvas size of 8× the original; the amount of recovered detail is determined by the generated artwork, not the canvas dimensions.

Every frame is checked for visible content and transparent background, and atlas splits must pass through transparent gaps. These remain art drafts: fine colored edge fringes and small design/hand differences may need cleanup after visual review. They are saved separately from the active HD overrides.

To try these in a native build, copy the files under `sprites/` into `hd-assets/sprites/`, enable HD textures, and select `ULTRA` mode. Existing replacements should be backed up before copying. Browser builds do not currently package filesystem HD overrides.
