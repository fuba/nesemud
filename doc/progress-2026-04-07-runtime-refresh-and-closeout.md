# Progress Update (2026-04-07): Runtime refresh and closeout

## Runtime replacement (port 18081)
- Rebuilt and restarted `nesd-dev` using current repository `main` with:
  - `docker compose up -d --build nesd`
- Container/image confirmation:
  - old image: `sha256:35675365a14af1cc4e6c56c155f538cd53fd6f1d69bd0072abf7a7a0e520403e`
  - new image: `sha256:a15b071a1a8c81bcd02581a5ed64286b58578f05849d0b506b9e2d7bf5965022`
- Endpoint checks after replacement:
  - `GET /web`: `200 OK` (`X-Nesd-Web-Build: webrtc-stats-v2`)
  - `GET /v1/openapi.json`: `NES Daemon API 0.1.0`

## Target ROM recheck (`edgerace.nes`)
- Source ROM:
  - `https://github.com/fuba/nes-race-danmaku/blob/main/build/edgerace.nes`
- Verification runs:
  - `go run ./cmd/nes-validate --suite owned-evidence --rom-dir /tmp/edgerace_rom --frames 240`
  - `go run ./cmd/nes-validate --suite owned-evidence --rom-dir /tmp/edgerace_rom --frames 2400`
- Result:
  - Action items `0` in both runs.
  - `paused=false`, `non_uniform_observed=true`, audio samples present.

## Repository closeout status
- Merged `fix/streaming-race-and-docker-limits` into `main` and pushed:
  - merge commit: `6a3e760`
- Project documentation updated to reflect runtime refresh and final closure state.
