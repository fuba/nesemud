package nes

import "testing"

func TestIRQDelayedOneInstructionAfterCLI(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 16*1024)
	// 0x8000: CLI; LDA #$01; NOP
	prg[0x0000] = 0x58
	prg[0x0001] = 0xA9
	prg[0x0002] = 0x01
	prg[0x0003] = 0xEA
	// IRQ vector -> 0x9000
	prg[0x3FFE] = 0x00
	prg[0x3FFF] = 0x90
	// Reset vector -> 0x8000
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}
	c.cpu.Reset(c)
	c.apu.frameIRQ = true

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("step1 failed: %v", err)
	}
	if got, want := c.cpu.PC, uint16(0x8001); got != want {
		t.Fatalf("PC after CLI = 0x%04X, want 0x%04X", got, want)
	}
	if c.cpu.P&flagI != 0 {
		t.Fatalf("I flag should be cleared by CLI")
	}

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("step2 failed: %v", err)
	}
	if got, want := c.cpu.PC, uint16(0x9000); got != want {
		t.Fatalf("PC after second instruction/IRQ = 0x%04X, want 0x%04X", got, want)
	}
}

func TestIRQNotDelayedWhenInterruptsAlreadyEnabled(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 16*1024)
	// 0x8000: NOP
	prg[0x0000] = 0xEA
	// IRQ vector -> 0x9000
	prg[0x3FFE] = 0x00
	prg[0x3FFF] = 0x90
	// Reset vector -> 0x8000
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}
	c.cpu.Reset(c)
	c.cpu.P &^= flagI // IRQ enabled before instruction starts
	c.apu.frameIRQ = true

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("step failed: %v", err)
	}
	if got, want := c.cpu.PC, uint16(0x9000); got != want {
		t.Fatalf("PC after NOP/IRQ = 0x%04X, want 0x%04X", got, want)
	}
}

func TestIRQCanTriggerImmediatelyAfterSEIWhenAlreadyPending(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 16*1024)
	// 0x8000: SEI
	prg[0x0000] = 0x78
	// IRQ vector -> 0x9000
	prg[0x3FFE] = 0x00
	prg[0x3FFF] = 0x90
	// Reset vector -> 0x8000
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}
	c.cpu.Reset(c)
	c.cpu.P &^= flagI // start with IRQ enabled
	c.apu.frameIRQ = true

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("step failed: %v", err)
	}
	if got, want := c.cpu.PC, uint16(0x9000); got != want {
		t.Fatalf("PC after SEI/IRQ = 0x%04X, want 0x%04X", got, want)
	}
	if c.cpu.P&flagI == 0 {
		t.Fatalf("I flag should be set after IRQ entry")
	}
}
