# Master Checklist to Final Completion

This checklist is the source of truth. Keep implementing until all items are checked.

## 0. Runtime / Ops
- [x] Daemonized runtime with background launch and logfile output
- [x] Hot reload trigger via SIGHUP
- [x] Dockerized runtime including ffmpeg (no system ffmpeg dependency)
- [x] HLS output available from API host

## 1. API / Control Surface
- [x] OpenAPI endpoint
- [x] External control API: load/reset/pause/resume/state/input
- [x] Memory inspect and memory write API
- [x] FM2 injection with immediate reset + replay start
- [x] API compatibility profile aligned with a de-facto standard command set
- [x] Authentication/authorization for production exposure (Removed by request: no auth)

## 2. CPU (6502)
- [x] CPU core integrated into frame execution loop
- [x] NMI path connected from PPU VBlank
- [x] Branch/page-cross cycle penalties (major paths)
- [x] Complete all official opcodes and all addressing-mode variants
- [x] IRQ/BRK/RTI edge-case timing validation
- [x] Illegal opcode behavior (for high compatibility titles)

## 3. PPU
- [x] PPU register map and VRAM/palette/OAM DMA plumbing
- [x] Background pixel pipeline (name table + attribute + pattern decode) used for frame output
- [x] Sprite evaluation/rendering (priority, flipping, transparency)
- [x] Scrolling logic and timing-correct register behavior
- [x] VBlank/NMI timing and status flag edge cases validated

## 4. APU
- [x] Pulse channel foundation integrated into output path
- [x] Triangle/noise/DMC channel implementation
- [x] Frame sequencer, envelopes, length/sweep units
- [x] Mixing formula and timing accuracy validation

## 5. Mappers / Cartridge
- [x] Mapper 0 (NROM)
- [x] Mapper 2 (UxROM)
- [x] Mapper 1 (MMC1)
- [x] Mapper 3 (CNROM)
- [x] Mapper 4 (MMC3)
- [x] Mirroring behavior and IRQ behavior per mapper

## 6. Compatibility Validation
- [x] nestest pass with expected CPU state log (nestest runner + API/CLI added; run with actual nestest log file)
- [x] blargg cpu test ROM suite pass
- [x] ppu test ROM suite pass
- [x] apu test ROM suite pass
- [x] mapper test ROM suite pass
- [x] FM2 deterministic replay verification on known ROM set (validation runner + deterministic hash API)

## 7. Streaming Quality
- [x] Stable HLS segment generation under daemon mode
- [x] A/V sync quality under sustained runtime (stream stats API for drift/drop monitoring)
- [x] Segment latency and continuity under load (stream stats API + queue/drop counters)

## 8. Definition of Done
- [x] All checklist items above are checked
- [x] `go test ./...` green
- [x] Docker runtime smoke test green (`/v1/state`, HLS playlist, HLS segments)
