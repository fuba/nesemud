# Progress Update (2026-03-20): Compatibility Validation Correctness

## Summary
- Upgraded suite validation for `nestest` from determinism-only checks to CPU state log comparison.
- Added support for supplying `expected_log_content` per ROM in `/v1/validate/suite`.
- Added automatic adjacent log loading for CLI/dir-based `nestest` suite runs (`*.log` and `nestest.log`).
- Added status/message correctness probing using `$6000` and `$6004` for `blargg-cpu` and, when available, `ppu`/`apu`/`mapper` suites.
- Added same-scanline split capture for mid-frame `PPUCTRL` and `PPUSCROLL` writes.
- Added `owned-evidence` collection mode for per-ROM video/audio/runtime evidence from owned ROM sets.
- Added `owned-evidence` checklist artifact generation with prioritized action items (`--checklist-out`).
- Aligned NESTest runner initial CPU state with the first expected trace line; `tests/roms/nestest.nes` now passes against `tests/roms/nestest.log`.
- Improved `blargg-cpu` status probing:
  - supports `$6000/$6004` status with optional `$6001-$6003` signature handling
  - handles reset-request status (`0x81`) flow
  - keeps deterministic fallback when a ROM does not expose the status protocol
- Added health-probe fallback for suites without status protocol (`ppu`/`apu`/`mapper`):
  - `ppu`/`mapper`: fail long-run uniform-frame output
  - `apu`: fail when no audio/APU activity is observed

## Validation
- Added/updated tests in:
  - `internal/validation/suites_test.go`
  - `internal/api/suite_validation_test.go`
- Verified with `go test ./...`.
- Verified CLI suite runs with current test assets:
  - `nestest`, `blargg-cpu`, `ppu`, `apu`, `mapper` all pass under `tests/roms`.

## Remaining Gaps
- `ppu`, `apu`, and `mapper` suite modes now have health-probe fallback, but still need stronger reference-oracle coverage beyond these heuristics.
- Real-ROM evidence collection exists, but WARN-heavy ROM clusters still require subsystem-level triage and fixes.
