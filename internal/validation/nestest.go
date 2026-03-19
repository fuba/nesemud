package validation

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"nesemud/internal/nes"
)

type NESTestRequest struct {
	ROMPath            string `json:"rom_path,omitempty"`
	ExpectedLogPath    string `json:"expected_log_path,omitempty"`
	ROMContentBase64   string `json:"rom_content_base64,omitempty"`
	ExpectedLogContent string `json:"expected_log_content,omitempty"`
	Instructions       int    `json:"instructions"`
}

type NESTestResult struct {
	ComparedLines int      `json:"compared_lines"`
	Matched       bool     `json:"matched"`
	Mismatches    []string `json:"mismatches"`
}

var nestLineRe = regexp.MustCompile(`A:([0-9A-F]{2})\s+X:([0-9A-F]{2})\s+Y:([0-9A-F]{2})\s+P:([0-9A-F]{2})\s+SP:([0-9A-F]{2})`)

func RunNESTest(req NESTestRequest) (NESTestResult, error) {
	if req.Instructions <= 0 {
		req.Instructions = 5000
	}
	c := nes.NewConsole()

	if req.ROMContentBase64 != "" {
		rom, err := base64.StdEncoding.DecodeString(req.ROMContentBase64)
		if err != nil {
			return NESTestResult{}, fmt.Errorf("invalid rom_content_base64: %w", err)
		}
		if err := c.LoadROMContent(rom); err != nil {
			return NESTestResult{}, err
		}
	} else {
		if req.ROMPath == "" {
			return NESTestResult{}, fmt.Errorf("rom input is required")
		}
		if err := c.LoadROMFromFile(req.ROMPath); err != nil {
			return NESTestResult{}, err
		}
	}

	var (
		expected []nes.CPUState
		err      error
	)
	if req.ExpectedLogContent != "" {
		expected, err = readExpectedNESTestFromBytes([]byte(req.ExpectedLogContent))
	} else {
		if req.ExpectedLogPath == "" {
			return NESTestResult{}, fmt.Errorf("expected log input is required")
		}
		expected, err = readExpectedNESTest(req.ExpectedLogPath)
	}
	if err != nil {
		return NESTestResult{}, err
	}
	// Align CPU registers to the first expected trace line.
	// NESTest logs typically start from a fixed harness state (e.g. PC=$C000),
	// which may differ from the ROM reset vector path.
	c.SetCPUState(expected[0])

	limit := req.Instructions
	if len(expected) < limit {
		limit = len(expected)
	}
	res := NESTestResult{Matched: true}
	for i := 0; i < limit; i++ {
		st := c.SnapshotCPU()
		exp := expected[i]
		if st.PC != exp.PC || st.A != exp.A || st.X != exp.X || st.Y != exp.Y || st.P != exp.P || st.SP != exp.SP {
			res.Matched = false
			res.Mismatches = append(res.Mismatches,
				fmt.Sprintf("line %d: got PC=%04X A=%02X X=%02X Y=%02X P=%02X SP=%02X expected PC=%04X A=%02X X=%02X Y=%02X P=%02X SP=%02X",
					i+1, st.PC, st.A, st.X, st.Y, st.P, st.SP, exp.PC, exp.A, exp.X, exp.Y, exp.P, exp.SP),
			)
			if len(res.Mismatches) >= 20 {
				break
			}
		}
		if err := c.StepInstruction(); err != nil {
			res.Matched = false
			res.Mismatches = append(res.Mismatches, fmt.Sprintf("line %d: step error: %v", i+1, err))
			break
		}
		res.ComparedLines++
	}
	return res, nil
}

func readExpectedNESTest(path string) ([]nes.CPUState, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseNESTestScanner(bufio.NewScanner(f))
}

func readExpectedNESTestFromBytes(data []byte) ([]nes.CPUState, error) {
	return parseNESTestScanner(bufio.NewScanner(bytes.NewReader(data)))
}

func parseNESTestScanner(s *bufio.Scanner) ([]nes.CPUState, error) {
	var out []nes.CPUState
	for s.Scan() {
		line := s.Text()
		st, ok := parseNESTestLine(line)
		if ok {
			out = append(out, st)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no parsable nestest lines found")
	}
	return out, nil
}

func parseNESTestLine(line string) (nes.CPUState, bool) {
	line = strings.TrimSpace(line)
	if len(line) < 4 {
		return nes.CPUState{}, false
	}
	pcVal, err := strconv.ParseUint(line[:4], 16, 16)
	if err != nil {
		return nes.CPUState{}, false
	}
	m := nestLineRe.FindStringSubmatch(line)
	if len(m) != 6 {
		return nes.CPUState{}, false
	}
	a, _ := strconv.ParseUint(m[1], 16, 8)
	x, _ := strconv.ParseUint(m[2], 16, 8)
	y, _ := strconv.ParseUint(m[3], 16, 8)
	p, _ := strconv.ParseUint(m[4], 16, 8)
	sp, _ := strconv.ParseUint(m[5], 16, 8)
	return nes.CPUState{
		PC: uint16(pcVal),
		A:  byte(a),
		X:  byte(x),
		Y:  byte(y),
		P:  byte(p),
		SP: byte(sp),
	}, true
}
