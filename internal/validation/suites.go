package validation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SuiteResult struct {
	Suite     string            `json:"suite"`
	ROMCount  int               `json:"rom_count"`
	Passed    int               `json:"passed"`
	Failed    int               `json:"failed"`
	ROMHashes map[string]string `json:"rom_hashes"`
	Errors    []string          `json:"errors"`
}

type ROMInput struct {
	Name string
	Data []byte
}

func RunSuiteByDir(suite string, romDir string, frames int) (SuiteResult, error) {
	patterns, err := suitePatterns(suite)
	if err != nil {
		return SuiteResult{}, err
	}
	roms, err := collectROMs(romDir, patterns)
	if err != nil {
		return SuiteResult{}, err
	}
	inputs := make([]ROMInput, 0, len(roms))
	for _, rp := range roms {
		b, err := os.ReadFile(rp)
		if err != nil {
			return SuiteResult{}, err
		}
		inputs = append(inputs, ROMInput{Name: rp, Data: b})
	}
	return RunSuiteByROMInputs(suite, inputs, frames)
}

func RunSuiteByROMInputs(suite string, roms []ROMInput, frames int) (SuiteResult, error) {
	patterns, err := suitePatterns(suite)
	if err != nil {
		return SuiteResult{}, err
	}
	selected := filterROMInputs(roms, patterns)
	res := SuiteResult{
		Suite:     suite,
		ROMCount:  len(selected),
		ROMHashes: map[string]string{},
	}
	if res.ROMCount == 0 {
		return SuiteResult{}, fmt.Errorf("no roms found for suite %s", suite)
	}
	for _, r := range selected {
		v, err := RunReplayValidation(ReplayValidationRequest{
			ROMContentBase64: encodeBase64(r.Data),
			Frames:           frames,
			Repeats:          2,
		})
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", r.Name, err))
			continue
		}
		res.ROMHashes[r.Name] = v.Hashes[0]
		if v.Deterministic {
			res.Passed++
		} else {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: non-deterministic", r.Name))
		}
	}
	return res, nil
}

func suitePatterns(suite string) ([]string, error) {
	s := strings.ToLower(strings.TrimSpace(suite))
	switch s {
	case "nestest":
		return []string{"nestest", "cpu"}, nil
	case "blargg-cpu":
		return []string{"blargg", "cpu"}, nil
	case "ppu":
		return []string{"ppu"}, nil
	case "apu":
		return []string{"apu"}, nil
	case "mapper":
		return []string{"mapper", "mmc", "uxrom", "cnrom"}, nil
	default:
		return nil, errors.New("unknown suite")
	}
}

func filterROMInputs(roms []ROMInput, patterns []string) []ROMInput {
	out := make([]ROMInput, 0, len(roms))
	for _, r := range roms {
		name := strings.ToLower(filepath.Base(strings.TrimSpace(r.Name)))
		if name == "" {
			continue
		}
		for _, p := range patterns {
			if strings.Contains(name, p) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func collectROMs(root string, patterns []string) ([]string, error) {
	var roms []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".nes") {
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		for _, p := range patterns {
			if strings.Contains(name, p) {
				roms = append(roms, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return roms, nil
}
