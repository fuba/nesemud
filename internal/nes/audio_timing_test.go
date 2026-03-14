package nes

import "testing"

func TestStepFrameCapturesMidFrameAPURegisterChanges(t *testing.T) {
	c := NewConsole()
	program := make([]byte, 0, 512)
	for i := 0; i < 200; i++ {
		program = append(program, 0xEA) // NOP
	}
	program = append(program,
		0xA9, 0x01, // LDA #$01
		0x8D, 0x15, 0x40, // STA $4015
		0xA9, 0x9F, // LDA #$9F
		0x8D, 0x00, 0x40, // STA $4000
		0xA9, 0x20, // LDA #$20
		0x8D, 0x02, 0x40, // STA $4002
		0xA9, 0x00, // LDA #$00
		0x8D, 0x03, 0x40, // STA $4003
	)
	c.cart = buildTestCartridge(program)
	c.cpu.Reset(c)

	c.StepFrame()

	firstNonZero := -1
	for i := 0; i < len(c.audioSamples); i += 2 {
		if c.audioSamples[i] != 0 {
			firstNonZero = i / 2
			break
		}
	}
	if firstNonZero == -1 {
		t.Fatalf("expected audio to become non-zero after APU writes")
	}
	if firstNonZero == 0 {
		t.Fatalf("expected initial samples to remain silent before mid-frame APU writes")
	}
}

func TestAPUStepCyclesAdvancesPulseSequencer(t *testing.T) {
	a := newAPU()
	a.WriteRegister(0x4015, 0x01)
	a.WriteRegister(0x4000, 0x90)
	a.WriteRegister(0x4002, 0x08)
	a.WriteRegister(0x4003, 0x00)

	before := a.pulse1.sequencePos
	a.StepCycles(int((a.pulse1.timer+1)*2), nil)
	if a.pulse1.sequencePos == before {
		t.Fatalf("expected pulse sequencer to advance after enough CPU cycles")
	}
}
