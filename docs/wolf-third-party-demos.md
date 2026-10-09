# Demo corpus and third-party recordings

The runtime comparator accepts both built-in demos and external recordings in
the original Wolf3D input format. A recording contains inputs and a map number;
it does not identify or include its game data. Always pair it with the exact
mapset and release used for recording. A mismatch against different maps is not
evidence of an engine desync.

## Run the complete built-in corpus

```bash
python3 scripts/wolf_demo_corpus_compare.py \
  --data /path/to/registered-WL6-data \
  --out build/demo-corpus-registered

python3 scripts/wolf_demo_corpus_compare.py \
  --out build/demo-corpus-shareware
```

Each command runs all four built-in recordings, plus four variants that keep
their steering but change the attack schedule. These alternate-fire cases are
generated stress tests, not third-party recordings. Add `--recorded-only` to
omit them. `--source /path/to/pinned-wolf3d` reuses an existing original-source
checkout. The default sound routing is `--sound-mode adlib-digi`; `off` and
`adlib` are also available for controlled comparisons. See
[the runtime comparison](wolf-demo-compare.md) for prerequisites,
compared attributes, supported endpoints and remaining coverage limits.

The runner continues after failed comparisons and writes `summary.json` with
each case's exit code, command, output directory and available runtime result.
It exits nonzero if any case fails. Its summary starts with `status: running`,
so an interrupted run cannot retain an earlier successful summary. Each case
has `run.log`, detailed traces and a fresh `input-manifest.json` recording the
selected data file sizes and SHA-256 hashes, optional external recording hash,
selected demo index, alternate-fire setting and pinned original-source revision.
Runtime and raycast result files from an earlier attempt are cleared before
comparison. A runtime success alone does not establish raycast success: inspect
the corpus case's final exit status too.

## Generate routes across all six episodes

Generate native-format inputs for every registered map without changing spawn,
health, inventory or gameplay RNG:

```bash
python3 scripts/wolf_generate_demo_corpus.py \
  --data /path/to/registered-WL6-data --maps 0-59 \
  --patterns idle patrol combat --commands 768 --seeds 1 \
  --out build/generated-demo-inputs

GDWOLF_DEMO_AREA_PLANE=1 GDWOLF_DEMO_STATICS=1 \
python3 scripts/wolf_demo_sweep_compare.py --actor-occupancy \
  --memory-profile registered-apogee-v1.4-2a969a97 \
  --manifest build/generated-demo-inputs/corpus.json \
  --out build/generated-demo-sweep
```

These are generated stress routes, not human or third-party recordings. Inputs
depend only on the explicit seed, map index and pattern. The generator saves
its own source, input hashes and all selected asset hashes. Both commands
require a new output directory.

The sweep freezes the C and Go binaries once, records their source hashes, runs
the boundary probes once and checks every route against original C plus x86
floor visibility. `--actor-occupancy` adds the complete occupancy grid and pool
slots to every route comparison; the one-time boundary probes omit that large
dump and record this choice in the summary. The summary records each failure and observed coverage;
later edits cannot alter the frozen run. `--runtime-only` explicitly omits the
x86 audit and reports `runtime_passed`, which is weaker than `passed`.
`--maps 35 36` and `--patterns combat` narrow a rerun without changing inputs.

Original-source undefined memory accesses are reported as failures. For
example, using a moving pushwall can produce an out-of-range door index, and
some original map spawns retain an area byte of 255. The host-C oracle rejects
these accesses so host memory layout cannot masquerade as DOS behavior. These
cases require separate executable-level evidence before a compatibility fix
can be considered verified. The explicit registered memory profile in the
example supplies that evidence for the supported aliases; see
[the DOS memory profile](wolf-dos-memory-profile.md). The runtime, corpus and
sweep runners all accept `--memory-profile`, default to `strict-source`, and
record the selected name and executable hash. Unknown aliases remain failures.

An opt-in map-guided recorder adds longer boss-directed routes:

```bash
GDWOLF_DEMO_ROUTE_DATA=/path/to/registered-WL6-data \
GDWOLF_DEMO_ROUTE_OUT=build/generated-demo-boss-inputs \
xvfb-run -a go test -run '^TestWolfDemoRecordMapRoutes$' -count=1 -v .
```

