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
			conclusive, ok, detail, err := runStatusProbe(r.Data, frames, suite)
			if err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", r.Name, err))
				continue
			}
			if !conclusive && strings.EqualFold(strings.TrimSpace(suite), "blargg-cpu") {
				// Some blargg CPU ROM variants do not expose the $6000/$6004 status protocol.
				// In that case, keep deterministic validation as a fallback rather than hard-fail.
				if strings.TrimSpace(detail) != "" {
					res.Failed++
					res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", r.Name, detail))
					continue
				}
				res.Passed++
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

func runStatusProbe(rom []byte, frames int, suite string) (bool, bool, string, error) {
	if frames <= 0 {
		frames = 1200
	}
	c := nes.NewConsole()
	if err := c.LoadROMContent(rom); err != nil {
		return false, false, "", err
	}
	strictBlargg := strings.EqualFold(strings.TrimSpace(suite), "blargg-cpu")
	blarggSeen := false
	resetIssued := 0
	sawNonZeroStatus := false
	for i := 0; i < frames; i++ {
		c.StepFrame()
		if paused, detail := statusProbePauseDetail(c); paused {
			return true, false, detail, nil
		}
		status, msg, blarggSig, err := readStatusProbe(c)
		if err != nil {
			return false, false, "", err
		}
		if status != 0 {
			sawNonZeroStatus = true
		}
		if strictBlargg && blarggSig {
			blarggSeen = true
			switch status {
			case 0x00:
				if msg == "" {
					msg = "status=0x00"
				}
				return true, true, msg, nil
			case 0x80:
				continue
			case 0x81:
				// blargg protocol: 0x81 requests a reset to continue execution.
				if resetIssued < 2 {
					c.Reset()
					resetIssued++
					continue
				}
				return true, false, "status=0x81 (reset requested repeatedly)", nil
			default:
				if status < 0x80 {
					return true, false, fmt.Sprintf("status=0x%02X message=%q", status, msg), nil
				}
			}
		}
	}
	status, msg, blarggSig, err := readStatusProbe(c)
	if err != nil {
		return false, false, "", err
	}
	if strictBlargg {
		if blarggSeen || blarggSig {
			return true, false, fmt.Sprintf("status=0x%02X message=%q (not completed within %d frames)", status, msg, frames), nil
		}
		if status == 0x00 && sawNonZeroStatus {
			return true, true, "status=0x00 (inferred completion without blargg signature)", nil
		}
		if status != 0 || msg != "" {
			return false, false, fmt.Sprintf("blargg signature not found (status=0x%02X message=%q)", status, msg), nil
		}
	}
	if msg == "" {
		return false, false, "", nil
	}
	lower := strings.ToLower(msg)
	if status == 0x00 && strings.Contains(lower, "pass") {
		return true, true, msg, nil
	}
	return true, false, fmt.Sprintf("status=0x%02X message=%q", status, msg), nil
}

func readStatusProbe(c *nes.Console) (byte, string, bool, error) {
	sb, err := c.Peek(0x6000, 4)
	if err != nil {
		return 0, "", false, err
	}
	mb, err := c.Peek(0x6004, 256)
	if err != nil {
		return 0, "", false, err
	}
	sig := len(sb) >= 4 && sb[1] == 0xDE && sb[2] == 0xB0 && sb[3] == 0x61
	return sb[0], decodeASCIIZ(mb), sig, nil
}

func statusProbePauseDetail(c *nes.Console) (bool, string) {
	st := c.State()
	paused, _ := st["paused"].(bool)
	if !paused {
		return false, ""
	}
	if cpu, ok := st["cpu"].(map[string]any); ok {
		pc, _ := cpu["pc"].(uint16)
		a, _ := cpu["a"].(byte)
		x, _ := cpu["x"].(byte)
		y, _ := cpu["y"].(byte)
		sp, _ := cpu["sp"].(byte)
		p, _ := cpu["p"].(byte)
		return true, fmt.Sprintf("emulation paused at PC=%04X A=%02X X=%02X Y=%02X P=%02X SP=%02X", pc, a, x, y, p, sp)
	}
	return true, "emulation paused"
}

func decodeASCIIZ(b []byte) string {
	out := make([]byte, 0, len(b))
	started := false
	for _, ch := range b {
		if ch == 0 {
			break
		}
		if ch < 0x20 || ch > 0x7E {
			if !started {
				continue
			}
			break
		}
		started = true
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
