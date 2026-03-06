#!/usr/bin/env bash
set -euo pipefail

ROM_DIR="${1:-./tests/roms}"
FRAMES="${2:-3000}"

run() {
  local suite="$1"
  echo "== suite: ${suite} =="
  go run ./cmd/nes-validate --suite "${suite}" --rom-dir "${ROM_DIR}" --frames "${FRAMES}" || true
  echo
}

run nestest
run blargg-cpu
run ppu
run apu
run mapper