It starts from each map's normal player spawn, health, inventory and RNG. It
uses map paths and observed player state to emit ordinary input commands,
including supply detours. Replay its generated manifest with the independent
sweep before treating any route as verified. A target appearing in the map
does not mean the route reaches or activates it; the coverage report records
that distinction from matched original-C traces.

## Add external recordings

For one recording:

```bash
scripts/wolf_demo_runtime_compare.sh \
  --data /path/to/matching-data \
  --demo-file /path/to/DEMO0.WL6 \
  --out build/demo-external
```

`--demo-file` and an explicit `--demo-index` are mutually exclusive. The loader
checks the original four-byte header and three-byte input records, preserves
signed controls, and rejects malformed lengths or unavailable maps. The file
extension alone does not prove compatibility; renamed videos, savegames,
source-port formats and recordings for later Wolfenstein games are not accepted
as native Wolf3D recordings.

For repeatable external testing, save a JSON manifest such as this under
ignored `build/`. Replace the example names, provenance and hash placeholders
with verified values before running it:

```json
{
  "version": 1,
  "recordings": [
    {
      "id": "author-route-1",
      "file": "recordings/DEMO0.WL6",
      "data": "matching-data",
      "source_url": "https://example.org/author-release-page",
      "recorded_version": "Exact engine release and mapset version",
      "sha256": "REPLACE_WITH_RECORDING_SHA256",
      "data_sha256": {
        "MAPHEAD.WL6": "REPLACE_WITH_MAPHEAD_SHA256",
        "GAMEMAPS.WL6": "REPLACE_WITH_GAMEMAPS_SHA256"
      }
    }
  ]
}
```

```bash
python3 scripts/wolf_demo_corpus_compare.py \
  --external-only --manifest build/third-party-corpus.json \
  --out build/demo-corpus-external
```

Paths inside the manifest are relative to the manifest's directory. `data` may
be omitted to use `--data`, or embedded shareware if neither is provided. IDs,
file, source URL, recorded version and recording SHA-256 are required. The runner
checks recording hashes before starting any comparisons. `data_sha256` is
optional but should pin at least both map files; when supplied, all of its
fingerprints must match. The version string records provenance; it does not
automatically choose or emulate that engine. Do not place proprietary game data
or third-party archives in tracked fixtures.

## The Way ID Did corpus, 2026-10-09

