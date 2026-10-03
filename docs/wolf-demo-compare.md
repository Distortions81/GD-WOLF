# Recorded demo comparison

This is the first gameplay replay stage after the isolated
[original C movement audit](wolf-source-compare.md). It feeds recorded inputs
to the actual port, including its doors, weapons, actors and pickups, and
compares player movement to compiled original C routines.

## Current scope

The reference compiles `ControlMovement`, `Thrust`, `ClipMove` and `TryMove`
from the pinned original `WL_AGENT.C`. It also uses the original sine-table
generation from `WL_MAIN.C`. Source hashes are verified before compilation.
The DOS signed-magnitude `FixedByFrac` assembly is implemented as equivalent
integer arithmetic in the host bridge. Tagged pointers are compared as
`uintptr_t` values on modern hosts.

The reference carries an independent player position, integer angle and turn
remainder across commands. It receives the **port's world snapshot** after
doors/use and before each player move: blocking tiles/statics, closed doors,
and shootable actor reservations and positions. This is a conditional player
movement comparison. Sharing that world does not establish parity of enemy
AI, combat, door timing, pickups, RNG consumption or terminal states.

Traces sample after each recorded command, which advances four 70 Hz Wolf
tics. They do not compare intermediate single-tic states. Rendering and its
visibility/targeting effects are not independently reproduced.

## Playback

```bash
go run . -data internal/wl6/shareware -demo-index 0
go run . -data internal/wl6/shareware -demo-file /path/to/demo.wl1
```

The four embedded demos use E1F1, E1F3, E1F5 and E1F7, with 1,152, 1,284, 671
and 633 commands respectively. Playback starts on hard difficulty with RNG
index zero before spawning actors and uses the original integer controls. Timed
initial actor states consume RNG in map order, as in original `SpawnNewObj`.
The first demo therefore starts gameplay at RNG index 9. Each command contains a
button bitmask and signed X/Y controls. The header's fourth byte is unused;
it may be nonzero. Malformed lengths and unavailable maps are rejected.

Recorded playback always uses original tile-solid door collision and sliding,
even when `Options > Modern Doors` is On. The saved interactive preference is
preserved. Interactive play keeps modern doors by default; native config can
select the original rules with:

```toml
[gameplay]
modern_doors = false
```

Playback ends at demo completion or port death/victory. These terminal outcomes
are not independently verified by the player reference.

## Compare and replay

```bash
./scripts/wolf_demo_player_compare.sh
./scripts/wolf_demo_player_compare.sh --stop-after-commands 1055
./scripts/wolf_demo_player_compare.sh --demo-index 1 --out /tmp/wolf-demo-2
./scripts/wolf_demo_player_compare.sh --self-test
```

Options also accept `--source` for an existing pinned checkout, `--demo-file`
for a recorded demo using the embedded shareware maps, and `--out` for artifacts.
Without `--source`, the script fetches the same pinned source as the movement
harness into ignored `build/wolfsrc-source`. Requirements are Go, Python 3,
a C99 compiler, Git for download, and Xvfb on headless Linux.

The default comparison stops at the first player mismatch or unsupported
terminal state and exits nonzero. A prefix comparison succeeds only for the
requested commands. Original C checks skip in ordinary `go test ./...` unless
the reference environment is configured; parser, playback, door option and
collision regressions run normally.

Default artifacts in `build/wolf-demo-player-compare`:

| File | Contents |
| --- | --- |
| `build.log`, `compare.log` | Compiler and comparison output |
| `commands.jsonl` | Exact decoded inputs and command/tic numbers |
| `reference.jsonl`, `port.jsonl` | Player state after movement, before enemy updates |
| `reference-input.txt` | Initial player state and every command/world snapshot sent to C |
| `port-runtime.jsonl` | Port health, ammo, RNG, doors and actor state after command completion |
| `mismatch.jsonl` | First differing player state; empty at an unsupported terminal boundary |
| `result.json` | Matched prefix length and success, mismatch or unsupported-terminal status |

Reproduce the C side without Go or a display:

```bash
build/wolf-demo-player-compare/wolf-demo-player-reference \
  < build/wolf-demo-player-compare/reference-input.txt
```

## First demo findings (2026-10-03)

1. Command 1, ending at tic 8: continuous trigonometry differed by one fixed
   unit. Playback now matches the original float-accumulated sine table,
   signed-magnitude multiplication and truncation.
2. Command 57, ending at tic 232: X-first collision rejected a diagonal move
   clearing a wall corner. Player movement now tests the whole step before
   trying axis sliding, matching original `ClipMove`.
3. Command 248, ending at tic 996: modern door behavior prevented the original
   slide. Playback now forces original door collision and sliding.

4. Before command 0: nine patrol spawns skipped the original random initial
   timers, leaving RNG at index 0 instead of 9. Playback now consumes those
   bytes and preserves the original zero-timer state until a state change.
