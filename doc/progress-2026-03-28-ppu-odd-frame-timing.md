# Progress Update (2026-03-28): PPU odd-frame timing

## What changed
- Implemented odd-frame dot skip on pre-render line in the PPU timing loop.
  - On odd frames with rendering enabled, pre-render line now advances with one fewer dot (skip at `scanline=261`, `cycle=340`).
- Refactored frame rollover path into a shared helper to keep normal frame end and odd-frame skip behavior consistent.

## Tests added
- `TestPPUOddFrameSkipAtPreRenderWhenRenderingEnabled`
- `TestPPUOddFrameSkipNotAppliedWhenRenderingDisabled`

## Verification
- `go test ./...` passed.
- `go test ./internal/nes -run TestFushigiBlobbyBottomBandStaysStableAfterStart -count=1 -v` passed.
- `go run ./cmd/nes-validate --suite owned-evidence --rom-dir ./dont_upload_roms --frames 240` re-run completed:
  - Action items remain `1`
  - Remaining backlog: `Hoshi no Kirby - Yume no Izumi no Monogatari (Japan).nes` (uniform frame output, mapper 4)

