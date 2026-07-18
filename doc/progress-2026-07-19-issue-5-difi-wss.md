# Issue #5 DIFI WSS closeout (2026-07-19)

## Scope

Issue #5 reported silent DIFI sample loss on the public
`wss://nesemud.home.fuba.dev/udp` path. The RF carrier and raster generator were
left unchanged because the local UDP/WebSocket measurements did not implicate
them.

## Implementation

- Added `GET /v1/rf/stats` with frame, DIFI packet, UDP byte/drop, pending
  sample-loss, and WebSocket relay counters.
- A transient UDP `EAGAIN`, `EWOULDBLOCK`, or `ENOBUFS` now remains pending
  until a DIFI Signal Context carrying `sample_loss_indicator` is delivered.
- Added a direct NCB1 WebSocket relay at `/udp` inside `nesd`, eliminating the
  UDP-to-Node copy and its silent backpressure bundle skipping.
- Slow WebSocket clients are disconnected and counted instead of remaining
  connected across an invisible timestamp discontinuity.
- Signal and Version Context packets are cached for newly connected receivers.
- Partial bundles are discarded when the final client leaves, preventing a
  stale timestamp jump on the next connection.
- Browser Origin validation permits the same origin, local development, and
  `https://crt-emulator2.fuba.me`.
- RF stop and fatal sender termination close all hijacked WebSocket connections;
  inbound application data is rejected with a 125-byte frame ceiling.
- The relay is limited to two clients and 32 queued bundles per client; full
  clients are rejected or disconnected instead of accumulating unbounded data.
- UDP-only loss indicators are removed from the otherwise continuous direct
  WebSocket copy while remaining set on the affected UDP Context packet.

## Production routing

`nesemud-edge` now proxies `/udp` to `nesd` on `127.0.0.1:18080`. The existing
Node UDP bridge remains available for crt-emulator development but is no longer
part of the public nesemud WSS path.

## Verification evidence

Both measurements used the same deployed sender and five seconds of DIFI data:

| Path | Data packets | Received samples | Missing samples | Loss | Sequence mismatches |
|---|---:|---:|---:|---:|---:|
| `ws://127.0.0.1:18080/udp` | 201,240 | 71,641,440 | 0 | 0% | 0 |
| `wss://nesemud.home.fuba.dev/udp` | 201,420 | 71,705,520 | 0 | 0% | 0 |

Both planned closes completed with WebSocket status 1000. The observed wall
durations were 5000.944 ms locally and 4999.426 ms through public TLS.

The sender statistics immediately after both measurements reported:

- `transport_drops`: 0
- `sample_loss_pending`: false
- `websocket_disconnects`: 0

Commands used:

```bash
go test ./...
go test -race ./internal/rfstream ./internal/api ./internal/daemon
node scripts/measure-difi-ws.mjs ws://127.0.0.1:18080/udp 5000
node scripts/measure-difi-ws.mjs wss://nesemud.home.fuba.dev/udp 5000
```
