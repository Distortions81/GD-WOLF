#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
OUT_DIR="$ROOT_DIR/build/wolf-demo-start-compare"
usage() {
  cat <<'EOF'
Compare actor initialization on all four built-in shareware demo maps with
compiled original C, then isolate UpdateFace RNG behavior from combat.

Usage: scripts/wolf_demo_start_compare.sh [--source <dir>] [--out <dir>]
  --source <dir>  Existing pinned id-Software/wolf3d checkout
  --out <dir>     Logs, initial state snapshots and replay inputs
                 (default: build/wolf-demo-start-compare)
  -h, --help     Show this help

Requirements: Go, Python 3, C99 compiler, Git for source download, and Xvfb
on headless Linux. This checks initialization and conditional face updates;
it does not run independent original enemy AI or combat.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source|--out)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      if [[ "$1" == --source ]]; then SOURCE_DIR="$2"; else OUT_DIR="$2"; fi
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
SOURCE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$SOURCE_DIR")"
OUT_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$OUT_DIR")"
if [[ ! -d "$SOURCE_DIR" ]]; then
  [[ "$SOURCE_DIR" == "$ROOT_DIR/build/wolfsrc-source" ]] || { echo "Missing source directory" >&2; exit 2; }
  mkdir -p "$(dirname "$SOURCE_DIR")"
  git clone https://github.com/id-Software/wolf3d.git "$SOURCE_DIR"
  git -C "$SOURCE_DIR" checkout --detach 05167784ef009d0d0daefe8d012b027f39dc8541
fi
mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"
PYTHONDONTWRITEBYTECODE=1 python3 tools/wolfsrc-reference/build_demo_start.py --source "$SOURCE_DIR" \
  --output "$OUT_DIR/wolf-demo-start-reference" --cc "${CC:-cc}" 2>&1 | tee "$OUT_DIR/build.log"
export GDWOLF_DEMO_START_REFERENCE="$OUT_DIR/wolf-demo-start-reference"
export GDWOLF_DEMO_START_OUT="$OUT_DIR"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run '^TestWolfDemoStartCompare$' -count=1 -parallel=1 \
  -timeout=5m -v . 2>&1 | tee "$OUT_DIR/compare.log"
