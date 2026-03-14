package streaming

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	webrtcVideoFrameDuration = time.Second / 60
	webrtcAudioPageDuration  = 20 * time.Millisecond
)

type WebRTCStreamer struct {
	mu sync.Mutex

	videoCmd *exec.Cmd
	audioCmd *exec.Cmd

	videoInW  io.WriteCloser
	audioInW  io.WriteCloser
	videoOutR io.ReadCloser
	audioConn net.PacketConn

	videoCh    chan []byte
	audioCh    chan []byte
	videoDone  chan struct{}
	audioDone  chan struct{}
	videoExit  chan error
	audioExit  chan error
	videoTrack *webrtc.TrackLocalStaticSample
	audioTrack *webrtc.TrackLocalStaticRTP
	peers      map[*webrtc.PeerConnection]struct{}

	videoPackets uint64
	audioPackets uint64
	framesIn     uint64
	samplesIn    uint64
	videoExited  bool
	audioExited  bool
	lastError    string
	videoStderr  bytes.Buffer
	audioStderr  bytes.Buffer
}

type WebRTCStats struct {
	Running      bool   `json:"running"`
	VideoPackets uint64 `json:"video_packets"`
	AudioPackets uint64 `json:"audio_packets"`
	PeerCount    int    `json:"peer_count"`
	FramesIn     uint64 `json:"frames_in"`
	SamplesIn    uint64 `json:"samples_in"`
	FFmpegExited bool   `json:"ffmpeg_exited"`
	LastError    string `json:"last_error,omitempty"`
}

func NewWebRTCStreamer() *WebRTCStreamer {
	return &WebRTCStreamer{
		peers: make(map[*webrtc.PeerConnection]struct{}),
	}
}

func (s *WebRTCStreamer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.videoCmd != nil || s.audioCmd != nil {
		return nil
	}

	videoCmd, videoInW, videoOutR, err := startWebRTCVideoEncoder(ctx, &s.videoStderr)
	if err != nil {
		return err
	}
	audioCmd, audioInW, audioConn, err := startWebRTCAudioEncoder(ctx, &s.audioStderr)
	if err != nil {
		_ = videoInW.Close()
		_ = videoOutR.Close()
		_ = videoCmd.Process.Kill()
		_, _ = videoCmd.Process.Wait()
		return err
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000},
		"video", "nesemud",
	)
	if err != nil {
		return err
	}
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio", "nesemud",
	)
	if err != nil {
		return err
	}

	s.videoCmd = videoCmd
	s.audioCmd = audioCmd
	s.videoInW = videoInW
	s.audioInW = audioInW
	s.videoOutR = videoOutR
	s.audioConn = audioConn
	s.videoCh = make(chan []byte, 8)
	s.audioCh = make(chan []byte, 8)
	s.videoDone = make(chan struct{})
	s.audioDone = make(chan struct{})
	s.videoExit = make(chan error, 1)
	s.audioExit = make(chan error, 1)
	s.videoTrack = videoTrack
	s.audioTrack = audioTrack
	s.videoPackets = 0
	s.audioPackets = 0
	s.framesIn = 0
	s.samplesIn = 0
	s.videoExited = false
	s.audioExited = false
	s.lastError = ""
	s.videoStderr.Reset()
	s.audioStderr.Reset()

	go s.writeLoop(ctx, s.videoInW, s.videoCh, s.videoDone, "video pipe write failed")
	go s.writeLoop(ctx, s.audioInW, s.audioCh, s.audioDone, "audio pipe write failed")
	go s.watchEncoderExit(s.videoCmd, s.videoExit, &s.videoExited, &s.videoStderr, "video ffmpeg")
	go s.watchEncoderExit(s.audioCmd, s.audioExit, &s.audioExited, &s.audioStderr, "audio ffmpeg")
	go s.forwardVideo(ctx)
	go s.forwardAudio(ctx)
	return nil
}

