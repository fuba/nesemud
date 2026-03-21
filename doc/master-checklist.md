# Master Checklist to Final Completion

This checklist is a high-level roadmap.
For current implementation reality and known gaps, see `doc/implementation-gap-checklist-2026-03-10.md`.

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
- [ ] IRQ/BRK/RTI edge-case timing validation against conformance ROMs
- [ ] Illegal opcode behavior and cycle timing audited against conformance ROMs

## 3. PPU
- [x] PPU register map and VRAM/palette/OAM DMA plumbing
- [x] Background pixel pipeline (name table + attribute + pattern decode) used for frame output
- [ ] Sprite evaluation/rendering fully audited under live mid-frame behavior
- [ ] Scrolling logic and timing-correct register behavior validated against real split/HUD titles
- [ ] VBlank/NMI timing and status flag edge cases validated against conformance ROMs

## 4. APU
- [x] Pulse channel foundation integrated into output path
- [x] Triangle/noise/DMC channel implementation
- [ ] Frame sequencer, envelopes, length/sweep units audited against conformance ROMs
- [ ] Mixing formula and timing accuracy validated against reference ROMs and real games

## 5. Mappers / Cartridge
- [x] Mapper 0 (NROM)
- [x] Mapper 2 (UxROM)
- [x] Mapper 1 (MMC1)
- [x] Mapper 3 (CNROM)
- [x] Mapper 4 (MMC3)
- [x] iNES cartridge header compatibility for four-screen mirroring, trainer loading, and PRG-RAM mapping
- [ ] Mirroring behavior and IRQ behavior per mapper validated under real titles
- [ ] Extended mapper set (`5/23/25/33/66/75/87/88/206`) fully audited against owned ROMs

## 6. Compatibility Validation
- [x] nestest pass with expected CPU state log
- [x] blargg cpu test ROM suite pass
- [x] ppu test ROM suite pass
- [x] apu test ROM suite pass
- [x] mapper test ROM suite pass
- [ ] FM2 deterministic replay verification on known ROM set with captured evidence

## 7. Streaming Quality
- [x] Stable HLS segment generation under daemon mode
- [x] A/V sync quality under sustained runtime (stream stats API for drift/drop monitoring)
- [x] Segment latency and continuity under load (stream stats API + queue/drop counters)
- [x] WebRTC low-latency path for interactive play
- [x] WebRTC/HLS switching in the web UI

## 8. Definition of Done
- [ ] All checklist items above are checked
- [ ] `go test ./...` green
- [ ] Docker runtime smoke test green (`/v1/state`, HLS playlist, HLS segments, WebRTC path)
