package nes

import "testing"

func TestMapper3CHRBankSwitchHonorsBusConflicts(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 2*16*1024)
	prg[0] = 0x01
	chr := make([]byte, 4*8*1024)
	chr[0] = 0x10
	chr[8*1024] = 0x20
	chr[16*1024] = 0x30
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   3,
		PRGBanks: 2,
		CHRBanks: 4,
	}

	c.writeCPU(0x8000, 0x03)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x20 {
		t.Fatalf("mapper3 chr read=0x%02X, want 0x20 after bus-conflicted select", got)
	}
}

func TestMapper33PRGAndCHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 4*8*1024)
	for b := 0; b < 4; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x20 + b)
		}
	}
	chr := make([]byte, 8*1024)
	chr[0] = 0x11
	chr[2*1024] = 0x22
	chr[4*1024] = 0x33
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   33,
		PRGBanks: 2,
		CHRBanks: 1,
	}

	c.writeCPU(0x8000, 0x01)
	if got := c.readCPU(0x8000); got != 0x21 {
		t.Fatalf("mapper33 bank0 read=0x%02X, want 0x21", got)
	}
	c.writeCPU(0x8001, 0x00)
	if got := c.readCPU(0xA000); got != 0x20 {
		t.Fatalf("mapper33 bank1 read=0x%02X, want 0x20", got)
	}

	c.writeCPU(0x8002, 0x02)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x22 {
		t.Fatalf("mapper33 chr2k read=0x%02X, want 0x22", got)
	}
	c.writeCPU(0xA000, 0x04)
	if got := c.ppu.ppuRead(c, 0x1000); got != 0x33 {
		t.Fatalf("mapper33 chr1k read=0x%02X, want 0x33", got)
	}
}

func TestMapper66PRGAndCHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 4*32*1024)
	for b := 0; b < 4; b++ {
		for i := 0; i < 32*1024; i++ {
			prg[b*32*1024+i] = byte(0x30 + b)
		}
	}
	chr := make([]byte, 4*8*1024)
	chr[0] = 0x10
	chr[8*1024] = 0x40
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   66,
		PRGBanks: 8,
		CHRBanks: 4,
	}

	c.writeCPU(0x8000, 0x11)
	if got := c.readCPU(0x8000); got != 0x31 {
		t.Fatalf("mapper66 prg read=0x%02X, want 0x31", got)
	}
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x40 {
		t.Fatalf("mapper66 chr read=0x%02X, want 0x40", got)
	}
}

func TestMapper75PRGAndCHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 16*8*1024)
	for b := 0; b < 16; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x40 + b)
		}
	}
	chr := make([]byte, 128*1024)
	chr[0] = 0x10
	chr[17*4*1024] = 0x55
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   75,
		PRGBanks: 8,
		CHRBanks: 4,
	}

	c.writeCPU(0x8000, 0x01)
	if got := c.readCPU(0x8000); got != 0x41 {
		t.Fatalf("mapper75 bank0 read=0x%02X, want 0x41", got)
	}
	c.writeCPU(0x9000, 0x06)
	c.writeCPU(0xE000, 0x01)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x55 {
		t.Fatalf("mapper75 chr read=0x%02X, want 0x55", got)
	}
	if c.cart.mirroring != MirroringVertical {
		t.Fatalf("mapper75 mirroring=%d, want vertical", c.cart.mirroring)
	}
}

func TestMapper87CHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 2*16*1024)
	chr := make([]byte, 4*8*1024)
	chr[0] = 0x12
	chr[8*1024] = 0x78
	chr[2*8*1024] = 0x34
	chr[3*8*1024] = 0x56
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   87,
		PRGBanks: 2,
		CHRBanks: 4,
	}

	c.writeCPU(0x6000, 0x02)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x78 {
		t.Fatalf("mapper87 chr read=0x%02X, want 0x78", got)
	}
	c.writeCPU(0x6000, 0x01)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x34 {
		t.Fatalf("mapper87 chr read after bit0 write=0x%02X, want 0x34", got)
	}
	c.writeCPU(0x6000, 0x03)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x56 {
		t.Fatalf("mapper87 chr read after bit0+bit1 write=0x%02X, want 0x56", got)
	}
	c.writeCPU(0x6000, 0x00)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x12 {
		t.Fatalf("mapper87 chr read after zero write=0x%02X, want 0x12", got)
	}
}

