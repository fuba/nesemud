package nes

import "testing"

func TestMapper2PRGBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 2*16*1024)
	for i := 0; i < 16*1024; i++ {
		prg[i] = 0x11
		prg[16*1024+i] = 0x22
	}
	// Reset vector in last bank -> $8000
	prg[16*1024+0x3FFC] = 0x00
	prg[16*1024+0x3FFD] = 0x80

	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   2,
		PRGBanks: 2,
	}

	if got := c.readCPU(0x8000); got != 0x11 {
		t.Fatalf("initial bank read = 0x%02X, want 0x11", got)
	}
	if got := c.readCPU(0xC000); got != 0x22 {
		t.Fatalf("fixed bank read = 0x%02X, want 0x22", got)
	}

	c.writeCPU(0x8000, 0x01)
	if got := c.readCPU(0x8000); got != 0x22 {
		t.Fatalf("switched bank read = 0x%02X, want 0x22", got)
	}
	if got := c.readCPU(0xC000); got != 0x22 {
		t.Fatalf("fixed bank after switch = 0x%02X, want 0x22", got)
	}
}
