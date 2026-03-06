# Progress: Compatibility Step 2 (2026-03-07)

## Added in this step
- Mapper support extended from mapper 0 only to mapper 0 + mapper 2 (UxROM).
- Cartridge now owns mapper behavior through read/write methods.
- CPU PRG writes in `0x8000-FFFF` are forwarded to mapper logic.
- PPU CHR read/write now goes through cartridge (preparing mapper-specific CHR behavior).
- PPU VBlank -> CPU NMI path connected.
- CPU now has explicit `NMI` handling routine.

## Tests Added
- `TestMapper2PRGBankSwitch`
- `TestCPUNMIJumpToVector`
- Existing PPU/CPU tests remain green.

## Current hard gaps to full completion
- APU is still not cycle-accurate and not producing authentic channel synthesis.
- PPU rendering is still skeleton-level (no background/sprite pipeline).
- Mapper coverage is still limited (MMC1/MMC3 etc. pending).
- Many corner-case timing details remain.