func TestMapper88CHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	chr := make([]byte, 128*1024)
	chr[64*1024] = 0x66
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   88,
		PRGBanks: 4,
		CHRBanks: 16,
	}

	c.writeCPU(0x8000, 0x02)
	c.writeCPU(0x8001, 0x00)
	if got := c.ppu.ppuRead(c, 0x1000); got != 0x66 {
		t.Fatalf("mapper88 chr read=0x%02X, want 0x66", got)
	}
}

func TestMapper206PRGAndCHRBankSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	for b := 0; b < 8; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x50 + b)
		}
	}
	chr := make([]byte, 16*1024)
	chr[2*1024] = 0x77
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   206,
		PRGBanks: 4,
		CHRBanks: 2,
	}

	c.writeCPU(0x8000, 0x06)
	c.writeCPU(0x8001, 0x03)
	if got := c.readCPU(0x8000); got != 0x53 {
		t.Fatalf("mapper206 prg read=0x%02X, want 0x53", got)
	}
	c.writeCPU(0x8000, 0x00)
	c.writeCPU(0x8001, 0x02)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x77 {
		t.Fatalf("mapper206 chr read=0x%02X, want 0x77", got)
	}
}

func TestMapper206PRGBankSelectionUsesModuloBankCount(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 6*8*1024)
	for b := 0; b < 6; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x70 + b)
		}
	}
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   206,
		PRGBanks: 3,
		CHRBanks: 1,
	}

	c.writeCPU(0x8000, 0x06)
	c.writeCPU(0x8001, 0x05)
	if got := c.readCPU(0x8000); got != 0x75 {
		t.Fatalf("mapper206 prg modulo bank select read=0x%02X, want 0x75", got)
	}
}

func TestMapper23PRGCHRMirroringAndIRQ(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	for b := 0; b < 8; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x60 + b)
		}
	}
	chr := make([]byte, 16*1024)
	chr[3*1024] = 0x44
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   23,
		PRGBanks: 4,
		CHRBanks: 2,
	}

	c.writeCPU(0x8000, 0x02)
	if got := c.readCPU(0x8000); got != 0x62 {
		t.Fatalf("mapper23 prg read=0x%02X, want 0x62", got)
	}
	c.writeCPU(0x9000, 0x01)
	if c.cart.mirroring != MirroringHorizontal {
		t.Fatalf("mapper23 mirroring=%d, want horizontal", c.cart.mirroring)
	}
	c.writeCPU(0xB000, 0x03)
	if got := c.ppu.ppuRead(c, 0x0000); got != 0x44 {
		t.Fatalf("mapper23 chr read=0x%02X, want 0x44", got)
	}
	c.writeCPU(0xF000, 0xFE)
	c.writeCPU(0xF001, 0x02)
	for i := 0; i < 300; i++ {
		c.cart.vrcClockIRQ()
	}
	if !c.cart.consumeIRQ() {
		t.Fatalf("expected mapper23 IRQ pending")
	}
}

func TestMapper25PRGCHRSwitch(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	for b := 0; b < 8; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x70 + b)
		}
	}
	chr := make([]byte, 16*1024)
	chr[5*1024] = 0x88
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   25,
		PRGBanks: 4,
		CHRBanks: 2,
	}

	c.writeCPU(0xA000, 0x03)
	if got := c.readCPU(0xA000); got != 0x73 {
		t.Fatalf("mapper25 prg read=0x%02X, want 0x73", got)
	}
	c.writeCPU(0xC000, 0x05)
	if got := c.ppu.ppuRead(c, 0x0800); got != 0x88 {
		t.Fatalf("mapper25 chr read=0x%02X, want 0x88", got)
	}
}

