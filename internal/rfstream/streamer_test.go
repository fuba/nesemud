// SPDX-License-Identifier: MIT

package rfstream

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestStreamerSendsContextBeforeFixedSizeData(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()

	streamer := &Streamer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := streamer.Start(ctx, receiver.LocalAddr().String(), 0x4e545343, DefaultRFCenterHz, SamplesPerPacket); err != nil {
		t.Fatal(err)
	}
	defer streamer.Stop()

	frame := solidFrame(0x40, 0x80, 0xc0)
	audio := sineStereo(440, StereoSamplesPerInputFrame/2)
	if err := streamer.WriteFrame(frame, audio); err != nil {
		t.Fatal(err)
	}
	if err := streamer.WriteFrame(frame, audio); err != nil {
		t.Fatal(err)
	}
	if err := streamer.WriteFrame(frame, audio); err != nil {
		t.Fatal(err)
	}

	buffer := make([]byte, 2048)
	if err := receiver.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	seenContext := false
	for packetIndex := 0; packetIndex < 20; packetIndex++ {
		count, _, readErr := receiver.ReadFromUDP(buffer)
		if readErr != nil {
			t.Fatal(readErr)
		}
		word := binary.BigEndian.Uint32(buffer[:4])
		switch word >> 28 {
		case 4:
			if binary.BigEndian.Uint16(buffer[14:16]) == 1 {
				seenContext = true
			}
		case 1:
			if !seenContext {
				t.Fatal("DIFI data arrived before Signal Context")
			}
			if count != DataHeaderBytes+SamplesPerPacket*4 {
				t.Fatalf("data bytes=%d, want fixed SPP-%d", count, SamplesPerPacket)
			}
			if got := binary.BigEndian.Uint32(buffer[4:8]); got != 0x4e545343 {
				t.Fatalf("stream ID=%#x", got)
			}
			return
		}
	}
	t.Fatal("no DIFI data packet received")
}

func TestStreamerRejectsInvalidInputAndLifecycleMisuse(t *testing.T) {
	streamer := &Streamer{}
	frame := make([]byte, RGBWidth*RGBHeight*3)
	audio := make([]int16, StereoSamplesPerInputFrame)
	if err := streamer.WriteFrame(frame, audio); err != ErrNotRunning {
		t.Fatalf("WriteFrame before Start error=%v", err)
	}
	if err := streamer.WriteFrame(frame[:len(frame)-1], audio); err == nil {
		t.Fatal("short frame accepted")
	}
	if err := streamer.WriteFrame(frame, audio[:len(audio)-1]); err == nil {
		t.Fatal("short audio block accepted")
	}
}

func TestWriteFrameDoesNotAllocateWhenStreamerIsStopped(t *testing.T) {
	streamer := &Streamer{}
	frame := make([]byte, RGBWidth*RGBHeight*3)
	audio := make([]int16, StereoSamplesPerInputFrame)
	allocations := testing.AllocsPerRun(100, func() {
		if err := streamer.WriteFrame(frame, audio); err != ErrNotRunning {
			t.Fatalf("WriteFrame error=%v", err)
		}
	})
	if allocations != 0 {
		t.Fatalf("stopped WriteFrame allocations=%f, want 0", allocations)
	}
}

func TestStreamerRejectsHostnameDestination(t *testing.T) {
	streamer := &Streamer{}
	if err := streamer.Start(context.Background(), "localhost:23000", 1, DefaultRFCenterHz, SamplesPerPacket); err == nil {
		t.Fatal("expected hostname destination to be rejected")
	}
}

func TestStreamerRejectsRemoteJumboDestination(t *testing.T) {
	streamer := &Streamer{}
	if err := streamer.Start(context.Background(), "192.0.2.1:23000", 1, DefaultRFCenterHz, 1820); err == nil {
		t.Fatal("expected remote SPP-1820 destination to be rejected")
	}
}

func TestStreamerValidatesInputWhileRunning(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	streamer := &Streamer{}
	if err := streamer.Start(context.Background(), receiver.LocalAddr().String(), 1, DefaultRFCenterHz, SamplesPerPacket); err != nil {
		t.Fatal(err)
	}
	defer streamer.Stop()
	frame := make([]byte, RGBWidth*RGBHeight*3)
	audio := make([]int16, StereoSamplesPerInputFrame)
	if err := streamer.WriteFrame(frame[:len(frame)-1], audio); err == nil || err == ErrNotRunning {
		t.Fatalf("short frame error=%v", err)
	}
	if err := streamer.WriteFrame(frame, audio[:len(audio)-1]); err != ErrInvalidStereoAudio {
		t.Fatalf("short audio error=%v", err)
	}
}