5. Every player command: the face animation omitted its shared gameplay RNG
   draws. Playback now advances the face before use, weapons and movement,
   as in original `T_Player`/`T_Attack`.

6. Actor updates used the interactive animation scheduler: actions ran on
   entry, and AI ran during pause frames. Demo playback now follows original
   `DoActor`, carries elapsed time across state transitions, and uses the
   original actor speed per Wolf tic. It also reserves the next tile on exact
   arrivals, retains reaction counters when damage alerts an actor, accepts
   zero-damage hits, and delays death-scream RNG until the state action.
7. Demo shots now use cached previous-command projections with the original
   fixed-point geometry and reserved-tile damage distance. Demo visibility
   follows the original floor-ray traversal and nine-tile actor visibility
   checks. The ray traversal now matches original 16-bit x86 execution under QEMU
   for every command in the first demo.
8. Opening doors had an artificial 0.01 head start and connected areas before
   their first movement. Demo playback now starts opening at zero and connects
   areas once the door moves.

9. Command 125: a fresh attack/use press during `T_Attack` must be suppressed
   and remain unheld on the next command. Storing the raw button bits prevented
   a later attack from starting. Playback now preserves the suppressed input.
10. Range misses were calling `DamageActor(0)`, alerting and stunning enemies.
    Playback now distinguishes a miss from a real hit that rolls zero damage.
11. Command 757: using an opening door forced it fully open. Playback now follows
    `OperateDoor`/`CloseDoor`, including original closing occupancy checks.
12. Command 443: a moving pushwall did not reserve its leading tile. Demo
    pushwalls now block both tiles and clear the trailing tile at each crossing.
13. Demo pickups now use original `TransformTile` reach on visible floor tiles.
    The default logical viewport is 240 pixels (`viewsize 15 * 16`), and cached
    actor visibility survives the original too-close projection early return.

The reference self-test matches all 360 movement angles plus strafing, reverse
movement, speed clamps and fractional turns. The first 1,055 commands (4,220
tics) now match conditional player movement, up from the previous 907-command
checkpoint. The port dies before command 1,055 (zero-based), so the unrestricted
1,152-command player-only run still stops at this unsupported boundary. The
broader runtime comparison now passes with a matching original death after
command 1,054. The bundled DOS executable independently confirms that endpoint
at tic 4,220, with 0 health and 41 ammo, and matches health/ammo after all 1,055
played commands. The original demo never consumes its remaining 97 commands;
those commands are not part of the verified gameplay run.

## Actor initialization and face RNG comparison

```bash
./scripts/wolf_demo_start_compare.sh
./scripts/wolf_demo_start_compare.sh --out /tmp/wolf-demo-start
```

The output directory contains `build.log`, `compare.log`, and each demo's
`demo-N-reference.json`, `demo-N-port.json` and `demo-N-input.txt`. Replay an
initialization directly with `wolf-demo-start-reference < demo-0-input.txt`.

This separate reference receives the two raw decoded map planes, rather than
port actor snapshots. It compiles original `ScanInfoPlane`, `SpawnNewObj`,
`SpawnStand`, `SpawnPatrol`, `SpawnDeadGuard`, `SpawnBoss`, `SpawnDoor` and
`UpdateFace`, including the original sprite, health and random tables. Source
hashes are checked at the same pinned revision as the movement harness. DOS
16-bit map words and tagged pointers are adapted to host types. The bridge's
wall-copy loop mirrors `SetupGameLevel`; player/static spawn callbacks omit
rendering and operations that consume no RNG. Unsupported boss families,
ghosts and the original source's undefined standing-dog spawn fail explicitly.

All 287 active actor starts across the four built-in demo maps match at hard
difficulty: class, fixed position, reserved tile, facing, area, ambush flag,
health, speed, move distance, shape, initial timer and final spawn RNG index.
The initial RNG indices are 9, 12, 28 and 15 respectively. This establishes
initialization parity on these maps, without validating subsequent AI updates.

The same reference checks 4,096 face updates, sharing each call's entry RNG
index and sound-suppression flag while carrying independent face counters.
This isolates the original `UpdateFace` behavior; actor/combat RNG consumption
is still unverified. Native playback uses the port audio state for the
chaingun pickup's suppression and resets the face counter on that pickup.
Original sound priority and timing are not independently reproduced, and
headless comparisons do not simulate audio playback.

## Actor timing and first-demo runtime

```bash
./scripts/wolf_actor_states_compare.sh
./scripts/wolf_demo_runtime_compare.sh
```

Both scripts accept `--source` and `--out`, verify original source hashes and
save `build.log` and `compare.log`. The timing harness compiles original
`DoActor` and 111 state definitions for guards, officers, SS, mutants, dogs and
Hans. Think and action callbacks record invocations without simulating combat.
All 26 supported state sequences match shape, timer, think eligibility and
cumulative attack callbacks for 96 updates with varying tic counts, starting
from full, zero and one remaining tic. Death sequences are not covered by
this isolated test.

