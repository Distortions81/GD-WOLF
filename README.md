# GD-WOLF

`GD-WOLF` is a Go/Ebiten Wolfenstein 3D port focused on gameplay parity with the original game while also adding a few modern conveniences for native and browser builds.

[PLAY IN BROWSER NOW](https://m45sci.xyz/u/dist/GD-WOLF/)

## Screenshot

![GD-WOLF E1F1 screenshot](e1f1.png)

## Current Status

The project currently includes:

- Wolfenstein 3D-style CPU ray casting with GPU-accelerated classic pixel rendering and a software fallback
- native and wasm builds
- native TOML config persistence for volume and input settings
- weapon pickups and map/loadout flow aligned more closely with Wolfenstein 3D
- pause/options menus, cheats, secret push walls, elevators, and level transitions
- gameplay flashes and a much closer Wolfenstein 3D death sequence
- native stereo positional sound, with simplified wasm audio
- actor collision and timing work updated to use Wolf-style tics instead of assuming a fixed render/update rate

Implementation is still incomplete. Guards, dogs, officers, SS, Hans Grosse,
and mutants are active in gameplay, but complete enemy AI parity has not been
verified. Mutant support remains partial, and several later enemies are missing.
Enemy implementation and verification work is tracked in
[enemy-parity.md](enemy-parity.md).

### Enemy AI Verification

The 2026-10-03 original-source audit matched **894,444 isolated movement
decisions**, with zero differences after fixing fallback direction selection,
blocked movement and humanoid locked-door handling. The reference compiles
the original C `TryWalk`, `SelectChaseDir`, `SelectDodgeDir`, and `SelectRunDir`
routines and compares them with the port on map snapshots and local obstruction
cases. Actor classes in this comparison are normalized to humanoid or dog.
An additional registered-data scan matched 15,024 isolated original-C decisions
across all 60 maps. A broader Go source-rule adapter matched 30,036 sampled
sight, attack-entry, movement and runtime checks on those maps.

This result verifies the tested movement decisions. It does not establish
complete AI parity over time: sight and hearing, state timing, attacks, damage,
and interactions across the full roster and map set still need independent
original-engine comparison. Go regressions cover some of these behaviors, but the broader AI
harness uses a Go reference adapter rather than the original engine. Recorded
demo playback and a source runtime comparison are now available. The runtime
reference matches the first built-in demo through its original death endpoint.
Original x86 raycasting matches every floor mask, and the shipped DOS executable
confirms health/ammo after every command and the same endpoint. Other demos and
the full enemy roster remain unverified.

## Project Additions Beyond Wolfenstein 3D

Alongside parity work, this port also includes some modern extras that are outside the original Wolfenstein 3D scope:

- selectable `DOS`, `HQ`, and `ULTRA` render modes, plus VSync control
- persistent native config storage for input/audio/render settings
- save slot previews with embedded thumbnails, plus browser save persistence on wasm
- a textured map-view mode for inspecting levels outside the original presentation

## Intentional Gameplay Deviations

Some behaviors still intentionally diverge from Wolfenstein 3D for feel or clarity.

### Doors

`Options > Modern Doors` enables a thinner center collision slab and prevents
sliding along it when pushing diagonally into a door. It defaults to On for
interactive play. Turn it Off for the original solid door tile and sliding rules.

The native setting is stored as `modern_doors` under `[gameplay]` in
`config.toml`. Demo playback always uses the original door rules, regardless
of this preference, and preserves the saved setting.

## Known Differences From Full Wolfenstein 3D Parity

- The project does not yet implement the full original enemy roster and behaviors.
- Complete enemy AI parity is unverified; the original-source runtime harness matches all four recorded shareware demos and all four recorded Apogee registered demos, not every path through the six episodes.
- The separate demo player movement harness shares the port's world snapshots with its C reference. The runtime harness independently evolves actors, combat, doors and pickups, with visible-floor masks audited against original x86 code.
- Some systems are Wolfenstein 3D-inspired rather than byte-faithful, especially presentation details around fades, flashes, and frontend behavior.

## Data

The project expects original Wolfenstein 3D v1.4-era data layouts. Older Wolf revisions are not supported.

The repo already includes the canonical shareware `WL1` v1.4 data under [internal/wl6/shareware](/home/dist/github/GD-WOLF/internal/wl6/shareware). For registered/full data, use matching `WL6` v1.4 files. The Apogee six-episode release uses a different graphics chunk layout from the pinned registered source headers; the loader detects both layouts. Run it with `go run . -data /path/to/WL6-files`.

When digitized sounds are present in the data files, they are preferred over synthesized fallback effects.

If no local data directory is found, the runtime falls back to embedded shareware assets. That fallback also powers the browser build.

For local native runs, optional runtime PNG replacements can be placed under [hd-assets](/home/dist/github/GD-WOLF/hd-assets/README.md). Matching filenames override decoded pictures, sprites, and wall textures without changing the underlying game data files.

## Build and Test

Build the native game and run the Go tests with:

```bash
go build -o gd-wolf .
go test ./...
```

Go resolves the pinned dependencies from `go.mod`. The palette test uses the embedded fixture included in this repo.

On headless Linux, run the tests under a virtual display with `xvfb-run -a go test ./...` so Ebiten can initialize.

Browser save import/export regression tests use Node.js 22 or later:

```bash
node web/wasm/save-actions.test.cjs
```

Compare movement decisions against compiled original Wolf3D C routines with:

```bash
./scripts/wolf_source_compare.sh
```

See [the original-source harness](docs/wolf-source-compare.md) for source setup,
trace artifacts, replay, and its coverage limits. This command checks isolated
movement decisions, not a full gameplay demo. The existing broader
[AI harness](docs/enemy-ai-harness.md) uses a Go reference adapter.

## Demo Playback and Comparison

Play the first built-in demo from the bundled shareware data:

```bash
go run . -data internal/wl6/shareware -demo-index 0
```

Indices 0–3 select the four built-in demos. To play a recorded demo file using
the selected game data, use `-demo-file /path/to/demo.wl1`. Playback ends when
the input finishes or the port reaches death/victory.

Compare recorded player movement with compiled original C:

```bash
./scripts/wolf_demo_player_compare.sh --stop-after-commands 1055
```

The first demo's first 1,055 commands (4,220 Wolf tics) match within this
conditional player-movement comparison. The unrestricted run stops at port
death before command 1,055. The broader original-C runtime comparison also
passes through the matching original death after command 1,054. The bundled DOS
executable also dies at tic 4,220, with 0 health and 41 ammo; it leaves the last
97 recorded commands unread. Health/ammo match DOS after every played command,
and original x86 raycasting matches all 4,321,280 floor-visibility bits.
See [demo comparison coverage and artifacts](docs/wolf-demo-compare.md).

Actor initialization on all four demo maps and conditional face-animation RNG
updates also match compiled original C:

```bash
./scripts/wolf_demo_start_compare.sh
```

Compare actor state timing and the first-demo runtime with compiled original C:

```bash
./scripts/wolf_actor_states_compare.sh
./scripts/wolf_demo_runtime_compare.sh
```

The state timing and first-demo runtime comparisons pass. The runtime script
also executes the original 16-bit raycaster under QEMU. Matching original death
is a supported playback endpoint; a differing terminal state fails.

Optionally repeat the shipped DOS executable check (Linux, DOSBox, about two minutes):

```bash
./scripts/wolf_demo_dos_compare.sh
```

## Browser Build

`GD-WOLF` also has a browser build. To build it locally:

```bash
./scripts/build_wasm.sh
```

The script writes fresh browser assets, including `gdwolf.wasm.gz`, into `build/wasm`. It requires:

- `wasm_exec.js` from your local Go toolchain
- optional `wasm-opt` on `PATH` for automatic optimization (`-O4` by default, override with `WASM_OPT_LEVEL`)

To serve the build locally:

```bash
go run ./cmd/wasmserve
```

By default `cmd/wasmserve` serves the current directory if it already contains the built app; otherwise it falls back to `build/wasm` and listens on `:8000`.
