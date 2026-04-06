# Progress Update (2026-03-31): mapper3 CHR bank select width

## What changed
- Updated mapper3 (`CNROM`) PRG-write bank select handling to preserve full bus-conflicted value bits.
  - Previously the selected value was hard-masked to 2 bits (`& 0x03`), which prevented valid high-bank selection on larger CHR configurations.
  - Selection now keeps upper bits and relies on existing modulo logic against actual `CHRBanks` when reading CHR.

## Tests added
- `TestMapper3CHRBankSwitchKeepsUpperBitsForLargeCHR`

## Verification
- `go test ./...` passed.
- `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240` re-run completed with no regressions.
  - Action items remain `1` (Kirby mapper4 uniform output).