The runtime harness compiles original spawning, actor state transitions,
sight/hearing, path/chase movement, damage/death, gun/knife targeting, weapon
attack scheduling, player movement, door/area/pushwall updates, static spawns,
dropped items, bonus collection, face updates and RNG tables. It initializes
from raw map planes and carries independent player/actor positions, reservations,
weapons, doors, walls, pickups, health and RNG across commands. Original
`DrawScaleds` static/actor placement and `TransformActor`/`TransformTile` geometry
are compiled; drawing, audio playback and scoring are stubbed; lives are not compared.
DOS projection height assembly uses equivalent integer division, and tagged
pointers/map words are adapted for host C. Unsupported actor families and
victory behavior fail explicitly.

The runtime now compiles original `T_Player`, `Cmd_Use` and `TakeDamage`, so
use requests and the death flag evolve independently. Its remaining shared input
is the visible-floor mask produced by the port's translation of `WL_DR_A.ASM`.
The runtime script audits those masks by assembling the verified original
raycaster instructions and executing them as 16-bit x86 under QEMU. Assembler
syntax, test-owned segment placement and drawing callbacks are adapted; the
original arithmetic and self-modifying quadrant branches execute unchanged.
Every one of the first demo's 4,321,280 floor bits matches. This audit uses
original-C tile bytes, door positions, pushwall position and view coordinates,
plus original render tables. Audio-priority simulation is still omitted. The
projection self-test matches original render tables and 2,160 actor/pickup
projection cases spanning every integer angle.

All 37 actors, weapons, player movement, door/wall occupancy, outgoing RNG,
enemy damage, cached projections and post-pickup health/ammo currently match
through command 1,054, including the original death flag. The script succeeds
at that matching terminal state. A terminal state that differs from the original
fails. This verifies the first demo's actual playback to its endpoint, rather
than requiring the port to consume commands the original never plays. It does
not establish parity of other demos, unsupported enemy families, audio or scoring.

Default runtime artifacts in `build/wolf-demo-runtime-compare`:

| File | Contents |
| --- | --- |
| `build.log`, `compare.log` | Compiler and comparison output |
| `reference-input.txt` | Raw map, commands, diagnostic snapshots and shared floor masks |
| `reference.jsonl`, `port.jsonl` | Player, actor, weapon, RNG, doors and wall occupancy before rendering |
| `reference-render.jsonl` | Original cached actor projections, post-pickup health/ammo and death flag |
| `raycast-input.jsonl` | Original view coordinates, exact tile bytes, door/pushwall positions and port floor masks |
| `raycast/`, `raycast.log` | Generated original x86 assembly/boot image, visibility bytes and audit result |
| `result.json` | Matched count, unread commands, and success, matched-terminal, mismatch or unsupported-terminal status |

Replay the original C runtime without Go or a display:

```bash
build/wolf-demo-runtime-compare/wolf-demo-runtime-reference \
  < build/wolf-demo-runtime-compare/reference-input.txt
```


## Original raycaster and shipped DOS checks

The runtime script runs the assembly visibility audit automatically. Repeat it
from saved artifacts without Go or a display:

```bash
./scripts/wolf_demo_raycast_compare.sh
./scripts/wolf_demo_raycast_compare.sh --runtime /tmp/wolf-demo-runtime --out /tmp/wolf-raycast
```

The audit needs Python 3, GNU `as`/`ld` and `qemu-system-i386`. It verifies the
pinned `WL_DR_A.ASM` hash, boots a small real-mode test program, and emits original
`spotvis` bytes for comparison. It compares floor traversal only; wall drawing,
heights and textures are omitted. A deliberately changed mask bit was confirmed
to fail at its exact command/tile.

Check the bundled DOS executable independently on Linux:

```bash
./scripts/wolf_demo_dos_compare.sh
./scripts/wolf_demo_dos_compare.sh --dosbox /path/to/dosbox --runtime /tmp/wolf-demo-runtime
```

This optional check takes about two minutes and requires DOSBox, Python 3,
X11/XTest libraries and Xvfb on headless Linux. It verifies the bundled v1.4
executable/map/demo-data hashes, copies the game into a fresh temporary directory
with its default viewport, acknowledges the startup prompt and reads only its
own DOSBox child's emulated `gametype`. No software is installed. Artifacts
include `gamestate.jsonl`, `dosbox.log`, `dosbox.conf` and `result.json`.

The first new `TimeCount` value after rendering identifies each completed
command. All 1,055 health/ammo snapshots match the original-C runtime. The DOS
counter stops at 4,220 with health 0 and ammo 41, confirming command 1,054 as the
original terminal command. The demo then returns to the title screen. Playback
of the unused tail, subsequent demos, DOS actor memory, audio behavior and
scoring are not verified by this DOS check.
