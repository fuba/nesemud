package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunReplayValidationDeterministic(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	if err := os.WriteFile(romPath, buildTestROM(), 0o644); err != nil {
		t.Fatalf("write rom: %v", err)
	}

	res, err := RunReplayValidation(ReplayValidationRequest{
		ROMPath: romPath,
		Frames:  30,
		Repeats: 2,
	})
	if err != nil {
		t.Fatalf("RunReplayValidation: %v", err)
	}
	if len(res.Hashes) != 2 {
		t.Fatalf("expected 2 hashes")
	}
	if !res.Deterministic {
		t.Fatalf("expected deterministic result")
	}
}

func buildTestROM() []byte {
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