Four native recordings and their matching replacement maps were obtained from
the public [Wolf3D.net mirror](https://beta.wolf3d.net/game/2462) of
[Wolf3DGuy's The Way ID Did release](https://www.moddb.com/downloads/wolfenstein-3d-the-way-id-did).
The downloaded 786,591-byte archive matches the author's published MD5
`f351bb3861a476bbb179b283c7099d27`. Its SHA-256 is
`5f00178093f396b8513c6835ac17c8f7384b4aecccd11e210e7e00c3e7afeba7`.
The inputs are `DEMO0.WL6` through `DEMO3.WL6` inside the archive's
`W3DTWID.pk3`, and parse as original-format input streams. The exact executable
used by their author to record them is unspecified.

| Recording | Map (zero-based) | Episode/floor | Commands | SHA-256 |
| --- | --- | --- | --- | --- |
| `DEMO0.WL6` | 54 | E6F5 | 1,894 | `075220cd286e8e25ebfdee5b4e3a2785c8b4a4a58f48b4d7065be44dbf1f3142` |
| `DEMO1.WL6` | 40 | E5F1 | 1,613 | `5dc123cfb6514c6eac89b8163a7e7f12def05f43ab585a9e574630a474a94668` |
| `DEMO2.WL6` | 16 | E2F7 | 1,399 | `bbb46b9bc7285a0ff8ff2335fde5938418d5a85ed49aabcd6431b3d2f4e932c4` |
| `DEMO3.WL6` | 32 | E4F3 | 1,789 | `53774a563e0befcf064d48612aebe81b82a96e10f41a54fb626ab53cba5e962f` |

Matching map hashes:

- `MAPHEAD.WL6`: `cd8786f534f1dcac98c38559c0789065286107e9119dd0f03f8e400d72d5da18`
- `GAMEMAPS.WL6`: `e22e01f9f17705ae62a64310d28a7375a98938213a04cd24b588a4abcbe37d82`

Fetch the small mod archive separately, then import it with local registered
base assets into a new directory:

```bash
curl --fail --location --connect-timeout 10 --max-time 60 \
  https://beta.wolf3d.net/file-download/download/private/2406 \
  --output /tmp/Wolfenstein_3D_The_Way_ID_Did.zip

python3 scripts/wolf_import_twiid_demo_corpus.py \
  --archive /tmp/Wolfenstein_3D_The_Way_ID_Did.zip \
  --registered-data /path/to/registered-WL6-data \
  --out build/twiid-corpus

python3 scripts/wolf_demo_corpus_compare.py \
  --external-only --manifest build/twiid-corpus/corpus.json \
  --out build/demo-corpus-twiid
```

The importer verifies the archive SHA-256, reads only the known demo/map/graphics
members, and copies `VSWAP.WL6`, `AUDIOHED.WL6` and `AUDIOT.WL6` from the supplied
registered installation. It writes provenance and a corpus manifest pinning
all eight data files. It does not download or execute a game executable. The
output directory must be new, so it cannot overwrite an installed game.

The first comparison found an actual loader incompatibility: the mod stores
60 map offsets in a 242-byte `MAPHEAD.WL6`; the loader previously required the
editor's full 100-slot, 402-byte header. The loader now accepts compact complete
offset tables, treats omitted slots as sparse, and rejects incomplete offset
words. The author's map bytes remain unchanged. Full runtime results are
recorded in the generated corpus summary.

All four routes passed the expanded original-C runtime comparison and the
independent original-x86 raycast audit in `adlib-digi` mode after that fix:

| Recording | Matched commands | Matched tics | Kills | Endpoint | Unread commands |
| --- | --- | --- | --- | --- | --- |
| `DEMO0.WL6` | 1,894 | 7,576 | 30 | Matching death | 0 |
| `DEMO1.WL6` | 1,593 | 6,372 | 25 | Matching death | 20 |
| `DEMO2.WL6` | 1,399 | 5,596 | 20 | Matching death | 0 |
| `DEMO3.WL6` | 1,789 | 7,156 | 33 | Matching death | 0 |

The verified total is 6,675 commands / 26,700 tics and 27,340,800 compared floor
visibility bits. DEMO1's final 20 inputs are not executed by the original after
death and are not claimed as verified gameplay. Each case also passed the
standalone projection, reward, elevator and sound probes. The clean run's
summary is `build/demo-corpus-twiid-final/summary.json`. This establishes parity
for these routes and the configured reference sound routing, not every map,
intermediate single-tic state or original recording executable version.

The final expanded regression repeats these four recordings alongside stock
and alternate-fire demos in all three supported sound modes. All 60 combined
cases pass with actor-grid, mutable-area-plane, static-slot and original-x86
visibility audits. See `build/demo-final-expanded/stock-{off,adlib,adlib-digi}`
and the [complete 432-case results](wolf-demo-compare.md#full-roster-and-generated-route-expansion).

## Other search findings

These sources were distinguished from a verified native input corpus:

| Source | Finding | Test suitability |
| --- | --- | --- |
| [Speed Demos Archive: Wolfenstein 3D](https://speeddemosarchive.com/Wolfenstein3D.html) | Karim “Kimo Xvirus” El-Sheikh's six-episode run credits Ricardo Barreira's Wolfdem recorder. The inspected episode-one download page links MP4 videos and video torrents. | The downloadable video is not an input recording. Original Wolfdem files and format/version details would be needed; the runner's notes also report changed score carryover. |
| [WEX60MAP author page](https://www.moddb.com/mods/extreme-60-level-pack-for-wolfenstein-3d/addons/wex60map) | Describes recordings in the companion `WEX60ETC.ZIP` and a matching replacement mapset. | The accompanying engine changes boss/ghost activation and ratios. Compatibility with the pinned original engine cannot be assumed; no archive was downloaded or replayed. |

For The Way ID Did, the author's [release update](https://www.moddb.com/mods/wolfenstein-3d-the-way-id-did-and-spear-of-destiny-the-way-id-did/news/wolfenstein-3d-the-way-id-did-update)
also distinguishes graphics packages for early Apogee releases, Apogee 1.4 and
GT/Activision/id releases. Use that release guidance when assembling an isolated
test data directory. The
[community listing](https://wl6.fandom.com/wiki/Wolfenstein_3D_The_Way_ID_Did)
provided the initial recorded-demo lead; the mirror archive supplied the actual
native input files and matching maps used above.
