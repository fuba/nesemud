package streaming

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FrameSource interface {
	SnapshotFrame() []byte
	SnapshotAudio() []int16
}

type HLSStreamer struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	videoW   io.WriteCloser
	audioW   io.WriteCloser
	playlist string
	outDir   string
	frameCh  chan packet
	videoCh  chan []byte
	audioCh  chan []byte
	doneCh   chan struct{}
	stats    Stats
}

func NewHLSStreamer() *HLSStreamer {
	return &HLSStreamer{}
}

type Stats struct {
	StartedAtUnix int64 `json:"started_at_unix"`
	LastWriteUnix int64 `json:"last_write_unix"`
	WrittenFrames int64 `json:"written_frames"`
	DroppedFrames int64 `json:"dropped_frames"`
	QueueDepth    int   `json:"queue_depth"`
	QueueCapacity int   `json:"queue_capacity"`
	Running       bool  `json:"running"`
}

type packet struct {
	frame   []byte
	samples []int16
}

func (s *HLSStreamer) Start(ctx context.Context, outDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil {
		return nil
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := clearHLSOutputDir(outDir); err != nil {
		return err
	}

	videoR, videoW, err := os.Pipe()
	if err != nil {
		return err
	}
	audioR, audioW, err := os.Pipe()
	if err != nil {
		_ = videoR.Close()
		_ = videoW.Close()
		return err
	}

	playlist := filepath.Join(outDir, "index.m3u8")
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-s", "256x240",
		"-r", "60",
		"-i", "pipe:3",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-i", "pipe:4",
		"-vf", "scale=1024:960:flags=neighbor",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "animation",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		"-g", "60",
		"-keyint_min", "60",
		"-c:a", "aac",
		"-b:a", "128k",
		"-f", "hls",
		"-hls_time", "1",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+independent_segments",
		playlist,
	)
	cmd.ExtraFiles = []*os.File{videoR, audioR}
	if err := cmd.Start(); err != nil {
		_ = videoR.Close()
		_ = videoW.Close()
		_ = audioR.Close()
		_ = audioW.Close()
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}
	_ = videoR.Close()
	_ = audioR.Close()

	s.cmd = cmd
	s.videoW = videoW
	s.audioW = audioW
	s.playlist = playlist
	s.outDir = outDir
	s.frameCh = make(chan packet, 8)
	s.videoCh = make(chan []byte, 8)
	s.audioCh = make(chan []byte, 8)
	s.doneCh = make(chan struct{})
	s.stats = Stats{
		StartedAtUnix: time.Now().Unix(),
		QueueCapacity: cap(s.frameCh),
		Running:       true,
	}
	go s.pump(ctx)
	return nil
}

func (s *HLSStreamer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return nil
	}
	close(s.frameCh)
	<-s.doneCh
	_ = s.videoW.Close()
	_ = s.audioW.Close()
	err := s.cmd.Wait()
	s.cmd = nil
	s.videoW = nil
	s.audioW = nil
	s.frameCh = nil
	s.videoCh = nil
	s.audioCh = nil
	s.doneCh = nil
	s.stats.Running = false
	s.stats.QueueDepth = 0
	if err != nil {
		return err
	}
	return nil
}

func (s *HLSStreamer) WriteFrame(frame []byte, samples []int16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.frameCh == nil {
		return errors.New("hls streamer not running")
	}
	pkt := packet{
		frame:   append([]byte(nil), frame...),
		samples: append([]int16(nil), samples...),
	}
	// Hold the lock during send so Stop cannot close the channel concurrently.
	select {
	case s.frameCh <- pkt:
		s.stats.QueueDepth = len(s.frameCh)
		return nil
	default:
		s.stats.DroppedFrames++
		s.stats.QueueDepth = len(s.frameCh)
		return nil
	}
}

func (s *HLSStreamer) PlaylistPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.playlist
}

func (s *HLSStreamer) OutputDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.outDir
}

func (s *HLSStreamer) pump(ctx context.Context) {
	defer close(s.doneCh)
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		s.writeLoop(ctx, s.videoW, s.videoCh)
	}()
	go func() {
		defer writers.Done()
		s.writeLoop(ctx, s.audioW, s.audioCh)
	}()
	defer func() {
		close(s.videoCh)
		close(s.audioCh)
		writers.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-s.frameCh:
			if !ok {
				return
			}
			audioBuf := make([]byte, len(pkt.samples)*2)
			for i, smp := range pkt.samples {
				binary.LittleEndian.PutUint16(audioBuf[i*2:], uint16(smp))
			}
			var dropped int64
			dropped += enqueueLatest(s.audioCh, audioBuf)
			dropped += enqueueLatest(s.videoCh, pkt.frame)
			s.mu.Lock()
			s.stats.WrittenFrames++
			s.stats.DroppedFrames += dropped
			s.stats.LastWriteUnix = time.Now().Unix()
			if s.frameCh != nil {
				s.stats.QueueDepth = len(s.frameCh)
			}
			s.mu.Unlock()
		}
	}
}

func (s *HLSStreamer) writeLoop(ctx context.Context, w io.Writer, ch <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case buf, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(buf); err != nil {
				return
			}
		}
	}
}

func enqueueLatest(ch chan []byte, data []byte) int64 {
	select {
	case ch <- data:
		return 0
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- data:
		return 1
	default:
		return 1
	}
}

func clearHLSOutputDir(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".m3u8") {
			continue
		}
		if err := os.Remove(filepath.Join(outDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *HLSStreamer) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}
