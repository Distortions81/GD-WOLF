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
./scripts/wolf_demo_player_compare.sh --stop-after-commands 907
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
   fixed-point geometry and reserved-tile damage distance. Visibility is still
   approximate and has not been compared to the original renderer.
8. Opening doors had an artificial 0.01 head start and connected areas before
   their first movement. Demo playback now starts opening at zero and connects
   areas once the door moves.

The reference self-test matches all 360 movement angles plus strafing, reverse
movement, speed clamps and fractional turns. The first 907 commands (3,628
tics) now match conditional player movement, up from 371. The port dies before
command 907 (zero-based), so the unrestricted 1,152-command run remains
incomplete. The player reference does not independently simulate enemy damage.

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

## Actor state timing and first-demo AI

```bash
./scripts/wolf_actor_states_compare.sh
./scripts/wolf_demo_ai_compare.sh
```

Both scripts accept `--source` and `--out`, verify original source hashes and
save `build.log` and `compare.log`. The timing harness compiles original
`DoActor` and 111 state definitions for guards, officers, SS, mutants, dogs and
Hans. Think and action callbacks record invocations without simulating combat.
All 26 supported state sequences match shape, timer, think eligibility and
cumulative attack callbacks for 96 updates with varying tic counts, starting
from full, zero and one remaining tic. Death sequences are not covered by
this isolated test.

The AI harness compiles original spawning, state transitions, sight/hearing,
path/chase movement, damage/death, shooting and biting routines. It initializes
from raw map planes and carries independent actor state, positions and tile
reservations. Each command shares port player position, entry RNG, noise,
running/weapon state, area connectivity, doors, visibility and exact
player-inflicted damage. Enemy damage and outgoing RNG are compared along
with all actor fields. OpenDoor requests are stubbed; door/pushwall simulation,
renderer visibility, player targeting, pickups and scoring are not independent.
Unsupported actor families fail explicitly.

Currently all 37 actors and enemy damage match through command 881 under
these inputs. Command 882 first differs in actor 15's next tile and one RNG
byte. The AI script exits nonzero at that mismatch; it does not certify the
whole demo. Dynamic wall changes are not yet fed to this reference, so its
world coverage must be checked before treating the mismatch as a port AI bug.
