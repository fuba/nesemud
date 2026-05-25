package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nesemud/internal/recording"
)

const metadataLimitBytes = 1 << 20

var metadataClient = http.Client{Timeout: 5 * time.Second}

func main() {
	var (
		hlsURL      = flag.String("hls-url", "http://127.0.0.1:18080/hls/index.m3u8", "source HLS playlist URL")
		infoURL     = flag.String("info-url", "http://127.0.0.1:18080/v1/state", "optional JSON metadata URL")
		outputDir   = flag.String("output-dir", "recordings", "directory for MP4 clips and manifest.json")
		sessionName = flag.String("session-name", "nesd-hls-recording", "session name stored in metadata")
		clipName    = flag.String("clip-name", "", "clip filename prefix; defaults to session name")
		reason      = flag.String("reason", "manual_stop", "recording reason stored in metadata")
		duration    = flag.Duration("duration", 0, "recording duration; 0 records until interrupted")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	startInfo, err := fetchJSON(ctx, *infoURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metadata start fetch failed: %v\n", err)
	}
	opts := recording.Options{
		OutputDir:     *outputDir,
		SessionName:   *sessionName,
		SourceHLSURL:  *hlsURL,
		SourceInfoURL: strings.TrimSpace(*infoURL),
		ClipName:      firstNonEmpty(*clipName, *sessionName),
		Reason:        *reason,
		StartInfo:     startInfo,
	}
	rec := recording.NewHLSRecorder(opts)
	clip, err := rec.RecordClip(ctx)
	endInfo, infoErr := fetchJSON(context.Background(), *infoURL)
	if infoErr == nil {
		clip.EndInfo = endInfo
		if writeErr := recording.UpdateClipMetadata(clip); writeErr != nil {
			fmt.Fprintf(os.Stderr, "metadata end update failed: %v\n", writeErr)
		}
	} else if strings.TrimSpace(*infoURL) != "" {
		fmt.Fprintf(os.Stderr, "metadata end fetch failed: %v\n", infoErr)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "recording failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(clip.OutputVideo)
}

func fetchJSON(ctx context.Context, url string) (map[string]any, error) {
	if strings.TrimSpace(url) == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := metadataClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %s", url, res.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(res.Body, metadataLimitBytes)).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
