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
