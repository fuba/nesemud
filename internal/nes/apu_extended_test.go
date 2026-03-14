package nes

import "testing"

func TestAPUPulse2GeneratesAudio(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x02) // enable pulse2
	c.writeCPU(0x4004, 0x9F) // duty=2, constant volume=15
	c.writeCPU(0x4006, 0x20)
	c.writeCPU(0x4007, 0x00)

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	nonZero := false
	for _, s := range samples {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero pulse2 waveform")
	}
	if c.readCPU(0x4015)&0x02 == 0 {
		t.Fatalf("expected pulse2 enabled in status")
	}
}

func TestAPUTriangleLinearCounterCanSilenceChannel(t *testing.T) {
	c := NewConsole()
	c.writeCPU(0x4015, 0x04)
	c.writeCPU(0x4008, 0x00) // linear reload=0, control clear
	c.writeCPU(0x400A, 0x20)
	c.writeCPU(0x400B, 0x00)

	samples := c.apu.GenerateFrameSamples(AudioRate/TargetFPS, c.readCPU)
	for _, s := range samples {
		if s != 0 {
			t.Fatalf("expected triangle silence when linear counter is zero")
		}
	}
}

func TestAPUSweepCanChangePulseTimer(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4015, 0x01)
	a.WriteRegister(0x4000, 0x10)
	a.WriteRegister(0x4001, 0x89) // enable sweep, period=0, shift=1
	a.WriteRegister(0x4002, 0x40)
	a.WriteRegister(0x4003, 0x00)

	before := a.pulse1.timer
	for i := 0; i < 4; i++ {
		a.tickFrameSequencer()
	}
	if a.pulse1.timer == before {
		t.Fatalf("expected sweep to change pulse timer")
	}
}

func TestAPUFrameCounterSetsIRQInFourStepMode(t *testing.T) {
	a := newAPU()
	a.StepCycles(14915, nil)
	if !a.frameIRQ {
		t.Fatalf("expected frame IRQ after 4-step sequence")
	}
	if got := a.ReadStatus(); got&0x40 == 0 {
		t.Fatalf("expected frame IRQ bit in status, got 0x%02X", got)
	}
	if a.frameIRQ {
		t.Fatalf("expected ReadStatus to clear frame IRQ latch")
	}
}

func TestAPUFrameCounterFiveStepSuppressesIRQ(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4017, 0x80)
	a.StepCycles(20000, nil)
	if a.frameIRQ {
		t.Fatalf("did not expect frame IRQ in 5-step mode")
	}
}

func TestAPUPeekStatusDoesNotClearFrameIRQ(t *testing.T) {
	a := newAPU()
	a.StepCycles(14915, nil)
	if got := a.PeekStatus(); got&0x40 == 0 {
		t.Fatalf("expected frame IRQ bit in peeked status, got 0x%02X", got)
	}
	if !a.frameIRQ {
		t.Fatalf("peek should not clear frame IRQ")
	}
}

func TestAPULengthCounterDoesNotLoadWhileChannelDisabled(t *testing.T) {
	a := newAPU()

	a.WriteRegister(0x4003, 0x08)
	if a.pulse1.lengthCount != 0 {
		t.Fatalf("expected pulse1 length to stay zero while disabled, got %d", a.pulse1.lengthCount)
	}

	a.WriteRegister(0x4007, 0x08)
	if a.pulse2.lengthCount != 0 {
		t.Fatalf("expected pulse2 length to stay zero while disabled, got %d", a.pulse2.lengthCount)
	}

	a.WriteRegister(0x400B, 0x08)
	if a.triangle1.lengthCount != 0 {
		t.Fatalf("expected triangle length to stay zero while disabled, got %d", a.triangle1.lengthCount)
	}

	a.WriteRegister(0x400F, 0x08)
	if a.noise1.lengthCount != 0 {
		t.Fatalf("expected noise length to stay zero while disabled, got %d", a.noise1.lengthCount)
	}
}

func TestAPUSweepDoesNotRewriteTimerWhenTargetOverflows(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4015, 0x01)
	a.WriteRegister(0x4000, 0x10)
	a.WriteRegister(0x4001, 0x91) // enable sweep, period=1, shift=1
	a.WriteRegister(0x4002, 0xFF)
	a.WriteRegister(0x4003, 0x07) // timer = 0x07FF

	before := a.pulse1.timer
	for i := 0; i < 4; i++ {
		a.tickFrameSequencer()
	}
	if a.pulse1.timer != before {
		t.Fatalf("expected overflowing sweep target to leave timer unchanged, got 0x%04X", a.pulse1.timer)
	}
}
