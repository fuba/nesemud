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

func TestAPUDMCLoopRestartsSample(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 32*1024)
	prg[0x4000] = 0xFF
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 2}

	c.writeCPU(0x4010, 0x40) // loop
	c.writeCPU(0x4012, 0x00)
	c.writeCPU(0x4013, 0x00) // one byte
	c.writeCPU(0x4015, 0x10)

	initialRestarts := c.apu.dmc.restartCount
	for i := 0; i < 10000; i++ {
		c.apu.StepCycles(1, c.readCPU)
		if c.apu.dmc.restartCount > initialRestarts+1 {
			break
		}
	}
	if c.apu.dmc.restartCount <= initialRestarts+1 {
		t.Fatalf("expected looped dmc sample to restart")
	}
}

func TestAPUDMCRaisesIRQWhenSampleEnds(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 32*1024)
	prg[0x4000] = 0x00
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 2}

	c.writeCPU(0x4010, 0x80) // irq enable
	c.writeCPU(0x4012, 0x00)
	c.writeCPU(0x4013, 0x00) // one byte
	c.writeCPU(0x4015, 0x10)

	for i := 0; i < 5000; i++ {
		c.apu.StepCycles(1, c.readCPU)
		if c.apu.dmc.irqPending {
			break
		}
	}
	if !c.apu.dmc.irqPending {
		t.Fatalf("expected dmc irq pending after sample end")
	}
	if got := c.apu.PeekStatus(); got&0x80 == 0 {
		t.Fatalf("expected dmc irq bit in status, got 0x%02X", got)
	}
}

func TestAPUDMCMemoryFetchRequestsCPUStall(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 32*1024)
	prg[0x4000] = 0xAA
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 2}

	c.writeCPU(0x4010, 0x00)
	c.writeCPU(0x4012, 0x00)
	c.writeCPU(0x4013, 0x00)
	c.writeCPU(0x4015, 0x10)

	for i := 0; i < 1000; i++ {
		c.apu.StepCycles(1, c.readCPU)
		if c.apu.pendingCPUStallCycles > 0 {
			break
		}
	}
	if c.apu.pendingCPUStallCycles == 0 {
		t.Fatalf("expected dmc fetch to request cpu stall")
	}
	if got := c.apu.consumeCPUStallCycles(); got != 4 {
		t.Fatalf("expected one dmc fetch stall of 4 cycles, got %d", got)
	}
}

func TestAPUDMCAddressWrapsFromFFFFTo8000(t *testing.T) {
	a := newAPU()
	a.dmc.currentAddr = 0xFFFF
	a.dmc.bytesRemain = 1

	a.serviceDMCReader(func(addr uint16) byte {
		if addr != 0xFFFF {
			t.Fatalf("read addr = 0x%04X, want 0xFFFF", addr)
		}
		return 0x00
	})

	if a.dmc.currentAddr != 0x8000 {
		t.Fatalf("expected dmc address to wrap to 0x8000, got 0x%04X", a.dmc.currentAddr)
	}
}

func TestAPUDMCSampleBufferLoadsBeforeOutputChanges(t *testing.T) {
	a := newAPU()
	a.dmc.currentAddr = 0xC000
	a.dmc.bytesRemain = 1
	a.dmc.outputLevel = 0x20
	a.dmc.bitsRemain = 1
	a.dmc.silence = true
	a.dmc.bufferEmpty = true

	a.serviceDMCReader(func(addr uint16) byte {
		if addr != 0xC000 {
			t.Fatalf("read addr = 0x%04X, want 0xC000", addr)
		}
		return 0xFF
	})
	if a.dmc.outputLevel != 0x20 {
		t.Fatalf("fetch should not immediately change output level, got 0x%02X", a.dmc.outputLevel)
	}

	a.tickDMC(nil)
	if a.dmc.outputLevel != 0x20 {
		t.Fatalf("loading shifter at cycle boundary should not yet change output level, got 0x%02X", a.dmc.outputLevel)
	}

	a.tickDMC(nil)
	if a.dmc.outputLevel != 0x22 {
		t.Fatalf("expected first shifted 1-bit to raise output level, got 0x%02X", a.dmc.outputLevel)
	}
}

func TestAPUDMCDisableStillClocksCurrentOutputCycle(t *testing.T) {
	a := newAPU()
	a.dmc.outputLevel = 0x20
	a.dmc.shiftReg = 0x01
	a.dmc.bitsRemain = 1
	a.dmc.silence = false
	a.dmc.enabled = false

	a.tickDMC(nil)

	if a.dmc.outputLevel != 0x22 {
		t.Fatalf("expected current output cycle to continue after disable, got 0x%02X", a.dmc.outputLevel)
	}
}

func TestAPUDMCRateTableMatchesNTSCPeriod(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4010, 0x0D)
	if a.dmc.timerValue != 84 {
		t.Fatalf("dmc timerValue for rate index 13 = %d, want 84", a.dmc.timerValue)
	}
}

func TestAPUNoisePeriodWriteDoesNotImmediatelyResetDivider(t *testing.T) {
	a := newAPU()
	a.noise1.timerValue = 7
	a.WriteRegister(0x400E, 0x0F)
	if a.noise1.timerValue != 7 {
		t.Fatalf("noise timerValue after 400E write = %d, want unchanged 7", a.noise1.timerValue)
	}
	if a.noise1.periodIdx != 0x0F {
		t.Fatalf("noise periodIdx = %d, want 15", a.noise1.periodIdx)
	}
}

func TestAPUDMCOutputLevelContributesEvenWhenChannelDisabled(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4011, 0x20)

	if got := a.sampleDMCLevel(); got != 0x20 {
		t.Fatalf("sampleDMCLevel = %v, want 32", got)
	}
}

func TestAPUDMCDisableStopsFetchButKeepsDACLevel(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4011, 0x30)
	a.WriteRegister(0x4015, 0x00)

	if got := a.sampleDMCLevel(); got != 0x30 {
		t.Fatalf("sampleDMCLevel after disable = %v, want 48", got)
	}
}
