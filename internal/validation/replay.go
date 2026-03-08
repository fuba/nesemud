package validation

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"nesemud/internal/nes"
)

type ReplayValidationRequest struct {
	ROMPath          string `json:"rom_path,omitempty"`
	FM2Path          string `json:"fm2_path,omitempty"`
	ROMContentBase64 string `json:"rom_content_base64,omitempty"`
	FM2Content       string `json:"fm2_content,omitempty"`
	Frames           int    `json:"frames"`
	Repeats          int    `json:"repeats"`
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

	var rom []byte
	if req.ROMContentBase64 != "" {
		var err error
		rom, err = base64.StdEncoding.DecodeString(req.ROMContentBase64)
		if err != nil {
			return "", fmt.Errorf("invalid rom_content_base64: %w", err)
		}
	} else {
		if req.ROMPath == "" {
			return "", fmt.Errorf("rom input is required")
		}
		var err error
		rom, err = os.ReadFile(req.ROMPath)
		if err != nil {
			return "", err
		}
	}
	if err := c.LoadROMContent(rom); err != nil {
		return "", err
	}

	if req.FM2Content != "" {
		if err := c.LoadFM2Content([]byte(req.FM2Content)); err != nil {
			return "", err
		}
	} else if req.FM2Path != "" {
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

func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
