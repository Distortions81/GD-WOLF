#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
OUT_DIR="$ROOT_DIR/build/wolf-demo-runtime-compare"
DEMO_INDEX=0
DEMO_INDEX_SET=0
DEMO_FILE=""
DATA_DIR=""
EXTRA_FIRE=0
SOUND_MODE=adlib-digi
MEMORY_PROFILE=strict-source
usage() {
  cat <<'EOF'
Compare recorded-demo movement, weapons, actors, RNG, doors, pushwalls and pickups
with compiled original C, then audit floor visibility with original x86 in QEMU.
Matching original death or elevator exit is a supported endpoint. Mismatches and unsupported
port terminal states exit nonzero.

Usage: scripts/wolf_demo_runtime_compare.sh [--source <dir>] [--out <dir>] [--data <dir>] [--demo-index <0-3> | --demo-file <file>] [--extra-fire]
  --source <dir>  Existing pinned id-Software/wolf3d checkout
  --out <dir>     Compiler and comparison logs
                 (default: build/wolf-demo-runtime-compare)
  --demo-index <0-3>  Built-in demo from selected data (default: 0)
  --demo-file <file>  External original-format recording for the selected data
  --data <dir>   Local Wolfenstein 3D data directory (default: embedded WL1)
  --extra-fire   Replay recorded steering with a deterministic alternate attack schedule
  --sound-mode <off|adlib|adlib-digi>  Original sound routing (default: adlib-digi)
  --memory-profile <strict-source|registered-apogee-v1.4-2a969a97>
                 Original memory bounds or named DOS executable layout (default: strict-source)
  -h, --help     Show this help

Requirements: Go, Python 3, C99 compiler, GNU as/ld, qemu-system-i386,
Git for source download, and Xvfb
on headless Linux. See docs/wolf-demo-compare.md for coverage limits.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --extra-fire) EXTRA_FIRE=1; shift ;;
    --source|--out|--demo-index|--demo-file|--data|--sound-mode|--memory-profile)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --source) SOURCE_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
        --demo-index) DEMO_INDEX="$2"; DEMO_INDEX_SET=1 ;;
        --demo-file) DEMO_FILE="$2" ;;
        --data) DATA_DIR="$2" ;;
        --sound-mode) SOUND_MODE="$2" ;;
        --memory-profile) MEMORY_PROFILE="$2" ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ "$DEMO_INDEX" =~ ^[0-3]$ ]] || { echo "Invalid demo index: expected 0-3" >&2; exit 2; }
[[ "$SOUND_MODE" == off || "$SOUND_MODE" == adlib || "$SOUND_MODE" == adlib-digi ]] || { echo "Invalid sound mode: expected off, adlib, or adlib-digi" >&2; exit 2; }
[[ "$MEMORY_PROFILE" == strict-source || "$MEMORY_PROFILE" == registered-apogee-v1.4-2a969a97 ]] || { echo "Invalid memory profile: expected strict-source or registered-apogee-v1.4-2a969a97" >&2; exit 2; }
[[ -z "$DEMO_FILE" || "$DEMO_INDEX_SET" == 0 ]] || { echo "Choose --demo-index or --demo-file, not both" >&2; exit 2; }
SOURCE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$SOURCE_DIR")"
OUT_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$OUT_DIR")"
if [[ -n "$DATA_DIR" ]]; then
  DATA_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$DATA_DIR")"
  [[ -d "$DATA_DIR" ]] || { echo "Missing data directory: $DATA_DIR" >&2; exit 2; }
fi
if [[ -n "$DEMO_FILE" ]]; then
  DEMO_FILE="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$DEMO_FILE")"
  [[ -f "$DEMO_FILE" ]] || { echo "Missing demo file: $DEMO_FILE" >&2; exit 2; }
