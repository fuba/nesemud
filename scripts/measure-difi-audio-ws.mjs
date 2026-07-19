#!/usr/bin/env node
// SPDX-License-Identifier: MIT

const url = process.argv[2];
const durationMs = Number(process.argv[3] || 3000);
if (!url || !Number.isFinite(durationMs) || durationMs < 100) {
  console.error('usage: measure-difi-audio-ws.mjs <ws[s]://url> [duration-ms]');
  process.exit(2);
}

const SAMPLE_RATE = 315_000_000 / 22;
const AUDIO_RATE = 48_000;
const AURAL_CARRIER = 2_750_000;
const AUDIO_DEVIATION = 25_000;
const NCB1_MAGIC = 0x4e434231;
const HISTORY = 1024;
const ringI = new Float64Array(HISTORY);
const ringQ = new Float64Array(HISTORY);
let rfSample = 0;
let nextAudioSample = 2;
let sumSquares = 0;
let peak = 0;
let minimum = Infinity;
let maximum = -Infinity;
let audioSamples = 0;
let phasorMagnitudeSum = 0;
let phasorMagnitudeMinimum = Infinity;
let dataPackets = 0;
let previousSequence = null;
let startedAt = null;
let stopTimer = null;
let measurementComplete = false;

function ringIndex(index) {
  return ((index % HISTORY) + HISTORY) % HISTORY;
}

function phasor(center) {
  let real = 0;
  let imaginary = 0;
  let weightSum = 0;
  for (let tap = -64; tap <= 64; tap += 1) {
    const index = center + tap;
    const slot = ringIndex(index);
    const weight = 0.5 + 0.5 * Math.cos(Math.PI * tap / 64);
    const phase = -2 * Math.PI * AURAL_CARRIER * index / SAMPLE_RATE;
    const cosine = Math.cos(phase);
    const sine = Math.sin(phase);
    real += (ringI[slot] * cosine - ringQ[slot] * sine) * weight;
    imaginary += (ringI[slot] * sine + ringQ[slot] * cosine) * weight;
    weightSum += weight;
  }
  return [real / weightSum, imaginary / weightSum];
}

function phaseDelta(first, second) {
  return Math.atan2(
    first[0] * second[1] - first[1] * second[0],
    first[0] * second[0] + first[1] * second[1],
  );
}

function demodulateAvailable() {
  const halfStep = Math.round(SAMPLE_RATE / (AUDIO_RATE * 2));
  const fullStep = Math.round(SAMPLE_RATE / AUDIO_RATE);
  while (true) {
    const center = Math.round(nextAudioSample * SAMPLE_RATE / AUDIO_RATE);
    if (center + 64 >= rfSample) return;
    const previous = phasor(center - fullStep);
    const middle = phasor(center - halfStep);
    const current = phasor(center);
    const phasorMagnitude = Math.hypot(current[0], current[1]);
    phasorMagnitudeSum += phasorMagnitude;
    phasorMagnitudeMinimum = Math.min(phasorMagnitudeMinimum, phasorMagnitude);
    const value = (phaseDelta(previous, middle) + phaseDelta(middle, current))
      * AUDIO_RATE / (2 * Math.PI * AUDIO_DEVIATION);
    sumSquares += value * value;
    peak = Math.max(peak, Math.abs(value));
    minimum = Math.min(minimum, value);
    maximum = Math.max(maximum, value);
    audioSamples += 1;
    nextAudioSample += 1;
  }
}

function inspectPacket(packet) {
  if (packet.byteLength < 28) return;
  const view = new DataView(packet.buffer, packet.byteOffset, packet.byteLength);
  const header = view.getUint32(0, false);
  if ((header >>> 28) !== 1) return;
  const declaredBytes = (header & 0xffff) * 4;
  if (declaredBytes !== packet.byteLength) throw new Error('DIFI packet size word does not match payload');
  if ((packet.byteLength - 28) % 4 !== 0) throw new Error('DIFI data payload is not aligned to IQ samples');
  const sequence = (header >>> 16) & 0x0f;
  if (previousSequence !== null && sequence !== ((previousSequence + 1) & 0x0f)) {
    throw new Error('DIFI data sequence is discontinuous');
  }
  previousSequence = sequence;
  for (let offset = 28; offset + 3 < packet.byteLength; offset += 4) {
    const slot = ringIndex(rfSample);
    ringI[slot] = view.getInt16(offset, false) / 32768;
    ringQ[slot] = view.getInt16(offset + 2, false) / 32768;
    rfSample += 1;
  }
  dataPackets += 1;
  demodulateAvailable();
  if (startedAt === null) {
    startedAt = performance.now();
    stopTimer = setTimeout(() => {
      measurementComplete = true;
      ws.close(1000, 'measurement complete');
    }, durationMs);
  }
}

function inspectMessage(input) {
  const message = new Uint8Array(input);
  const view = new DataView(message.buffer, message.byteOffset, message.byteLength);
  if (message.byteLength < 8 || view.getUint32(0, false) !== NCB1_MAGIC) {
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
}, durationMs + 15_000);

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
  if (!measurementComplete) process.exitCode = 1;
  console.log(JSON.stringify({
    url,
    data_packets: dataPackets,
    rf_samples: rfSample,
    audio_samples: audioSamples,
    average_aural_phasor_magnitude: audioSamples === 0 ? 0 : phasorMagnitudeSum / audioSamples,
    minimum_aural_phasor_magnitude: audioSamples === 0 ? 0 : phasorMagnitudeMinimum,
    normalized_deviation_rms: audioSamples === 0 ? 0 : Math.sqrt(sumSquares / audioSamples),
    normalized_deviation_peak: peak,
    normalized_deviation_min: audioSamples === 0 ? 0 : minimum,
    normalized_deviation_max: audioSamples === 0 ? 0 : maximum,
    close_code: event.code,
  }, null, 2));
});
