package nes

import "testing"

func TestPPUSpriteOverlayRender(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	// sprite tile 1 row pattern: all pixels set to color 1.
	base := 16
	for r := 0; r < 8; r++ {
		c.cart.CHR[base+r] = 0xFF
		c.cart.CHR[base+8+r] = 0x00
	}
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[0x11] = 0x30 // sprite palette 0, color 1

	// sprite 0: y=9 means visible at y=10, tile=1, attr=0, x=10.
	c.ppu.oam[0] = 9
	c.ppu.oam[1] = 1
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 10

	buf := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, buf)

	o := (10*FrameWidth + 10) * 3
	if buf[o] == 0 && buf[o+1] == 0 && buf[o+2] == 0 {
		t.Fatalf("expected sprite pixel to be visible")
	}
}

func TestPPUSpritePriorityLowerOAMInFront(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	// Tile 1 -> palette color index 1, Tile 2 -> palette color index 2.
	for r := 0; r < 8; r++ {
		c.cart.CHR[16+r] = 0xFF
		c.cart.CHR[16+8+r] = 0x00
		c.cart.CHR[32+r] = 0x00
		c.cart.CHR[32+8+r] = 0xFF
	}
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[0x11] = 0x30
	c.ppu.palette[0x12] = 0x16

	// Same position overlap:
	// OAM[0] should be in front on real NES.
	c.ppu.oam[0] = 9
	c.ppu.oam[1] = 1
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 10

	o := 4
	c.ppu.oam[o+0] = 9
	c.ppu.oam[o+1] = 2
	c.ppu.oam[o+2] = 0
	c.ppu.oam[o+3] = 10

	buf := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, buf)

	at := (10*FrameWidth + 10) * 3
	want := nesPaletteRGB[c.ppu.palette[0x11]&0x3F]
	if buf[at] != want[0] || buf[at+1] != want[1] || buf[at+2] != want[2] {
		t.Fatalf("expected sprite with lower OAM index to be in front")
	}
}
