# Progress Update (2026-03-31): IRQ sampling around `CLI/SEI`

## What changed
- Extended instruction-boundary IRQ handling to model both directions of interrupt-mask transition timing.
  - `I:1 -> 0` (`CLI`/`PLP`/`RTI` style unmask): pending IRQ is deferred by one instruction boundary.
  - `I:0 -> 1` (`SEI`/`PLP`/`RTI` style mask): pending IRQ can still be taken on the current boundary using pre-instruction sampling behavior.
- Added a forced IRQ entry path in the CPU core for the pre-instruction-sampling window.

## Tests added
- `TestIRQCanTriggerImmediatelyAfterSEIWhenAlreadyPending`

## Verification
- `go test ./...` passed.
- `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240` re-run completed:
  - Action items remain `1` (`Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes` stays `WARN`).

