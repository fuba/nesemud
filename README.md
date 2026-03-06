# nesemud

`nesemud` is a NES daemon implemented from scratch in Go for CLI/Docker-based operation.

Core features:
- Daemon-style runtime
- Full control through HTTP API (ROM/input/reset/memory inspection)
- FM2 replay injection (immediate reset on injection)
- HLS streaming (video + audio)
- Compatibility validation via CLI/API

## Quick Start
```bash
docker compose up -d --build
curl -sS http://127.0.0.1:18080/v1/state
```

HLS URL:
- `http://127.0.0.1:18080/hls/index.m3u8`

Stop:
```bash
docker compose down
```

## Manuals
- Full operations manual: [`doc/OPERATIONS_MANUAL.md`](./doc/OPERATIONS_MANUAL.md)
- Test ROM assets: [`doc/rom-test-assets.md`](./doc/rom-test-assets.md)
- Development checklist: [`doc/master-checklist.md`](./doc/master-checklist.md)
- Third-party and licensing notes: [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md)

## License
- Project code: MIT ([`LICENSE`](./LICENSE))
- Third-party software/test ROM/spec handling: [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md)
