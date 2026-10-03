# First-level HD art coverage

Checked against the embedded WL1 first map and runtime asset lookups on 2026-09-30.

All **136 gameplay sprite shapes** are covered: guard and dog animations, scenery and pickups, player weapons, and the map player marker. The accuracy audit replaced older actor/scenery/marker artwork, removed baked floor mattes, restored clipped contours, and registered every sprite on a 512px canvas (hanging fixtures remain 1024px). Selected sources and exact prompts are under `art/e1f1-hd/revision-2/`. Their inventory is in `art/e1f1-hd/requirements.json`. The chandelier, green hanging lamp and standing lamp now contain only the fixture; their soft illumination is projected onto the actual floor by the renderer.

All **13 wall designs** used by the map have HD replacements, plus the activated elevator wall (tile22). Gray tiles01/02 and blue tiles08/09 are matched A/B pairs built as continuous two-tile-wide artwork and checked in A–B–A–B repeats. Decorated variants reuse these corrected materials. The portrait variants use silhouettes, and the eagle decorations use plain red surfaces.

Both elevator switch states have separate HD vertical faces. The map's 20 ordinary doors and two elevator doors use HD page overrides 098, 100 and 102; the loader generates shaded pages099, 101 and 103. These are the `AllPages` entries selected by the door renderer.

Wall variants are composed once in the engine from shared base textures and transparent decorations. Both portraits share one silhouette; blue windows share iron bars. Saved materials, decorations, layouts, controls and doors are in `art/walls-hd/v1/walls/`; sprites are in `art/e1f1-hd/sprites/`. Copy the selected pack to the ignored local `hd-assets/` directory using the respective README instructions. The native HD loader coverage tests exercise the saved packs directly, so stale local copies cannot hide missing tracked artwork.

The in-game HUD still uses the original artwork: status-bar background, health faces, number/blank glyphs, weapon icons and key icons. Its renderer composes into a 320px-wide buffer, so an HD UI pass needs both new pictures and a composition change. Shared menu artwork and bitmap text also remain original.

Floors and ceilings use palette colors and gradients, with no missing texture files. The native ULTRA renderer now keeps their background opaque when compositing translucent sprites. The browser build does not currently package filesystem HD overrides.
