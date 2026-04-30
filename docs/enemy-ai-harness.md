# Enemy AI Harness

This repo now has a source-backed enemy AI comparison harness modeled after the `GD-DOOM` reference-compare workflow.

## What It Does

The harness compares:

- a normalized `GD-WOLF` enemy movement decision
- against an independent `WOLFSRC` reference decision adapter written from the vendored source rules

The current harness focuses on the highest-value shared decision points:

- first-sighting state transitions
- attack-entry decisions
- `SelectChaseDir`
- `SelectDodgeDir`
- `SelectRunDir`
- `TryWalk`-style move acceptance for those decisions
- short scripted multi-tic traces built on top of those same compares

It intentionally normalizes both sides into the same output shape:

- chosen direction
- intended destination tile
- whether a move exists
- whether the move is a wait-on-door result

That lets the harness compare behavior directly even though `GD-WOLF` stores movement as `goalX/goalY` and `WOLFSRC` mutates `tilex/tiley` immediately after `TryWalk`.

## Tests

Smoke coverage runs in the normal test suite:

```bash
go test -run TestEnemyAIHarnessSmoke .
```

An exhaustive real-map scan is available behind an env var:

```bash
GDWOLF_SCAN_WOLFSRC=1 go test -run TestEnemyAIHarnessScanSharewareNearbyPlacements .
```

You can also write every mismatch to a JSONL report:

```bash
GDWOLF_SCAN_WOLFSRC=1 \
GDWOLF_SCAN_WOLFSRC_REPORT=/tmp/gd-wolf-enemy-ai-scan.jsonl \
go test -run TestEnemyAIHarnessScanSharewareNearbyPlacements .
```

That scan:

1. loads each embedded shareware map
2. finds live enemies
3. places the player on nearby valid walkable positions
4. compares first-sighting, attack-entry, and movement decisions against the local `WOLFSRC` adapter
5. records every mismatch it sees during the scan
6. fails at the end if any actionable mismatches were found

Each JSONL record includes:

- compare layer
- map and actor identity
- player and enemy position
- normalized `want` and `got` states
- a compact diff string

For normal use, prefer the wrapper script:

```bash
./scripts/enemy_ai_harness_scan.sh
./scripts/enemy_ai_harness_scan.sh /tmp/gd-wolf-enemy-ai-scan.jsonl
```

The wrapper keeps the scan single-threaded and applies the same default Go heap cap as the soak harness:

- `-parallel=1`
- `GOMAXPROCS=1`
- `GOMEMLIMIT=12GiB`

## Trace Scan

The harness also has a multi-tic trace mode that advances the real runtime while checking source-backed parity at each step.

Current built-in trace scripts:

- `stationary`
- `circle_strafe`
- `backpedal`
- `zigzag_strafe`
- `door_bait`

Smoke coverage:

```bash
go test -run TestEnemyAITraceHarnessSmoke .
```

Env-gated real-map trace scan:

```bash
GDWOLF_SCAN_WOLFSRC_TRACE=1 \
go test -run TestEnemyAIHarnessScanSharewareTraceScripts -parallel=1 -v .
```

Door-focused trace scan:

```bash
GDWOLF_SCAN_WOLFSRC_TRACE_DOORS=1 \
go test -run TestEnemyAIHarnessScanSharewareDoorTraceScripts -parallel=1 -v .
```

That trace scan:

1. starts from cached first-tic level snapshots
2. picks one nearby placement per enemy
3. runs a deterministic player script for several steps
4. compares first-sighting, attack-entry, and movement parity before each runtime advance
5. minimizes each trace mismatch to the earliest reproducing step count
6. fails on the first accumulated mismatch set, optionally writing JSONL records through `GDWOLF_SCAN_WOLFSRC_REPORT`

The door trace variant uses door-adjacent starting placements and a `door_bait` player script to stress:

- door wait decisions
- repeated open/wait transitions
- chase retargeting around doorway blockers

## Sequential Soak

For longer deterministic AI stress runs, use the sequential soak instead of the Go fuzz scheduler.

It always runs:

- single-threaded
- map by map in order
- with either a fixed time budget per level or a fixed run count per level

Script entrypoint:

```bash
./scripts/enemy_ai_soak.sh 30s
./scripts/enemy_ai_soak.sh 2m
./scripts/enemy_ai_soak.sh 10000
```

The soak wrapper also forces lower resource usage by default:

- `-parallel=1`
- `GOMAXPROCS=1`
- `GOMEMLIMIT=12GiB`

You can override the Go heap limit if needed:

```bash
GDWOLF_GO_MEM_LIMIT=8GiB ./scripts/enemy_ai_soak.sh 1m
```

Direct test invocation:

```bash
GDWOLF_ENEMY_SOAK=1 GDWOLF_ENEMY_SOAK_PER_LEVEL=30s \
  go test -run TestEnemyAISequentialLevelSoak -count=1 -parallel=1 -v .

GDWOLF_ENEMY_SOAK=1 GDWOLF_ENEMY_SOAK_RUNS_PER_MAP=10000 \
  go test -run TestEnemyAISequentialLevelSoak -count=1 -parallel=1 -v .
```

The soak reuses the same nearby-placement and circle-strafe scenario generator as the fuzz target, but schedules scenarios deterministically per level instead of letting the fuzz engine distribute time globally.

## Why This Is Not A Direct Runtime Compare

Unlike `GD-DOOM`, this repo does not currently have a practical runnable reference binary harness for the original engine in the local environment.

So the current harness uses:

- the vendored `WOLFSRC` code as the authority
- an independent Go reference adapter for the specific AI routines under test

That keeps the workflow repeatable and useful now, while still leaving room for a fuller trace-based compare later if a runnable reference path becomes practical.
