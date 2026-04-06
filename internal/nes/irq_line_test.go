package nes

import "testing"

func TestStepInstructionServicesAPUIRQ(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 2*16*1024)
	// NOP at reset vector target.
	prg[0x0000] = 0xEA
	// IRQ vector -> $9000 (index 0x7FFE in 32KiB PRG window).
	prg[0x7FFE] = 0x00
	prg[0x7FFF] = 0x90
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 2,
		CHRBanks: 1,
	}
	c.cpu.PC = 0x8000
	c.cpu.setFlag(flagI, false)
	c.apu.frameIRQ = true

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("StepInstruction returned error: %v", err)
	}
	if got := c.cpu.PC; got != 0x9000 {
		t.Fatalf("cpu PC after APU IRQ = 0x%04X, want 0x9000", got)
	}
	if !c.apu.frameIRQ {
		t.Fatalf("frame IRQ should remain pending until status read")
	}
}

func TestStepInstructionDoesNotConsumeMapperIRQWhenDisabled(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 2*16*1024)
	prg[0x0000] = 0xEA
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   4,
		PRGBanks: 2,
		CHRBanks: 1,
	}
	c.cpu.PC = 0x8000
	c.cpu.setFlag(flagI, true)
	c.cart.mmc3IRQPending = true

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("StepInstruction returned error: %v", err)
	}
	if !c.cart.mmc3IRQPending {
		t.Fatalf("mapper IRQ should remain pending while I flag is set")
	}
}

