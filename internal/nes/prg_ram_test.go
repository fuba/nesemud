package nes

import "testing"

func TestPRGRAMReadWriteForMapper1(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:       make([]byte, 2*16*1024),
		CHR:       make([]byte, 8*1024),
		PRGRAM:    make([]byte, 8*1024),
		Mapper:    1,
		PRGBanks:  2,
		CHRBanks:  1,
		mirroring: MirroringHorizontal,
	}

	c.writeCPU(0x6000, 0x12)
	c.writeCPU(0x7FFF, 0x34)

	if got := c.readCPU(0x6000); got != 0x12 {
		t.Fatalf("read 0x6000 = 0x%02X, want 0x12", got)
	}
	if got := c.readCPU(0x7FFF); got != 0x34 {
		t.Fatalf("read 0x7FFF = 0x%02X, want 0x34", got)
	}
}

func TestFourScreenMirroringKeepsNametablesDistinct(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:       make([]byte, 16*1024),
		CHR:       make([]byte, 8*1024),
		Mapper:    4,
		PRGBanks:  1,
		CHRBanks:  1,
		mirroring: MirroringFourScreen,
	}

	c.ppu.ppuWrite(c, 0x2000, 0x11)
	c.ppu.ppuWrite(c, 0x2400, 0x22)
	c.ppu.ppuWrite(c, 0x2800, 0x33)
	c.ppu.ppuWrite(c, 0x2C00, 0x44)

	if got := c.ppu.ppuRead(c, 0x2000); got != 0x11 {
		t.Fatalf("table 0 read = 0x%02X, want 0x11", got)
	}
	if got := c.ppu.ppuRead(c, 0x2400); got != 0x22 {
		t.Fatalf("table 1 read = 0x%02X, want 0x22", got)
	}
	if got := c.ppu.ppuRead(c, 0x2800); got != 0x33 {
		t.Fatalf("table 2 read = 0x%02X, want 0x33", got)
	}
	if got := c.ppu.ppuRead(c, 0x2C00); got != 0x44 {
		t.Fatalf("table 3 read = 0x%02X, want 0x44", got)
	}
}
