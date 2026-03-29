# Progress Update (2026-03-30): IRQ edge handling and MMC3 IRQ gating

## What changed
- Added IRQ servicing delay after instruction-level interrupt unmask transitions.
  - When an instruction changes `I:1 -> 0` (for example `CLI`), pending IRQ is now deferred for one instruction boundary.
  - Applied consistently in both `StepInstruction` and `StepFrame` flows.
- Refined MMC3 IRQ clock gating in the PPU path.
  - Mapper4 scanline IRQ clocking now requires rendering-active scanlines and an A12-high fetch-capable pattern-table configuration.
  - This avoids unconditional per-scanline MMC3 IRQ clocking when pattern fetches stay below `$1000`.

## Tests added/updated
- Added:
  - `TestIRQDelayedOneInstructionAfterCLI`
  - `TestIRQNotDelayedWhenInterruptsAlreadyEnabled`
  - `TestMapper4IRQNotClockedWhenPatternFetchesStayBelow1000`
- Updated:
  - `TestMapper4IRQClockedWhenRenderingEnabled` now sets `PPUCTRL` to a pattern-table-high configuration explicitly.

## Verification
- `go test ./...` passed.
- `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240` re-run completed.
  - Action items remain `1` (`Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes` still uniform with `PC=0x0029`).

## Kirby trace note
- Direct instruction trace on the local Kirby ROM shows control eventually returns to `PC=0x0000`, executes `BRK` (`0x00`), and then loops through the ROM IRQ/BRK vector at `0x0029`.
- This indicates the remaining issue is still on execution-path correctness before the BRK loop (not a direct unsupported-opcode halt).
