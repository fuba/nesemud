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

func TestWrite6000WithoutPRGRAMDoesNotReprogramMMC1(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 4*16*1024)
	for b := 0; b < 4; b++ {
		for i := 0; i < 16*1024; i++ {
			prg[b*16*1024+i] = byte(0x20 + b)
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

	for i := 0; i < 5; i++ {
		c.writeCPU(0x6000, 0x01)
	}

	if got := c.readCPU(0x8000); got != 0x20 {
		t.Fatalf("0x6000 writes unexpectedly changed MMC1 PRG bank: got 0x%02X want 0x20", got)
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

func TestMapper4PRGRAMRequiresEnableBit(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:              make([]byte, 4*16*1024),
		CHR:              make([]byte, 8*1024),
		PRGRAM:           make([]byte, 8*1024),
		Mapper:           4,
		PRGBanks:         4,
		CHRBanks:         1,
		mirroring:        MirroringHorizontal,
		mmc3PRGRAMEnable: false,
		mmc3PRGWriteDeny: false,
	}

	c.writeCPU(0x6000, 0x77)
	if got := c.readCPU(0x6000); got != 0x00 {
		t.Fatalf("disabled mapper4 PRG-RAM read = 0x%02X, want 0x00", got)
	}

	c.writeCPU(0xA001, 0x80)
	c.writeCPU(0x6000, 0x77)
	if got := c.readCPU(0x6000); got != 0x77 {
		t.Fatalf("enabled mapper4 PRG-RAM read = 0x%02X, want 0x77", got)
	}
}

func TestMapper4PRGRAMWriteProtectBitBlocksWrites(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:              make([]byte, 4*16*1024),
		CHR:              make([]byte, 8*1024),
		PRGRAM:           make([]byte, 8*1024),
		Mapper:           4,
		PRGBanks:         4,
		CHRBanks:         1,
		mirroring:        MirroringHorizontal,
		mmc3PRGRAMEnable: true,
		mmc3PRGWriteDeny: false,
	}

	c.writeCPU(0x6000, 0x11)
	if got := c.readCPU(0x6000); got != 0x11 {
		t.Fatalf("baseline mapper4 PRG-RAM read = 0x%02X, want 0x11", got)
	}

	c.writeCPU(0xA001, 0xC0)
	c.writeCPU(0x6000, 0x22)
	if got := c.readCPU(0x6000); got != 0x11 {
		t.Fatalf("write-protected mapper4 PRG-RAM read = 0x%02X, want 0x11", got)
	}
}
