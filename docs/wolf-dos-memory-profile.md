# Registered DOS memory profile

`registered-apogee-v1.4-2a969a97` identifies the registered Apogee v1.4 executable
with SHA-256 `2a969a97bc644d04be030d0d08bfda5477179d60f5dbfcc04616a18fa083ce77`.
It supplies verified memory aliases for source expressions that access outside
their declared arrays. Unset or `strict-source` keeps those accesses as explicit
unsupported results. A profile is an executable compatibility target, not a
claim that every data set was shipped with that executable.

The source reference never uses adjacent host-C globals as a substitute for DOS
memory. Its profile reads are reconstructed from current independently evolved
game state. Unknown profiles, unallocated static slots, and unverified alias
pointer accesses fail explicitly.

## Independent executable evidence

`tools/wolfsrc-reference/capture_demo_dos_memory.py` launches the supplied
executable in a private DOSBox directory and reads only that child process's
memory. It replaces all four compressed demo chunks with one native recording.
The executable, maps, and other compressed graphic chunks retain their supplied
bytes. An explicit `--map-fixture` can additionally replace one map; that change
is separately identified and hashed. The capture manifest records original and
modified file hashes. Captures themselves do not assert gameplay parity.

`tools/wolfsrc-reference/analyze_demo_dos_memory.py` locates the data segment using
the actual player near pointer and object fields, then verifies every real door,
all 4096 initial wall cells, and every allocated static visibility pointer. With
`--reference-runtime` it separately runs source setup and compares all eight
bytes of every allocated static record, plus original door-state values.

The following offsets are relative to the verified executable's data segment:

| Symbol | Offset |
| --- | ---: |
| `areabyplayer` | `0x49da` |
| `update` | `0x4bb4` |
| `actorat` | `0x4cb8` |
| `tilemap` | `0x6cb8` |
| `doorobjlist` | `0x7db8` |
| `laststatobj` | `0x8038` |
| `statobjlist` | `0x803a` |
| `player` pointer | `0x8cba` |
| `gamestate` | `0x8cc0` |
| `spotvis` | `0x9c48` |

Real DOS `objtype`, `doorobj_t`, and `statobj_t` occupy 60, 10, and 8 bytes.
The source bridge stores wider host pointers and reconstructs DOS layout
explicitly where required.

## Area 255

Some original bosses are spawned on raw ambush tile 106 without the ordinary
guard's area normalization. Their byte area wraps `106-107` to 255. Reading the
16-bit `areabyplayer[255]` accesses `update[36:38]`, not a valid area entry.

The captured E2M8 startup shows this alias changing from zero to `0x0101` while
`TimeCount` is still zero and before the player object is published. All captured
gameplay samples retain `0x0101`. Source `DrawPlayScreen` marks the play border
and status bar before `SetupGameLevel` and `PlayLoop`.

The reference uses the original `VW_MarkUpdateBlock` and `DrawPlayBorder` routines
to maintain these flags. The `VH_UpdateScreen` adapter reproduces the pinned
assembly's dirty-byte clear; death-camera border drawing marks the affected
rows again before another actor step. The profile does not redefine area 255
as an ordinary connected area or change the boss's stored area byte.

## Moving walls interpreted as doors

`Cmd_Use` tests bit `0x80` of a moving-wall marker (`0xc0 | texture`) and clears
only that bit. It can consequently call `OperateDoor` with an index from 64 to
127, outside the 64 declared doors.

For index `n`, the aliased ten-byte door starts at byte `10*n-642` relative to
`statobjlist`. Each static is serialized as tile X byte, tile Y byte, 16-bit near
visibility pointer, signed 16-bit shape, flags byte, and item byte. The visibility
pointer is `0x9c48 + x*64 + y` when initially spawned; later pointer changes must
be preserved independently of tile coordinates. Offset -2 contains the
`laststatobj` near pointer.

Independent unchanged-map captures establish these ordinary-input cases:

| Map | Command (zero-based) | False door | Lock | Action | Original result |
| --- | ---: | ---: | ---: | ---: | --- |
| 4 | 66 | 72 | 13 | 32 | No switch case; no-op |
| 27 | 44 | 91 | 27 | -23779 | No switch case; no-op |
| 19 | 69 | 73 | 31 | 2306 | No switch case; no-op |

The alias bytes are unchanged in samples before and after each call. Initial
source static records match DOS for 300, 294, and 269 allocated records on these
maps; E2M8 adds another 255 matching records.

The door action values are **open 0, closed 1, opening 2, closing 3**. A previous
handwritten bridge declaration reversed open and closed, which ordinary
named-state tests did not expose. A controlled false-door fixture and shipped
executable disassembly revealed the mistake. The builder now extracts this
enum directly from pinned `WL_DEF.H`, and DOS analysis checks real door actions.
Any older raw action traces require regeneration.

A controlled E3M1 fixture moves only the original player start to `(17,41)`,
facing south toward the existing secret wall at `(17,42)`. Commands are idle,
use, release, use, then idle. The second use interprets moving texture 8 as door
72. Its aliased action is 3 (closing), so original `OpenDoor` writes action 2
(opening). This word overlaps static slot 10's shape at `(50,16)`.

The shipped executable changes that shape from 3 to 2 at command 3 / tic 16.
All 924 static records (132 allocated slots across tics 4 through 28) match the
compiled original source plus the profile's serialized word write. An E1M1
fixture also confirms that false door 65, whose action is 0, takes `CloseDoor`
and returns without mutation when its aliased cell is occupied.

The profile serializes changed action/timer words back to tile coordinates,
visibility pointer, shape, or flags/item according to the exact overlapping
two-byte field. It preserves visibility independently of coordinates. A write
outside allocated statics or outside the verified `spotvis` pointer range is
unsupported. `CloseDoor` retains the source's obstruction tests, area sound,
action change, and occupancy write; an out-of-map access or dereference of a
raw wall tag as an object is unsupported. A later actor read of an out-of-range
`doorposition` is also explicitly unsupported rather than host-dependent.

## Reproduction

Run the capture under an isolated X11 display (or `xvfb-run -a`) with DOSBox
available, then validate its dump against the raw map and source reference:

```sh
python3 tools/wolfsrc-reference/capture_demo_dos_memory.py \
  --dosbox /path/to/dosbox --data /path/to/registered-data \
  --demo-file /path/to/recording.wl6 --out build/dos-memory-capture \
  --stop-tic 280 --snapshot-tics 0 4 256 260 264 268 272 276 280

python3 tools/wolfsrc-reference/analyze_demo_dos_memory.py \
  --capture build/dos-memory-capture \
  --reference-input /path/to/reference-input.txt \
  --reference-state /path/to/reference.jsonl \
  --reference-runtime build/full-roster-reference/wolf-demo-runtime-reference
```

These DOS captures use disabled hardware audio and sampled memory snapshots.
They establish the stated layout and alias behavior; they do not replace the
per-command source comparison, the x86 visibility audit, or audio verification.
