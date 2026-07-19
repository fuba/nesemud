// SPDX-License-Identifier: MIT

package rfstream

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var (
	ErrAlreadyRunning = errors.New("RF streamer already running")
	ErrNotRunning     = errors.New("RF streamer not running")
)

type StreamerStats struct {
	Running              bool    `json:"running"`
	FramesAccepted       uint64  `json:"frames_accepted"`
	FramesDropped        uint64  `json:"frames_dropped"`
	FramesSent           uint64  `json:"frames_sent"`
	DataPackets          uint64  `json:"data_packets"`
	ContextPackets       uint64  `json:"context_packets"`
	VersionPackets       uint64  `json:"version_packets"`
	UDPPacketsSent       uint64  `json:"udp_packets_sent"`
	UDPBytesSent         uint64  `json:"udp_bytes_sent"`
	TransportDrops       uint64  `json:"transport_drops"`
	SampleLossPending    bool    `json:"sample_loss_pending"`
	QueueDepth           int     `json:"queue_depth"`
	WebSocketClients     int     `json:"websocket_clients"`
	WebSocketBundles     uint64  `json:"websocket_bundles"`
	WebSocketPackets     uint64  `json:"websocket_packets"`
	WebSocketBytes       uint64  `json:"websocket_bytes"`
	WebSocketDisconnects uint64  `json:"websocket_disconnects"`
	AudioInputPeak       float64 `json:"audio_input_peak"`
	AudioInputRMS        float64 `json:"audio_input_rms"`
	AudioRealSamples     uint64  `json:"audio_real_samples"`
	AudioPaddedSamples   uint64  `json:"audio_padded_samples"`
	LastError            string  `json:"last_error,omitempty"`
}

type sendOutcome uint8

const (
	sendFatal sendOutcome = iota
	sendDelivered
	sendDropped
)

type inputFrame struct {
	sequence uint64
	rgb      []byte
	audio    []int16
}

type Streamer struct {
	mu sync.Mutex

	conn              *net.UDPConn
	cancel            context.CancelFunc
	inputQueue        chan inputFrame
	done              chan struct{}
	running           bool
	streamID          uint32
	rfCenterHz        int64
	samplesPerPacket  int
	nextInputSequence uint64
	stats             StreamerStats
	writePacket       func([]byte) (int, error)
	relay             *webSocketRelay
}

func (s *Streamer) Start(ctx context.Context, address string, streamID uint32, rfCenterHz int64, samplesPerPacket int) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if samplesPerPacket != 356 && samplesPerPacket != 360 && samplesPerPacket != 1820 {
		return fmt.Errorf("unsupported DIFI samples per packet: %d", samplesPerPacket)
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse DIFI output address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return errors.New("DIFI output host must be an IPv4 literal")
	}
	if samplesPerPacket == 1820 && !ip.IsLoopback() {
		return errors.New("DIFI SPP-1820 output is restricted to loopback transport")
	}
	remote, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return fmt.Errorf("resolve DIFI output address: %w", err)
	}
	conn, err := net.DialUDP("udp4", nil, remote)
	if err != nil {
		return fmt.Errorf("connect DIFI output: %w", err)
	}
	if err := conn.SetWriteBuffer(4 << 20); err != nil {
		_ = conn.Close()
		return fmt.Errorf("set DIFI UDP write buffer: %w", err)
	}
	rawConn, err := conn.SyscallConn()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("access DIFI UDP socket: %w", err)
	}
	writePacket := func(packet []byte) (int, error) {
		var count int
		var writeErr error
		err := rawConn.Write(func(fd uintptr) bool {
			count, writeErr = unix.Write(int(fd), packet)
			return true
		})
		if err != nil {
			return count, err
		}
		return count, writeErr
	}

	s.mu.Lock()
	if s.done != nil {
		s.mu.Unlock()
		_ = conn.Close()
		return ErrAlreadyRunning
	}
	workerCtx, cancel := context.WithCancel(ctx)
	queue := make(chan inputFrame, 64)
	frames := make(chan []IQSample, 2)
	framePool := make(chan []IQSample, 3)
	for range cap(framePool) {
		framePool <- make([]IQSample, SamplesPerFrame)
	}
	done := make(chan struct{})
	s.conn, s.cancel, s.inputQueue, s.done = conn, cancel, queue, done
	s.running, s.streamID, s.rfCenterHz, s.samplesPerPacket = true, streamID, rfCenterHz, samplesPerPacket
	s.nextInputSequence = 0
	s.stats = StreamerStats{Running: true}
	s.writePacket = writePacket
	s.relay = newWebSocketRelay(webSocketClientQueue)
	s.mu.Unlock()

	go s.run(workerCtx, cancel, conn, queue, frames, framePool, done, streamID, rfCenterHz, samplesPerPacket)
	return nil
}