func TestStreamerStopIsIdempotent(t *testing.T) {
	streamer := &Streamer{}
	if err := streamer.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := streamer.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestWaitUntilHonorsDeadlineAndCancellation(t *testing.T) {
	start := time.Now()
	if !waitUntil(context.Background(), start.Add(2*time.Millisecond)) {
		t.Fatal("deadline wait was cancelled")
	}
	if elapsed := time.Since(start); elapsed < 2*time.Millisecond {
		t.Fatalf("wait returned early after %v", elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitUntil(ctx, time.Now().Add(time.Second)) {
		t.Fatal("cancelled wait reported success")
	}
}

func TestRFInputBufferSequenceGapDoesNotRewindForLateFrames(t *testing.T) {
	buffer := newRFInputBuffer()
	buffer.append(inputFrame{
		sequence: 1,
		rgb:      solidFrame(4, 5, 6),
		audio:    make([]int16, StereoSamplesPerInputFrame),
	})
	if buffer.nextSequence != 2 || len(buffer.audio) != StereoSamplesPerInputFrame {
		t.Fatalf("gap state sequence=%d audio=%d", buffer.nextSequence, len(buffer.audio))
	}
	if buffer.realAudioSamples != StereoSamplesPerInputFrame/2 || buffer.paddedAudioSamples != StereoSamplesPerInputFrame/2 {
		t.Fatalf("padding counters real=%d padded=%d", buffer.realAudioSamples, buffer.paddedAudioSamples)
	}
	late := inputFrame{
		sequence: 0,
		rgb:      solidFrame(1, 2, 3),
		audio:    make([]int16, StereoSamplesPerInputFrame),
	}
	buffer.append(late)
	if buffer.nextSequence != 2 || len(buffer.audio) != StereoSamplesPerInputFrame {
		t.Fatalf("late input rewound timeline: sequence=%d audio=%d", buffer.nextSequence, len(buffer.audio))
	}
	if &buffer.lastRGB[0] != &late.rgb[0] {
		t.Fatal("late input did not refresh the next video field")
	}
}

func TestRFInputBufferReportsNormalizedProgramLevel(t *testing.T) {
	buffer := newRFInputBuffer()
	audio := sineStereo(440, StereoSamplesPerInputFrame/2)
	buffer.append(inputFrame{sequence: 0, rgb: solidFrame(1, 2, 3), audio: audio})

	peak, rms := audioLevel(buffer.audioWindow(len(audio) / 2))
	if peak < 0.48 || peak > 0.50 {
		t.Fatalf("normalized peak=%g, want 0.48..0.50", peak)
	}
	if rms < 0.33 || rms > 0.36 {
		t.Fatalf("normalized RMS=%g, want 0.33..0.36", rms)
	}
	if buffer.realAudioSamples != uint64(len(audio)/2) || buffer.paddedAudioSamples != 0 {
		t.Fatalf("audio counters real=%d padded=%d", buffer.realAudioSamples, buffer.paddedAudioSamples)
	}
}

func TestRFInputBufferWaitsForRealAudioInsteadOfSpeculativePadding(t *testing.T) {
	buffer := newRFInputBuffer()
	queue := make(chan inputFrame, 1)
	audio := sineStereo(440, StereoSamplesPerInputFrame/2)
	go func() {
		time.Sleep(5 * time.Millisecond)
		queue <- inputFrame{sequence: 0, rgb: solidFrame(1, 2, 3), audio: audio}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if !buffer.fill(ctx, queue, 0, len(audio)/2) {
		t.Fatal("buffer fill stopped before receiving input")
	}
	if buffer.realAudioSamples != uint64(len(audio)/2) || buffer.paddedAudioSamples != 0 {
		t.Fatalf("audio counters real=%d padded=%d", buffer.realAudioSamples, buffer.paddedAudioSamples)
	}
	_, rms := audioLevel(buffer.audioWindow(len(audio) / 2))
	if rms < 0.33 {
		t.Fatalf("real audio was replaced by speculative silence: RMS=%g", rms)
	}
}

func TestConcurrentWriteAndStop(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	streamer := &Streamer{}
	if err := streamer.Start(context.Background(), receiver.LocalAddr().String(), 1, DefaultRFCenterHz, SamplesPerPacket); err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, RGBWidth*RGBHeight*3)
	audio := make([]int16, StereoSamplesPerInputFrame)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			err := streamer.WriteFrame(frame, audio)
			if err != nil && err != ErrNotRunning {
				t.Errorf("WriteFrame: %v", err)
				return
			}
		}
	}()
	if err := streamer.Stop(); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestTransientUDPDropMarksSampleLossUntilContextDelivery(t *testing.T) {
	streamer := &Streamer{}
	streamer.writePacket = func([]byte) (int, error) {
		return 0, unix.ENOBUFS
	}
	if outcome := streamer.sendPacket([]byte{1, 2, 3}); outcome != sendDropped {
		t.Fatalf("send outcome=%v, want dropped", outcome)
	}
	stats := streamer.Stats()
	if stats.TransportDrops != 1 || !stats.SampleLossPending {
		t.Fatalf("stats after drop=%+v", stats)
	}

	contextPacket := streamer.nextContextPacket(1, 0, timestamp{}, DefaultRFCenterHz, false)
	if state := binary.BigEndian.Uint32(contextPacket[96:100]); state&(1<<12) == 0 {
		t.Fatalf("context state=%#x, sample-loss indicator is clear", state)
	}
	streamer.writePacket = func(packet []byte) (int, error) {
		return len(packet), nil
	}
	if outcome := streamer.sendContextPacket(contextPacket); outcome != sendDelivered {
		t.Fatalf("context send outcome=%v, want delivered", outcome)
	}
	if stats := streamer.Stats(); stats.SampleLossPending {
		t.Fatalf("sample loss remained pending after context delivery: %+v", stats)
	}
}

func TestFatalUDPWriteDoesNotMasqueradeAsTransportDrop(t *testing.T) {
	streamer := &Streamer{}
	streamer.writePacket = func([]byte) (int, error) {
		return 0, errors.New("permanent write failure")
	}
	if outcome := streamer.sendPacket([]byte{1}); outcome != sendFatal {
		t.Fatalf("send outcome=%v, want fatal", outcome)
	}
	stats := streamer.Stats()
	if stats.TransportDrops != 0 || stats.LastError == "" {
		t.Fatalf("fatal write stats=%+v", stats)
	}
}

func TestStoppedStreamerRejectsWebSocket(t *testing.T) {
	streamer := &Streamer{}
	req := httptest.NewRequest(http.MethodGet, "/udp", nil)
	rec := httptest.NewRecorder()
	streamer.ServeWebSocket(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestUDPSampleLossIndicatorIsNotPublishedToLosslessWebSocket(t *testing.T) {
	streamer := &Streamer{
		relay: newWebSocketRelay(4),
		stats: StreamerStats{SampleLossPending: true},
	}
	client := streamer.relay.addClientForTest()
	var udpPacket []byte
	streamer.writePacket = func(packet []byte) (int, error) {
		udpPacket = append([]byte(nil), packet...)
		return len(packet), nil
	}
	packet := streamer.nextContextPacket(1, 0, timestamp{}, DefaultRFCenterHz, false)
	if outcome := streamer.sendContextPacket(packet); outcome != sendDelivered {
		t.Fatalf("send outcome=%v", outcome)
	}
	streamer.relay.mu.Lock()
	streamer.relay.flushLocked()
	streamer.relay.mu.Unlock()
	bundle := <-client.queue
	packetLength := int(binary.BigEndian.Uint16(bundle[8:10]))
	wsPacket := bundle[10 : 10+packetLength]
	if state := binary.BigEndian.Uint32(udpPacket[96:100]); state&(1<<12) == 0 {
		t.Fatalf("UDP context state=%#x, sample-loss indicator is clear", state)
	}
	if state := binary.BigEndian.Uint32(wsPacket[96:100]); state&(1<<12) != 0 {
		t.Fatalf("WebSocket context state=%#x, UDP-only loss leaked into WSS", state)
	}
}
