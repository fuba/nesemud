package recording

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRecordClipWritesSidecarAndManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper process uses POSIX shell")
	}
	dir := t.TempDir()
	rec := NewHLSRecorder(Options{
		OutputDir:    dir,
		SessionName:  "session one",
		SourceHLSURL: "http://127.0.0.1:18080/hls/index.m3u8",
		ClipName:     "clip one",
		Reason:       "manual_stop",
		StartInfo: map[string]any{
			"loop":          float64(8),
			"policy_engine": "nesd_clone_safety",
		},
		EndInfo: map[string]any{
			"loop":          float64(9),
			"policy_engine": "nesd_clone_mpc",
		},
	})
	rec.commandFactory = helperCommandFactory

	clip, err := rec.RecordClip(context.Background())
	if err != nil {
		t.Fatalf("RecordClip: %v", err)
	}
	if clip.Status != "manual_stop" {
		t.Fatalf("Status = %q, want manual_stop", clip.Status)
	}
	if clip.OutputVideo == "" {
		t.Fatalf("OutputVideo is empty")
	}
	if _, err := os.Stat(clip.OutputVideo); err != nil {
		t.Fatalf("output video missing: %v", err)
	}

	sidecarBytes, err := os.ReadFile(clip.OutputVideo + ".json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var sidecar ClipMetadata
	if err := json.Unmarshal(sidecarBytes, &sidecar); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	if sidecar.SourceHLSURL != rec.options.SourceHLSURL {
		t.Fatalf("SourceHLSURL = %q", sidecar.SourceHLSURL)
	}
	if sidecar.StartInfo["policy_engine"] != "nesd_clone_safety" {
		t.Fatalf("StartInfo policy_engine = %#v", sidecar.StartInfo["policy_engine"])
	}
	if sidecar.EndInfo["policy_engine"] != "nesd_clone_mpc" {
		t.Fatalf("EndInfo policy_engine = %#v", sidecar.EndInfo["policy_engine"])
	}

	manifestBytes, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if len(manifest.Clips) != 1 {
		t.Fatalf("manifest clips = %d, want 1", len(manifest.Clips))
	}
	if manifest.Clips[0].OutputVideo != clip.OutputVideo {
		t.Fatalf("manifest output = %q, want %q", manifest.Clips[0].OutputVideo, clip.OutputVideo)
	}

	clip.EndInfo = map[string]any{"loop": float64(10)}
	if err := UpdateClipMetadata(clip); err != nil {
		t.Fatalf("UpdateClipMetadata: %v", err)
	}
	manifestBytes, err = os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read updated manifest: %v", err)
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal updated manifest: %v", err)
	}
	if len(manifest.Clips) != 1 {
		t.Fatalf("updated manifest clips = %d, want 1", len(manifest.Clips))
	}
	if manifest.Clips[0].EndInfo["loop"] != float64(10) {
		t.Fatalf("updated EndInfo loop = %#v", manifest.Clips[0].EndInfo["loop"])
	}
}

func TestRecordClipReturnsAfterContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper process uses POSIX shell")
	}
	dir := t.TempDir()
	rec := NewHLSRecorder(Options{
		OutputDir:    dir,
		SessionName:  "cancel session",
		SourceHLSURL: "http://127.0.0.1:18080/hls/index.m3u8",
		ClipName:     "cancel clip",
		Reason:       "context_cancelled",
	})
	rec.commandFactory = sleepingHelperCommandFactory
	rec.stopTimeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := rec.RecordClip(ctx); err != nil {
		t.Fatalf("RecordClip after cancel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("manifest missing after cancel: %v", err)
	}
}

func TestRecordClipRejectsUnsupportedURLScheme(t *testing.T) {
	dir := t.TempDir()
	rec := NewHLSRecorder(Options{
		OutputDir:    dir,
		SessionName:  "bad url",
		SourceHLSURL: "file:///etc/passwd",
	})
	rec.commandFactory = helperCommandFactory

	if _, err := rec.RecordClip(context.Background()); err == nil {
		t.Fatalf("RecordClip accepted unsupported URL scheme")
	}
}

func helperCommandFactory(ctx context.Context, _ string, args ...string) *exec.Cmd {
	cmdArgs := []string{"-test.run=TestHLSRecorderHelperProcess", "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "NESD_HLS_RECORDER_HELPER=1")
	return cmd
}

func sleepingHelperCommandFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := helperCommandFactory(ctx, name, args...)
	cmd.Env = append(cmd.Env, "NESD_HLS_RECORDER_SLEEP=1")
	return cmd
}

func TestHLSRecorderHelperProcess(t *testing.T) {
	if os.Getenv("NESD_HLS_RECORDER_HELPER") != "1" {
		return
	}
	args := os.Args
	output := ""
	for i := len(args) - 1; i >= 0; i-- {
		if filepath.Ext(args[i]) == ".mp4" {
			output = args[i]
			break
		}
	}
	if output == "" {
		os.Exit(3)
	}
	if err := os.WriteFile(output, []byte("fake mp4"), 0o644); err != nil {
		os.Exit(4)
	}
	if os.Getenv("NESD_HLS_RECORDER_SLEEP") == "1" {
		time.Sleep(10 * time.Second)
	}
	os.Exit(0)
}
