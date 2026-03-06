package nes

import "testing"

func TestAPUNoiseGeneratesAudio(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x08) // enable noise
	c.writeCPU(0x400C, 0x0F) // volume 15
	c.writeCPU(0x400E, 0x00) // period index 0
	c.writeCPU(0x400F, 0x00)

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	nonZero := false
	for _, s := range samples {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero noise waveform")
	}
	if c.readCPU(0x4015)&0x08 == 0 {
		t.Fatalf("expected noise enabled in status")
	}
}

func TestAPUDMCReadsMemoryAndOutputs(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 32*1024)
	// DMC sample starts at 0xC000, which is PRG offset 0x4000.
	prg[0x4000] = 0xFF
	prg[0x4001] = 0x00
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 2}

	c.writeCPU(0x4010, 0x00)
	c.writeCPU(0x4011, 0x20)
	c.writeCPU(0x4012, 0x00)
	c.writeCPU(0x4013, 0x01)
	c.writeCPU(0x4015, 0x10)

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	nonZero := false
	for _, s := range samples {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero dmc waveform")
	}
	if c.readCPU(0x4015)&0x10 == 0 {
		t.Fatalf("expected dmc enabled in status")
	}
}
