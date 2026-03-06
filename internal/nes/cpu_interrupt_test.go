package nes

import "testing"

func TestCPUNMIJumpToVector(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 16*1024)
	// Reset vector -> 0x8000, NMI vector -> 0x9000
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	prg[0x3FFA] = 0x00
	prg[0x3FFB] = 0x90
	c.cart = &Cartridge{PRG: prg, CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}
	c.cpu.Reset(c)
	c.cpu.PC = 0x8123

	c.cpu.NMI(c)
	if c.cpu.PC != 0x9000 {
		t.Fatalf("PC=0x%04X, want 0x9000", c.cpu.PC)
	}
	if c.cpu.SP != 0xF7 {
		t.Fatalf("SP=0x%02X, want 0xF7", c.cpu.SP)
	}
}