func (s *Streamer) WriteFrame(rgb24 []byte, stereoS16 []int16) error {
	s.mu.Lock()
	if !s.running || s.inputQueue == nil {
		s.mu.Unlock()
		return ErrNotRunning
	}
	s.mu.Unlock()
	if len(rgb24) != RGBWidth*RGBHeight*3 {
		return fmt.Errorf("RGB24 frame has %d bytes, want %d", len(rgb24), RGBWidth*RGBHeight*3)
	}
	if len(stereoS16) != StereoSamplesPerInputFrame {
		return ErrInvalidStereoAudio
	}
	frame := inputFrame{rgb: append([]byte(nil), rgb24...), audio: append([]int16(nil), stereoS16...)}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.inputQueue == nil {
		return ErrNotRunning
	}
	frame.sequence = s.nextInputSequence
	s.nextInputSequence++
	s.stats.FramesAccepted++
	select {
	case s.inputQueue <- frame:
	default:
		select {
		case <-s.inputQueue:
			s.stats.FramesDropped++
		default:
		}
		s.inputQueue <- frame
	}
	s.stats.QueueDepth = len(s.inputQueue)
	return nil
}

func (s *Streamer) Stop() error {
	s.mu.Lock()
	if s.done == nil {
		s.mu.Unlock()
		return nil
	}
	cancel, conn, done, relay := s.cancel, s.conn, s.done, s.relay
	s.running = false
	s.stats.Running = false
	cancel()
	s.mu.Unlock()
	if relay != nil {
		relay.closeAll()
	}
	_ = conn.Close()
	<-done
	return nil
}

func (s *Streamer) Stats() StreamerStats {
	s.mu.Lock()
	stats := s.stats
	relay := s.relay
	s.mu.Unlock()
	if relay != nil {
		ws := relay.stats()
		stats.WebSocketClients = ws.Clients
		stats.WebSocketBundles = ws.Bundles
		stats.WebSocketPackets = ws.Packets
		stats.WebSocketBytes = ws.Bytes
		stats.WebSocketDisconnects = ws.Disconnects
	}
	return stats
}

func (s *Streamer) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.running || s.relay == nil {
		s.mu.Unlock()
		http.Error(w, "RF output unavailable", http.StatusServiceUnavailable)
		return
	}
	relay := s.relay
	s.mu.Unlock()
	relay.serveHTTP(w, r)
}

func (s *Streamer) run(ctx context.Context, cancel context.CancelFunc, conn *net.UDPConn, queue <-chan inputFrame, frames chan []IQSample, framePool chan []IQSample, done chan struct{}, streamID uint32, rfCenterHz int64, samplesPerPacket int) {
	var workers sync.WaitGroup
	workerDone := make(chan struct{}, 2)
	workers.Add(2)
	go func() {
		defer workers.Done()
		defer func() { workerDone <- struct{}{} }()
		s.synthesizeLoop(ctx, queue, frames, framePool)
	}()
	go func() {
		defer workers.Done()
		defer func() { workerDone <- struct{}{} }()
		s.sendLoop(ctx, frames, framePool, streamID, rfCenterHz, samplesPerPacket)
	}()
	<-workerDone
	cancel()
	workers.Wait()
	_ = conn.Close()
	var relay *webSocketRelay
	s.mu.Lock()
	if s.done == done {
		s.running = false
		s.stats.Running = false
		s.stats.QueueDepth = 0
		s.conn, s.cancel, s.inputQueue, s.writePacket = nil, nil, nil, nil
		relay = s.relay
		close(done)
		s.done = nil
	}
	s.mu.Unlock()
	if relay != nil {
		relay.closeAll()
	}
}

