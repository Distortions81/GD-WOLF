#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOSBOX="${GDWOLF_DOSBOX:-$(command -v dosbox || true)}"
RUNTIME_DIR="$ROOT_DIR/build/wolf-demo-runtime-compare"
OUT_DIR="$ROOT_DIR/build/wolf-demo-dos-compare"
usage() {
  cat <<'EOF'
Capture health/ammo from the bundled DOS executable's first demo and compare
saved original-C runtime artifacts. Takes about two minutes on Linux.

Usage: scripts/wolf_demo_dos_compare.sh [--dosbox <exe>] [--runtime <dir>] [--out <dir>]
  --dosbox <exe>   DOSBox executable (default: GDWOLF_DOSBOX or dosbox on PATH)
  --runtime <dir>  Artifacts from wolf_demo_runtime_compare.sh
  --out <dir>      Game-state trace, DOSBox log/config and result.json
  -h, --help       Show this help

Requires DOSBox, Python 3, X11/XTest libraries and Xvfb on headless Linux.
Reads only the memory of its own DOSBox child. Does not install software.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dosbox|--runtime|--out)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --dosbox) DOSBOX="$2" ;;
        --runtime) RUNTIME_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n "$DOSBOX" ]] || { echo "DOSBox is required; pass --dosbox or install it" >&2; exit 2; }
RUNNER=()
if [[ -z "${DISPLAY:-}" ]]; then RUNNER=(xvfb-run -a -s '-screen 0 800x600x24'); fi
"${RUNNER[@]}" python3 "$ROOT_DIR/tools/wolfsrc-reference/capture_demo_dos.py" \
  --dosbox "$DOSBOX" --data "$ROOT_DIR/internal/wl6/shareware" \
  --runtime "$RUNTIME_DIR" --out "$OUT_DIR"
