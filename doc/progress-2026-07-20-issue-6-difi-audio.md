# Issue #6: DIFI RF audio modulation recovery

## Symptom

The public DIFI WebSocket carried a stable +2.750 MHz aural carrier, but FM
demodulation recovered only about `0.0013` normalized deviation RMS while the
emulator audio RMS was about `0.053`.

## Root cause

After the first RF frame, `rfInputBuffer.fill` waited only 2 ms for the next
emulator frame. NES frames arrive every 16.7 ms. The RF synthesizer therefore
inserted speculative silent frames and advanced its input sequence ahead of the
producer. Later real frames were classified as late and their audio was
discarded. Because producer and consumer then advanced at the same average
rate, the sequence offset never recovered.

The failure was confirmed after a restart with `3,200` real audio samples and
`636,000` padded samples in the RF input buffer.

## Fix

- Remove speculative timeout padding from `rfInputBuffer.fill`.
- Continue padding only explicit sequence gaps created by queue overflow.
- Publish normalized RF input peak/RMS and real/padded sample counters at
  `/v1/rf/stats`.
- Add a regression test that delays a real frame beyond 2 ms and verifies that
  it is preserved instead of replaced with silence.
- Add an FM round-trip test using the same 129-tap Hann extraction and phase
  discriminator as crt-emulator v2.
- Add `scripts/measure-difi-audio-ws.mjs` for validated NCB1/DIFI WebSocket
  measurement.

## Verification

After deployment with automatic gameplay active:

- RF input RMS: `0.05381`
- RF padded samples: `0`
- Public WSS normalized deviation RMS: `0.05166`
- Aural phasor magnitude: `0.11997` (configured carrier scale: `0.12`)
- Public WSS close code: `1000`
- `go test ./...`: pass
- `go test -race ./internal/rfstream`: pass

The recovered RF audio level now tracks the emulator audio level instead of
being approximately 32 dB below it.
