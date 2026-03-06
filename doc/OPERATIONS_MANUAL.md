# nesemud Operations Manual

## 1. Overview
`nesemud` runs a NES emulator as a daemon, controls it through HTTP APIs, and streams video/audio through HLS.

Design assumptions:
- All runtime control is API-driven
- Logs are file-based
- In Docker mode, FFmpeg runs inside the container (no host FFmpeg dependency)

## 2. Architecture
- Daemon entrypoint: `cmd/nesd`
- Emulator core: `internal/nes`
- API layer: `internal/api`
- HLS streamer: `internal/streaming`
- Validation tools: `cmd/nes-validate`, `internal/validation`

Default port:
- `:18080`

Default endpoints:
- API: `http://127.0.0.1:18080/v1/...`
- HLS: `http://127.0.0.1:18080/hls/index.m3u8`

## 3. Requirements
Local runtime:
- Go 1.24+
- `ffmpeg` available on `PATH`

Docker runtime:
- Docker / Docker Compose
- Linux host with support for `network_mode: host`

## 4. Start and Stop
### 4.1 Docker (recommended)
Start:
```bash
docker compose up -d --build
```

Stop:
```bash
docker compose down
```

Basic checks:
```bash
curl -sS http://127.0.0.1:18080/v1/state
curl -sS http://127.0.0.1:18080/v1/stream/stats
ls -la runtime/hls
```

### 4.2 Local direct run
Foreground:
```bash
go run ./cmd/nesd serve --config ./config.example.json
```

Detached-style command:
```bash
go run ./cmd/nesd daemon --config ./config.example.json
```

## 5. Configuration
Config JSON example:
```json
{
  "listen_addr": ":18080",
  "log_file": "./nesd.log",
  "hls_dir": "./hls"
}
```

Fields:
- `listen_addr`: API/HLS bind address
- `log_file`: daemon log file path
- `hls_dir`: HLS output directory

Hot reload:
- Send `SIGHUP` to reload configuration (`listen_addr` is not re-bound dynamically)

## 6. API Reference
OpenAPI (minimal spec):
```bash
curl -sS http://127.0.0.1:18080/v1/openapi.json
```

### 6.1 Runtime state
- `GET /v1/state`
- Returns runtime status and core CPU/PPU/APU fields, including `rom_loaded` and `replay_active`

### 6.2 ROM loading
- `POST /v1/rom/load`
- Body:
```json
{"path":"/absolute/or/container/path/game.nes"}
```

### 6.3 Control
- `POST /v1/control/reset`
- `POST /v1/control/pause`
- `POST /v1/control/resume`

### 6.4 FM2 replay
- `POST /v1/replay/fm2`
- Provide either `path` or `content`
- Injection immediately resets internal state and starts replay from frame 0

Path example:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/replay/fm2 \
  -H 'Content-Type: application/json' \
  -d '{"path":"/tests/fm2/sample.fm2"}'
```

Inline content example:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/replay/fm2 \
  -H 'Content-Type: application/json' \
  -d '{"content":"version 3\n|0|........|........|\n"}'
```

### 6.5 Memory read/write
- `GET /v1/memory/{addr}?len=N`
- `PUT /v1/memory/{addr}`

`PUT` body accepts either `value` or `bytes`:
```json
{"value":171}
```
```json
{"bytes":[1,2,3,4]}
```

Notes:
- `addr` supports hex like `0x10`
- Read/write length is limited to 1..4096 bytes

### 6.6 Controller input
- `PUT /v1/input/player/1`
- `PUT /v1/input/player/2`

Body format:
```json
{
  "a": false,
  "b": false,
  "select": false,
  "start": false,
  "up": false,
  "down": false,
  "left": false,
  "right": false
}
```

### 6.7 RetroArch-compatible command endpoint
- `POST /v1/retroarch/command`
- `GET /v1/retroarch/command/list`

Supported commands:
- `RESET`
- `SOFT_RESET`
- `PAUSE`
- `RESUME`
- `UNPAUSE`

Accepted command input formats:
- Query string: `?cmd=RESET`
- JSON body: `{"command":"RESET"}`
- Plain text body: `RESET`

### 6.8 Validation APIs
- `POST /v1/validate/replay`
- `POST /v1/validate/nestest`
- `POST /v1/validate/suite`

Example:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/validate/suite \
  -H 'Content-Type: application/json' \
  -d '{"suite":"ppu","rom_dir":"/tests/roms","frames":3000}'
```

### 6.9 Stream statistics
- `GET /v1/stream/stats`

Key fields:
- `running`: streamer status
- `written_frames`: frames successfully enqueued/written toward FFmpeg pipeline
- `dropped_frames`: frames dropped under queue pressure
- `queue_depth`, `queue_capacity`: internal queue status

## 7. HLS Streaming
Playlist URL:
- `http://127.0.0.1:18080/hls/index.m3u8`

Current streaming profile:
- Video: 256x240 raw RGB24 input, H.264 output
- Audio: PCM S16LE stereo 48kHz input, AAC output
- HLS: 1-second segments, list size 5

Validation:
```bash
curl -sS http://127.0.0.1:18080/v1/stream/stats
head -n 20 runtime/hls/index.m3u8
```

## 8. Operational Flow
1. Start daemon
2. Load ROM with `POST /v1/rom/load`
3. Optionally inject replay with `POST /v1/replay/fm2`
4. Verify runtime state with `GET /v1/state`
5. Play stream from `/hls/index.m3u8`

## 9. Troubleshooting
### 9.1 HLS not generated
Check:
```bash
curl -sS http://127.0.0.1:18080/v1/stream/stats
tail -n 200 runtime/nesd.log
ls -la runtime/hls
```

Look for:
- `written_frames` increasing
- `runtime/hls/index.m3u8` and `.ts` files present
- FFmpeg running inside container

### 9.2 No audio
Check:
- Whether segments include an audio stream
```bash
docker compose exec -T nesd ffprobe -v error -show_streams /data/hls/index0.ts
```
- Whether audio level is non-silent (`mean_volume` not `-inf`)
```bash
docker compose exec -T nesd sh -lc 'f=$(ls -1 /data/hls/*.ts | tail -n 1); ffmpeg -hide_banner -i "$f" -vn -af volumedetect -f null - 2>&1 | tail -n 20'
```

### 9.3 API not responding
Check:
- Daemon process is running
- Port `18080` is not in conflict
- `GET /v1/state` returns successfully

## 10. Validation and Tests
Unit tests:
```bash
go test ./...
```

Compatibility suites via CLI:
```bash
go run ./cmd/nes-validate --suite nestest --rom-dir ./tests/roms --frames 2000
go run ./cmd/nes-validate --suite blargg-cpu --rom-dir ./tests/roms --frames 3000
go run ./cmd/nes-validate --suite ppu --rom-dir ./tests/roms --frames 3000
go run ./cmd/nes-validate --suite apu --rom-dir ./tests/roms --frames 3000
go run ./cmd/nes-validate --suite mapper --rom-dir ./tests/roms --frames 3000
```

Batch run:
```bash
./scripts/run-compat-suites.sh ./tests/roms 3000
```

## 11. Current Constraints
- "Perfect compatibility" is still an ongoing target; refer to `doc/master-checklist.md` for scope tracking.
- FM2 support currently focuses on frame input records; advanced FM2 metadata/extensions are not fully covered.
- OpenAPI is intentionally minimal; this manual section (Chapter 6) reflects current implemented API behavior.
