package nes

import "testing"

func TestAPUTriangleGeneratesAudio(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x04) // enable triangle
	c.writeCPU(0x4008, 0x81)
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

func TestAPUTriangleWaveSequenceRisesAfterMidpoint(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4015, 0x04)
	a.WriteRegister(0x4008, 0x81)
	a.WriteRegister(0x400A, 0x02)
	a.WriteRegister(0x400B, 0x00)
	a.triangle1.linearCount = a.triangle1.linearReload

	levels := make([]float64, 0, 32)
	for i := 0; i < 32; i++ {
		levels = append(levels, a.sampleTriangleLevel())
		a.triangle1.sequencePos = (a.triangle1.sequencePos + 1) & 0x1F
	}

	if levels[0] <= levels[15] {
		t.Fatalf("expected first half to descend: first=%v mid=%v", levels[0], levels[15])
	}
	if levels[16] >= levels[31] {
		t.Fatalf("expected second half to rise: second-half start=%v end=%v", levels[16], levels[31])
	}
	if levels[15] != 0 || levels[16] != 0 {
		t.Fatalf("expected midpoint valley at zero, got %v and %v", levels[15], levels[16])
	}
}
