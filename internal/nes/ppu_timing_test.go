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
