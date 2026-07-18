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

## RF Output

RF output can be enabled in the daemon JSON configuration:

```json
{
  "rf_output": {
    "enabled": true,
    "address": "127.0.0.1:23000",
    "allow_remote": false,
    "stream_id": 1314149187,
    "rf_center_hz": 189000000,
    "samples_per_packet": 356
  }
}
```

RF output is exclusively DIFI NTSC-M RF Profile 1.0. NES RGB and 48 kHz PCM are
modulated into one continuous 6 MHz NTSC-M channel: VSB-AM-compatible video at
−1.75 MHz and FM audio at +2.75 MHz. The daemon sends Q1.15 complex I/Q at
315/22 Msps in SPP-356 DIFI UDP datagrams, including Signal Context at 10 Hz
and Version Context at 1 Hz. There is no separate audio transport. Use SPP-356
for an unknown path MTU. SPP-1820 is restricted to same-host loopback
transport; remote destinations must use SPP-356 or SPP-360.

The RF raster is emitted as two sequential 262.5-line fields with half-line
offset equalizing/vertical-sync pulses. A deterministic 60-to-60000/1001 field
cadence drops video only at the defined 1000-field boundary; 48 kHz audio stays
continuous at exactly 8008 samples per five RF frames.

The raw stream is approximately 458 Mbit/s before UDP/IP overhead and has no
transport encryption or authentication. Output is disabled by default and
should stay on a trusted loopback path. A non-loopback IPv4 unicast destination
is accepted only when `allow_remote` is explicitly set to `true`; hostnames,
multicast, broadcast, and unspecified destinations are rejected.

When RF output is enabled, the daemon also exposes the same generated DIFI
packets directly at `ws://<daemon>/udp` as CRT-compatible NCB1 WebSocket
bundles. The production endpoint is `wss://nesemud.home.fuba.dev/udp`. This
direct path avoids the UDP-to-Node relay and never silently skips a bundle: a
client that cannot sustain the RF bitrate is disconnected and counted instead.
Browser connections are accepted from the same origin, local development
origins, and `https://crt-emulator2.fuba.me`.

Sender and relay counters are available from `GET /v1/rf/stats`, including RF
frames, DIFI packet counts, UDP bytes, `transport_drops`, pending sample-loss
notification, active WebSocket clients, and explicit slow-client disconnects.
After a transient UDP `EAGAIN`, `EWOULDBLOCK`, or `ENOBUFS`, the next delivered
DIFI Signal Context sets `sample_loss_indicator`.

Measure timestamp and sequence continuity without the CRT renderer:

```bash
node scripts/measure-difi-ws.mjs wss://nesemud.home.fuba.dev/udp 5000
```

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
