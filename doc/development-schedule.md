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
