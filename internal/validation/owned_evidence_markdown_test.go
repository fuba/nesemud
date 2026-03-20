package validation

import (
	"strings"
	"testing"
)

func TestFormatOwnedROMEvidenceMarkdown(t *testing.T) {
	report := OwnedROMEvidenceReport{
		ROMCount: 4,
		Results: []OwnedROMEvidence{
			{
				Name:               "ok.nes",
				FrameCount:         120,
				Paused:             false,
				UniformFrame:       false,
				AudioActiveSamples: 1200,
				AudioPeakAbs:       1024,
				APUWrite4015:       2,
				APUWrite4017:       1,
			},
			{
				Name:         "warn.nes",
				FrameCount:   60,
				UniformFrame: true,
			},
			{
				Name:                 "ok-after-boot.nes",
				FrameCount:           120,
				UniformFrame:         true,
				NonUniformObserved:   true,
				FirstNonUniformFrame: 45,
			},
			{
				Name:         "fail.nes",
				FrameCount:   40,
				Paused:       true,
				UniformFrame: true,
				Error:        "unsupported opcode",
			},
		},
	}

	md := FormatOwnedROMEvidenceMarkdown(report)
	if !strings.Contains(md, "# Owned ROM Evidence Report") {
		t.Fatalf("missing header: %s", md)
	}
	if !strings.Contains(md, "| ok.nes | OK |") {
		t.Fatalf("missing ok row: %s", md)
	}
	if !strings.Contains(md, "| warn.nes | WARN |") {
		t.Fatalf("missing warn row: %s", md)
	}
	if !strings.Contains(md, "| ok-after-boot.nes | OK |") {
		t.Fatalf("missing recovered row: %s", md)
	}
	if !strings.Contains(md, "| fail.nes | ERROR |") {
		t.Fatalf("missing error row: %s", md)
	}
	if !strings.Contains(md, "- Needs review: 2") {
		t.Fatalf("unexpected summary counts: %s", md)
	}
}

func TestFormatOwnedROMEvidenceChecklistMarkdown(t *testing.T) {
	report := OwnedROMEvidenceReport{
		ROMCount: 4,
		Results: []OwnedROMEvidence{
			{Name: "ok.nes"},
			{Name: "warn-video.nes", UniformFrame: true, AudioActiveSamples: 12},
			{Name: "warn-boot.nes", UniformFrame: true},
			{Name: "error.nes", Error: "load rom: invalid header"},
		},
	}

	md := FormatOwnedROMEvidenceChecklistMarkdown(report)
	if !strings.Contains(md, "# Owned ROM Evidence Checklist") {
		t.Fatalf("missing checklist header: %s", md)
	}
	if !strings.Contains(md, "- Action items: 3") {
		t.Fatalf("unexpected action count: %s", md)
	}
	if !strings.Contains(md, "`ERROR` error.nes (loader/core): load rom: invalid header") {
		t.Fatalf("missing error row: %s", md)
	}
	if !strings.Contains(md, "`WARN` warn-video.nes (ppu/mapper): uniform frame output") {
		t.Fatalf("missing video warn row: %s", md)
	}
	if !strings.Contains(md, "`WARN` warn-boot.nes (cpu/boot): uniform frame output") {
		t.Fatalf("missing boot warn row: %s", md)
	}
}