func (s *Streamer) synthesizeLoop(ctx context.Context, queue <-chan inputFrame, output chan<- []IQSample, framePool chan []IQSample) {
	defer close(output)
	synth := NewSynthesizer()
	inputs := newRFInputBuffer()
	var outputField uint64
	var outputFrame uint64
	for {
		firstIndex := sourceFieldIndex(outputField)
		secondIndex := sourceFieldIndex(outputField + 1)
		audioCount := audioSamplesForRFFrame(outputFrame)
		if !inputs.fill(ctx, queue, secondIndex, audioCount+2) {
			return
		}
		first := inputs.video[firstIndex]
		second := inputs.video[secondIndex]
		audio := inputs.audioWindow(audioCount + 2)
		audioPeak, audioRMS := audioLevel(audio[:audioCount])
		s.recordAudioInput(audioPeak, audioRMS, inputs.realAudioSamples, inputs.paddedAudioSamples)
		var buffer []IQSample
		select {
		case <-ctx.Done():
			return
		case buffer = <-framePool:
		}
		audioStart := float64((outputFrame*8008)%5) / 5
		iq, err := synth.synthesizeFrameMonoInto(buffer, first, second, audio, audioStart)
		if err != nil {
			recycleIQFrame(ctx, framePool, buffer)
			s.recordError(err)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case output <- iq:
		}
		inputs.consumeAudio(audioCount)
		outputField += 2
		outputFrame++
		inputs.discardVideoBefore(sourceFieldIndex(outputField))
	}
}

type rfInputBuffer struct {
	video              map[uint64][]byte
	audio              []float64
	audioHead          int
	nextSequence       uint64
	lastRGB            []byte
	blackRGB           []byte
	realAudioSamples   uint64
	paddedAudioSamples uint64
}

func newRFInputBuffer() *rfInputBuffer {
	return &rfInputBuffer{
		video:    make(map[uint64][]byte),
		audio:    make([]float64, 0, 6400),
		blackRGB: make([]byte, RGBWidth*RGBHeight*3),
	}
}

func (b *rfInputBuffer) fill(ctx context.Context, queue <-chan inputFrame, videoIndex uint64, audioSamples int) bool {
	for b.nextSequence <= videoIndex || len(b.audio)-b.audioHead < audioSamples {
		frame, ok := receiveInput(ctx, queue)
		if !ok {
			return false
		}
		b.append(frame)
	}
	return true
}

func (b *rfInputBuffer) append(frame inputFrame) {
	if frame.sequence < b.nextSequence {
		b.lastRGB = frame.rgb
		return
	}
	for b.nextSequence < frame.sequence {
		fallback := b.lastRGB
		if fallback == nil {
			fallback = b.blackRGB
		}
		b.video[b.nextSequence] = fallback
		for range StereoSamplesPerInputFrame / 2 {
			b.audio = append(b.audio, 0)
		}
		b.paddedAudioSamples += StereoSamplesPerInputFrame / 2
		b.nextSequence++
	}
	b.video[frame.sequence] = frame.rgb
	for index := 0; index < len(frame.audio); index += 2 {
		mono := float64(int32(frame.audio[index])+int32(frame.audio[index+1])) / (2 * 32768)
		b.audio = append(b.audio, mono)
	}
	b.realAudioSamples += uint64(len(frame.audio) / 2)
	b.lastRGB = frame.rgb
	b.nextSequence = frame.sequence + 1
}

func audioLevel(samples []float64) (float64, float64) {
	var peak, sumSquares float64
	for _, sample := range samples {
		absolute := sample
		if absolute < 0 {
			absolute = -absolute
		}
		if absolute > peak {
			peak = absolute
		}
		sumSquares += sample * sample
	}
	if len(samples) == 0 {
		return 0, 0
	}
	return peak, math.Sqrt(sumSquares / float64(len(samples)))
}

func (s *Streamer) recordAudioInput(peak, rms float64, realSamples, paddedSamples uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.AudioInputPeak = peak
	s.stats.AudioInputRMS = rms
	s.stats.AudioRealSamples = realSamples
	s.stats.AudioPaddedSamples = paddedSamples
}

func (b *rfInputBuffer) audioWindow(samples int) []float64 {
	return b.audio[b.audioHead : b.audioHead+samples]
}

func (b *rfInputBuffer) consumeAudio(samples int) {
	b.audioHead += samples
	if b.audioHead >= 4096 {
		copy(b.audio, b.audio[b.audioHead:])
		b.audio = b.audio[:len(b.audio)-b.audioHead]
		b.audioHead = 0
	}
}

func (b *rfInputBuffer) discardVideoBefore(sequence uint64) {
	for index := range b.video {
		if index < sequence {
			delete(b.video, index)
		}
	}
}

