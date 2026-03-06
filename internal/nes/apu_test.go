package nes

import "testing"

func TestAPUPulseGeneratesAudio(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x01) // enable pulse1
	c.writeCPU(0x4000, 0x9F) // duty=2, constant volume=15
	c.writeCPU(0x4002, 0x20)
	c.writeCPU(0x4003, 0x00)

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	if len(samples) == 0 {
		t.Fatalf("expected non-empty samples")
	}
	nonZero := false
	for _, s := range samples {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero waveform")
	}
	if c.readCPU(0x4015)&0x01 == 0 {
		t.Fatalf("expected pulse1 enabled in status")
	}
}

func TestAPUDisablePulseSilencesAudio(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x01)
	c.writeCPU(0x4000, 0x9F)
	c.writeCPU(0x4002, 0x10)
	c.writeCPU(0x4003, 0x00)
	c.writeCPU(0x4015, 0x00) // disable all

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	for _, s := range samples {
		if s != 0 {
			t.Fatalf("expected silence when disabled, got %d", s)
		}
	}
}
