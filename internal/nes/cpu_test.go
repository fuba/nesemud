package nes

import "testing"

func TestCPUResetVectorAndBasicProgram(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0x42, // LDA #$42
		0x85, 0x10, // STA $10
		0xA2, 0x05, // LDX #$05
		0xE8,       // INX
		0x86, 0x11, // STX $11
	})
	c.cart = cart
	c.cpu.Reset(c)

	for i := 0; i < 5; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step failed at %d: %v", i, err)
		}
	}

	if got := c.readCPU(0x0010); got != 0x42 {
		t.Fatalf("ram[0x10] = 0x%02X, want 0x42", got)
	}
	if got := c.readCPU(0x0011); got != 0x06 {
		t.Fatalf("ram[0x11] = 0x%02X, want 0x06", got)
	}
}

func TestCPUBranchAndJSRRTS(t *testing.T) {
	c := NewConsole()
	program := []byte{
		0x20, 0x10, 0x80, // JSR $8010
		0xA9, 0x01, // LDA #$01
		0xD0, 0x02, // BNE +2
		0xA9, 0xFF, // LDA #$FF (skip)
		0x85, 0x20, // STA $20
		0xEA,       // NOP
		0xEA,       // NOP
		0xEA,       // NOP
		0xEA,       // NOP
		0xEA,       // NOP
		0xA2, 0x07, // $8010: LDX #$07
		0x60, // RTS
	}
	cart := buildTestCartridge(program)
	c.cart = cart
	c.cpu.Reset(c)

	for i := 0; i < 8; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step failed at %d: %v", i, err)
		}
	}

	if c.cpu.X != 0x07 {
		t.Fatalf("X = 0x%02X, want 0x07", c.cpu.X)
	}
	if got := c.readCPU(0x0020); got != 0x01 {
		t.Fatalf("ram[0x20] = 0x%02X, want 0x01", got)
	}
}

func buildTestCartridge(program []byte) *Cartridge {
	prg := make([]byte, 16*1024)
	copy(prg, program)
	// Reset vector -> $8000
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	return &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}
}
