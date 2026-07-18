package daemon

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"nesemud/internal/api"
	"nesemud/internal/config"
	"nesemud/internal/nes"
	"nesemud/internal/rfstream"
	"nesemud/internal/streaming"
)

type Service struct {
	cfgPath string
	cfg     config.Config
	logger  *log.Logger
	core    *nes.Console
	hls     *streaming.HLSStreamer
	webrtc  *streaming.WebRTCStreamer
	rf      *rfstream.Streamer
	server  *http.Server
}

func New(cfgPath string, cfg config.Config, logger *log.Logger) *Service {
	core := nes.NewConsole()
	hls := streaming.NewHLSStreamer()
	webrtc := streaming.NewWebRTCStreamer()
	rf := &rfstream.Streamer{}
	apiSrv := api.NewServer(core, hls, webrtc, rf)
	return &Service{
		cfgPath: cfgPath,
		cfg:     cfg,
		logger:  logger,
		core:    core,
		hls:     hls,
		webrtc:  webrtc,
		rf:      rf,
		server: &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           withHLSStatic(apiSrv.Handler(), cfg.HLSDir),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (s *Service) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	rfConfig := s.cfg.RFOutput

	if err := s.hls.Start(ctx, s.cfg.HLSDir); err != nil {
		return err
	}
	defer func() { _ = s.hls.Stop() }()
	if err := s.webrtc.Start(ctx); err != nil {
		return err
	}
	defer func() { _ = s.webrtc.Stop() }()
	if err := s.startRFOutput(ctx, rfConfig); err != nil {
		return err
	}
	if rfConfig.Enabled {
		defer func() { _ = s.rf.Stop() }()
	}

	go s.frameLoop(ctx, rfConfig.Enabled)
	go s.watchReloadSignals(ctx)

	s.logger.Printf("listening on %s", s.cfg.ListenAddr)
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		s.logger.Printf("shutting down: %v", ctx.Err())
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		_ = s.server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Service) frameLoop(ctx context.Context, rfEnabled bool) {
	clockStart := time.Now()
	frameNumber := uint64(1)
	deadline := frameDeadline(clockStart, frameNumber)
	timer := time.NewTimer(frameWait(deadline, time.Now()))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.core.StepFrame()
			frame := s.core.SnapshotFrame()
			audio := s.core.SnapshotAudio()
			if err := s.hls.WriteFrame(frame, audio); err != nil {
				s.logger.Printf("hls write error: %v", err)
			}
			if err := s.webrtc.WriteFrame(frame, audio); err != nil {
				s.logger.Printf("webrtc write error: %v", err)
			}
			s.writeRFFrame(rfEnabled, frame, audio)
			frameNumber++
			deadline = frameDeadline(clockStart, frameNumber)
			if time.Since(deadline) > 12*time.Second/nes.TargetFPS {
				clockStart = time.Now()
				frameNumber = 1
				deadline = frameDeadline(clockStart, frameNumber)
			}
			timer.Reset(frameWait(deadline, time.Now()))
		}
	}
}

func frameDeadline(start time.Time, frameNumber uint64) time.Time {
	return start.Add(time.Duration(frameNumber) * time.Second / nes.TargetFPS)
}

func frameWait(deadline, now time.Time) time.Duration {
	wait := deadline.Sub(now)
	if wait < 0 {
		return 0
	}
	return wait
}

func limitFrameCatchUp(deadline, now time.Time) time.Time {
	const maxCatchUpFrames = 12
	framePeriod := time.Second / nes.TargetFPS
	if now.Sub(deadline) > maxCatchUpFrames*framePeriod {
		return frameDeadline(now, 1)
	}
	return deadline
}

func (s *Service) startRFOutput(ctx context.Context, cfg config.RFOutputConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if err := s.rf.Start(ctx, cfg.Address, cfg.StreamID, cfg.RFCenterHz, cfg.SamplesPerPacket); err != nil {
		return fmt.Errorf("start RF output: %w", err)
	}
	return nil
}

func (s *Service) writeRFFrame(enabled bool, frame []byte, audio []int16) {
	if !enabled || !s.rf.Stats().Running {
		return
	}
	if err := s.rf.WriteFrame(frame, audio); err != nil {
		s.logger.Printf("rf write error: %v", err)
	}
}

func (s *Service) watchReloadSignals(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	defer signal.Stop(ch)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			s.reload()
		}
	}
}

func (s *Service) reload() {
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		s.logger.Printf("reload failed: %v", err)
		return
	}
	if cfg.RFOutput != s.cfg.RFOutput {
		s.logger.Printf("RF output changes require restart; keeping active RF configuration")
		cfg.RFOutput = s.cfg.RFOutput
	}
	s.cfg = cfg
	s.logger.Printf("reloaded config from %s", s.cfgPath)
}

func withHLSStatic(next http.Handler, hlsDir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/hls/", http.StripPrefix("/hls/", http.FileServer(http.Dir(hlsDir))))
	mux.Handle("/", next)
	return mux
}

func StartDetached(executable string, args []string, logFile string) error {
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := execCommand(executable, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("daemon started pid=%d log=%s\n", cmd.Process.Pid, logFile)
	return nil
}

var execCommand = func(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
