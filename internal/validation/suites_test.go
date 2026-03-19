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

func TestRunSuiteByROMInputsUsesNESTestLog(t *testing.T) {
	lines := "8000  EA        NOP                             A:00 X:00 Y:00 P:24 SP:FA\n"
	lines += "8001  4C 00 80  JMP $8000                       A:00 X:00 Y:00 P:24 SP:FA\n"

	res, err := RunSuiteByROMInputs("nestest", []ROMInput{
		{
			Name:               "nestest.nes",
			Data:               buildSimpleROM(),
			ExpectedLogContent: lines,
		},
	}, 2)
	if err != nil {
		t.Fatalf("RunSuiteByROMInputs: %v", err)
	}
	if res.Passed != 1 || res.Failed != 0 {
		t.Fatalf("expected nestest suite pass, got passed=%d failed=%d errors=%v", res.Passed, res.Failed, res.Errors)
	}
}

func TestRunSuiteByROMInputsNESTestRequiresLog(t *testing.T) {
	res, err := RunSuiteByROMInputs("nestest", []ROMInput{
		{
			Name: "nestest.nes",
			Data: buildSimpleROM(),
		},
	}, 2)
	if err != nil {
		t.Fatalf("RunSuiteByROMInputs: %v", err)
	}
	if res.Passed != 0 || res.Failed != 1 {
		t.Fatalf("expected nestest suite fail without log, got passed=%d failed=%d", res.Passed, res.Failed)
	}
	if len(res.Errors) == 0 {
		t.Fatalf("expected missing log error")
	}
}

func TestRunSuiteByDirLoadsAdjacentNESTestLog(t *testing.T) {
	d := t.TempDir()
	romPath := filepath.Join(d, "nestest.nes")
	logPath := filepath.Join(d, "nestest.log")
	if err := os.WriteFile(romPath, buildSimpleROM(), 0o644); err != nil {
		t.Fatalf("write rom: %v", err)
	}
	lines := "8000  EA        NOP                             A:00 X:00 Y:00 P:24 SP:FA\n"
	lines += "8001  4C 00 80  JMP $8000                       A:00 X:00 Y:00 P:24 SP:FA\n"
	if err := os.WriteFile(logPath, []byte(lines), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	res, err := RunSuiteByDir("nestest", d, 2)
	if err != nil {
		t.Fatalf("RunSuiteByDir: %v", err)
	}
	if res.Passed != 1 || res.Failed != 0 {
		t.Fatalf("expected pass with adjacent log, got passed=%d failed=%d errors=%v", res.Passed, res.Failed, res.Errors)
	}
}

func TestRunSuiteByROMInputsBlarggCPUUsesStatusMessagePass(t *testing.T) {
	res, err := RunSuiteByROMInputs("blargg-cpu", []ROMInput{
		{
			Name: "blargg_cpu_test.nes",
			Data: buildStatusMessageROM(0x00, "Passed"),
		},
	}, 2)
	if err != nil {
		t.Fatalf("RunSuiteByROMInputs: %v", err)
	}
	if res.Passed != 1 || res.Failed != 0 {
		t.Fatalf("expected blargg-cpu pass, got passed=%d failed=%d errors=%v", res.Passed, res.Failed, res.Errors)
	}
}

func TestRunSuiteByROMInputsBlarggCPUUsesStatusMessageFail(t *testing.T) {
	res, err := RunSuiteByROMInputs("blargg-cpu", []ROMInput{
		{
			Name: "blargg_cpu_test.nes",
			Data: buildStatusMessageROM(0x01, "Failed"),
		},
	}, 2)
	if err != nil {
		t.Fatalf("RunSuiteByROMInputs: %v", err)
	}
	if res.Passed != 0 || res.Failed != 1 {
		t.Fatalf("expected blargg-cpu fail, got passed=%d failed=%d errors=%v", res.Passed, res.Failed, res.Errors)
	}
}

func TestRunSuiteByROMInputsPPUSuitePrefersStatusProbeWhenAvailable(t *testing.T) {
	res, err := RunSuiteByROMInputs("ppu", []ROMInput{
		{
			Name: "ppu_case.nes",
			Data: buildStatusMessageROM(0x01, "Failed"),
		},
	}, 2)
	if err != nil {
		t.Fatalf("RunSuiteByROMInputs: %v", err)
	}
	if res.Passed != 0 || res.Failed != 1 {
		t.Fatalf("expected ppu suite fail from status probe, got passed=%d failed=%d errors=%v", res.Passed, res.Failed, res.Errors)
	}
}

func TestRunSuiteByROMInputsPPUSuiteFallsBackWithoutStatusProbe(t *testing.T) {
	res, err := RunSuiteByROMInputs("ppu", []ROMInput{
		{
			Name: "ppu_case.nes",
			Data: buildSimpleROM(),
		},
	}, 2)
	if err != nil {
		t.Fatalf("RunSuiteByROMInputs: %v", err)
	}
	if res.Passed != 1 || res.Failed != 0 {
		t.Fatalf("expected ppu suite deterministic fallback pass, got passed=%d failed=%d errors=%v", res.Passed, res.Failed, res.Errors)
	}
}

func buildStatusMessageROM(status byte, msg string) []byte {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 16*1024)
	pc := 0
	put := func(v byte) {
		prg[pc] = v
		pc++
	}
	// LDA #status ; STA $6000
	put(0xA9)
	put(status)
	put(0x8D)
	put(0x00)
	put(0x60)
	for i := 0; i < len(msg); i++ {
		// LDA #msg[i] ; STA $6004+i
		put(0xA9)
		put(msg[i])
		put(0x8D)
		put(byte(0x04 + i))
		put(0x60)
	}
	// NUL terminate message.
	put(0xA9)
	put(0x00)
	put(0x8D)
	put(byte(0x04 + len(msg)))
	put(0x60)
	// JMP $8000
	put(0x4C)
	put(0x00)
	put(0x80)

	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)
	return rom
}
