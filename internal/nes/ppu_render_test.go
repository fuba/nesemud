package nes

import "testing"

func TestPPUBackgroundRenderWritesFrame(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	// tile 0, row 0 => low plane bits all 1, high plane bits 0 => color index 1.
	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
	}
	// top-left tile in nametable uses tile 0.
	c.ppu.ntRAM[0] = 0
	// universal background color and palette entry 1.
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30

	buf := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, buf)

	if buf[0] == 0 && buf[1] == 0 && buf[2] == 0 {
		t.Fatalf("expected non-black top-left pixel")
	}
}