func (s *WebRTCStreamer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.videoCmd == nil && s.audioCmd == nil {
		return nil
	}

	close(s.videoCh)
	close(s.audioCh)
	<-s.videoDone
	<-s.audioDone

	for pc := range s.peers {
		_ = pc.Close()
	}
	s.peers = make(map[*webrtc.PeerConnection]struct{})

	_ = s.videoInW.Close()
	_ = s.audioInW.Close()
	_ = s.videoOutR.Close()
	_ = s.audioConn.Close()

	var errs []string
	if s.videoExit != nil {
		if err := <-s.videoExit; err != nil {
			errs = append(errs, err.Error())
		}
	}
	if s.audioExit != nil {
		if err := <-s.audioExit; err != nil {
			errs = append(errs, err.Error())
		}
	}

	s.videoCmd = nil
	s.audioCmd = nil
	s.videoInW = nil
	s.audioInW = nil
	s.videoOutR = nil
	s.audioConn = nil
	s.videoCh = nil
	s.audioCh = nil
	s.videoDone = nil
	s.audioDone = nil
	s.videoExit = nil
	s.audioExit = nil
	s.videoTrack = nil
	s.audioTrack = nil

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

func (s *WebRTCStreamer) WriteFrame(frame []byte, samples []int16) error {
	s.mu.Lock()
	videoCh := s.videoCh
	audioCh := s.audioCh
	s.framesIn++
	s.samplesIn += uint64(len(samples))
	s.mu.Unlock()
	if videoCh == nil || audioCh == nil {
		return errors.New("webrtc streamer not running")
	}

	videoPayload := append([]byte(nil), frame...)
	audioPayload := make([]byte, len(samples)*2)
	for i, smp := range samples {
		binary.LittleEndian.PutUint16(audioPayload[i*2:], uint16(smp))
	}

	enqueueLatestBytes(videoCh, videoPayload)
	enqueueLatestBytes(audioCh, audioPayload)
	return nil
}

func enqueueLatestBytes(ch chan []byte, payload []byte) {
	select {
	case ch <- payload:
	default:
		select {
		case <-ch:
		default:
		}
		ch <- payload
	}
}

func (s *WebRTCStreamer) Answer(ctx context.Context, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	s.mu.Lock()
	videoTrack := s.videoTrack
	audioTrack := s.audioTrack
	s.mu.Unlock()
	if videoTrack == nil || audioTrack == nil {
		return webrtc.SessionDescription{}, errors.New("webrtc streamer not running")
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	videoSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, err
	}
	audioSender, err := pc.AddTrack(audioTrack)
	if err != nil {
		_ = videoSender.Stop()
		_ = pc.Close()
		return webrtc.SessionDescription{}, err
	}
	go drainRTCP(ctx, videoSender)
	go drainRTCP(ctx, audioSender)

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateDisconnected:
			s.mu.Lock()
			delete(s.peers, pc)
			s.mu.Unlock()
			_ = pc.Close()
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, err
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, err
	}
	select {
	case <-ctx.Done():
		_ = pc.Close()
		return webrtc.SessionDescription{}, ctx.Err()
	case <-gatherComplete:
	}

	s.mu.Lock()
	s.peers[pc] = struct{}{}
	s.mu.Unlock()
	return *pc.LocalDescription(), nil
}

func startWebRTCVideoEncoder(ctx context.Context, stderr *bytes.Buffer) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, nil, nil, err
	}
	stderr.Reset()
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
		"-an",
		"-c:v", "libvpx",
		"-deadline", "realtime",
		"-cpu-used", "8",
		"-g", "60",
		"-b:v", "600k",
		"-f", "ivf",
		"pipe:4",
	)
	cmd.ExtraFiles = []*os.File{inR, outW}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = inR.Close()
		_ = inW.Close()
		_ = outR.Close()
		_ = outW.Close()
		return nil, nil, nil, err
	}
	_ = inR.Close()
	_ = outW.Close()
	return cmd, inW, outR, nil
}

