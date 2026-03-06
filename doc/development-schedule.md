# Development Schedule

## Milestone 0 (done)
- Initialize Go project from scratch.
- Implement daemon runtime with background launch and logfile output.
- Implement HTTP API for control, memory peek/poke, state read.
- Implement FM2 parser and "inject then reset" behavior.
- Implement HLS pipeline using ffmpeg (video+audio) inside Docker.
- Add baseline unit tests for FM2 parser, iNES loader, API behavior.

## Milestone 1 (in progress)
- [done] Add 6502 CPU execution foundation and integrate into frame loop.
- [next] Complete all official opcodes and addressing modes.
- [next] Add page-crossing and branch cycle penalty accuracy.
- [next] Add interrupt paths (NMI/IRQ/BRK/RTI accuracy) and validation tests.

## Milestone 2
- Implement PPU register model and scanline/dot rendering pipeline.
- Implement APU channels and frame sequencer.
- Replace fallback renderer/audio with PPU/APU outputs.

## Milestone 3
- Extend mapper support beyond NROM (MMC1/MMC3/UxROM/CNROM/AXROM).
- Implement compatibility validation suite (nestest, blargg, MMC test ROMs).
- Build deterministic replay validation for FM2 against known hashes.

## Milestone 4
- Improve API interoperability (RetroArch command subset + richer schema).
- Add authn/authz, TLS reverse proxy examples, and operational hardening.
- Add CI with emulator conformance tests and artifact publishing.
