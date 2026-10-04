# Original-source comparison harness

Run the original Wolfenstein 3D C movement routines and GD-WOLF with identical
map occupancy, actor position, player position, direction, flags and RNG index.
The workflow follows GD-DOOM's reference comparison: compile the reference,
record both outputs, stop at the first desync, and replay the recorded input.

```bash
./scripts/wolf_source_compare.sh
```

The script downloads [id Software's original source](https://github.com/id-Software/wolf3d)
at revision `05167784ef009d0d0daefe8d012b027f39dc8541` into the ignored
`build/wolfsrc-source` directory. An existing checkout can be used offline:

```bash
./scripts/wolf_source_compare.sh --source /path/to/wolf3d --out /tmp/wolf-compare
```

Requirements: Go, Python 3, a C99 compiler (`CC` or `cc`), Git for the initial
download, and `xvfb-run` on headless Linux. A supplied source directory must
already exist. Output defaults to `build/wolf-source-compare`.

## Reference implementation

`tools/wolfsrc-reference/build.py` verifies SHA-256 hashes of `WL_STATE.C` and
`ID_US_A.ASM`, then extracts and compiles:

- `TryWalk`, including its original collision macros
- `SelectChaseDir`
- `SelectDodgeDir`
- `SelectRunDir`
- the original direction tables and 256-byte random table

The function bodies come directly from the verified C source. The only edits
inside those bodies widen two actor-pointer casts and their temporary variable
from DOS `unsigned` to `uintptr_t`. This preserves tagged wall/door values and
real host pointers without truncation. Generated reference code stays in the
output directory; it is not part of game builds or tracked source.

`bridge.c` supplies a minimal map and actor layout, records `OpenDoor` calls, and
implements the increment/wrap/table lookup from the original RNG assembly.
The Go comparison invokes the port's actual movement methods. Expected
decisions are never calculated by the existing Go reference adapter.

## Coverage and outputs

Generated scenarios cover all ten embedded shareware maps, each live enemy,
nearby player placements, chase/dodge/run decisions, and four RNG starting
indices. The local obstruction scan covers 256 neighbor masks, all nine
directions, all four routines, and humanoid/dog door behavior.

To sample enemy decisions throughout the registered six-episode data, supply a
local WL6 directory:

```bash
./scripts/wolf_source_compare.sh --data /path/to/WL6-files --out /tmp/wolf-registered-enemies
./scripts/wolf_source_compare.sh --data /path/to/WL6-files --map-index 37 --out /tmp/wolf-map-37-enemies
```

The registered scan chooses one nearby valid player position per live enemy
and compares chase, dodge and run with two RNG starting indices. On the local
Apogee v1.4 data, 15,024 decisions covering 2,504 enemies across all 60 maps
matched the pinned original C. This is sampled decision coverage, not a
continuous playthrough or proof of full enemy behavior parity.

The reference self-test separately checks RNG behavior at all 256 indices,
door opening/wait behavior, dog blocking, and the original run fallback arc.

Compared fields are movement success, direction, destination, door wait,
opened door number, first-attack flag, and final RNG index. Failed movement
direction and RNG consumption remain visible; neither is normalized away.

Artifacts:

| File | Contents |
| --- | --- |
| `build.log` | Reference compiler output |
| `compare.log` | Go test results and first desync |
| `inputs.jsonl.gz` | Every exact comparison input, compressed |
| `reference.jsonl` | Original C outputs with scenario IDs |
| `port.jsonl` | Port outputs with the same IDs |
| `mismatch.jsonl` | Differing fields, full input and both outputs |
| `mismatch-inputs.jsonl` | Failed inputs ready for direct replay |

Run only the reference contract checks:

```bash
./scripts/wolf_source_compare.sh --self-test
```

Replay failed inputs into a different output directory:

```bash
./scripts/wolf_source_compare.sh \
  --input build/wolf-source-compare/mismatch-inputs.jsonl \
  --out /tmp/wolf-replay
```

The same `--input` option accepts `inputs.jsonl.gz` to replay the whole recorded
comparison. To collect multiple desyncs, set a positive limit:

```bash
./scripts/wolf_source_compare.sh --max-mismatches 100
```

The command exits nonzero for any desync, invalid input, compiler error, or
reference process error. With no configured reference binary, the external
tests skip during ordinary `go test ./...`; the comparator's field-difference
test still runs. The script always supplies a reference binary and rebuilds it.

## Scope

These are isolated movement decisions from map snapshots, not a full original
DOS engine or per-tic demo replay. The adapter preserves walls, blocking
scenery, blocking actors, closed doors and door locks. It models an already
open door as clear occupancy. Areas are held constant, and actor classes are
normalized to humanoid or dog for the supported movement routines.

Rendering, sound playback, door animation, actor state timers, physical movement
between tiles, combat and the player simulation are outside this C reference's
coverage. The existing [AI harness](enemy-ai-harness.md) continues to provide
broader scripted runtime and attack checks through its Go reference adapter.
The [recorded-demo runtime comparison](wolf-demo-compare.md) independently
checks more systems along the eight tested demo routes.

## First verified desync

On 2026-10-03, the initial compare matched 15,716 decisions before finding:

```text
map=0/actor=7/pos=0/mode=2/seed=0
dir: original=8 port=0; x: original=31 port=32; move: original=false port=true
```

`SelectRunDir` in the original source searches the enum interval from north to
west (or in reverse): north, northwest, west. The initial port fallback searched
four cardinal directions and could move east in this blocked scenario.
The first movement fix pass now uses the original search arc, so this saved
input passes and remains available as a regression case.

A minimal reproduction is tracked in `testdata/wolfsrc-compare/run-fallback.jsonl`:

```bash
./scripts/wolf_source_compare.sh \
  --input testdata/wolfsrc-compare/run-fallback.jsonl \
  --out /tmp/wolf-run-fallback
```

The full initial audit used `--max-mismatches 1000000` and completed 894,444
comparisons in about 257 seconds. It recorded 9,503 differing scenarios:

| Routine | Differing scenarios |
| --- | ---: |
| `SelectRunDir` | 6,454 |
| `SelectChaseDir` | 2,314 |
| `SelectDodgeDir` | 479 |
| `TryWalk` | 256 |

These counts include repeated seeds and placements, not 9,503 distinct bugs.
The `TryWalk` failures all exercise `nodir`: the source rejects it while the
port's direction masking turns it into an eastward step. Dodge differences
include locked-door decisions. No RNG-index differences were observed in this
audit. These are the baseline counts before the movement fixes below.

## First movement fix pass (2026-10-03)

The port now matches the original north/northwest/west fallback scan in both
chase and run decisions, sets `dir = nodir` when chase/dodge/run selection cannot
move, and rejects `nodir` instead of masking it into an eastward step. Ordinary
Go regressions cover the blocked movement cases, and the initial saved
run-fallback input now matches the compiled C reference.

The broader Go reference adapter was corrected to use the same fallback arc,
consume random bytes only at the original decision point, and propagate the
blocked direction and first-attack flag into runtime traces.

The full repeat audit compared the same 894,444 decisions in about 266 seconds.
Differences fell from 9,503 to 815, resolving 8,688 failing scenarios:

| Routine | Initial differences | After movement fixes |
| --- | ---: | ---: |
| `SelectRunDir` | 6,454 | 408 |
| `SelectChaseDir` | 2,314 | 184 |
| `SelectDodgeDir` | 479 | 223 |
| `TryWalk` | 256 | 0 |

All 815 scenarios left after this pass had an original destination at a locked
door: 583 at the E1F5 door at (57, 10), and 232 at the E1F6 door at (29, 2).
The first four maps and all 18,432 local obstruction cases matched. Native
and wasm builds passed, as did the ordinary Go suite and the new RNG regression.
The C comparison still exited nonzero for these door differences.

## Locked-door fix pass (2026-10-03)

Original `TryWalk` calls `OpenDoor` for humanoid actors regardless of the lock;
player key checks happen separately in `OperateDoor`. The port had rejected
locked doors in both its actor passability test and its actor door-opening
helper. Removing those actor-only restrictions resolved all 815 saved failures
when replayed unchanged against the compiled C routines. The broader Go
reference adapter was corrected too.

Ordinary regressions cover all five lock types for guard, officer, SS, boss and
mutant movement, including waiting, reopening a closing door and crossing once
open. Player key checks still require the matching key. Dogs cannot open closed
doors, but can cross doors that are already open. Native and wasm builds and the
full Go suite pass.

The full repeat audit matched all 894,444 original C decisions in about 277
seconds, with zero differences across all ten shareware maps and the 18,432
local obstruction cases. This closes every discrepancy from the initial
movement audit within the comparison scope described above.

A minimal fixture covers the four movement routines at a locked gold-key door,
a blocked dog, and a humanoid opening an elevator door:

```bash
./scripts/wolf_source_compare.sh \
  --input testdata/wolfsrc-compare/locked-door.jsonl \
  --out /tmp/wolf-locked-door
```
