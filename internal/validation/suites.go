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

func RunSuiteByDir(suite string, romDir string, frames int) (SuiteResult, error) {
	patterns, err := suitePatterns(suite)
	if err != nil {
		return SuiteResult{}, err
	}
	roms, err := collectROMs(romDir, patterns)
	if err != nil {
		return SuiteResult{}, err
	}
	res := SuiteResult{
		Suite:     suite,
		ROMCount:  len(roms),
		ROMHashes: map[string]string{},
	}
	if res.ROMCount == 0 {
		return SuiteResult{}, fmt.Errorf("no roms found for suite %s under %s", suite, romDir)
	}
	for _, rp := range roms {
		v, err := RunReplayValidation(ReplayValidationRequest{
			ROMPath: rp,
			Frames:  frames,
			Repeats: 2,
		})
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", rp, err))
			continue
		}
		res.ROMHashes[rp] = v.Hashes[0]
		if v.Deterministic {
			res.Passed++
		} else {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: non-deterministic", rp))
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
