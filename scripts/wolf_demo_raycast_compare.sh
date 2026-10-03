#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/build/wolfsrc-source"
RUNTIME_DIR="$ROOT_DIR/build/wolf-demo-runtime-compare"
OUT_DIR="$ROOT_DIR/build/wolf-demo-raycast-compare"
usage() {
  cat <<'EOF'
Run original WL_DR_A.ASM ray traversal under QEMU against saved demo floor masks.
First generate raycast-input.jsonl with wolf_demo_runtime_compare.sh.

Usage: scripts/wolf_demo_raycast_compare.sh [--source <dir>] [--runtime <dir>] [--out <dir>]
  --source <dir>   Pinned original source checkout
  --runtime <dir>  Runtime artifacts containing the C reference and raycaster trace
  --out <dir>      Assembly, boot image, visibility bytes and result.json
  -h, --help       Show this help

Requirements: Python 3, GNU as/ld, and qemu-system-i386. No display is needed.
EOF
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source|--runtime|--out)
      [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; exit 2; }
      case "$1" in
        --source) SOURCE_DIR="$2" ;;
        --runtime) RUNTIME_DIR="$2" ;;
        --out) OUT_DIR="$2" ;;
      esac
      shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
PYTHONDONTWRITEBYTECODE=1 python3 "$ROOT_DIR/tools/wolfsrc-reference/compare_demo_raycast.py" \
  --source "$SOURCE_DIR" --reference "$RUNTIME_DIR/wolf-demo-runtime-reference" \
  --trace "$RUNTIME_DIR/raycast-input.jsonl" --out "$OUT_DIR"
