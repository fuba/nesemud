# Progress Update (2026-03-14): Cartridge Header Compatibility

## Summary
- Added iNES loader compatibility for four-screen mirroring.
- Added generic PRG-RAM mapping for supported mappers and battery-backed cartridges.
- Added trainer loading into CPU `$7000-$71FF`.
- Added compatibility handling for legacy dirty iNES headers that incorrectly populate bytes 12-15.

## Validation
- Added focused tests for:
  - four-screen nametable separation
  - PRG-RAM reads/writes on MMC1
  - trainer mapping in the CPU address space
  - legacy dirty-header mapper decoding
- Verified with `go test ./...`.

## Remaining Gaps
- Mapper IRQ behavior still needs broader real-ROM validation.
- MMC5, split-scroll PPU behavior, and APU timing still require more conformance evidence.
