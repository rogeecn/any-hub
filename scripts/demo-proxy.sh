#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-docker}"
CONFIG="configs/${MODE}.sample.toml"

if [[ ! -f "${CONFIG}" ]]; then
  echo "Unknown mode '${MODE}'. Available samples: docker, npm" >&2
  exit 1
fi

echo "Starting any-hub with ${CONFIG}"
exec go run . --config "${CONFIG}"
