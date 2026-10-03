#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
OUT_DIR="$ROOT_DIR/build/wolf-actor-states-compare"
usage() {
  cat <<'EOF'
Compare original DoActor timing, think callbacks and attack actions for
26 supported actor state sequences, including zero initial timers.

Usage: scripts/wolf_actor_states_compare.sh [--source <dir>] [--out <dir>]
  --source <dir>  Existing pinned id-Software/wolf3d checkout
  --out <dir>     Compiler and comparison logs
                 (default: build/wolf-actor-states-compare)
  -h, --help     Show this help

Requirements: Go, Python 3, C99 compiler, Git for source download, and Xvfb
on headless Linux. See docs/wolf-demo-compare.md for coverage limits.
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
PYTHONDONTWRITEBYTECODE=1 python3 tools/wolfsrc-reference/build_actor_states.py --source "$SOURCE_DIR" \
  --output "$OUT_DIR/wolf-actor-states-reference" --cc "${CC:-cc}" 2>&1 | tee "$OUT_DIR/build.log"
export GDWOLF_ACTOR_STATES_REFERENCE="$OUT_DIR/wolf-actor-states-reference"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run '^TestWolfActorStatesCompare$' -count=1 -parallel=1 \
  -timeout=5m -v . 2>&1 | tee "$OUT_DIR/compare.log"
