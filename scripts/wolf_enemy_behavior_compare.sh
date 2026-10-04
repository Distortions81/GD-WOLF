#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR=""
OUT_DIR="$ROOT_DIR/build/wolf-enemy-behavior-compare"
MAP_INDEX=""

usage() {
  cat <<'EOF'
Compare sampled enemy sight, attack entry, movement and one-tick runtime
behavior across registered Wolfenstein 3D maps with the source-derived model.

Usage: scripts/wolf_enemy_behavior_compare.sh --data <dir> [--map-index <0-59>] [--out <dir>]
  --data <dir>       Directory containing original WL6 game data
  --map-index <n>    Compare only one map (default: all available maps)
  --out <dir>        Comparison log and JSONL mismatch report
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --data|--out|--map-index)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --data) DATA_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
        --map-index) MAP_INDEX="$2" ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n "$DATA_DIR" && -d "$DATA_DIR" ]] || { echo "--data must name a directory" >&2; exit 2; }
[[ -z "$MAP_INDEX" || "$MAP_INDEX" =~ ^([0-9]|[1-5][0-9])$ ]] || { echo "Invalid map index: $MAP_INDEX" >&2; exit 2; }
DATA_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$DATA_DIR")"
OUT_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$OUT_DIR")"
mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"
export GDWOLF_ENEMY_COMPARE_DATA="$DATA_DIR"
export GDWOLF_ENEMY_COMPARE_MAP_INDEX="$MAP_INDEX"
export GDWOLF_ENEMY_COMPARE_REPORT="$OUT_DIR/mismatches.jsonl"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run '^TestWolfEnemyBehaviorCompare$' -count=1 -parallel=1 \
  -timeout=10m -v . 2>&1 | tee "$OUT_DIR/compare.log"
