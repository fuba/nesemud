package nes

import "testing"

func TestMapper1PRGBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 4*16*1024)
	for b := 0; b < 4; b++ {
		for i := 0; i < 16*1024; i++ {
			prg[b*16*1024+i] = byte(0x10 + b)
		}
	}
	c.cart = &Cartridge{
		PRG:       prg,
		CHR:       make([]byte, 8*1024),
		Mapper:    1,
		PRGBanks:  4,
		CHRBanks:  1,
		mirroring: MirroringHorizontal,
	}
	c.cart.mmc1Reset()

	writeMMC1Reg(c, 0xE000, 0x01) // select bank 1 at 0x8000 (mode 3)
	if got := c.readCPU(0x8000); got != 0x11 {
		t.Fatalf("0x8000 bank read = 0x%02X, want 0x11", got)
	}
	if got := c.readCPU(0xC000); got != 0x13 {
		t.Fatalf("0xC000 fixed bank read = 0x%02X, want 0x13", got)
	}
}

func TestMirroringModes(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 1, mirroring: MirroringHorizontal}
	c.cart.mmc1Reset()

	// Horizontal: 0x2000 and 0x2400 should mirror.
	c.ppu.ppuWrite(c, 0x2000, 0xAA)
	if got := c.ppu.ppuRead(c, 0x2400); got != 0xAA {
		t.Fatalf("horizontal mirror read = 0x%02X, want 0xAA", got)
	}

	// Switch to vertical through MMC1 control reg.
	writeMMC1Reg(c, 0x8000, 0x02)
	c.ppu.ppuWrite(c, 0x2000, 0x55)
	if got := c.ppu.ppuRead(c, 0x2800); got != 0x55 {
		t.Fatalf("vertical mirror read = 0x%02X, want 0x55", got)
	}
}

func writeMMC1Reg(c *Console, addr uint16, value byte) {
	for i := 0; i < 5; i++ {
		bit := (value >> i) & 1
		c.writeCPU(addr, bit)
	}
}
