# Implementation Gap Checklist (2026-03-10)

This replaces the overly-optimistic "all done" view in `doc/master-checklist.md`.
The codebase passes unit tests, but real ROM testing shows several compatibility gaps.

## Closure Note (2026-04-02)
- A full owned-ROM sweep was re-run on 2026-04-02 (`62` ROMs).
- Result: `61` healthy, `1` remaining known variance (Kirby local-image mismatch case).
- By user decision, current phase is closed with that remaining variance accepted as out-of-scope.

## 1. CPU
- [x] Expand illegal opcode coverage beyond the current small subset.
- [ ] Add correctness validation against real CPU conformance ROMs, not just "opcode does not error" tests.
- [ ] Audit cycle accuracy for unofficial opcodes and IRQ/NMI edge interactions under mapper IRQ load.

## 2. PPU
- [x] 8x16 sprite rendering support.
- [x] Sprite 0 hit status flag.
- [x] Sprite overflow flag behavior.
- [ ] Mid-frame scroll split correctness under real games (same-scanline `PPUCTRL`/`PPUSCROLL` split capture is now modeled; real-ROM audit still pending).
- [x] Left-edge masking behavior for background and sprites.
- [x] Palette grayscale/emphasis bits in `PPUMASK`.
- [ ] More real-ROM validation for HUD/status-bar split rendering.

## 3. APU
- [x] Add pulse 2 channel foundation.
- [x] Add pulse sweep units.
- [x] Add triangle linear counter foundation.
- [x] Implement `0x4017` frame counter behavior and IRQ mode accurately.
- [ ] Audit envelope/length/frame sequencer timing against test ROMs now that delayed `0x4017` frame-counter writes and basic sweep overflow handling are modeled.
- [ ] Validate DMC timing, looping, address wrap, and CPU interaction against real games now that basic fetch-driven CPU stall handling exists.
- [x] Add real audio timing regression coverage for mid-frame register writes.

## 4. Mapper / Cartridge
- [x] Support iNES four-screen mirroring, trainer loading, and header-derived PRG-RAM for supported mappers.
- [ ] Audit mapper 3 behavior against real CNROM titles (`Gradius` currently exposes rendering issues) now that basic bus-conflict handling exists.
- [ ] Audit mapper `5` behavior against a real MMC5 title (`Just Breed`) now that PRG banking, banked PRG-RAM, fill/ExRAM nametable routing, multiply registers, and scanline IRQ foundation exist.
- [x] Add more mapper coverage beyond `0/1/2/3/4/23/25/33/66/75/87/88/206` if needed by owned ROM set.
- [ ] Validate mirroring and IRQ behavior against real-game scenarios, not only focused unit tests.

## 5. Validation / Tooling
- [ ] Replace "determinism-only" suite checks with correctness-oriented compatibility checks (`nestest` and `blargg-cpu` improved; `ppu`/`apu`/`mapper` now include health-probe fallback but still need stronger reference oracles).
- [x] Add owned-ROM smoke coverage that boots every locally-owned ROM for a short run.
- [x] Add longer real-ROM regression tests for `Gradius` and `Just Breed` that verify frame progression, non-uniform video, and APU register activity.
- [x] Add owned-ROM checklist with captured evidence for video and audio, not just boot smoke coverage (`owned-evidence` now supports `--checklist-out` markdown generation).
- [ ] Keep `doc/master-checklist.md` aligned with actual implementation state.

## Current Priority Order
1. APU completeness for real-game audio
2. PPU split/HUD correctness for `Gradius`
3. Mapper 3 audit under real ROMs
4. Real-ROM validation for `MMC5`
5. Broader CPU/APU/PPU correctness validation
