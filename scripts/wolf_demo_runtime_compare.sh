#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
OUT_DIR="$ROOT_DIR/build/wolf-demo-runtime-compare"
DEMO_INDEX=0
DATA_DIR=""
EXTRA_FIRE=0
usage() {
  cat <<'EOF'
Compare recorded-demo movement, weapons, actors, RNG, doors, pushwalls and pickups
with compiled original C, then audit floor visibility with original x86 in QEMU.
Matching original death is a supported endpoint. Mismatches and unsupported
port terminal states exit nonzero.

Usage: scripts/wolf_demo_runtime_compare.sh [--source <dir>] [--out <dir>] [--data <dir>] [--demo-index <0-3>] [--extra-fire]
  --source <dir>  Existing pinned id-Software/wolf3d checkout
  --out <dir>     Compiler and comparison logs
                 (default: build/wolf-demo-runtime-compare)
  --demo-index <0-3>  Built-in demo from selected data (default: 0)
  --data <dir>   Local Wolfenstein 3D data directory (default: embedded WL1)
  --extra-fire   Replay recorded steering with a deterministic alternate attack schedule
  -h, --help     Show this help

Requirements: Go, Python 3, C99 compiler, GNU as/ld, qemu-system-i386,
Git for source download, and Xvfb
on headless Linux. See docs/wolf-demo-compare.md for coverage limits.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --extra-fire) EXTRA_FIRE=1; shift ;;
    --source|--out|--demo-index|--data)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --source) SOURCE_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
        --demo-index) DEMO_INDEX="$2" ;;
        --data) DATA_DIR="$2" ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ "$DEMO_INDEX" =~ ^[0-3]$ ]] || { echo "Invalid demo index: expected 0-3" >&2; exit 2; }
SOURCE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$SOURCE_DIR")"
OUT_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$OUT_DIR")"
if [[ -n "$DATA_DIR" ]]; then
  DATA_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$DATA_DIR")"
  [[ -d "$DATA_DIR" ]] || { echo "Missing data directory: $DATA_DIR" >&2; exit 2; }
fi
if [[ ! -d "$SOURCE_DIR" ]]; then
  [[ "$SOURCE_DIR" == "$ROOT_DIR/build/wolfsrc-source" ]] || { echo "Missing source directory" >&2; exit 2; }
  mkdir -p "$(dirname "$SOURCE_DIR")"
  git clone https://github.com/id-Software/wolf3d.git "$SOURCE_DIR"
  git -C "$SOURCE_DIR" checkout --detach 05167784ef009d0d0daefe8d012b027f39dc8541
fi
mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"
PYTHONDONTWRITEBYTECODE=1 python3 tools/wolfsrc-reference/build_demo_runtime.py --source "$SOURCE_DIR" \
  --output "$OUT_DIR/wolf-demo-runtime-reference" --cc "${CC:-cc}" 2>&1 | tee "$OUT_DIR/build.log"
export GDWOLF_DEMO_RUNTIME_REFERENCE="$OUT_DIR/wolf-demo-runtime-reference"
export GDWOLF_DEMO_RUNTIME_OUT="$OUT_DIR"
export GDWOLF_DEMO_INDEX="$DEMO_INDEX"
export GDWOLF_DEMO_RUNTIME_DATA="$DATA_DIR"
export GDWOLF_DEMO_EXTRA_FIRE="$EXTRA_FIRE"
export GDWOLF_DEMO_RAYCAST_OUT="$OUT_DIR/raycast-input.jsonl"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
export GOCACHE="${GOCACHE:-/tmp/gd-wolf-go-cache}"
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run '^TestWolfDemo(Runtime|Projection)Compare$' -count=1 -parallel=1 \
  -timeout=5m -v . 2>&1 | tee "$OUT_DIR/compare.log"
"$ROOT_DIR/scripts/wolf_demo_raycast_compare.sh" --source "$SOURCE_DIR" \
  --runtime "$OUT_DIR" --out "$OUT_DIR/raycast" 2>&1 | tee "$OUT_DIR/raycast.log"
