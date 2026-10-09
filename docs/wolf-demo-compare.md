# Recorded demo comparison

These harnesses replay recorded inputs through the port and pinned original C.
They cover player movement, actors, combat, inventory, doors, pickups, gameplay
sound state and RNG. The runtime comparison also audits floor visibility with
original 16-bit x86 code. They build on the isolated
[original C movement audit](wolf-source-compare.md).

## Player-only comparison scope

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
go run . -data /path/to/registered-data -demo-index 1 -demo-sound-mode adlib-digi
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

Playback defaults to `adlib-digi`: synthesized sound with original digitized
sound routing. `-demo-sound-mode off` and `-demo-sound-mode adlib` select the
other supported modes. The selected mode affects face RNG and can change the
recorded route; see [deterministic demo sound](#deterministic-demo-sound-and-rng).

Playback ends at demo completion, an elevator exit, or port death/victory. These
terminal outcomes are not independently verified by the player-only reference.

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
This isolates the original `UpdateFace` behavior and does not independently
verify actor/combat RNG consumption. The broader runtime harness below now
evolves sound priority, expiry and face suppression independently, including
in headless runs.

## Actor timing and recorded-demo runtime

For independent multi-tick enemy encounters, run:

```bash
./scripts/wolf_enemy_runtime_compare.sh --data /path/to/registered-data --out /tmp/wolf-enemy-runtime
```

This compiles the pinned original C and compares the first available encounter
for each of the thirteen spawnable Wolf3D actor families across registered maps.
Dynamic Hitler is covered by the separate Mecha transformation probe. Each scenario runs
up to 80 actor updates with scripted nearby player positions, noise and varied
tic counts. The comparison checks every actor's state, RNG, doors and player
damage at each step; it stops at player death. Per-encounter JSONL files in
`--out` retain inputs and both states for replay. These scenarios found that
Hans and mutants were spawned with the wrong HP for the selected difficulty;
the values now follow original `starthitpoints`. The probe forces one target
active and does not simulate rendering-based activation or player weapons.
The registered Gretel encounter matches 19 actor updates through player death.
A separate open-room encounter matches 95 updates, requiring both chase movement
and player damage. These probes run with sound off and compare every actor's
state, RNG, damage and doors. Gretel now has her original spawn stats,
nonrotating state sequences, six-shot burst, guard-style accuracy, sounds and
gold-key drop. Source-derived Go regressions cover her damage/death, score and
key drop; those are not original-C death probes. The encounters do not establish
coverage of every boss interaction.

```bash
./scripts/wolf_actor_states_compare.sh
./scripts/wolf_demo_runtime_compare.sh
./scripts/wolf_demo_runtime_compare.sh --demo-index 1 --sound-mode adlib --out /tmp/wolf-demo-e1f3
./scripts/wolf_demo_runtime_compare.sh --demo-index 2 --out /tmp/wolf-demo-e1f5
./scripts/wolf_demo_runtime_compare.sh --demo-index 3 --out /tmp/wolf-demo-e1f7
./scripts/wolf_demo_runtime_compare.sh --data /path/to/registered-data --demo-index 0 --extra-fire --out /tmp/wolf-combat-e1f1
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
dropped items, bonus collection, face updates and RNG tables. It checks score,
kill count and the remaining pickup/drop positions and shapes after each
command, along with the existing actor, weapon, player and RNG comparisons. It initializes
from raw map planes and carries independent player/actor positions, reservations,
weapons, doors, walls, pickups, health and RNG across commands. Original
`DrawScaleds` static/actor placement and `TransformActor`/`TransformTile` geometry
are compiled. Drawing and hardware audio output are omitted; original sound
priority and synthesized-effect expiry are compiled as described below.
Original `GivePoints` and
`GiveExtraMan` now handle scoring, the 40,000-point extra-life threshold and the
nine-life cap independently of the port.
DOS projection height assembly uses equivalent integer division, and tagged
pointers/map words are adapted for host C. The runtime includes every Wolf3D
enemy family, projectiles, BJ's victory run and boss death cameras. Spear-only
actors are outside this reference.

The runtime now compiles original `T_Player`, `Cmd_Use` and `TakeDamage`, so
use requests and the death flag evolve independently. Its remaining shared input
is the visible-floor mask produced by the port's translation of `WL_DR_A.ASM`.
The runtime script audits those masks by assembling the verified original
raycaster instructions and executing them as 16-bit x86 under QEMU. Assembler
syntax, test-owned segment placement and drawing callbacks are adapted; the
original arithmetic and self-modifying quadrant branches execute unchanged.
Every one of the first demo's 4,321,280 floor bits matches. This audit uses
original-C tile bytes, door positions, pushwall position and view coordinates,
plus original render tables. The projection self-test matches original render
tables and 2,160 actor/pickup
projection cases spanning every integer angle.

The runtime comparison now matches all four embedded shareware demos through
their original death endpoints. E1F1 plays 1,055 commands and leaves 97 recorded
commands unread; E1F3, E1F5 and E1F7 play all 1,284, 671 and 633 commands.
The script succeeds at a matching terminal state and fails if the terminal
state differs. The original x86 floor-visibility audit also matches every
played command in each demo. These recordings cover only the enemies they
encounter; hardware audio limitations are described below.

`--extra-fire` preserves each recorded demo's first 200 commands, then uses
the same steering with a deterministic alternate attack schedule. This takes
different continuous combat paths while the C reference independently moves
the player and enemies. All four registered demo maps match through their
resulting death endpoints, with 1–6 kills and peaks of 8–23 simultaneously
alerted enemies. The script also audits the supplied visibility masks against
the original x86 raycaster; the map-37 alternate route matched 1,220,608 floor
bits. These paths still use the recorded steering, so they do not cover every
possible door use or firing angle.

The additional maps exposed three runtime differences. Original `DoActor`
suspends any inactive actor in a disconnected area, while `DrawScaleds`
permanently activates actors when their tile becomes visible. A dead actor can
leave a nonshootable pointer at its former reserved goal tile; `CloseDoor`
still treats that pointer as occupied. Finally, an SS chooses its dropped item
at death, using the player's current best weapon, rather than choosing it at
spawn. The port now follows these behaviors during recorded playback.

Registered WL6 data can be compared with `--data` while keeping the game files
outside the repository:

```bash
./scripts/wolf_demo_runtime_compare.sh --data /path/to/registered-data --demo-index 0 --out /tmp/wolf-registered-demo-0
```

The Apogee v1.4 six-episode data uses the WL1 graphics chunk layout under
WL6 filenames. The loader detects that layout from its demo chunks; the
registered layout defined in the pinned `GFXV_WL6.H` remains supported. All
four demos in this Apogee data match original C runtime state through their
recorded endpoints: 691, 1,899, 1,140 and 1,656 commands on maps 37, 43, 56
and 31. The original x86 floor-visibility audit also passes every played
command, covering 22,061,056 floor bits. These comparisons cover the recorded
routes, not every room or enemy family across all six episodes. Registered
game data is not bundled with the repository.

Four publicly distributed native recordings from **Wolfenstein 3D: The Way ID
Did** also pass with their matching mod maps and `adlib-digi`: 6,675 commands,
26,700 Wolf tics and 27,340,800 original-x86 floor bits. All four end at matching
deaths; one leaves 20 recorded commands unread. Importing the unchanged mod
revealed a valid 60-offset `MAPHEAD` that the loader incorrectly required to
have 100 offsets. The loader now accepts complete compact headers and rejects
truncated offsets. These recordings are distinct from the stock demos and
require the author's matching map data.

For batch commands, the checksummed importer, provenance and all four third-party
results, see [Demo corpus and third-party recordings](wolf-third-party-demos.md).

## Deterministic demo sound and RNG

The runtime script and corpus runner default to `--sound-mode adlib-digi`.
`--sound-mode adlib` keeps effects on the synthesized channel; `--sound-mode off`
disables effects. Native playback uses the equivalent `-demo-sound-mode` option.
The mode is part of the comparison input and is recorded in each result.

The reference compiles original `SD_PlaySound`, `SD_SoundPlaying`,
`SDL_PCPlaySound`, `SDL_PCStopSound`, `SDL_PCService`, `SDL_ALPlaySound`,
`SDL_ALStopSound` and `SDL_ALSoundService` from `ID_SD.C`. Sound IDs and the
shareware/registered `wolfdigimap` come from the pinned source. Hardware writes
and interrupt masking are omitted. The original C service models, disabled in
the shipping source, preserve the counters used by the active `DOFX` assembly.
The harness verifies the `ID_SD_A.ASM` hash and counter semantics; it does not
execute that audio assembly. PC service code is included in the oracle, but
PC speaker mode is not exposed or covered by the port comparisons.

Both sides advance two 140 Hz sound-service steps per 70 Hz Wolf tic, grouped
at command entry. They use the selected game's raw sound lengths and priorities,
not host playback time. The reference independently checks the synthesized
channel's sound ID, priority and remaining duration. `adlib-digi` routes mapped
effects to the separate digitized channel, as in original `SD_PlaySound`.
A 4,176-state original-C probe covers all 87 sound IDs, priority interruption,
expiry and face-RNG suppression in all three supported modes. The runtime
script runs it automatically. All eight stock recordings also pass in both
`adlib` and `adlib-digi`, with 9,029 played commands per mode.

This exposed a real registered-demo desync that a sound-free reference missed.
In registered demo 1 (map 43), the chaingun pickup suppresses face updates while
`GETGATLINGSND` occupies the synthesized channel. The old reference first
consumed an extra RNG byte at command 1,110 (zero-based): index 189 instead of
188. It eventually finished alive with 23 kills, 100 health and 6,300 points.
The sound-aware original C and port instead match a death at command 1,898,
with 51 kills, 0 health, 28 ammo and 13,600 points. Pure AdLib testing also caught
a door sound starting one command early: opening sound now begins on the first
`MoveDoors` update and opening/closing sounds obey original area connectivity.

This verifies gameplay sound state under the stated deterministic schedule.
The physical DOS interrupt phase, digitized playback duration and channel
expiry, audible PCM output, panning and sample mixing remain unverified. The
native PCM mixer can still layer effect voices even when the deterministic
synthesized channel rejects one by priority. This harness does not establish
hardware-timed audio or audible mixing parity.

## Expanded attribute checks (2026-10-09)

The runtime now checks health, ammo, score, lives, the next extra-life threshold,
all key bits, current/best/chosen weapons, secret and treasure counts/totals, kill
counts/totals, face frame/timer, actor activation and state family/frame duration,
moving pushwall phase and position, all area-reachability flags, and synthesized
sound ID/priority/remaining duration. Persistent
inventory and face state are checked both before rendering and after pickups.
Actor position, reservations, health, animation, reaction, flags, doors, player
movement, weapon attacks, RNG, remaining pickups and floor visibility retain
their existing comparisons.

All eight built-in demos pass these additional checks: 9,029 played commands
(36,116 Wolf tics). The registered routes use maps 37, 43, 56 and 31; the
shareware routes use maps 0, 2, 4 and 6. A separate original-C pickup probe
covers 152 boundary cases across every supported pickup type, score thresholds,
full inventories, all key bits, the life cap, weapon upgrades and knife attacks.
The runtime comparison script runs this probe automatically.
A second original-C probe checks 1,440 elevator uses: every integer angle,
normal/secret exit floors and fresh/held use buttons.
The final `adlib-digi` batch passes all sixteen stock/alternate-fire routes:
12,416 commands and 50,855,936 original-x86 floor bits. Adding the four TWIId
recordings brings the verified corpus to 20 runs, 19,091 commands, 76,364 Wolf
tics and 78,196,736 floor bits. Results are saved under
`build/demo-attributes-final/{shareware,registered}` and
`build/demo-corpus-twiid-final`; each corpus `summary.json` reports `passed`.
An exported stock shareware demo separately passes through the external-file
manifest path; this validates ingestion and is not a third-party recording.

These checks and source inspection found and fixed:

- Score awards omitted extra lives, and full-heal pickups omitted their life.
  Full-heal now uses the original +99 health behavior even on a lethal frame.
- Ammo collected during frame zero of a knife attack must restore the selected
  gun. Weapon pickups during an attack must also switch the active attack
  sequence without resetting its frame or timer.
- Demo movement omitted exit-tile checks. Each thrust now checks the tile,
  including the intermediate strafe leg; victory prevents further firing and
  player damage. The subsequent run/jump and boss death-camera states are now
  compared with original C in separate probes.
- Elevator use now ends a demo on the matching normal or secret exit command,
  without saving or loading the next level. Integer direction boundaries match
  original `Cmd_Use`; the runtime reports the terminal type and unread tail.

Save format version 6 preserves the next extra-life threshold and the new
projectile angle/speed, marking flags and frozen initial animation state. Older
prototype saves are rejected by the existing version check.

This is coverage of recorded paths and explicit boundary scenarios. Physical
audio timing and mixing, palette and HUD drawing, and the display/wait portions
of victory presentation remain unverified. The runtime now compares original
`gamestate.TimeCount` after each command; hardware interrupt phase and time
spent in display waits are outside that check. Unsupported reference routines
and out-of-bounds original memory accesses fail the run rather than certify it.

## Full-roster and generated-route expansion

Registered support now includes Schabbs, Giftmacher, Fatface, Fake Hitler,
Mecha-Hitler, Hitler, all four ghosts, needles, rockets, flames and smoke.
The original-C encounter suite covers every spawnable family. Fourteen
additional scenarios require actual movement and attacks, then exercise lethal
damage, projectile removal, Mecha's transformation and boss death cameras in
each sound mode. A separate `MoveObj` probe compares 3,528 boundary vectors.
Four projectile wall-impact scenarios additionally require transient removal,
slot reuse and rocket explosion states. A death-camera probe compares player
coordinates, angle and state for all 360 angles, with and without an obstruction
that forces the camera farther from the boss (720 cases).
A static-object probe checks all 50 defined map entries, pickup ordering after
slot reuse, the 399-object setup limit and 400-object runtime limit. Map entries
71 and 72 now preserve the original nonbonus clip-shaped object and free slot;
73/74 read beyond the non-Spear source table and are explicitly unsupported.

The BJ probe compares 2,160 combinations of all 360 player angles, normal versus
attacking player states, and three sound modes through the victory endpoint.
Another 48 cases enter the exit tile through real movement, including a strafe
and forward move that spawn two BJ actors in one command. These probes use a
shared fully visible floor mask to isolate the state machines; ordinary demo
runs retain the separate x86 raycast audit.

The broader routes exposed additional differences now addressed in playback:

- Knife hits call the original noise path, including accepted zero-damage hits.
- Pain sprites use the original two-direction `CalcRotate` result.
- Door-jamb bits remain in the raycaster's tile bytes even on floor cells.
- Player death preserves the current weapon animation state in demos.
- The final input sets original `ex_completed` before gameplay; that command
  still runs completely and can replace completion with death or another exit.
- Actor movement checks the inclusive one-tile player boundary only where
  original `MoveObj` does; exact tile arrival follows the source snap behavior.
- Ambush floor areas follow the original neighbor precedence and spawn order;
  the mutable floor-area plane is compared separately from walls and collision.
- Dropped items reuse the first free original static-object slot and respect
  its 400-slot limit. Optional static comparisons retain the original order,
  raw flags/item numbers and visibility pointers, including removed items.
- Collision and doors use the original 150-slot actor pool and `actorat` grid,
  including nonshootable bodies, stale pointers, slot reuse and door clearing.
- Doors keep their original registry entry when a pushwall overwrites their
  map cell; later doors retain their original indices.

Set `GDWOLF_DEMO_OCCUPANCY=1` to compare all 4,096 occupancy tags and actor pool
slots alongside the usual runtime state. `GDWOLF_DEMO_AREA_PLANE=1` adds the
4,096 mutable floor words; `GDWOLF_DEMO_STATICS=1` adds allocated static slots,
including collected items. See the
[generated-route commands](wolf-third-party-demos.md#generate-routes-across-all-six-episodes)
for reproducible broader coverage. Passing stock recordings does not establish
zero desyncs on untested routes.

The optional `--memory-profile registered-apogee-v1.4-2a969a97` selects the
verified registered DOS executable layout. It resolves area-255 reads and
moving-wall false-door accesses using independent DOS memory evidence, including
an actual write to a static object's shape. `strict-source` remains the default
and rejects such out-of-bounds accesses. Unknown memory regions still fail
explicitly in both the port and oracle. See the
[DOS memory profile](wolf-dos-memory-profile.md) for executable hashes, exact
offsets, source-enum correction and reproduction commands. Native playback uses
the same name with `-demo-memory-profile`.
Each replay starts a fresh game. Static bytes retained across prior levels or
successive attract-loop demos are outside these comparisons.
Living actor health is exact; dead actor health is normalized to zero on both
sides, so the traces do not verify the original negative overkill remainder.

The final frozen run is `build/demo-final-expanded/summary.json`. All **432
cases pass**, with zero observed desyncs under the registered DOS profile:

| Corpus | Cases | Matched commands | Original-x86 floor bits |
| --- | ---: | ---: | ---: |
| All 60 maps, three 192-command patterns, seed 0 | 180 | 29,627 | 121,352,192 |
| All 60 maps, three 768-command patterns, seed 1 | 180 | 78,911 | 323,219,456 |
| Stock, alternate-fire and four TWIId recordings, sound off | 20 | 19,091 | 78,196,736 |
| Same recordings, AdLib | 20 | 19,091 | 78,196,736 |
| Same recordings, AdLib plus digitized routing | 20 | 19,091 | 78,196,736 |
| Map-guided boss recordings | 12 | 6,749 | 27,643,904 |
| **Total** | **432** | **172,560** | **706,805,760** |

These commands cover 690,240 Wolf tics. Every route enables actor occupancy,
area-plane and static-slot comparisons, plus the x86 floor-mask audit. All six
batches use identical frozen C/Go binaries and source fingerprints. The aggregate
checks raw results, input hashes, terminal states, unread tails, actual trace
fields and binary hashes before reporting `passed`. All six standalone probe
batches also pass. The separate complete Go suite, original-C enemy/static/DOS
alias suite, seven Python tool tests and wasm build pass.

The earlier strict-source sweeps stopped at explicit memory guards on moving
walls and area 255. Those cases pass with the independently verified DOS profile;
strict-source still reports unsupported access rather than inventing behavior.

Natural boss routes exercise Hans, Gift and Gretel chase/attack states. Schabbs
becomes active but remains standing; Mecha/Hitler and Fat are not activated by
these attempts. Their combat and projectile evidence comes from the controlled
original-C probes. Passing the finite corpus does not establish parity for every
possible input sequence or the presentation/audio timing excluded above.

Default runtime artifacts in `build/wolf-demo-runtime-compare`:

| File | Contents |
| --- | --- |
| `build.log`, `compare.log` | Compiler and comparison output |
| `reference-input.txt` | Raw map, commands, diagnostic snapshots and shared floor masks |
| `reference.jsonl`, `port.jsonl` | Player, actor, weapon, RNG, sound, doors and wall occupancy before rendering |
| `reference-render.jsonl` | Original cached actor projections, post-pickup health/ammo and death flag |
| `port-render.jsonl` | Port post-pickup inventory, counters and face state |
| `reference-AUDIOHED.bin`, `reference-AUDIOT.bin` | Raw selected sound bank files used by the original C scheduler |
| `input-manifest.json` | Selected data and external recording SHA-256 fingerprints, source revision, sound mode and replay options |
| `raycast-input.jsonl` | Original view coordinates, exact tile bytes, door/pushwall positions and port floor masks |
| `raycast/`, `raycast.log` | Generated original x86 assembly/boot image, visibility bytes and audit result |
| `result.json` | Matched count, unread commands, sound mode/data variant, and success, matched-terminal, mismatch or unsupported-terminal status |

Replay the original C runtime without Go or a display. This example uses the
default shareware demo 0; use `sound_mode`, `registered` (0/1) and `map` from
`result.json` for another run:

```bash
GDWOLF_DEMO_SOUND_MODE=adlib-digi \
GDWOLF_DEMO_REGISTERED=0 \
GDWOLF_DEMO_MAP_INDEX=0 \
GDWOLF_DEMO_COMMAND_COUNT=1152 \
GDWOLF_DEMO_AUDIO_HEAD=build/wolf-demo-runtime-compare/reference-AUDIOHED.bin \
GDWOLF_DEMO_AUDIO_DATA=build/wolf-demo-runtime-compare/reference-AUDIOT.bin \
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
