# Progress Update (2026-03-24): Owned ROM Backlog Reduction

## Implemented
- Added mapper4 battery-backed PRG-RAM default inference when iNES byte8 is zero.
  - `mapper 4 + battery + byte8=0` now defaults to 8KiB PRG-RAM.
- Added `0xEB` illegal opcode alias support (`USBC/SBC #imm`).
- Added VRC mapper IRQ ACK handling at `$F003`.
  - This clears pending IRQ and reapplies the ACK-latched enable state.
- Added compatibility fallback for KIL/JAM opcodes (`0x02/0x12/.../0xF2`) as inert 1-byte NOPs.

## Tests Added/Updated
- Added:
  - `TestLoadINESMapper4BatteryDefaultsPRGRAMTo8KiB`
  - `TestMapper25IRQAckViaF003ClearsPending`
  - `TestCPUIllegalUSBCImmediateAlias`
- Updated:
  - `TestStateIncludesLastCPUErrorWhenPaused` now uses `0x8B` as the unsupported-opcode probe.

## Validation
- `go test ./...` passed.
- Re-ran owned ROM evidence:
  - `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240 --markdown-out ./doc/owned-rom-evidence-2026-03-24.md --checklist-out ./doc/owned-rom-checklist-2026-03-24.md`
  - Action items reduced to `1` (from previous `3`):
    - remaining: `Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes` (uniform frame output, mapper4)
  - `Racer Mini Yonku - Japan Cup (Japan).nes` and `Wario no Mori (Japan).nes` moved out of backlog.

