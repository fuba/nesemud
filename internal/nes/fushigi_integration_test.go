package nes

import (
	"os"
	"testing"
)

func TestFushigiBlobbyBottomBandStaysStableAfterStart(t *testing.T) {
	const romPath = "../../dont_upload_roms/Fushigi na Blobby - Blobania no Kiki (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("fushigi rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load rom: %v", err)
	}

	const startPressFrame = 1250
	const totalFrames = 2000
	for f := 1; f <= totalFrames; f++ {
		if f == startPressFrame {
			c.SetController(1, Buttons{Start: true})
		} else {
			c.SetController(1, Buttons{})
		}
		c.StepFrame()
	}
	c.SetController(1, Buttons{})

	st := c.State()
	if paused, _ := st["paused"].(bool); paused {
		t.Fatalf("unexpected pause: %v", st["last_cpu_error"])
	}
	frame := c.SnapshotFrame()
	if isUniformRGBFrame(frame) {
		t.Fatalf("expected non-uniform frame after gameplay start")
	}

	artifactBlue := countBrightBlueInBottomBand(frame)
	if artifactBlue > 200 {
		t.Fatalf("detected bottom-band blue artifact noise: count=%d", artifactBlue)
	}
}

func countBrightBlueInBottomBand(frame []byte) int {
	if len(frame) < FrameSizeRGB {
		return 0
	}
	count := 0
	for y := 208; y < FrameHeight; y++ {
		for x := 0; x < FrameWidth; x++ {
			o := (y*FrameWidth + x) * 3
			r := frame[o+0]
			g := frame[o+1]
			b := frame[o+2]
			if b > 180 && r < 40 && g < 80 {
				count++
			}
		}
	}
	return count
}

func isUniformRGBFrame(frame []byte) bool {
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