func startWebRTCAudioEncoder(ctx context.Context, stderr *bytes.Buffer) (*exec.Cmd, io.WriteCloser, net.PacketConn, error) {
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, nil, nil, err
	}
	audioURL := "rtp://" + conn.LocalAddr().String() + "?pkt_size=1200"
	stderr.Reset()
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-i", "pipe:3",
		"-vn",
		"-c:a", "libopus",
		"-application", "lowdelay",
		"-frame_duration", "20",
		"-b:a", "128k",
		"-f", "rtp",
		"-payload_type", "111",
		audioURL,
	)
	cmd.ExtraFiles = []*os.File{inR}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = inR.Close()
		_ = inW.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}
	_ = inR.Close()
	return cmd, inW, conn, nil
}

func (s *WebRTCStreamer) writeLoop(ctx context.Context, w io.WriteCloser, ch <-chan []byte, done chan<- struct{}, label string) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(payload); err != nil {
				s.recordError(label + ": " + err.Error())
				return
			}
		}
	}
}

func (s *WebRTCStreamer) forwardVideo(ctx context.Context) {
	reader, _, err := ivfreader.NewWith(s.videoOutR)
	if err != nil {
		s.recordError("ivf reader init failed: " + err.Error())
		return
	}
	for {
		payload, _, err := reader.ParseNextFrame()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || ctx.Err() != nil {
				return
			}
			s.recordError("ivf frame parse failed: " + err.Error())
			return
		}
		s.mu.Lock()
		track := s.videoTrack
		s.mu.Unlock()
		if track == nil {
			return
		}
		if err := track.WriteSample(media.Sample{Data: payload, Duration: webrtcVideoFrameDuration}); err != nil {
			s.recordError("video sample write failed: " + err.Error())
			return
		}
		s.mu.Lock()
		s.videoPackets++
		s.mu.Unlock()
	}
}

func (s *WebRTCStreamer) forwardAudio(ctx context.Context) {
	buf := make([]byte, 2048)
	for {
		_ = s.audioConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := s.audioConn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			if ctx.Err() != nil {
				return
			}
			s.recordError("audio rtp read failed: " + err.Error())
			return
		}
		var packet rtp.Packet
		if err := packet.Unmarshal(buf[:n]); err != nil {
			s.recordError("audio rtp parse failed: " + err.Error())
			return
		}
		s.mu.Lock()
		track := s.audioTrack
		s.mu.Unlock()
		if track == nil {
			return
		}
		if err := track.WriteRTP(&packet); err != nil {
			s.recordError("audio rtp write failed: " + err.Error())
			return
		}
		s.mu.Lock()
		s.audioPackets++
		s.mu.Unlock()
	}
}

func (s *WebRTCStreamer) watchEncoderExit(cmd *exec.Cmd, exitCh chan<- error, exited *bool, stderr *bytes.Buffer, label string) {
	err := cmd.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if s.lastError == "" {
			s.lastError = label + " exited: " + err.Error()
		}
	}
	if out := strings.TrimSpace(stderr.String()); out != "" && s.lastError == "" {
		s.lastError = label + ": " + out
	}
	*exited = true
	exitCh <- err
	close(exitCh)
}

func (s *WebRTCStreamer) recordError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastError == "" {
		s.lastError = msg
	}
}

func (s *WebRTCStreamer) Stats() WebRTCStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return WebRTCStats{
		Running:      (s.videoCmd != nil && !s.videoExited) || (s.audioCmd != nil && !s.audioExited),
		VideoPackets: s.videoPackets,
		AudioPackets: s.audioPackets,
		PeerCount:    len(s.peers),
		FramesIn:     s.framesIn,
		SamplesIn:    s.samplesIn,
		FFmpegExited: s.videoExited || s.audioExited,
		LastError:    s.lastError,
	}
}

func drainRTCP(ctx context.Context, sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if _, _, err := sender.Read(buf); err != nil {
				return
			}
		}
	}
}
