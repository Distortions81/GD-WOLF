#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
OUT_DIR="$ROOT_DIR/build/wolf-enemy-runtime-compare"
DATA_DIR=""
usage() {
  cat <<'EOF'
Compare multi-tick enemy encounters with independently running original C.

Usage: scripts/wolf_enemy_runtime_compare.sh [--source <dir>] [--out <dir>] [--data <dir>]
  --source <dir>  Existing pinned id-Software/wolf3d checkout
  --out <dir>     Compiler, comparison logs and replayable JSONL traces
  --data <dir>   Local Wolfenstein 3D data directory (default: embedded WL1)
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source|--out|--data)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --source) SOURCE_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
        --data) DATA_DIR="$2" ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
SOURCE_DIR="$(realpath "$SOURCE_DIR")"
OUT_DIR="$(realpath -m "$OUT_DIR")"
if [[ -n "$DATA_DIR" ]]; then
  DATA_DIR="$(realpath "$DATA_DIR")"
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
export GDWOLF_DEMO_OCCUPANCY=1
export GDWOLF_DEMO_AREA_PLANE=1
export GDWOLF_ENEMY_RUNTIME_REFERENCE="$OUT_DIR/wolf-demo-runtime-reference"
export GDWOLF_ENEMY_RUNTIME_OUT="$OUT_DIR"
export GDWOLF_ENEMY_RUNTIME_DATA="$DATA_DIR"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
export GOCACHE="${GOCACHE:-/tmp/gd-wolf-go-cache}"
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run '^(TestWolfEnemyRuntimeCompare|TestWolfRegisteredActorsRuntime|TestWolfActorMoveContactCompare|TestWolfProjectilesRuntimeCompare|TestWolfDemoDeathCamCompare|TestWolfAmbushAreaCompare|TestWolfPushWallIntoOpenDoorCompare|TestWolfDemoDOSAliasedDoorCompare|TestWolfStaticPoolCompare|TestWolfStaticCatalogCompare)$' -count=1 -parallel=1 \
  -timeout=5m -v . 2>&1 | tee "$OUT_DIR/compare.log"
