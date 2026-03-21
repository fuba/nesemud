package nes

import "testing"

func TestRenderFrameCrossesIntoAdjacentNametableWhenScrolled(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{PRG: make([]byte, 16*1024), CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1, mirroring: MirroringVertical}

	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
		c.cart.CHR[16+r] = 0x00
		c.cart.CHR[24+r] = 0xFF
	}

	c.ppu.ntRAM[31] = 0
	c.ppu.ntRAM[0x400] = 1
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16
	c.ppu.mask = 0x0A
	c.writeCPU(0x2005, 248)
	c.writeCPU(0x2005, 0)
	c.ppu.vramAddr = c.ppu.tempAddr
	c.ppu.captureLineState(c, 0)

	frame := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, frame)

	left := frame[0:3]
	rightOffset := 8 * 3
	right := frame[rightOffset : rightOffset+3]

	if left[0] != nesPaletteRGB[c.ppu.palette[1]][0] ||
		left[1] != nesPaletteRGB[c.ppu.palette[1]][1] ||
		left[2] != nesPaletteRGB[c.ppu.palette[1]][2] {
		t.Fatalf("left pixel = %v, want palette[1] color", left)
	}
	if right[0] != nesPaletteRGB[c.ppu.palette[2]][0] ||
		right[1] != nesPaletteRGB[c.ppu.palette[2]][1] ||
		right[2] != nesPaletteRGB[c.ppu.palette[2]][2] {
		t.Fatalf("right pixel = %v, want palette[2] color", right)
	}
}
