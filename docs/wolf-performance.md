# Runtime performance checks

The October 2026 optimization pass reduces repeated door-map scans, wall-command
setup and temporary audio/gameplay buffers. Behavior checks accompany the
benchmarks so a faster result cannot silently change demo playback or output.

## Measurements

These are local microbenchmarks against baseline commit `1250296`, using Go
1.26.6 on Linux/amd64, an AMD Ryzen 9 7950X and `GOMAXPROCS=2`. Times are medians
of five runs for area connectivity and GPU commands, and three for gameplay.
They measure individual operations, not total frame time or browser FPS.

| Operation | Before | After | Allocation change |
| --- | ---: | ---: | --- |
| GPU wall-command setup, 320 columns | 11.35 µs | 4.74 µs | Already allocation-free |
| GPU wall-command setup, 1280 columns | 44.69 µs | 17.66 µs | Already allocation-free |
| GPU wall-command setup, 1920 columns | 68.94 µs | 26.65 µs | Already allocation-free |
| Area connectivity, E1M1, closed doors, interactive | 10.38 µs | 2.56 µs | Already allocation-free |
| Area connectivity, E1M1, all doors open, interactive | 156.19 µs | 2.84 µs | 128 B / 9 allocations → 0 |
| Area connectivity, E1M7, all doors open, interactive | 243.95 µs | 3.30 µs | 360 B / 8 allocations → 0 |
| Area connectivity, E1M1, all doors open, demo | 193.03 µs | 0.38 µs | 128 B / 9 allocations → 0 |
| Actor path crossing a tile boundary | 47.60 ns | 32.08 ns | 32 B / 1 allocation → 0 |
| Demo actor projection refresh | 4.81 µs | 4.81 µs | 4096 B / 1 allocation → 0 |

The GPU figures use the front-facing fixture; the angled fixture improves by a
similar amount. All-open-door fixtures stress connectivity traversal and do not
represent an average gameplay frame. Buffers and texture caches are warmed
before timing.

An 8192-byte music read previously allocated two temporary buffers totaling
16 KiB. Complete PCM frames now render directly into the caller's buffer, with
up to three unread bytes retained between calls. The active-synth benchmarks
report 3–4 B/op and a rounded 0 allocations/op from occasional internal synth
buffer growth; silence reports zero. Active music synthesis time is broadly
unchanged. The benefit is lower allocation and garbage-collection pressure.

Raw before/after measurements from this run are retained locally in the ignored
`build/optimization-measurements/` directory, including `summary.json` with
sample counts and medians.

## Implementation boundaries

- Area connectivity builds its open-door edges once per update and reuses BFS
  storage. Each update reads current door state and area labels, including
  pushwall changes. Demo playback uses its existing persistent door registry.
- GPU wall commands reuse texture metadata across adjacent columns with the
  same wall ID, side and override, and append each rectangle's vertices together.
- Typical actor paths use stack storage for boundary crossings; longer paths
  can still grow their storage.
- Demo floor visibility owns reusable scratch storage. Its returned mask is
  valid until the next raycast; externally supplied probe masks remain separate.
- Music retains the existing sequencing, gain and locking behavior while
  removing intermediate PCM byte buffers.

## Repeat the benchmarks

On headless Linux, use Xvfb so Ebiten can initialize. Run comparisons on the same
machine with the same Go version, process count and benchmark fixtures.

```bash
GOMAXPROCS=2 xvfb-run -a go test -run '^$' \
  -bench '^(BenchmarkPlayerAreaConnectivity|BenchmarkGPUWallCommands|BenchmarkGPUCommandRect)$' \
  -benchmem -benchtime=200ms -count=5 .

GOMAXPROCS=2 xvfb-run -a go test -run '^$' \
  -bench '^(BenchmarkDemoActorProjections|BenchmarkActorPathBlocked)$' \
  -benchmem -count=3 .

GOMAXPROCS=2 xvfb-run -a go test -run '^$' \
  -bench '^BenchmarkMusicStreamRead$' \
  -benchmem -benchtime=4096x -count=3 .
```

`BenchmarkWallRaycast` also measures 320/1280/1920 columns with different worker
counts. Worker scheduling is unchanged in this pass; tune it only with evidence
from the intended machine and process count.

## Behavior checks

```bash
xvfb-run -a go test ./...
GD_WOLF_GPU_TEST=1 xvfb-run -a go test \
  -run '^(TestGPU|TestMusicStream|TestPlayerArea)' -count=1 .
GOOS=js GOARCH=wasm go build -o /tmp/gd-wolf-performance.wasm .
```

The GPU tests compare exact software/GPU pixels, including transparency and
floor lighting, and check texture transitions and vertex-batch boundaries.
Music tests compare one second of PCM with hashes captured before this change
for intro, menu and silence, using both aligned and irregular reads. They also
verify that switching songs discards a partial old frame. Area tests compare
with the previous map-scan algorithm across all ten shareware maps, both play
modes, seven door-state patterns and every floor-area starting position.

Gameplay validation also uses the original-C enemy, projectile, camera,
pushwall, DOS-alias and static-pool probes described in the
[demo comparison guide](wolf-demo-compare.md). The complete registered and
third-party demo corpus is rerun when gameplay or visibility behavior changes.

The optimization pass passed the complete 432-case corpus with zero observed
desyncs: 172,560 commands, 690,240 Wolf tics and 706,805,760 original-x86 floor
bits. Both all-map sweeps, the boss routes, and the stock/third-party recordings
in all three sound modes match the baseline totals. Every route enables actor
occupancy, area-plane, static-slot and x86 visibility comparisons. The aggregate
audit verifies matching frozen source/binary hashes and raw result consistency;
its local report is `build/demo-optimize-final/summary.json`.

The complete Go suite, exact pixel/PCM checks, original-C encounter and boundary
probes, seven Python tool tests and WebAssembly build also pass. This validates
the tested corpus; it does not establish parity for every possible input or
physical audio timing.
