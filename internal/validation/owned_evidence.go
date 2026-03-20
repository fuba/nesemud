package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"nesemud/internal/nes"
)

type OwnedROMEvidence struct {
	Name                 string `json:"name"`
	Mapper               uint8  `json:"mapper"`
	FrameCount           uint64 `json:"frame_count"`
	Paused               bool   `json:"paused"`
	UniformFrame         bool   `json:"uniform_frame"`
	NonUniformObserved   bool   `json:"non_uniform_observed"`
	FirstNonUniformFrame int    `json:"first_non_uniform_frame,omitempty"`
	ExtendedRun          bool   `json:"extended_run,omitempty"`
	ExtraFrames          int    `json:"extra_frames,omitempty"`
	AudioActiveSamples   int    `json:"audio_active_samples"`
	AudioPeakAbs         int    `json:"audio_peak_abs"`
	APUWrite4015         uint64 `json:"apu_write_4015"`
	APUWrite4017         uint64 `json:"apu_write_4017"`
	Error                string `json:"error,omitempty"`
}

type OwnedROMEvidenceReport struct {
	ROMCount int                `json:"rom_count"`
	Results  []OwnedROMEvidence `json:"results"`
}

func CollectOwnedROMEvidence(romDir string, frames int) (OwnedROMEvidenceReport, error) {
	if frames <= 0 {
		frames = 180
	}
	entries, err := os.ReadDir(romDir)
	if err != nil {
		return OwnedROMEvidenceReport{}, err
	}
	romNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".nes") {
			continue
		}
		romNames = append(romNames, entry.Name())
	}
	sort.Strings(romNames)

	report := OwnedROMEvidenceReport{
		ROMCount: len(romNames),
		Results:  make([]OwnedROMEvidence, 0, len(romNames)),
	}
	for _, name := range romNames {
		path := filepath.Join(romDir, name)
		ev := OwnedROMEvidence{Name: name}
		data, err := os.ReadFile(path)
		if err != nil {
			ev.Error = fmt.Sprintf("read rom: %v", err)
			report.Results = append(report.Results, ev)
			continue
		}
		if cart, err := nes.LoadINES(data); err == nil {
			ev.Mapper = cart.Mapper
		}
		c := nes.NewConsole()
		if err := c.LoadROMContent(data); err != nil {
			ev.Error = fmt.Sprintf("load rom: %v", err)
			report.Results = append(report.Results, ev)
			continue
		}
		const frameProbeInterval = 15
		runEvidenceFrames(c, &ev, frames, frameProbeInterval)
		if frames >= 60 {
			st := c.State()
			paused, _ := st["paused"].(bool)
			if !paused && !ev.NonUniformObserved {
				extra := max(frames, 180)
				runEvidenceFrames(c, &ev, extra, frameProbeInterval)
				ev.ExtendedRun = true
				ev.ExtraFrames = extra
			}
		}

		st := c.State()
		if frameCount, ok := st["frame_count"].(uint64); ok {
			ev.FrameCount = frameCount
		}
		if paused, ok := st["paused"].(bool); ok {
			ev.Paused = paused
		}
		frame := c.SnapshotFrame()
		ev.UniformFrame = isUniformFrame(frame)
		if !ev.NonUniformObserved && !ev.UniformFrame {
			ev.NonUniformObserved = true
			ev.FirstNonUniformFrame = int(ev.FrameCount)
		}
		if audio, ok := st["audio"].(map[string]any); ok {
			if v, ok := audio["active_samples"].(int); ok {
				ev.AudioActiveSamples = v
			}
			if v, ok := audio["peak_abs"].(int); ok {
				ev.AudioPeakAbs = v
			}
		}
		if apu, ok := st["apu"].(map[string]any); ok {
			if v, ok := apu["write_count_4015"].(uint64); ok {
				ev.APUWrite4015 = v
			}
			if v, ok := apu["write_count_4017"].(uint64); ok {
				ev.APUWrite4017 = v
			}
		}
		report.Results = append(report.Results, ev)
	}

	return report, nil
}

func runEvidenceFrames(c *nes.Console, ev *OwnedROMEvidence, frames int, probeInterval int) {
	if frames <= 0 {
		return
	}
	for i := 0; i < frames; i++ {
		c.StepFrame()
		if ev.NonUniformObserved {
			continue
		}
		if i%probeInterval == 0 || i == frames-1 {
			if !isUniformFrame(c.SnapshotFrame()) {
				ev.NonUniformObserved = true
				st := c.State()
				if fc, ok := st["frame_count"].(uint64); ok {
					ev.FirstNonUniformFrame = int(fc)
				}
			}
		}
	}
}

func isUniformFrame(frame []byte) bool {
	if len(frame) < 3 {
		return true
	}
	r, g, b := frame[0], frame[1], frame[2]
	for i := 3; i+2 < len(frame); i += 3 {
		if frame[i] != r || frame[i+1] != g || frame[i+2] != b {
			return false
		}
	}
	return true
}
