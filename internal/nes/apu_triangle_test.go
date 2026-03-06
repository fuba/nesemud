package nes

import "testing"

func TestAPUTriangleGeneratesAudio(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x04) // enable triangle
	c.writeCPU(0x4008, 0x80)
	c.writeCPU(0x400A, 0x20)
	c.writeCPU(0x400B, 0x00)

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	nonZero := false
	for _, s := range samples {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero triangle waveform")
	}
	if c.readCPU(0x4015)&0x04 == 0 {
		t.Fatalf("expected triangle enabled in status")
	}
}
