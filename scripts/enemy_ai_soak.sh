#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE_VALUE="${1:-30s}"
GO_MEM_LIMIT="${GDWOLF_GO_MEM_LIMIT:-12GiB}"

SOAK_ENV=()
MODE_LABEL="per-level budget"
if [[ "${MODE_VALUE}" =~ ^[0-9]+$ ]]; then
  MODE_LABEL="runs per map"
  SOAK_ENV+=( "GDWOLF_ENEMY_SOAK_RUNS_PER_MAP=${MODE_VALUE}" )
else
  SOAK_ENV+=( "GDWOLF_ENEMY_SOAK_PER_LEVEL=${MODE_VALUE}" )
fi

cd "${ROOT_DIR}"

echo "Running sequential enemy AI soak"
echo "  ${MODE_LABEL}: ${MODE_VALUE}"
echo "  parallel: 1"
echo "  GOMEMLIMIT: ${GO_MEM_LIMIT}"

GOMEMLIMIT="${GO_MEM_LIMIT}" \
GOMAXPROCS=1 \
env \
  "${SOAK_ENV[@]}" \
GDWOLF_ENEMY_SOAK=1 \
go test -run TestEnemyAISequentialLevelSoak -count=1 -parallel=1 -v .
