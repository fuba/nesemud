// SPDX-License-Identifier: MIT

package daemon

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nesemud/internal/config"
	"nesemud/internal/rfstream"
)

func TestFrameClockCatchesUpBriefDelayAndRebasesLargeDelay(t *testing.T) {
	start := time.Unix(100, 0)
	deadline := frameDeadline(start, 1)
	if want := start.Add(time.Second / 60); !deadline.Equal(want) {
		t.Fatalf("next deadline=%v, want %v", deadline, want)
	}
	if wait := frameWait(deadline, deadline.Add(20*time.Millisecond)); wait != 0 {
		t.Fatalf("late frame wait=%v, want immediate catch-up", wait)
	}
	lateNow := deadline.Add(250 * time.Millisecond)
	if got, want := limitFrameCatchUp(deadline, lateNow), frameDeadline(lateNow, 1); !got.Equal(want) {
		t.Fatalf("rebased deadline=%v, want %v", got, want)
	}
	briefNow := deadline.Add(100 * time.Millisecond)
	if got := limitFrameCatchUp(deadline, briefNow); !got.Equal(deadline) {
		t.Fatalf("brief-delay deadline=%v, want unchanged %v", got, deadline)
	}
}

func TestFrameDeadlineHasNoSixtyHertzTruncationDrift(t *testing.T) {
	start := time.Unix(100, 123)
	if got, want := frameDeadline(start, 60), start.Add(time.Second); !got.Equal(want) {
		t.Fatalf("60th deadline=%v, want %v", got, want)
	}
	if got, want := frameDeadline(start, 3600), start.Add(time.Minute); !got.Equal(want) {
		t.Fatalf("3600th deadline=%v, want %v", got, want)
	}
}

func TestNewInitializesRFStreamer(t *testing.T) {
	service := New("", config.Default(), log.New(&bytes.Buffer{}, "", 0))
	if service.rf == nil {
		t.Fatal("RF streamer was not initialized")
	}
}

func TestStartRFOutputUsesRunConfiguration(t *testing.T) {
	disabled := config.Default()
	disabled.RFOutput.Address = "invalid-address"
	service := New("", disabled, log.New(&bytes.Buffer{}, "", 0))
	if err := service.startRFOutput(context.Background(), disabled.RFOutput); err != nil {
		t.Fatalf("disabled RF output: %v", err)
	}
	if service.rf.Stats().Running {
		t.Fatal("disabled RF output was started")
	}

	enabled := disabled.RFOutput
	enabled.Enabled = true
	if err := service.startRFOutput(context.Background(), enabled); err == nil {
		t.Fatal("expected invalid RF output address error")
	}
}

func TestWriteRFFrameIgnoresStoppedStreamer(t *testing.T) {
	var output bytes.Buffer
	service := New("", config.Default(), log.New(&output, "", 0))
	frame := make([]byte, rfstream.RGBWidth*rfstream.RGBHeight*3)
	audio := make([]int16, rfstream.StereoSamplesPerInputFrame)
	service.writeRFFrame(true, frame, audio)
	if output.Len() != 0 {
		t.Fatalf("stopped RF output logged %q", output.String())
	}

	output.Reset()
	service.writeRFFrame(false, frame, audio)
	if output.Len() != 0 {
		t.Fatalf("disabled RF output logged %q", output.String())
	}
}

func TestReloadKeepsActiveRFConfigurationUntilRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"rf_output": {
			"enabled": true,
			"address": "127.0.0.1:24000"
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	service := New(path, config.Default(), log.New(&output, "", 0))
	service.reload()
	if service.cfg.RFOutput.Enabled {
		t.Fatal("reload changed RF output without restarting its workers")
	}
	if !strings.Contains(output.String(), "RF output changes require restart") {
		t.Fatalf("log output=%q", output.String())
	}
}
