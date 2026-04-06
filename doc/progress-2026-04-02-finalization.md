# Progress Update (2026-04-02): Finalization and documentation closeout

## Validation rerun
- Executed full owned-ROM evidence over `./dont_upload_roms`:
  - `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240`
- Output summary:
  - ROM count: `62`
  - Healthy runs: `61`
  - Action items: `1`
- Remaining action item:
  - `Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes` (`mapper 4`, uniform frame output, final PC `0x0029`)

## Hash verification note (Kirby)
- Local ROM hashes:
  - Full CRC32: `63C58D8E`
  - Payload (header-excluded) CRC32: `A3EE28F8`
  - Full MD5: `A741098486E5C4D6769F49046656EEBC`
  - Full SHA1: `F8A34EE9F436E7FCD6080D721D6DBAFFB9F62A30`
- Known reference hash set differs (`CRC32: E4A7D436`), indicating the local image is not byte-identical to that reference.

## Closure decision
- User requested to treat current emulator state as complete.
- The remaining Kirby mismatch is accepted as an out-of-scope variance for this phase.
- Documentation was updated to reflect closure status and current evidence.

