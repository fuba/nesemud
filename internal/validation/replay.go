package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"

	"nes_recorder/internal/nes"
)

type ReplayValidationRequest struct {
	ROMPath string `json:"rom_path"`
	FM2Path string `json:"fm2_path,omitempty"`
	Frames  int    `json:"frames"`
	Repeats int    `json:"repeats"`
}

type ReplayValidationResult struct {
	Hashes        []string `json:"hashes"`
	Deterministic bool     `json:"deterministic"`
}

func RunReplayValidation(req ReplayValidationRequest) (ReplayValidationResult, error) {
	if req.Frames <= 0 {
		req.Frames = 600
	}
	if req.Repeats <= 0 {
		req.Repeats = 2
	}
	res := ReplayValidationResult{Hashes: make([]string, 0, req.Repeats)}

	for i := 0; i < req.Repeats; i++ {
		h, err := runOnce(req)
		if err != nil {
			return ReplayValidationResult{}, err
		}
		res.Hashes = append(res.Hashes, h)
	}
	res.Deterministic = true
	for i := 1; i < len(res.Hashes); i++ {
		if res.Hashes[i] != res.Hashes[0] {
			res.Deterministic = false
			break
		}
	}
	return res, nil
}

func runOnce(req ReplayValidationRequest) (string, error) {
	c := nes.NewConsole()
	if err := c.LoadROMFromFile(req.ROMPath); err != nil {
		return "", err
	}
	if req.FM2Path != "" {
		b, err := os.ReadFile(req.FM2Path)
		if err != nil {
			return "", err
		}
		if err := c.LoadFM2Content(b); err != nil {
			return "", err
		}
	}

	h := sha256.New()
	for i := 0; i < req.Frames; i++ {
		c.StepFrame()
		st, _ := json.Marshal(c.State())
		_, _ = h.Write(st)
		_, _ = h.Write(c.SnapshotFrame())
		audio := c.SnapshotAudio()
		ab, _ := json.Marshal(audio)
		_, _ = h.Write(ab)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
