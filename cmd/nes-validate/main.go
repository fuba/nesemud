package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"nesemud/internal/validation"
)

func main() {
	suite := flag.String("suite", "nestest", "suite name: nestest|blargg-cpu|ppu|apu|mapper|nestest-log|owned-evidence")
	romDir := flag.String("rom-dir", "./tests/roms", "directory containing .nes files")
	romPath := flag.String("rom", "", "rom path for nestest-log")
	logPath := flag.String("log", "", "expected log path for nestest-log")
	frames := flag.Int("frames", 1200, "frames per rom")
	instructions := flag.Int("instructions", 5000, "instructions for nestest-log")
	markdownOut := flag.String("markdown-out", "", "optional markdown output path for owned-evidence")
	checklistOut := flag.String("checklist-out", "", "optional checklist markdown output path for owned-evidence")
	flag.Parse()

	if *suite == "nestest-log" {
		if *romPath == "" || *logPath == "" {
			fmt.Fprintln(os.Stderr, "--rom and --log are required for nestest-log")
			os.Exit(2)
		}
		res, err := validation.RunNESTest(validation.NESTestRequest{
			ROMPath:         *romPath,
			ExpectedLogPath: *logPath,
			Instructions:    *instructions,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		if !res.Matched {
			os.Exit(2)
		}
		return
	}

	if *suite == "owned-evidence" {
		res, err := validation.CollectOwnedROMEvidence(*romDir, *frames)
		if err != nil {
			fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		if *markdownOut != "" {
			md := validation.FormatOwnedROMEvidenceMarkdown(res)
			if err := os.WriteFile(*markdownOut, []byte(md), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write markdown: %v\n", err)
				os.Exit(1)
			}
		}
		if *checklistOut != "" {
			md := validation.FormatOwnedROMEvidenceChecklistMarkdown(res)
			if err := os.WriteFile(*checklistOut, []byte(md), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write checklist markdown: %v\n", err)
				os.Exit(1)
			}
		}
		for _, ev := range res.Results {
			if ev.Error != "" || ev.Paused {
				os.Exit(2)
			}
		}
		return
	}

	res, err := validation.RunSuiteByDir(*suite, *romDir, *frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
	if res.Failed > 0 {
		os.Exit(2)
	}
}
