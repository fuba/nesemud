package nes

import "testing"

func TestAPUEnvelopeDecay(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4015, 0x01)
	a.WriteRegister(0x4000, 0x00) // envelope mode, period=0
	a.WriteRegister(0x4002, 0x20)
	a.WriteRegister(0x4003, 0x00)

	for i := 0; i < 8; i++ {
		a.tickFrameSequencer()
	}
	if a.pulse1.env.decay >= 15 {
		t.Fatalf("expected envelope to decay, got %d", a.pulse1.env.decay)
	}
}

func TestAPULengthCounterStopsPulse(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4015, 0x01)
	a.WriteRegister(0x4000, 0x10) // constant volume
	a.WriteRegister(0x4002, 0x20)
	a.WriteRegister(0x4003, 0xF8) // short length index

	if a.pulse1.lengthCount == 0 {
		t.Fatalf("expected length loaded")
	}
	for i := 0; i < 80; i++ {
		a.tickFrameSequencer()
	}
	if a.pulse1.lengthCount != 0 {
		t.Fatalf("expected length to reach zero, got %d", a.pulse1.lengthCount)
	}
}

func TestAPUFrameCounterFiveStepWriteAppliesAfterShortDelay(t *testing.T) {
	a := newAPU()
	a.pulse1.enabled = true
	a.pulse1.lengthCount = 2

	a.WriteRegister(0x4017, 0x80)
	if a.pulse1.lengthCount != 2 {
		t.Fatalf("expected no immediate half-frame clock on 4017 write")
	}

	a.StepCycles(1, nil)
	if a.pulse1.lengthCount != 2 {
		t.Fatalf("expected 4017 write to remain delayed for at least 1 cycle")
	}
	a.StepCycles(1, nil)
	if a.pulse1.lengthCount >= 2 {
		t.Fatalf("expected delayed 5-step write to clock half-frame within 2 cycles")
	}
}

func TestAPUFrameCounterWriteDelayDependsOnCPUParity(t *testing.T) {
	a := newAPU()
	a.pulse1.enabled = true
	a.pulse1.lengthCount = 2

	a.cpuCycleParity = false
	a.WriteRegister(0x4017, 0x80)
	a.StepCycles(1, nil)
	if a.pulse1.lengthCount != 2 {
		t.Fatalf("expected 4017 write on even parity to stay pending for first cycle")
	}
	a.StepCycles(1, nil)
	if a.pulse1.lengthCount >= 2 {
		t.Fatalf("expected 4017 write on even parity to apply after 2 cycles")
	}

	a = newAPU()
	a.pulse1.enabled = true
	a.pulse1.lengthCount = 2
	a.cpuCycleParity = true
	a.WriteRegister(0x4017, 0x80)
	a.StepCycles(2, nil)
	if a.pulse1.lengthCount != 2 {
		t.Fatalf("expected 4017 write on odd parity to stay pending for first 2 cycles")
	}
	a.StepCycles(1, nil)
	if a.pulse1.lengthCount >= 2 {
		t.Fatalf("expected 4017 write on odd parity to apply after 3 cycles")
	}
}

func TestAPUFrameCounterWriteClearsIRQAndResetsLater(t *testing.T) {
	a := newAPU()
	a.frameIRQ = true
	a.frameCounterCycle = 100

	a.WriteRegister(0x4017, 0x40)
	if a.frameIRQ {
		t.Fatalf("expected 4017 write with IRQ inhibit to clear frame IRQ immediately")
	}
	if a.frameCounterCycle != 100 {
		t.Fatalf("expected frame counter reset to be delayed")
	}

	a.StepCycles(3, nil)
	if a.frameCounterCycle > 1 {
		t.Fatalf("expected frame counter to reset within 3 cycles, got %d", a.frameCounterCycle)
	}
}
