# nesemud

`nesemud` is a NES daemon implemented from scratch in Go for CLI/Docker-based operation.

Core features:
- Daemon-style runtime
- Full control through HTTP API (ROM/input/reset/memory inspection)
- FM2 replay injection (immediate reset on injection)
- HLS streaming (video + audio)
- HLS recording to MP4 with JSON sidecars and `manifest.json`
- Compatibility validation via CLI/API

## Quick Start
```bash
docker compose up -d --build
curl -sS http://127.0.0.1:18080/v1/state
```

HLS URL:
- Production: `http://127.0.0.1:18080/hls/index.m3u8`
- Development (`ENV=DEVELOPMENT`): `http://127.0.0.1:18081/hls/index.m3u8`

Record the live HLS stream:
```bash
go run ./cmd/nesd-record-hls \
  --hls-url http://127.0.0.1:18080/hls/index.m3u8 \
  --info-url http://127.0.0.1:18080/v1/state \
  --output-dir recordings \
  --session-name manual-run \
  --duration 30s
```

The recorder writes an MP4 clip, a matching `.mp4.json` sidecar, and a cumulative `manifest.json`.

Stop:
```bash
docker compose down
```

## Environment Profiles
- Default profile is `PRODUCTION`
- Local development profile is `DEVELOPMENT`

`docker compose` reads `.env` automatically.

Example files:
- `.env.example` (default):
  - `ENV=PRODUCTION`
  - `NESD_CONTAINER_NAME=nesd`
- `.env` (local dev):
  - `ENV=DEVELOPMENT`
  - `NESD_CONTAINER_NAME=nesd-dev`

Container name is controlled by `NESD_CONTAINER_NAME` in `docker-compose.yml`.

## Manuals
- Full operations manual: [`doc/OPERATIONS_MANUAL.md`](./doc/OPERATIONS_MANUAL.md)
- Test ROM assets: [`doc/rom-test-assets.md`](./doc/rom-test-assets.md)
- Development checklist: [`doc/master-checklist.md`](./doc/master-checklist.md)
- Third-party and licensing notes: [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md)

## License
- Project code: MIT ([`LICENSE`](./LICENSE))
- Third-party software/test ROM/spec handling: [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md)
