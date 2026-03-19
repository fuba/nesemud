package validation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nesemud/internal/nes"
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
	Name               string
	Data               []byte
	ExpectedLogContent string
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
		in := ROMInput{Name: rp, Data: b}
		if strings.EqualFold(strings.TrimSpace(suite), "nestest") {
			logContent, ok, err := loadAdjacentNESTestLog(rp)
			if err != nil {
				return SuiteResult{}, err
			}
			if ok {
				in.ExpectedLogContent = logContent
			}
		}
		inputs = append(inputs, in)
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
		if strings.EqualFold(strings.TrimSpace(suite), "nestest") {
			if strings.TrimSpace(r.ExpectedLogContent) == "" {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("%s: expected_log_content is required for nestest suite", r.Name))
				continue
			}
			nres, err := RunNESTest(NESTestRequest{
				ROMContentBase64:   encodeBase64(r.Data),
				ExpectedLogContent: r.ExpectedLogContent,
				Instructions:       frames,
			})
			if err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", r.Name, err))
				continue
			}
			if nres.Matched {
				res.Passed++
			} else {
				res.Failed++
				for _, mm := range nres.Mismatches {
					res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", r.Name, mm))
				}
			}
			continue
		}
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
		if !v.Deterministic {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: non-deterministic", r.Name))
			continue
		}
		if suiteUsesStatusProbe(suite) {
			conclusive, ok, detail, err := runStatusProbe(r.Data, frames)
			if err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", r.Name, err))
				continue
			}
			if !conclusive && strings.EqualFold(strings.TrimSpace(suite), "blargg-cpu") {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("%s: no status message at $6004", r.Name))
				continue
			}
			if !ok {
				if conclusive {
					res.Failed++
					res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", r.Name, detail))
					continue
				}
			}
		}
		res.Passed++
	}
	return res, nil
}

func suitePatterns(suite string) ([]string, error) {
	s := strings.ToLower(strings.TrimSpace(suite))
	switch s {
	case "nestest":
		return []string{"nestest"}, nil
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

func loadAdjacentNESTestLog(romPath string) (string, bool, error) {
	base := strings.TrimSuffix(romPath, filepath.Ext(romPath))
	candidates := []string{
		base + ".log",
		filepath.Join(filepath.Dir(romPath), "nestest.log"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return string(b), true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
	}
	return "", false, nil
}

func suiteUsesStatusProbe(suite string) bool {
	switch strings.ToLower(strings.TrimSpace(suite)) {
	case "blargg-cpu", "ppu", "apu", "mapper":
		return true
	default:
		return false
	}
}

func runStatusProbe(rom []byte, frames int) (bool, bool, string, error) {
	if frames <= 0 {
		frames = 1200
	}
	c := nes.NewConsole()
	if err := c.LoadROMContent(rom); err != nil {
		return false, false, "", err
	}
	for i := 0; i < frames; i++ {
		c.StepFrame()
	}
	sb, err := c.Peek(0x6000, 1)
	if err != nil {
		return false, false, "", err
	}
	mb, err := c.Peek(0x6004, 256)
	if err != nil {
		return false, false, "", err
	}
	msg := decodeASCIIZ(mb)
	if msg == "" {
		return false, false, "", nil
	}
	lower := strings.ToLower(msg)
	if sb[0] == 0x00 && strings.Contains(lower, "pass") {
		return true, true, msg, nil
	}
	return true, false, fmt.Sprintf("status=0x%02X message=%q", sb[0], msg), nil
}

func decodeASCIIZ(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, ch := range b {
		if ch == 0 {
			break
		}
		if ch < 0x20 || ch > 0x7E {
			break
		}
		out = append(out, ch)
	}
	return strings.TrimSpace(string(out))
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
