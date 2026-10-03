#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
OUT_DIR="$ROOT_DIR/build/wolf-demo-player-compare"
DEMO_INDEX=0
DEMO_FILE=""
COMMAND_LIMIT=0
SELF_TEST=0
usage() {
  cat <<'EOF'
Replay Wolf3D demo commands and compare player movement with original C.
The reference evolves its own player position using shared port world snapshots.
Enemy/combat state is recorded only on the port; this is not a full-engine compare.

Usage: scripts/wolf_demo_player_compare.sh [options]
  --source <dir>  Existing pinned id-Software/wolf3d checkout
  --out <dir>     Logs, commands, player traces and port runtime trace
  --demo-index <0-3>  Built-in shareware demo (default: 0, E1F1)
  --demo-file <file>  Recorded demo using the embedded shareware map data
  --stop-after-commands <n>  Compare a prefix (0 = whole demo)
  --self-test     Check the reference protocol and all 360 movement angles
  -h, --help     Show this help

Requirements: Go, Python 3, C99 compiler, Git for source download, and Xvfb
on headless Linux. Exits nonzero on mismatch or unsupported death/victory.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source|--out|--demo-index|--demo-file|--stop-after-commands)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --source) SOURCE_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
        --demo-index) DEMO_INDEX="$2" ;;
        --demo-file) DEMO_FILE="$2" ;;
        --stop-after-commands) COMMAND_LIMIT="$2" ;;
      esac
      shift 2 ;;
    --self-test) SELF_TEST=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ "$DEMO_INDEX" =~ ^[0-3]$ && "$COMMAND_LIMIT" =~ ^[0-9]+$ ]] || {
  echo "Invalid demo index or command limit" >&2; exit 2;
}
SOURCE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$SOURCE_DIR")"
OUT_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$OUT_DIR")"
if [[ -n "$DEMO_FILE" ]]; then
  DEMO_FILE="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$DEMO_FILE")"
  case "$DEMO_FILE" in
    "$OUT_DIR"/*) echo "Demo input must be outside --out to preserve the recording" >&2; exit 2 ;;
  esac
fi
if [[ ! -d "$SOURCE_DIR" ]]; then
  [[ "$SOURCE_DIR" == "$ROOT_DIR/build/wolfsrc-source" ]] || { echo "Missing source directory" >&2; exit 2; }
  mkdir -p "$(dirname "$SOURCE_DIR")"
  git clone https://github.com/id-Software/wolf3d.git "$SOURCE_DIR"
  git -C "$SOURCE_DIR" checkout --detach 05167784ef009d0d0daefe8d012b027f39dc8541
fi
mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"
python3 tools/wolfsrc-reference/build_demo_player.py --source "$SOURCE_DIR" \
  --output "$OUT_DIR/wolf-demo-player-reference" --cc "${CC:-cc}" 2>&1 | tee "$OUT_DIR/build.log"
export GDWOLF_DEMO_PLAYER_REFERENCE="$OUT_DIR/wolf-demo-player-reference"
export GDWOLF_DEMO_OUT="$OUT_DIR"
export GDWOLF_DEMO_INDEX="$DEMO_INDEX"
export GDWOLF_DEMO_FILE="$DEMO_FILE"
export GDWOLF_DEMO_COMMAND_LIMIT="$COMMAND_LIMIT"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
TESTS='^TestWolfDemoPlayer(ReferenceProtocol|Compare)$'
if [[ "$SELF_TEST" == 1 ]]; then TESTS='^TestWolfDemoPlayerReferenceProtocol$'; fi
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run "$TESTS" -count=1 -parallel=1 -timeout=5m -v . \
  2>&1 | tee "$OUT_DIR/compare.log"