func receiveInput(ctx context.Context, queue <-chan inputFrame) (inputFrame, bool) {
	select {
	case <-ctx.Done():
		return inputFrame{}, false
	case frame := <-queue:
		return frame, true
	}
}

func (s *Streamer) sendLoop(ctx context.Context, frames <-chan []IQSample, framePool chan []IQSample, streamID uint32, rfCenterHz int64, samplesPerPacket int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var dataSequence, contextSequence, versionSequence uint8
	var sampleIndex, nextContext, nextVersion uint64
	var carry []IQSample
	var epoch timestamp
	var wallEpoch time.Time
	started := false
	dataPacketBuffer := make([]byte, DataHeaderBytes+1820*4)

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-frames:
			if !ok {
				return
			}
			if !started {
				wallEpoch = time.Now().Add(5 * time.Millisecond)
				epoch = timestampFromTime(wallEpoch)
				if s.sendContextPacket(s.nextContextPacket(streamID, contextSequence, epoch, rfCenterHz, true)) == sendFatal {
					recycleIQFrame(ctx, framePool, frame)
					return
				}
				contextSequence++
				if s.sendPacket(EncodeVersionPacket(streamID, versionSequence, epoch, wallEpoch, true)) == sendFatal {
					recycleIQFrame(ctx, framePool, frame)
					return
				}
				versionSequence++
				s.addControlPackets(1, 1)
				nextContext = SampleRateNumerator / SampleRateDenominator / 10
				nextVersion = SampleRateNumerator / SampleRateDenominator
				started = true
			}

			combined := frame
			if len(carry) > 0 {
				needed := samplesPerPacket - len(carry)
				packetSamples := make([]IQSample, 0, samplesPerPacket)
				packetSamples = append(packetSamples, carry...)
				packetSamples = append(packetSamples, frame[:needed]...)
				if !s.sendData(ctx, dataPacketBuffer, packetSamples, streamID, &dataSequence, &epoch, &wallEpoch, &sampleIndex, &nextContext, &contextSequence, &nextVersion, &versionSequence, rfCenterHz, samplesPerPacket) {
					recycleIQFrame(ctx, framePool, frame)
					return
				}
				combined = frame[needed:]
				carry = nil
			}
			for len(combined) >= samplesPerPacket {
				if !s.sendData(ctx, dataPacketBuffer, combined[:samplesPerPacket], streamID, &dataSequence, &epoch, &wallEpoch, &sampleIndex, &nextContext, &contextSequence, &nextVersion, &versionSequence, rfCenterHz, samplesPerPacket) {
					recycleIQFrame(ctx, framePool, frame)
					return
				}
				combined = combined[samplesPerPacket:]
			}
			carry = append(carry[:0], combined...)
			s.mu.Lock()
			s.stats.FramesSent++
			s.mu.Unlock()
			recycleIQFrame(ctx, framePool, frame)
		}
	}
}

func recycleIQFrame(ctx context.Context, framePool chan<- []IQSample, frame []IQSample) {
	select {
	case <-ctx.Done():
	case framePool <- frame[:SamplesPerFrame]:
	}
}

func (s *Streamer) sendData(ctx context.Context, packetBuffer []byte, samples []IQSample, streamID uint32, dataSequence *uint8, epoch *timestamp, wallEpoch *time.Time, sampleIndex, nextContext *uint64, contextSequence *uint8, nextVersion *uint64, versionSequence *uint8, rfCenterHz int64, samplesPerPacket int) bool {
	if !paceData(ctx, *wallEpoch, *sampleIndex, samplesPerPacket) {
		return false
	}
	if *sampleIndex >= *nextContext {
		stamp := timestampAtSample(*epoch, *sampleIndex)
		if s.sendContextPacket(s.nextContextPacket(streamID, *contextSequence, stamp, rfCenterHz, false)) == sendFatal {
			return false
		}
		*contextSequence++
		*nextContext += SampleRateNumerator / SampleRateDenominator / 10
		s.addControlPackets(1, 0)
	}
	if *sampleIndex >= *nextVersion {
		stamp := timestampAtSample(*epoch, *sampleIndex)
		if s.sendPacket(EncodeVersionPacket(streamID, *versionSequence, stamp, time.Now(), false)) == sendFatal {
			return false
		}
		*versionSequence++
		*nextVersion += SampleRateNumerator / SampleRateDenominator
		s.addControlPackets(0, 1)
	}
	packet, err := EncodeDataPacketInto(packetBuffer, streamID, *dataSequence, timestampAtSample(*epoch, *sampleIndex), samples)
	if err != nil {
		s.recordError(err)
		return false
	}
	if s.sendPacket(packet) == sendFatal {
		return false
	}
	*dataSequence++
	*sampleIndex += uint64(len(samples))
	s.mu.Lock()
	s.stats.DataPackets++
	s.mu.Unlock()

	return true
}

