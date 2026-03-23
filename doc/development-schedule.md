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

## Progress Update (2026-03-22)
- Fixed MMC3 PRG bank masking to use lower 6 bits and added regression coverage.
- Fixed MMC3 IRQ clock gating so scanline IRQ only clocks while rendering is enabled.
- Corrected CPU IRQ line handling so mapper/APU IRQs are level-checked (not consumed on poll), preventing masked-IRQ loss.
- Connected APU frame/DMC IRQ pending state into CPU IRQ servicing path.
- Added VRC4 (`mapper 25`) battery-backed PRG-RAM default inference when iNES byte8 is zero.
- Re-ran full owned-ROM evidence at `--frames 240`; paused/error count reached `0` (uniform WARN backlog remains for deeper PPU/mapper audits).
- Added PPU frame-sequence tracking (`frame_id`) and changed `StepFrame` execution to advance until exactly one PPU frame boundary per call.
- Moved frame finalization to vblank entry (`scanline 241, cycle 1`) so vblank-time register writes cannot corrupt per-scanline render-state reconstruction.
- Expanded per-scanline split segment capacity and kept the newest state reachable at the right edge when capacity is hit.
- Reproduced and fixed the long-run display corruption reported on `Fushigi na Blobby`; post-fix frame dumps remain stable in title/credits/gameplay scenes.

## Progress Update (2026-03-23)
- Added MMC3 PRG-RAM control handling (`$A001` odd): enable + write-protect semantics are now enforced on mapper4 `$6000-$7FFF` accesses.
- Added mapper4 PRG-RAM protection regression tests for enable-required reads and write-protect behavior.
- Fixed mapper206 PRG bank selection math to use modulo over actual 8KiB bank count (correct behavior for non-power-of-two bank counts).
- Added mapper206 regression coverage for modulo-based PRG bank selection.
- Aligned interrupt timing/clocking:
  - NMI is now queued with one-instruction latency instead of immediate post-step injection.
  - IRQ/NMI entry cycles are now propagated to PPU/APU/audio advancement (no more CPU-only cycle jumps).
- Added interrupt-entry timing regressions for both NMI and IRQ paths.
- Re-ran owned-ROM evidence (`--frames 240`): paused/error remains `0`, and uniform-stuck backlog reduced from `4` to `3` (`Donald Land` cleared).