func TestMapper5PRGCHRSwitchAndFillNametable(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	for b := 0; b < 8; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x80 + b)
		}
	}
	chr := make([]byte, 16*1024)
	chr[6*1024] = 0x99
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      chr,
		Mapper:   5,
		PRGBanks: 4,
		CHRBanks: 2,
	}

	c.writeCPU(0x5100, 0x03)
	c.writeCPU(0x5114, 0x82)
	if got := c.readCPU(0x8000); got != 0x82 {
		t.Fatalf("mapper5 prg read=0x%02X, want 0x82", got)
	}

	c.writeCPU(0x5101, 0x03)
	c.writeCPU(0x5126, 0x06)
	if got := c.ppu.ppuRead(c, 0x1800); got != 0x99 {
		t.Fatalf("mapper5 chr read=0x%02X, want 0x99", got)
	}

	c.writeCPU(0x5105, 0xFF)
	c.writeCPU(0x5106, 0x23)
	c.writeCPU(0x5107, 0x02)
	st := scanlineRenderState{
		mapper:        5,
		mmc5ExRAMMode: c.cart.mmc5ExRAMMode,
		mmc5FillTile:  c.cart.mmc5FillTile,
		mmc5FillAttr:  c.cart.mmc5FillAttr,
		mmc5NTMap:     c.cart.mmc5NTMap,
	}
	if got := c.ppu.readNametableWithState(c, st, 0x2000); got != 0x23 {
		t.Fatalf("mapper5 fill tile = 0x%02X, want 0x23", got)
	}
	if got := c.ppu.readNametableWithState(c, st, 0x23C0); got != 0xAA {
		t.Fatalf("mapper5 fill attr = 0x%02X, want 0xAA", got)
	}

	c.writeCPU(0x5102, 0x02)
	c.writeCPU(0x5103, 0x01)
	c.writeCPU(0x6000, 0x5A)
	if got := c.readCPU(0x6000); got != 0x5A {
		t.Fatalf("mapper5 prg-ram read=0x%02X, want 0x5A", got)
	}
}

func TestMapper5NametableWriteTargetsExRAMAndIgnoresFill(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 32*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   5,
		PRGBanks: 2,
		CHRBanks: 1,
	}

	c.writeCPU(0x5104, 0x00)
	c.writeCPU(0x5105, 0x02) // table 0 -> exram
	c.ppu.ppuWrite(c, 0x2000, 0x3C)
	if got := c.cart.mmc5ExRAM[0]; got != 0x3C {
		t.Fatalf("mapper5 exram write=0x%02X, want 0x3C", got)
	}

	c.writeCPU(0x5105, 0x03) // table 0 -> fill
	c.writeCPU(0x5106, 0x11)
	c.ppu.ppuWrite(c, 0x2000, 0x77)
	if got := c.ppu.readNametableWithState(c, scanlineRenderState{
		mapper:       5,
		mmc5NTMap:    c.cart.mmc5NTMap,
		mmc5FillTile: c.cart.mmc5FillTile,
		mmc5FillAttr: c.cart.mmc5FillAttr,
	}, 0x2000); got != 0x11 {
		t.Fatalf("mapper5 fill target should ignore write, got 0x%02X", got)
	}
}

