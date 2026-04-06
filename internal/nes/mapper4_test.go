package nes

import "testing"

func TestMapper4PRGBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	for b := 0; b < 8; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x20 + b)
		}
	}
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   4,
		PRGBanks: 4,
		CHRBanks: 1,
	}

	c.writeCPU(0x8000, 0x06)
	c.writeCPU(0x8001, 0x03)
	if got := c.readCPU(0x8000); got != 0x23 {
		t.Fatalf("mapper4 bank6 read=0x%02X, want 0x23", got)
	}
}

func TestMapper4IRQCounter(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 4}
	c.cart.mmc3Write(0xC000, 0x02)
	c.cart.mmc3Write(0xC001, 0x00)
	c.cart.mmc3Write(0xE001, 0x00)

	c.cart.mmc3ClockIRQ()
	c.cart.mmc3ClockIRQ()
	c.cart.mmc3ClockIRQ()
	if !c.cart.consumeIRQ() {
		t.Fatalf("expected IRQ pending")
	}
}

func TestMapper4IRQNotClockedWhenRenderingDisabled(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 4}
	c.cart.mmc3Write(0xC000, 0x00)
	c.cart.mmc3Write(0xC001, 0x00)
	c.cart.mmc3Write(0xE001, 0x00)

	// Run far enough to cross cycle 260 several times with rendering disabled.
	for i := 0; i < 10; i++ {
		c.ppu.step(c, 114)
	}
	if c.cart.consumeIRQ() {
		t.Fatalf("did not expect IRQ while rendering is disabled")
	}
}

func TestMapper4IRQClockedWhenRenderingEnabled(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 4}
	c.cart.mmc3Write(0xC000, 0x00)
	c.cart.mmc3Write(0xC001, 0x00)
	c.cart.mmc3Write(0xE001, 0x00)

	c.ppu.mask = 0x18 // enable background + sprites
	c.ppu.ctrl = 0x10 // background pattern table at $1000 (A12 high fetches)
	c.ppu.step(c, 114)

	if !c.cart.consumeIRQ() {
		t.Fatalf("expected IRQ when rendering is enabled")
	}
}

func TestMapper4IRQNotClockedWhenPatternFetchesStayBelow1000(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 4}
	c.cart.mmc3Write(0xC000, 0x00)
	c.cart.mmc3Write(0xC001, 0x00)
	c.cart.mmc3Write(0xE001, 0x00)

	c.ppu.mask = 0x18 // rendering enabled
	c.ppu.ctrl = 0x00 // bg/sprite pattern tables at $0000 in 8x8 mode
	c.ppu.step(c, 114)

	if c.cart.consumeIRQ() {
		t.Fatalf("did not expect IRQ without A12-high pattern fetches")
	}
}

func TestMapper4PRGBankSelectUsesLower6Bits(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 10*8*1024)
	for b := 0; b < 10; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x40 + b)
		}
	}
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   4,
		PRGBanks: 5,
		CHRBanks: 1,
	}

	c.writeCPU(0x8000, 0x06)
	c.writeCPU(0x8001, 0xC3) // upper bits should be ignored for PRG bank select

	if got := c.readCPU(0x8000); got != 0x43 {
		t.Fatalf("mapper4 masked bank6 read=0x%02X, want 0x43", got)
	}
}
