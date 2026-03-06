# Progress: Compatibility Step 3 (2026-03-07)

## Added in this step
- APU foundation added (pulse1 focused):
  - CPU register writes for `0x4000-0x4017` are now accepted.
  - APU status read via `0x4015` implemented.
  - Frame audio generation switched from pure silence to APU-generated samples.
- Console state API now includes APU status.

## Tests Added
- `TestAPUPulseGeneratesAudio`
- `TestAPUDisablePulseSilencesAudio`

## Current gap to final completion
- APU still needs triangle/noise/DMC and frame-sequencer accuracy.
- PPU rendering pipeline still needs full background/sprite logic.
- Mapper coverage and exact timing still need more implementation.