func TestMapper5MultiplyAndScanlineIRQ(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 32*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   5,
		PRGBanks: 2,
		CHRBanks: 1,
	}

	c.writeCPU(0x5205, 7)
	c.writeCPU(0x5206, 9)
	if got := c.readCPU(0x5205); got != 63 {
		t.Fatalf("mapper5 multiply low = %d, want 63", got)
	}
	if got := c.readCPU(0x5206); got != 0 {
		t.Fatalf("mapper5 multiply high = %d, want 0", got)
	}

	c.writeCPU(0x5203, 3)
	c.writeCPU(0x5204, 0x80)
	for i := 0; i < 4; i++ {
		c.cart.mmc5ClockScanline()
	}
	if !c.cart.consumeIRQ() {
		t.Fatalf("expected mapper5 scanline IRQ pending")
	}
	if status := c.readCPU(0x5204); status&0x80 != 0 {
		t.Fatalf("expected reading 0x5204 to clear pending IRQ, got 0x%02X", status)
	}
}

func TestMapper5PRGMode1Uses16KWindows(t *testing.T) {
	c := NewConsole()
	prg := make([]byte, 8*8*1024)
	for b := 0; b < 8; b++ {
		for i := 0; i < 8*1024; i++ {
			prg[b*8*1024+i] = byte(0x40 + b)
		}
	}
	c.cart = &Cartridge{
		PRG:      prg,
		CHR:      make([]byte, 8*1024),
		Mapper:   5,
		PRGBanks: 4,
		CHRBanks: 1,
	}

	c.writeCPU(0x5100, 0x01)
	c.writeCPU(0x5115, 0x82)
	c.writeCPU(0x5117, 0x05)

	if got := c.readCPU(0x8000); got != 0x42 {
		t.Fatalf("mode1 lower 16K start = 0x%02X, want 0x42", got)
	}
	if got := c.readCPU(0xA000); got != 0x43 {
		t.Fatalf("mode1 lower 16K upper half = 0x%02X, want 0x43", got)
	}
	if got := c.readCPU(0xC000); got != 0x44 {
		t.Fatalf("mode1 upper 16K start = 0x%02X, want 0x44", got)
	}
	if got := c.readCPU(0xE000); got != 0x45 {
		t.Fatalf("mode1 upper 16K upper half = 0x%02X, want 0x45", got)
	}
}

func TestMapper5PRGRAMBankingAt6000And8000(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 64*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   5,
		PRGBanks: 4,
		CHRBanks: 1,
	}

	c.writeCPU(0x5102, 0x02)
	c.writeCPU(0x5103, 0x01)

	c.writeCPU(0x5113, 0x02)
	c.writeCPU(0x6000, 0x5A)
	c.writeCPU(0x5113, 0x03)
	c.writeCPU(0x6000, 0xA5)
	c.writeCPU(0x5113, 0x02)
	if got := c.readCPU(0x6000); got != 0x5A {
		t.Fatalf("mapper5 banked prg-ram at 0x6000 = 0x%02X, want 0x5A", got)
	}
	c.writeCPU(0x5113, 0x03)
	if got := c.readCPU(0x6000); got != 0xA5 {
		t.Fatalf("mapper5 banked prg-ram second bank = 0x%02X, want 0xA5", got)
	}

	c.writeCPU(0x5100, 0x03)
	c.writeCPU(0x5114, 0x01)
	c.writeCPU(0x8000, 0x3C)
	if got := c.readCPU(0x8000); got != 0x3C {
		t.Fatalf("mapper5 prg-ram mapped at 0x8000 = 0x%02X, want 0x3C", got)
	}
	c.writeCPU(0x5114, 0x02)
	if got := c.readCPU(0x8000); got == 0x3C {
		t.Fatalf("expected mapper5 0x8000 window to switch prg-ram banks")
	}
	c.writeCPU(0x8000, 0x7E)
	if got := c.readCPU(0x8000); got != 0x7E {
		t.Fatalf("mapper5 second prg-ram bank at 0x8000 = 0x%02X, want 0x7E", got)
	}
	c.writeCPU(0x5114, 0x01)
	if got := c.readCPU(0x8000); got != 0x3C {
		t.Fatalf("mapper5 restored prg-ram bank at 0x8000 = 0x%02X, want 0x3C", got)
	}
}
