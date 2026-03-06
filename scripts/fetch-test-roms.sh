#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-./tests/roms}"
mkdir -p "${OUT_DIR}"

fetch() {
  local url="$1"
  local out="$2"
  echo "fetch ${out}"
  curl -L -sS "${url}" -o "${OUT_DIR}/${out}"
}

# NOTE: These assets are fetched from public emulator test ROM repositories.
# Check upstream licenses before redistribution.
fetch "https://raw.githubusercontent.com/christopherpow/nes-test-roms/master/other/nestest.nes" "nestest.nes"
fetch "https://raw.githubusercontent.com/christopherpow/nes-test-roms/master/other/nestest.log" "nestest.log"
fetch "https://raw.githubusercontent.com/christopherpow/nes-test-roms/master/instr_test-v5/official_only.nes" "blargg_cpu_official_only.nes"
fetch "https://raw.githubusercontent.com/christopherpow/nes-test-roms/master/ppu_vbl_nmi/rom_singles/01-vbl_basics.nes" "ppu_vbl_basics.nes"
fetch "https://raw.githubusercontent.com/christopherpow/nes-test-roms/master/apu_test/rom_singles/1-len_ctr.nes" "apu_len_ctr.nes"
fetch "https://raw.githubusercontent.com/christopherpow/nes-test-roms/master/mmc3_test_2/rom_singles/1-clocking.nes" "mmc3_mapper_clocking.nes"

echo "done: ${OUT_DIR}"
