# Progress Update (2026-03-14): MMC5 PRG Banking

## Summary
- Implemented MMC5 PRG bank decoding with correct `0=RAM / 1=ROM` handling for `$5114-$5117`.
- Implemented MMC5 PRG-RAM banking at `$6000-$7FFF` via `$5113`.
- Implemented writes into MMC5 PRG-RAM when `$8000-$DFFF` windows are configured as RAM.
- Corrected 16KB MMC5 PRG window behavior in mode 1.

## Validation
- Added focused tests for:
  - mode 1 16KB PRG window decoding
  - banked PRG-RAM at `$6000`
  - PRG-RAM mapped into `$8000`
- Re-ran MMC5 real-ROM smoke coverage for `Just Breed`.

## Remaining Gaps
- MMC5 still lacks broader validation for more advanced features such as vertical split and extended attribute-heavy titles.
