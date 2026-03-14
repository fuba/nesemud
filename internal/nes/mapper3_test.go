package nes

import "testing"

func TestMapper3CHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 16*1024)
	prg[0] = 0x01
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	chr := make([]byte, 2*8*1024)
	chr[0] = 0x11
	chr[8*1024] = 0x77

	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   3,
		PRGBanks: 1,
		CHRBanks: 2,
	}

	if got := c.ppu.ppuRead(c, 0x0000); got != 0x11 {
		t.Fatalf("initial chr read=0x%02X, want 0x11", got)
	}
	c.writeCPU(0x8000, 0x01)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x77 {
		t.Fatalf("switched chr read=0x%02X, want 0x77", got)
	}
}
