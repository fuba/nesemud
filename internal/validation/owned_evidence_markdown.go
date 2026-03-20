package validation

import (
	"fmt"
	"sort"
	"strings"
)

func FormatOwnedROMEvidenceMarkdown(report OwnedROMEvidenceReport) string {
	var b strings.Builder
	b.WriteString("# Owned ROM Evidence Report\n\n")
	b.WriteString(fmt.Sprintf("- ROM count: %d\n", report.ROMCount))

	var okCount int
	for _, r := range report.Results {
		if ownedEvidenceStatus(r) == "OK" {
			okCount++
		}
	}
	b.WriteString(fmt.Sprintf("- Healthy runs: %d\n", okCount))
	b.WriteString(fmt.Sprintf("- Needs review: %d\n\n", len(report.Results)-okCount))

	b.WriteString("| ROM | Result | Frames | Paused | Uniform Frame | Non-Uniform Seen | First Non-Uniform Frame | Audio Active | Audio Peak | APU 4015 | APU 4017 | Notes |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, r := range report.Results {
		result, notes := ownedEvidenceStatus(r), ownedEvidenceNotes(r)
		firstNonUniform := "-"
		if r.NonUniformObserved && r.FirstNonUniformFrame > 0 {
			firstNonUniform = fmt.Sprintf("%d", r.FirstNonUniformFrame)
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %t | %t | %t | %s | %d | %d | %d | %d | %s |\n",
			r.Name,
			result,
			r.FrameCount,
			r.Paused,
			r.UniformFrame,
			r.NonUniformObserved,
			firstNonUniform,
			r.AudioActiveSamples,
			r.AudioPeakAbs,
			r.APUWrite4015,
			r.APUWrite4017,
			sanitizeMarkdownCell(notes),
		))
	}
	b.WriteString("\n")
	return b.String()
}

func ownedEvidenceStatus(r OwnedROMEvidence) string {
	if r.Error != "" {
		return "ERROR"
	}
	if r.Paused {
		return "FAIL"
	}
	if ownedEvidenceUniformStuck(r) {
		return "WARN"
	}
	return "OK"
}

func ownedEvidenceNotes(r OwnedROMEvidence) string {
	if r.Error != "" {
		return r.Error
	}
	if r.Paused {
		return "paused during run"
	}
	if ownedEvidenceUniformStuck(r) {
		return "uniform frame output"
	}
	return ""
}

func sanitizeMarkdownCell(v string) string {
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "|", "\\|")
	return strings.TrimSpace(v)
}

type ownedEvidenceChecklistItem struct {
	Name     string
	Severity string
	Owner    string
	Reason   string
}

func FormatOwnedROMEvidenceChecklistMarkdown(report OwnedROMEvidenceReport) string {
	items := buildOwnedEvidenceChecklistItems(report)
	var b strings.Builder
	b.WriteString("# Owned ROM Evidence Checklist\n\n")
	b.WriteString(fmt.Sprintf("- ROM count: %d\n", report.ROMCount))
	b.WriteString(fmt.Sprintf("- Action items: %d\n", len(items)))
	b.WriteString(fmt.Sprintf("- Healthy runs: %d\n\n", report.ROMCount-len(items)))

	b.WriteString("## Priority Backlog\n\n")
	if len(items) == 0 {
		b.WriteString("No action items.\n")
		return b.String()
	}
	for _, item := range items {
		b.WriteString(fmt.Sprintf("- [ ] `%s` %s (%s): %s\n",
			item.Severity,
			item.Name,
			item.Owner,
			item.Reason,
		))
	}
	b.WriteString("\n")
	return b.String()
}

func buildOwnedEvidenceChecklistItems(report OwnedROMEvidenceReport) []ownedEvidenceChecklistItem {
	items := make([]ownedEvidenceChecklistItem, 0, len(report.Results))
	for _, r := range report.Results {
		if ownedEvidenceStatus(r) == "OK" {
			continue
		}
		items = append(items, ownedEvidenceChecklistItem{
			Name:     r.Name,
			Severity: ownedEvidenceStatus(r),
			Owner:    ownedEvidenceOwner(r),
			Reason:   ownedEvidenceNotes(r),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		li := ownedEvidenceSeverityRank(items[i].Severity)
		lj := ownedEvidenceSeverityRank(items[j].Severity)
		if li != lj {
			return li < lj
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items
}

func ownedEvidenceSeverityRank(severity string) int {
	switch severity {
	case "ERROR":
		return 0
	case "FAIL":
		return 1
	case "WARN":
		return 2
	default:
		return 3
	}
}

func ownedEvidenceOwner(r OwnedROMEvidence) string {
	if r.Error != "" {
		return "loader/core"
	}
	if r.Paused {
		return "cpu/irq"
	}
	if ownedEvidenceUniformStuck(r) {
		if r.AudioActiveSamples > 0 || r.AudioPeakAbs > 0 || r.APUWrite4015 > 0 || r.APUWrite4017 > 0 {
			return "ppu/mapper"
		}
		return "cpu/boot"
	}
	return "unknown"
}

func ownedEvidenceUniformStuck(r OwnedROMEvidence) bool {
	return r.UniformFrame && !r.NonUniformObserved
}
