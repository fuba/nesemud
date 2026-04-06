package nes

import "testing"

func TestPPUVBlankAndNMITiming(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 0}
	c.ppu.ctrl = 0x80 // NMI enable

	nmiSeen := false
	for i := 0; i < 30000; i++ {
		if c.ppu.step(c, 1) {
			nmiSeen = true
			break
		}
	}
	if !nmiSeen {
		t.Fatalf("expected NMI at vblank start")
	}
	if c.ppu.status&0x80 == 0 {
		t.Fatalf("expected vblank flag set")
	}

	for i := 0; i < 8000; i++ {
		_ = c.ppu.step(c, 1)
	}
	if c.ppu.scanline == 0 && c.ppu.status&0x80 != 0 {
		t.Fatalf("expected vblank cleared at new frame")
	}
}

func TestPPUOddFrameSkipAtPreRenderWhenRenderingEnabled(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 0}
	c.ppu.frameID = 1 // odd frame
	c.ppu.scanline = 261
	c.ppu.cycle = 338
	c.ppu.mask = 0x08 // background rendering enabled

	_ = c.ppu.step(c, 1) // 3 PPU dots

	if got, want := c.ppu.frameID, uint64(2); got != want {
		t.Fatalf("frame id = %d, want %d", got, want)
	}
	if got, want := c.ppu.scanline, 0; got != want {
		t.Fatalf("scanline = %d, want %d", got, want)
	}
	if got, want := c.ppu.cycle, 1; got != want {
		t.Fatalf("cycle = %d, want %d (odd-frame skip should remove one dot)", got, want)
	}
}

func TestPPUOddFrameSkipNotAppliedWhenRenderingDisabled(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{Mapper: 0}
	c.ppu.frameID = 1 // odd frame
	c.ppu.scanline = 261
	c.ppu.cycle = 338
	c.ppu.mask = 0x00 // rendering disabled

	_ = c.ppu.step(c, 1) // 3 PPU dots

	if got, want := c.ppu.frameID, uint64(2); got != want {
		t.Fatalf("frame id = %d, want %d", got, want)
	}
	if got, want := c.ppu.scanline, 0; got != want {
		t.Fatalf("scanline = %d, want %d", got, want)
	}
	if got, want := c.ppu.cycle, 0; got != want {
		t.Fatalf("cycle = %d, want %d (no odd-frame skip when rendering disabled)", got, want)
	}
}
