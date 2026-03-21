package nes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnedROMSmoke(t *testing.T) {
	entries, err := os.ReadDir("../../dont_upload_roms")
	if err != nil {
		t.Skipf("owned rom directory not available: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".nes") {
			continue
		}

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("../../dont_upload_roms", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read rom: %v", err)
			}

			c := NewConsole()
			if err := c.LoadROMContent(data); err != nil {
				t.Fatalf("load rom: %v", err)
			}

			for i := 0; i < 30; i++ {
				c.StepFrame()
			}

			st := c.State()
			if paused, _ := st["paused"].(bool); paused {
				t.Fatalf("expected rom to keep running")
			}
			if frameCount, _ := st["frame_count"].(uint64); frameCount < 30 {
				t.Fatalf("expected frame_count to advance, got %d", frameCount)
			}
		})
	}
}
