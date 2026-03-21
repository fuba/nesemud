package nes

import "testing"

func TestPPUScrollAffectsBackgroundSampling(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{PRG: make([]byte, 16*1024), CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}

	// tile 0 -> color index 1
	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
	}
	// tile 1 -> color index 2
	for r := 0; r < 8; r++ {
		c.cart.CHR[16+r] = 0x00
		c.cart.CHR[24+r] = 0xFF
	}

	c.ppu.ntRAM[0] = 0
	c.ppu.ntRAM[1] = 1
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x20
	c.ppu.mask = 0x0A

	bufNoScroll := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, bufNoScroll)
	pixNoScroll := [3]byte{bufNoScroll[0], bufNoScroll[1], bufNoScroll[2]}

	c.writeCPU(0x2005, 8)
	c.writeCPU(0x2005, 0)
	c.ppu.vramAddr = c.ppu.tempAddr
	c.ppu.captureLineState(c, 0)
	bufScroll := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, bufScroll)
	pixScroll := [3]byte{bufScroll[0], bufScroll[1], bufScroll[2]}

	if pixNoScroll == pixScroll {
		t.Fatalf("expected different pixel with scroll applied")
	}
}

func TestPPUMaskHidesBackgroundLeftEdge(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{PRG: make([]byte, 16*1024), CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}

	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
	}

	c.ppu.mask = 0x08
	c.ppu.ntRAM[0] = 0
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30

	buf := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, buf)

	left := buf[0:3]
	visible := buf[8*3 : 8*3+3]
	wantBackdrop := nesPaletteRGB[c.ppu.palette[0]&0x3F]
	wantTile := nesPaletteRGB[c.ppu.palette[1]&0x3F]

	if left[0] != wantBackdrop[0] || left[1] != wantBackdrop[1] || left[2] != wantBackdrop[2] {
		t.Fatalf("expected left edge background pixel to be masked")
	}
	if visible[0] != wantTile[0] || visible[1] != wantTile[1] || visible[2] != wantTile[2] {
		t.Fatalf("expected background after left edge to remain visible")
	}
}
