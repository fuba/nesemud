package nes

import "testing"

func TestRenderFrameRewindsPrefetchedVisibleScanlineAddress(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{PRG: make([]byte, 16*1024), CHR: make([]byte, 8*1024), Mapper: 0, PRGBanks: 1}

	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
		c.cart.CHR[16+r] = 0x00
		c.cart.CHR[24+r] = 0xFF
	}

	c.ppu.ntRAM[0] = 0
	c.ppu.ntRAM[2] = 1
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16
	c.ppu.lineState[0] = scanlineRenderState{
		valid:      true,
		prefetched: true,
		mask:       0x0A,
		vramAddr:   0x0002,
	}

	frame := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, frame)

	pixel := frame[0:3]
	want := nesPaletteRGB[c.ppu.palette[1]]
	if pixel[0] != want[0] || pixel[1] != want[1] || pixel[2] != want[2] {
		t.Fatalf("left pixel = %v, want palette[1] color after prefetch rewind", pixel)
	}
}
