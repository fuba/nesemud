# Development Schedule

## Milestone 0 (done)
- Initialize Go project from scratch.
- Implement daemon runtime with background launch and logfile output.
- Implement HTTP API for control, memory peek/poke, state read.
- Implement FM2 parser and "inject then reset" behavior.
- Implement HLS pipeline using ffmpeg (video+audio) inside Docker.
- Add baseline unit tests for FM2 parser, iNES loader, API behavior.

## Milestone 1 (done)
- [done] Add 6502 CPU execution foundation and integrate into frame loop.
- [done] Complete all official opcodes and addressing modes.
- [done] Add page-crossing and branch cycle penalty accuracy.
- [done] Add interrupt paths (NMI/IRQ/BRK/RTI accuracy) and validation tests.

## Milestone 2 (done)
- Implement PPU register model and scanline/dot rendering pipeline.
- Implement APU channels and frame sequencer.
- Replace fallback renderer/audio with PPU/APU outputs.

## Milestone 3 (done)
- Extend mapper support beyond NROM (MMC1/MMC3/UxROM/CNROM/AXROM).
- Implement compatibility validation suite (nestest, blargg, MMC test ROMs).
- Build deterministic replay validation for FM2 against known hashes.

## Milestone 4 (done for current scope)
- Improve API interoperability (RetroArch command subset + richer schema).
- Add authn/authz, TLS reverse proxy examples, and operational hardening.
  - authn/authz was intentionally removed by explicit user request.
- Add CI with emulator conformance tests and artifact publishing.

## Final Status (2026-03-06)
- Master checklist fully checked.
- `go test ./...` green.
- Docker smoke test green.
- Public test ROM fetch script added for reproducible validation runs.

## Progress Update (2026-03-14)
- Added iNES cartridge compatibility improvements for four-screen mirroring, PRG-RAM, trainer loading, and legacy dirty-header mapper decoding.
- Added regression tests covering those cartridge compatibility paths.
- Added MMC5 PRG banking fixes for banked PRG-RAM and correct 16KB window decoding.

## Progress Update (2026-03-20)
- Upgraded `nestest` suite validation to use expected CPU state logs (correctness-oriented).
- Added `blargg-cpu` suite pass/fail probing via `$6000/$6004` status reporting.
- Added `/v1/validate/suite` support for per-ROM `expected_log_content`.
- Updated NESTest trace runner to align CPU initial state with the first expected log line, so `tests/roms/nestest.nes + nestest.log` passes.
- Added mid-frame `PPUCTRL`/`PPUSCROLL` split-state capture for same-scanline rendering transitions.
- Added `owned-evidence` validation mode to collect per-ROM video/audio/runtime evidence from owned ROM directories.
- Added `owned-evidence` checklist markdown generation (`--checklist-out`) with prioritized action items.
- Refined `blargg-cpu` suite probing to support status protocol variants while keeping deterministic fallback for ROMs that do not expose `$6000/$6004`.
- Added health-probe fallback checks for `ppu`/`apu`/`mapper` suites when status protocol is unavailable.
- Improved `owned-evidence` scoring by tracking intermediate non-uniform frame observation.
- Added adaptive extension in `owned-evidence` (uniform runs get additional frames) plus mapper hotspot reporting for triage.
- Added uniform-color transition tracking in `owned-evidence`, reducing false WARN classification for active-but-uniform scenes.
- Added CPU pause diagnostics (`last_cpu_error` in state + owned-evidence pause metadata) to accelerate real-ROM failure triage.
