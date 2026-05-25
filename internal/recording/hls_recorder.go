package recording

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Options struct {
	OutputDir     string
	SessionName   string
	SourceHLSURL  string
	SourceInfoURL string
	ClipName      string
	Reason        string
	StartInfo     map[string]any
	EndInfo       map[string]any
}

type Manifest struct {
	OutputDir   string         `json:"output_dir"`
	SessionName string         `json:"session_name"`
	Clips       []ClipMetadata `json:"clips"`
}

type ClipMetadata struct {
	StartedAt       time.Time      `json:"started_at"`
	FinishedAt      time.Time      `json:"finished_at"`
	SessionName     string         `json:"session_name"`
	Status          string         `json:"status"`
	RecordingReason string         `json:"recording_reason"`
	SourceHLSURL    string         `json:"source_hls_url"`
	SourceInfoURL   string         `json:"source_info_url,omitempty"`
	OutputVideo     string         `json:"output_video"`
	StartInfo       map[string]any `json:"start_info,omitempty"`
	EndInfo         map[string]any `json:"end_info,omitempty"`
	FFmpegError     string         `json:"ffmpeg_error,omitempty"`
}

type commandFactory func(context.Context, string, ...string) *exec.Cmd

type HLSRecorder struct {
	options        Options
	commandFactory commandFactory
	stopTimeout    time.Duration
}

func NewHLSRecorder(options Options) *HLSRecorder {
	return &HLSRecorder{
		options:        normalizeOptions(options),
		commandFactory: exec.CommandContext,
		stopTimeout:    3 * time.Second,
	}
}

func processGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func signalProcessGroup(cmd *exec.Cmd, signal syscall.Signal) error {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return err
	}
	return syscall.Kill(-pgid, signal)
}

func (r *HLSRecorder) RecordClip(ctx context.Context) (ClipMetadata, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(r.options.SourceHLSURL) == "" {
		return ClipMetadata{}, errors.New("source HLS URL is required")
	}
	if err := validateHLSURL(r.options.SourceHLSURL); err != nil {
		return ClipMetadata{}, err
	}
	if err := os.MkdirAll(r.options.OutputDir, 0o755); err != nil {
		return ClipMetadata{}, err
	}

	startedAt := time.Now()
	output := filepath.Join(r.options.OutputDir, buildClipFilename(r.options.ClipName, startedAt))
	args := []string{
		"-hide_banner",
		"-nostdin",
		"-loglevel", "error",
		"-rw_timeout", "15000000",
		"-y",
		"-i", r.options.SourceHLSURL,
		"-c", "copy",
		"-movflags", "+faststart",
		output,
	}
	cmd := r.commandFactory(ctx, "ffmpeg", args...)
	if cmd == nil {
		return ClipMetadata{}, errors.New("command factory returned nil")
	}
	cmd.SysProcAttr = processGroupAttr()
	if err := cmd.Start(); err != nil {
		return ClipMetadata{}, fmt.Errorf("start ffmpeg: %w", err)
	}

	err := waitForCommand(ctx, cmd, r.stopTimeout)
	clip := ClipMetadata{
		StartedAt:       startedAt,
		FinishedAt:      time.Now(),
		SessionName:     r.options.SessionName,
		Status:          r.options.Reason,
		RecordingReason: r.options.Reason,
		SourceHLSURL:    r.options.SourceHLSURL,
		SourceInfoURL:   r.options.SourceInfoURL,
		OutputVideo:     output,
		StartInfo:       cloneMap(r.options.StartInfo),
		EndInfo:         cloneMap(r.options.EndInfo),
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		clip.FFmpegError = err.Error()
	}
	if writeErr := writeClipMetadata(r.options.OutputDir, clip); writeErr != nil {
		return clip, writeErr
	}
	if err != nil && clip.FFmpegError != "" {
		return clip, err
	}
	return clip, nil
}

var manifestMu sync.Mutex

func writeClipMetadata(outputDir string, clip ClipMetadata) error {
	sidecar := clip.OutputVideo + ".json"
	if err := writeJSONFile(sidecar, clip); err != nil {
		return err
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()
	manifestPath := filepath.Join(outputDir, "manifest.json")
	manifest := Manifest{
		OutputDir:   outputDir,
		SessionName: clip.SessionName,
	}
	if b, err := os.ReadFile(manifestPath); err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &manifest); err != nil {
			return fmt.Errorf("read manifest: %w", err)
		}
	}
	manifest.OutputDir = outputDir
	if manifest.SessionName == "" {
		manifest.SessionName = clip.SessionName
	}
	manifest.Clips = append(manifest.Clips, clip)
	return writeJSONFile(manifestPath, manifest)
}

func UpdateClipMetadata(clip ClipMetadata) error {
	if strings.TrimSpace(clip.OutputVideo) == "" {
		return errors.New("output video is required")
	}
	outputDir := filepath.Dir(clip.OutputVideo)
	if err := writeJSONFile(clip.OutputVideo+".json", clip); err != nil {
		return err
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()
	manifestPath := filepath.Join(outputDir, "manifest.json")
	var manifest Manifest
	if b, err := os.ReadFile(manifestPath); err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &manifest); err != nil {
			return fmt.Errorf("read manifest: %w", err)
		}
	}
	manifest.OutputDir = outputDir
	if manifest.SessionName == "" {
		manifest.SessionName = clip.SessionName
	}
	replaced := false
	for i := range manifest.Clips {
		if manifest.Clips[i].OutputVideo == clip.OutputVideo {
			manifest.Clips[i] = clip
			replaced = true
			break
		}
	}
	if !replaced {
		manifest.Clips = append(manifest.Clips, clip)
	}
	return writeJSONFile(manifestPath, manifest)
}

func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func waitForCommand(ctx context.Context, cmd *exec.Cmd, stopTimeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		signalProcess(cmd)
		select {
		case err := <-done:
			if err != nil {
				return ctx.Err()
			}
			return nil
		case <-time.After(stopTimeout):
			killProcess(cmd)
			return ctx.Err()
		}
	}
}

func signalProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if signalProcessGroup(cmd, syscall.SIGTERM) == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
}

func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if signalProcessGroup(cmd, syscall.SIGKILL) == nil {
		return
	}
	_ = cmd.Process.Kill()
}

func validateHLSURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid source HLS URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("source HLS URL must use http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("source HLS URL userinfo is not allowed")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("source HLS URL host is required")
	}
	return nil
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.OutputDir) == "" {
		options.OutputDir = "."
	}
	if strings.TrimSpace(options.SessionName) == "" {
		options.SessionName = "nesd-hls-recording"
	}
	if strings.TrimSpace(options.ClipName) == "" {
		options.ClipName = options.SessionName
	}
	if strings.TrimSpace(options.Reason) == "" {
		options.Reason = "manual_stop"
	}
	return options
}

var nonFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func buildClipFilename(name string, t time.Time) string {
	base := strings.Trim(nonFilenameChars.ReplaceAllString(name, "_"), "._-")
	if base == "" {
		base = "clip"
	}
	return fmt.Sprintf("%s_%s.mp4", base, t.Format("20060102_150405"))
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
