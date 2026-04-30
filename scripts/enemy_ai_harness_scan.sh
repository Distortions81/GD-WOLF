#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_MEM_LIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"
REPORT_PATH="${1:-}"

cd "${ROOT_DIR}"

echo "Running enemy AI source-compare scan"
echo "  parallel: 1"
echo "  GOMEMLIMIT: ${GO_MEM_LIMIT}"
if [[ -n "${REPORT_PATH}" ]]; then
  echo "  report: ${REPORT_PATH}"
fi

SCAN_ENV=(
  "GDWOLF_SCAN_WOLFSRC=1"
)
if [[ -n "${REPORT_PATH}" ]]; then
  SCAN_ENV+=( "GDWOLF_SCAN_WOLFSRC_REPORT=${REPORT_PATH}" )
fi

GOMEMLIMIT="${GO_MEM_LIMIT}" \
GOMAXPROCS=1 \
env \
  "${SCAN_ENV[@]}" \
go test -run TestEnemyAIHarnessScanSharewareNearbyPlacements -count=1 -parallel=1 -v .
