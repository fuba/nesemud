package nes

import (
	"os"
	"testing"
)

func TestGradiusLongRunProducesAudioAndVideo(t *testing.T) {
	const romPath = "../../dont_upload_roms/Gradius (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("gradius rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load gradius rom: %v", err)
	}

	for i := 0; i < 360; i++ {
		c.StepFrame()
	}

	if c.paused {
		t.Fatalf("expected gradius long run to remain active")
	}

	frame := c.SnapshotFrame()
	if isUniformFrame(frame) {
		t.Fatalf("expected rendered gradius frame to contain visible detail")
	}

	st := c.State()
	apuState, ok := st["apu"].(map[string]any)
	if !ok {
		t.Fatalf("expected apu state to be present")
	}
	if writes, _ := apuState["write_count_4015"].(uint64); writes == 0 {
		t.Fatalf("expected gradius to touch APU enable register")
	}
	if writes, _ := apuState["write_count_4017"].(uint64); writes == 0 {
		t.Fatalf("expected gradius to configure APU frame counter")
	}
}
