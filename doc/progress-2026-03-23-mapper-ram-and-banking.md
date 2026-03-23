# Progress Update (2026-03-23): Mapper RAM/Banks Hardening

## Implemented
- Added MMC3 PRG-RAM protection handling (`$A001` odd write):
  - Bit7 controls PRG-RAM enable.
  - Bit6 controls PRG-RAM write-protect.
- Updated mapper4 `$6000-$7FFF` read/write paths to respect MMC3 PRG-RAM enable/protect state.
- Fixed mapper206 PRG bank index math to apply modulo over actual 8KiB bank count (instead of precedence-affected bitmask behavior).

## Tests Added
- `TestMapper4PRGRAMRequiresEnableBit`
- `TestMapper4PRGRAMWriteProtectBitBlocksWrites`
- `TestMapper206PRGBankSelectionUsesModuloBankCount`

## Validation
- `go test ./...` passed.
- Re-ran owned ROM evidence:
  - `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240`
  - paused/error count: `0`
  - remaining uniform-stuck backlog: `4`
    - `Donald Land (Japan).nes`
    - `Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes`
    - `Racer Mini Yonku - Japan Cup (Japan).nes`
    - `Wario no Mori (Japan).nes`

## Notes
- A mapper4 battery-RAM default inference trial was evaluated but reverted in this phase due Kirby regression (early invalid-opcode trap path). The current branch keeps behavior stable while preserving the new MMC3 PRG-RAM control correctness and mapper206 bank math fix.

## Follow-up (same day): NMI Timing/Clocking Alignment
- Adjusted NMI delivery to be queued with one-instruction latency (`nmiPending` + `nmiDelayInstr`) instead of being injected in the same post-instruction phase that observed vblank.
- Ensured IRQ entry cycles are always propagated into PPU/APU stepping and audio sample scheduling (interrupt entry no longer advances CPU cycles alone).
- Added regression tests:
  - `TestStepInstructionAdvancesSubsystemsDuringNMIEntryCycles`
  - `TestStepInstructionAdvancesSubsystemsDuringIRQEntryCycles`
- Re-ran `owned-evidence` at `--frames 240`:
  - paused/error: `0`
  - uniform-stuck backlog improved from `4` to `3`
    - `Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes`
    - `Racer Mini Yonku - Japan Cup (Japan).nes`
    - `Wario no Mori (Japan).nes`
