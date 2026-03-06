package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectROMsByPattern(t *testing.T) {
	d := t.TempDir()
	files := []string{"nestest.nes", "blargg_cpu.nes", "random.txt", "ppu_test.nes"}
	for _, f := range files {
		p := filepath.Join(d, f)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	roms, err := collectROMs(d, []string{"ppu"})
	if err != nil {
		t.Fatalf("collectROMs: %v", err)
	}
	if len(roms) != 1 {
		t.Fatalf("expected 1 ppu rom, got %d", len(roms))
	}
}
