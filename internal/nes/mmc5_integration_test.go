package nes

import (
	"os"
	"testing"
)

func TestMMC5JustBreedSmoke(t *testing.T) {
	const romPath = "../../dont_upload_roms/Just Breed (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("mmc5 smoke rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load mmc5 rom: %v", err)
	}

	for i := 0; i < 120; i++ {
		c.StepFrame()
	}

	st := c.State()
	if romLoaded, _ := st["rom_loaded"].(bool); !romLoaded {
		t.Fatalf("expected rom_loaded after smoke run")
	}
	if paused, _ := st["paused"].(bool); paused {
		t.Fatalf("expected mmc5 smoke run to stay unpaused")
	}
	if frameCount, _ := st["frame_count"].(uint64); frameCount == 0 {
		t.Fatalf("expected frame_count to advance")
	}
}

func TestMMC5JustBreedLongRunProducesAudioAndVideo(t *testing.T) {
	const romPath = "../../dont_upload_roms/Just Breed (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("mmc5 validation rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load mmc5 rom: %v", err)
	}

	for i := 0; i < 600; i++ {
		c.StepFrame()
	}

	if c.paused {
		t.Fatalf("expected mmc5 long run to remain active")
	}

	frame := c.SnapshotFrame()
	if isUniformFrame(frame) {
		t.Fatalf("expected non-uniform rendered frame after long run")
	}

	st := c.State()
	apuState, ok := st["apu"].(map[string]any)
	if !ok {
		t.Fatalf("expected apu state to be present")
	}
	if writes, _ := apuState["write_count_4015"].(uint64); writes == 0 {
		t.Fatalf("expected mmc5 title to touch APU enable register")
	}
	if writes, _ := apuState["write_count_4017"].(uint64); writes == 0 {
		t.Fatalf("expected mmc5 title to configure APU frame counter")
	}
}

func isUniformFrame(frame []byte) bool {
	if len(frame) < 3 {
		return true
	}
	base0, base1, base2 := frame[0], frame[1], frame[2]
	for i := 3; i+2 < len(frame); i += 3 {
		if frame[i] != base0 || frame[i+1] != base1 || frame[i+2] != base2 {
			return false
		}
	}
	return true
}