fi
if [[ ! -d "$SOURCE_DIR" ]]; then
  [[ "$SOURCE_DIR" == "$ROOT_DIR/build/wolfsrc-source" ]] || { echo "Missing source directory" >&2; exit 2; }
  mkdir -p "$(dirname "$SOURCE_DIR")"
  git clone https://github.com/id-Software/wolf3d.git "$SOURCE_DIR"
  git -C "$SOURCE_DIR" checkout --detach 05167784ef009d0d0daefe8d012b027f39dc8541
fi
mkdir -p "$OUT_DIR"
rm -f "$OUT_DIR/result.json" "$OUT_DIR/raycast/result.json" "$OUT_DIR/raycast-input.jsonl"
cd "$ROOT_DIR"
PYTHONDONTWRITEBYTECODE=1 python3 - "$OUT_DIR" "${DATA_DIR:-$ROOT_DIR/internal/wl6/shareware}" "$DEMO_FILE" "$DEMO_INDEX" "$EXTRA_FIRE" "$SOUND_MODE" "$MEMORY_PROFILE" <<'PY'
import hashlib
import json
from pathlib import Path
import sys
from scripts.wolf_demo_corpus_compare import memory_profile_metadata

out, data, demo, index, extra, sound_mode, memory_profile = sys.argv[1:]
def fingerprint(path):
    raw = path.read_bytes()
    return {"bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()}

data_path = Path(data)
names = {"MAPHEAD", "GAMEMAPS", "VSWAP", "VGADICT", "VGAHEAD", "VGAGRAPH", "AUDIOHED", "AUDIOT"}
manifest = {
    "reference_revision": "05167784ef009d0d0daefe8d012b027f39dc8541",
    "data_directory": str(data_path),
    "data_files": {p.name: fingerprint(p) for p in sorted(data_path.iterdir())
                   if p.is_file() and p.stem.upper() in names and p.suffix.upper() in (".WL1", ".WL6")},
    "demo_index": None if demo else int(index),
    "demo_file": {"path": demo, **fingerprint(Path(demo))} if demo else None,
    "extra_fire": extra == "1",
    "sound_mode": sound_mode,
    "memory_profile": memory_profile_metadata(memory_profile),
}
(Path(out) / "input-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
PY
PYTHONDONTWRITEBYTECODE=1 python3 tools/wolfsrc-reference/build_demo_runtime.py --source "$SOURCE_DIR" \
  --output "$OUT_DIR/wolf-demo-runtime-reference" --cc "${CC:-cc}" 2>&1 | tee "$OUT_DIR/build.log"
export GDWOLF_DEMO_RUNTIME_REFERENCE="$OUT_DIR/wolf-demo-runtime-reference"
export GDWOLF_DEMO_RUNTIME_OUT="$OUT_DIR"
export GDWOLF_DEMO_INDEX="$DEMO_INDEX"
export GDWOLF_DEMO_FILE="$DEMO_FILE"
export GDWOLF_DEMO_RUNTIME_DATA="$DATA_DIR"
export GDWOLF_DEMO_EXTRA_FIRE="$EXTRA_FIRE"
export GDWOLF_DEMO_SOUND_MODE="$SOUND_MODE"
export GDWOLF_DEMO_MEMORY_PROFILE="$MEMORY_PROFILE"
export GDWOLF_DEMO_RAYCAST_OUT="$OUT_DIR/raycast-input.jsonl"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
export GOCACHE="${GOCACHE:-/tmp/gd-wolf-go-cache}"
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run '^TestWolfDemo(Runtime|Projection|Rewards|Elevator|Sound|Victory|VictoryEntry)Compare$' -count=1 -parallel=1 \
  -timeout=5m -v . 2>&1 | tee "$OUT_DIR/compare.log"
"$ROOT_DIR/scripts/wolf_demo_raycast_compare.sh" --source "$SOURCE_DIR" \
  --runtime "$OUT_DIR" --out "$OUT_DIR/raycast" 2>&1 | tee "$OUT_DIR/raycast.log"
