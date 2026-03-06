# Progress: CPU Core Foundation (2026-03-07)

## Implemented
- Added 6502 CPU core skeleton (`internal/nes/cpu.go`) and integrated it into `Console`.
- Added reset vector based startup (`$FFFC/$FFFD`) for cartridge boot flow.
- Implemented core instructions required by bootstrapping and basic game logic:
  - load/store: `LDA/LDX/LDY`, `STA/STX/STY`
  - increment/decrement: `INX/INY/DEX/DEY`
  - arithmetic/flags: `ADC/SBC`, `CLC/SEC`
  - control flow: `JMP/JSR/RTS`, `BNE/BEQ`, `NOP`, `BRK`
  - stack: `PHA/PLA/PHP/PLP`
- Switched frame loop from pure dummy stepping to CPU cycle stepping (`~29780 cycles/frame`).
- Extended state API output with CPU registers and cycle counter.

## Tests Added
- `TestCPUResetVectorAndBasicProgram`
- `TestCPUBranchAndJSRRTS`

## Current Gap to "Perfect Compatibility"
- Full opcode coverage (including illegal opcodes) is not complete.
- Exact cycle penalties/page-crossing behavior is not complete.
- PPU and APU are still placeholder-level and must be replaced.
- Mapper support is still mapper 0 only.