func paceData(ctx context.Context, wallEpoch time.Time, sampleIndex uint64, samplesPerPacket int) bool {
	batchPackets := 4
	if samplesPerPacket == 1820 {
		batchPackets = 1
	}
	if sampleIndex%uint64(samplesPerPacket*batchPackets) != 0 {
		return true
	}
	deadline := wallEpoch.Add(sampleDuration(sampleIndex))
	wait := time.Until(deadline)
	if wait <= 0 {
		return true
	}
	return waitUntil(ctx, deadline)
}

func waitUntil(ctx context.Context, deadline time.Time) bool {
	for {
		if ctx.Err() != nil {
			return false
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return true
		}
		if remaining > time.Millisecond {
			timer := time.NewTimer(remaining - 500*time.Microsecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
			}
		}
		// Sub-millisecond waits stay on this dedicated sender thread. This avoids
		// scheduler oversleep reducing the RF sample clock under emulator load.
	}
}

func (s *Streamer) sendPacket(packet []byte) sendOutcome {
	s.mu.Lock()
	relay := s.relay
	s.mu.Unlock()
	if relay != nil {
		relay.publishPacket(packet)
	}
	return s.sendUDPPacket(packet)
}

func (s *Streamer) sendUDPPacket(packet []byte) sendOutcome {
	s.mu.Lock()
	writePacket := s.writePacket
	s.mu.Unlock()
	if writePacket == nil {
		s.recordError(ErrNotRunning)
		return sendFatal
	}
	count, err := writePacket(packet)
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.ENOBUFS) {
		s.mu.Lock()
		s.stats.TransportDrops++
		s.stats.SampleLossPending = true
		s.stats.LastError = err.Error()
		s.mu.Unlock()
		return sendDropped
	}
	if err != nil {
		s.recordError(err)
		return sendFatal
	}
	if count != len(packet) {
		s.recordError(fmt.Errorf("short DIFI UDP write: wrote %d of %d bytes", count, len(packet)))
		return sendFatal
	}
	s.mu.Lock()
	s.stats.UDPPacketsSent++
	s.stats.UDPBytesSent += uint64(count)
	s.mu.Unlock()
	return sendDelivered
}

func (s *Streamer) nextContextPacket(streamID uint32, sequence uint8, stamp timestamp, rfCenterHz int64, changed bool) []byte {
	s.mu.Lock()
	sampleLoss := s.stats.SampleLossPending
	s.mu.Unlock()
	return EncodeContextPacket(streamID, sequence, stamp, rfCenterHz, changed, sampleLoss)
}

func (s *Streamer) sendContextPacket(packet []byte) sendOutcome {
	s.mu.Lock()
	relay := s.relay
	s.mu.Unlock()
	if relay != nil {
		webSocketPacket := packet
		if len(packet) >= 100 && binary.BigEndian.Uint32(packet[96:100])&(1<<12) != 0 {
			webSocketPacket = append([]byte(nil), packet...)
			state := binary.BigEndian.Uint32(webSocketPacket[96:100]) &^ (1 << 12)
			binary.BigEndian.PutUint32(webSocketPacket[96:100], state)
		}
		relay.publishPacket(webSocketPacket)
	}
	outcome := s.sendUDPPacket(packet)
	if outcome != sendDelivered || len(packet) < 100 || binary.BigEndian.Uint32(packet[96:100])&(1<<12) == 0 {
		return outcome
	}
	s.mu.Lock()
	s.stats.SampleLossPending = false
	s.mu.Unlock()
	return outcome
}

func (s *Streamer) addControlPackets(contexts, versions uint64) {
	s.mu.Lock()
	s.stats.ContextPackets += contexts
	s.stats.VersionPackets += versions
	s.mu.Unlock()
}

func (s *Streamer) recordError(err error) {
	s.mu.Lock()
	s.stats.LastError = err.Error()
	s.mu.Unlock()
}
