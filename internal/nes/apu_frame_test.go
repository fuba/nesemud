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
