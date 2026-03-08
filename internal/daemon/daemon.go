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
	"nesemud/internal/streaming"
)

type Service struct {
	cfgPath string
	cfg     config.Config
	logger  *log.Logger
	core    *nes.Console
	hls     *streaming.HLSStreamer
	server  *http.Server
}

func New(cfgPath string, cfg config.Config, logger *log.Logger) *Service {
	core := nes.NewConsole()
	hls := streaming.NewHLSStreamer()
	apiSrv := api.NewServer(core, hls)
	return &Service{
		cfgPath: cfgPath,
		cfg:     cfg,
		logger:  logger,
		core:    core,
		hls:     hls,
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

	if err := s.hls.Start(ctx, s.cfg.HLSDir); err != nil {
		return err
	}
	defer func() { _ = s.hls.Stop() }()

	go s.frameLoop(ctx)
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

func (s *Service) frameLoop(ctx context.Context) {
	t := time.NewTicker(time.Second / nes.TargetFPS)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.core.StepFrame()
			frame := s.core.SnapshotFrame()
			audio := s.core.SnapshotAudio()
			if err := s.hls.WriteFrame(frame, audio); err != nil {
				s.logger.Printf("hls write error: %v", err)
			}
		}
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
