package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNESTestLine(t *testing.T) {
	line := "C000  A9 00     LDA #$00                        A:00 X:10 Y:20 P:24 SP:FD"
	st, ok := parseNESTestLine(line)
	if !ok {
		t.Fatalf("expected parsable line")
	}
	if st.PC != 0xC000 || st.X != 0x10 || st.SP != 0xFD {
		t.Fatalf("unexpected parsed state: %+v", st)
	}
}

func TestRunNESTestWithSyntheticLog(t *testing.T) {
	d := t.TempDir()
	rom := filepath.Join(d, "t.nes")
	log := filepath.Join(d, "nestest.log")
	if err := os.WriteFile(rom, buildSimpleROMWithResetVector(0x8000), 0o644); err != nil {
		t.Fatalf("write rom: %v", err)
	}
	lines := "8000  EA        NOP                             A:00 X:00 Y:00 P:24 SP:FA\n"
	lines += "8001  4C 00 80  JMP $8000                       A:00 X:00 Y:00 P:24 SP:FA\n"
	if err := os.WriteFile(log, []byte(lines), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	res, err := RunNESTest(NESTestRequest{
		ROMPath:         rom,
		ExpectedLogPath: log,
		Instructions:    2,
	})
	if err != nil {
		t.Fatalf("RunNESTest: %v", err)
	}
	if !res.Matched {
		t.Fatalf("expected match, got mismatches: %v", res.Mismatches)
	}
}

func TestRunNESTestAlignsInitialCPUStateToExpectedTrace(t *testing.T) {
	d := t.TempDir()
	rom := filepath.Join(d, "t.nes")
	log := filepath.Join(d, "nestest.log")
	if err := os.WriteFile(rom, buildSimpleROMWithResetVector(0x9000), 0o644); err != nil {
		t.Fatalf("write rom: %v", err)
	}
	lines := "8000  EA        NOP                             A:00 X:00 Y:00 P:24 SP:FA\n"
	lines += "8001  4C 00 80  JMP $8000                       A:00 X:00 Y:00 P:24 SP:FA\n"
	if err := os.WriteFile(log, []byte(lines), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	res, err := RunNESTest(NESTestRequest{
		ROMPath:         rom,
		ExpectedLogPath: log,
		Instructions:    2,
	})
	if err != nil {
		t.Fatalf("RunNESTest: %v", err)
	}
	if !res.Matched {
		t.Fatalf("expected match, got mismatches: %v", res.Mismatches)
	}
}

func buildSimpleROMWithResetVector(resetVector uint16) []byte {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 16*1024)
	prg[0] = 0xEA
	prg[1] = 0x4C
	prg[2] = 0x00
	prg[3] = 0x80
	prg[0x3FFC] = byte(resetVector & 0x00FF)
	prg[0x3FFD] = byte((resetVector >> 8) & 0x00FF)
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)
	return rom
}

func buildSimpleROM() []byte {
	return buildSimpleROMWithResetVector(0x8000)
}
