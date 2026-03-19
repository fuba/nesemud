package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectOwnedROMEvidence(t *testing.T) {
	d := t.TempDir()
	romPath := filepath.Join(d, "sample.nes")
	if err := os.WriteFile(romPath, buildValidationROM(), 0o644); err != nil {
		t.Fatalf("write rom: %v", err)
	}

	report, err := CollectOwnedROMEvidence(d, 5)
	if err != nil {
		t.Fatalf("CollectOwnedROMEvidence: %v", err)
	}
	if report.ROMCount != 1 {
		t.Fatalf("rom_count=%d, want 1", report.ROMCount)
	}
	if len(report.Results) != 1 {
		t.Fatalf("results len=%d, want 1", len(report.Results))
	}
	r := report.Results[0]
	if r.Name == "" {
		t.Fatalf("expected rom name")
	}
	if r.FrameCount == 0 {
		t.Fatalf("expected frame_count to advance")
	}
}

func TestCollectOwnedROMEvidenceSkipsNonNESFiles(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	report, err := CollectOwnedROMEvidence(d, 5)
	if err != nil {
		t.Fatalf("CollectOwnedROMEvidence: %v", err)
	}
	if report.ROMCount != 0 {
		t.Fatalf("rom_count=%d, want 0", report.ROMCount)
	}
}

func buildValidationROM() []byte {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 16*1024)
	prg[0] = 0xEA
	prg[1] = 0x4C
	prg[2] = 0x00
	prg[3] = 0x80
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)
	return rom
}
