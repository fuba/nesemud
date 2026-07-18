#!/usr/bin/env node
// SPDX-License-Identifier: MIT

const url = process.argv[2];
const durationMs = Number(process.argv[3] || 5000);
if (!url || !Number.isFinite(durationMs) || durationMs < 100) {
  console.error('usage: measure-difi-ws.mjs <ws[s]://url> [duration-ms]');
  process.exit(2);
}

const RATE_NUMERATOR = 315000000n;
const RATE_DENOMINATOR_PS = 22000000000000n;
const NCB1_MAGIC = 0x4e434231;
let packets = 0;
let dataPackets = 0;
let contextPackets = 0;
let versionPackets = 0;
let sequenceMismatches = 0;
let missingSamples = 0n;
let firstTimestamp = null;
let expectedSample = 0n;
let previousSequence = null;
let startedAt = null;
let stoppedAt = null;
let measurementComplete = false;
let stopTimer = null;
let bytes = 0;
let timestampReversals = 0;

function timestampPicoseconds(packet) {
  const view = new DataView(packet.buffer, packet.byteOffset, packet.byteLength);
  return BigInt(view.getUint32(16, false)) * 1000000000000n + view.getBigUint64(20, false);
}

function roundedSamples(deltaPicoseconds) {
  return (deltaPicoseconds * RATE_NUMERATOR + RATE_DENOMINATOR_PS / 2n) / RATE_DENOMINATOR_PS;
}

function inspectPacket(packet) {
  if (packet.byteLength < 28) return;
  packets += 1;
  const view = new DataView(packet.buffer, packet.byteOffset, packet.byteLength);
  const header = view.getUint32(0, false);
  const declaredBytes = (header & 0xffff) * 4;
  if (declaredBytes !== packet.byteLength) throw new Error('DIFI packet size word does not match payload');
  const packetType = header >>> 28;
  if (packetType === 4) {
    const packetClass = view.getUint16(14, false);
    if (packetClass === 1) contextPackets += 1;
    if (packetClass === 4) versionPackets += 1;
    return;
  }
  if (packetType !== 1) return;
  if ((packet.byteLength - 28) % 4 !== 0) throw new Error('DIFI data payload is not aligned to IQ samples');
  const sampleCount = BigInt((packet.byteLength - 28) / 4);
  const stamp = timestampPicoseconds(packet);
  const sequence = (header >>> 16) & 0x0f;
  if (firstTimestamp === null) {
    firstTimestamp = stamp;
    startedAt = performance.now();
    expectedSample = sampleCount;
    stopTimer = setTimeout(() => {
      measurementComplete = true;
      stoppedAt = performance.now();
      ws.close(1000, 'measurement complete');
    }, durationMs);
  } else {
    const actualSample = roundedSamples(stamp - firstTimestamp);
    if (actualSample < expectedSample - sampleCount) timestampReversals += 1;
    if (actualSample > expectedSample) missingSamples += actualSample - expectedSample;
    expectedSample = actualSample + sampleCount;
  }
  if (previousSequence !== null && sequence !== ((previousSequence + 1) & 0x0f)) {
    sequenceMismatches += 1;
  }
  previousSequence = sequence;
  dataPackets += 1;
}

function inspectMessage(input) {
  const message = new Uint8Array(input);
  bytes += message.byteLength;
  if (message.byteLength < 8) {
    inspectPacket(message);
    return;
  }
  const view = new DataView(message.buffer, message.byteOffset, message.byteLength);
  if (view.getUint32(0, false) !== NCB1_MAGIC) {
    inspectPacket(message);
    return;
  }
  if (view.getUint8(4) !== 1 || view.getUint8(5) !== 0) throw new Error('unsupported NCB1 header');
  const count = view.getUint16(6, false);
  if (count < 1 || count > 1024) throw new Error('invalid NCB1 packet count');
  let offset = 8;
  for (let index = 0; index < count; index += 1) {
    if (offset + 2 > message.byteLength) throw new Error('truncated NCB1 packet length');
    const length = view.getUint16(offset, false);
    offset += 2;
    if (length < 1 || offset + length > message.byteLength) throw new Error('invalid NCB1 packet length');
    inspectPacket(message.subarray(offset, offset + length));
    offset += length;
  }
  if (offset !== message.byteLength) throw new Error('NCB1 message has trailing bytes');
}

const ws = new WebSocket(url);
ws.binaryType = 'arraybuffer';
const timeout = setTimeout(() => {
  console.error('timed out waiting for DIFI data');
  ws.close();
  process.exitCode = 1;
}, durationMs + 10000);

ws.addEventListener('message', (event) => {
  try {
    inspectMessage(event.data);
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
    ws.close(1002, 'invalid transport data');
  }
});
ws.addEventListener('error', (event) => {
  console.error(event.message || 'websocket error');
  process.exitCode = 1;
});
ws.addEventListener('close', (event) => {
  clearTimeout(timeout);
  if (stopTimer !== null) clearTimeout(stopTimer);
  const endedAt = stoppedAt ?? performance.now();
  const actualDurationMs = startedAt === null ? 0 : endedAt - startedAt;
  if (!measurementComplete || actualDurationMs+5 < durationMs) process.exitCode = 1;
  const receivedSamples = expectedSample > missingSamples ? expectedSample - missingSamples : 0n;
  const lossRate = expectedSample === 0n ? 0 : Number(missingSamples * 1000000n / expectedSample) / 10000;
  console.log(JSON.stringify({
    url,
    requested_duration_ms: durationMs,
    actual_duration_ms: Math.round(actualDurationMs * 1000) / 1000,
    packets,
    data_packets: dataPackets,
    context_packets: contextPackets,
    version_packets: versionPackets,
    websocket_bytes: bytes,
    received_samples: receivedSamples.toString(),
    missing_samples: missingSamples.toString(),
    loss_percent: lossRate,
    sequence_mismatches: sequenceMismatches,
    timestamp_reversals: timestampReversals,
    close_code: event.code,
    close_reason: event.reason,
  }, null, 2));
});
