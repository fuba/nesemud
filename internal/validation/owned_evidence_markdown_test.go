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
				Mapper:             1,
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
				Mapper:       4,
				FrameCount:   60,
				UniformFrame: true,
			},
			{
				Name:                 "ok-after-boot.nes",
				Mapper:               2,
				FrameCount:           120,
				UniformFrame:         true,
				NonUniformObserved:   true,
				FirstNonUniformFrame: 45,
				ExtendedRun:          true,
				ExtraFrames:          180,
			},
			{
				Name:         "fail.nes",
				Mapper:       5,
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
	if !strings.Contains(md, "| ok.nes | 1 | OK |") {
		t.Fatalf("missing ok row: %s", md)
	}
	if !strings.Contains(md, "| warn.nes | 4 | WARN |") {
		t.Fatalf("missing warn row: %s", md)
	}
	if !strings.Contains(md, "| ok-after-boot.nes | 2 | OK |") {
		t.Fatalf("missing recovered row: %s", md)
	}
	if !strings.Contains(md, "| fail.nes | 5 | ERROR |") {
		t.Fatalf("missing error row: %s", md)
	}
	if !strings.Contains(md, "- Needs review: 2") {
		t.Fatalf("unexpected summary counts: %s", md)
	}
	if !strings.Contains(md, "uniform frame output (mapper 4)") {
		t.Fatalf("missing mapper detail note: %s", md)
	}
}

func TestFormatOwnedROMEvidenceChecklistMarkdown(t *testing.T) {
	report := OwnedROMEvidenceReport{
		ROMCount: 4,
		Results: []OwnedROMEvidence{
			{Name: "ok.nes"},
			{Name: "warn-video.nes", Mapper: 4, UniformFrame: true, AudioActiveSamples: 12},
			{Name: "warn-boot.nes", Mapper: 2, UniformFrame: true},
			{Name: "error.nes", Mapper: 4, Error: "load rom: invalid header"},
		},
	}

	md := FormatOwnedROMEvidenceChecklistMarkdown(report)
	if !strings.Contains(md, "# Owned ROM Evidence Checklist") {
		t.Fatalf("missing checklist header: %s", md)
	}
	if !strings.Contains(md, "- Action items: 3") {
		t.Fatalf("unexpected action count: %s", md)
	}
	if !strings.Contains(md, "## Hotspots") {
		t.Fatalf("missing hotspots section: %s", md)
	}
	if !strings.Contains(md, "| 4 | 2 |") {
		t.Fatalf("missing mapper hotspot aggregation: %s", md)
	}
	if !strings.Contains(md, "`ERROR` error.nes (loader/core): load rom: invalid header") {
		t.Fatalf("missing error row: %s", md)
	}
	if !strings.Contains(md, "`WARN` warn-video.nes (ppu/mapper): uniform frame output (mapper 4)") {
		t.Fatalf("missing video warn row: %s", md)
	}
	if !strings.Contains(md, "`WARN` warn-boot.nes (cpu/boot): uniform frame output (mapper 2)") {
		t.Fatalf("missing boot warn row: %s", md)
	}
}
