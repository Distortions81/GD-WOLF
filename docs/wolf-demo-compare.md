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
index zero and uses the original integer controls. Each command contains a
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
./scripts/wolf_demo_player_compare.sh --stop-after-commands 310
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

The reference self-test matches all 360 movement angles plus strafing, reverse
movement, speed clamps and fractional turns. The first 310 demo commands
(1,240 tics) match conditional player movement. The port then dies before
command 310 (zero-based), so the unrestricted 1,152-command demo run remains
incomplete. This is not a verified original-versus-port death desync: the
current reference does not simulate enemy damage.

The next comparison stage needs independent original actor initialization,
state progression, RNG consumption, visibility/targeting and combat, using
the saved runtime trace to locate the first divergence before death.
