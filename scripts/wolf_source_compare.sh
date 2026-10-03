#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_REVISION=05167784ef009d0d0daefe8d012b027f39dc8541
SOURCE_DIR="${ROOT_DIR}/build/wolfsrc-source"
OUT_DIR="${ROOT_DIR}/build/wolf-source-compare"
INPUT_PATH=""
SELF_TEST=0
MAX_MISMATCHES=1

usage() {
  cat <<'EOF'
Compile original Wolf3D C movement routines and compare against GD-WOLF.
Usage: scripts/wolf_source_compare.sh [options]
  --source <dir>  Existing id-Software/wolf3d checkout (must match pinned files)
  --out <dir>     Logs, inputs and JSONL traces (default: build/wolf-source-compare)
  --input <file>  Replay input JSONL (plain or gzip) instead of generating scenarios
  --self-test     Check the reference protocol and RNG without comparing the port
  --max-mismatches <n>  Collect up to n desyncs (default: stop at the first)
  -h, --help     Show this help

With no --source, downloads the pinned source into ignored build/wolfsrc-source.
Requirements: Go, Python 3, a C99 compiler (CC or cc), Git for source download;
on headless Linux, xvfb-run. Exits nonzero on the first behavior mismatch.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source|--out|--input|--max-mismatches)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --source) SOURCE_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
        --input) INPUT_PATH="$2" ;;
        --max-mismatches) MAX_MISMATCHES="$2" ;;
      esac
      shift 2 ;;
    --self-test) SELF_TEST=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# Resolve user paths before changing to the repository root.
OUT_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$OUT_DIR")"
SOURCE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$SOURCE_DIR")"
if [[ -n "$INPUT_PATH" ]]; then
  INPUT_PATH="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$INPUT_PATH")"
  # Never overwrite a replay input with fresh trace outputs.
  [[ "$INPUT_PATH" != "$OUT_DIR/inputs.jsonl.gz" && "$INPUT_PATH" != "$OUT_DIR/mismatch-inputs.jsonl" ]] || {
    echo "Replay requires a different --out directory" >&2; exit 2;
  }
fi
if [[ ! -d "$SOURCE_DIR" ]]; then
  [[ "$SOURCE_DIR" == "$ROOT_DIR/build/wolfsrc-source" ]] || {
    echo "Source directory does not exist: $SOURCE_DIR" >&2; exit 2;
  }
  mkdir -p "$(dirname "$SOURCE_DIR")"
  git clone https://github.com/id-Software/wolf3d.git "$SOURCE_DIR"
  git -C "$SOURCE_DIR" checkout --detach "$SOURCE_REVISION"
fi
mkdir -p "$OUT_DIR"
cd "$ROOT_DIR"
python3 tools/wolfsrc-reference/build.py --source "$SOURCE_DIR" \
  --output "$OUT_DIR/wolf-source-reference" --cc "${CC:-cc}" 2>&1 | tee "$OUT_DIR/build.log"

export GDWOLF_WOLFSRC_REFERENCE="$OUT_DIR/wolf-source-reference"
export GDWOLF_WOLFSRC_OUT="$OUT_DIR"
export GDWOLF_WOLFSRC_INPUT="$INPUT_PATH"
export GDWOLF_WOLFSRC_MAX_MISMATCHES="$MAX_MISMATCHES"
export GOMAXPROCS=1
export GOMEMLIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
TESTS='^TestWolfSource(ReferenceProtocol|Compare)$'
if [[ "$SELF_TEST" == 1 ]]; then TESTS='^TestWolfSourceReferenceProtocol$'; fi
RUNNER=()
if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null || { echo "Headless Ebiten tests require xvfb-run" >&2; exit 2; }
  RUNNER=(xvfb-run -a)
fi
"${RUNNER[@]}" go test -run "$TESTS" -count=1 -parallel=1 -timeout=10m -v . \
  2>&1 | tee "$OUT_DIR/compare.log"
