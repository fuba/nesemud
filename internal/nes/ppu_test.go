package nes

import "testing"

func TestPPUAddrDataIncrement(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge(nil)

	c.writeCPU(0x2006, 0x20)
	c.writeCPU(0x2006, 0x00)
	c.writeCPU(0x2007, 0x12)
	c.writeCPU(0x2007, 0x34)

	c.writeCPU(0x2006, 0x20)
	c.writeCPU(0x2006, 0x00)
	_ = c.readCPU(0x2007)
	if got := c.readCPU(0x2007); got != 0x12 {
		t.Fatalf("first read = 0x%02X, want 0x12", got)
	}
	if got := c.readCPU(0x2007); got != 0x34 {
		t.Fatalf("second read = 0x%02X, want 0x34", got)
	}
}

func TestPPUStatusClearsVBlankAndLatch(t *testing.T) {
	c := NewConsole()
	c.ppu.status = 0x80
	c.ppu.addrLatch = true

	v := c.readCPU(0x2002)
	if v&0x80 == 0 {
		t.Fatalf("expected vblank bit set in returned status")
	}
	if c.ppu.status&0x80 != 0 {
		t.Fatalf("expected vblank cleared after read")
	}
	if c.ppu.addrLatch {
		t.Fatalf("expected addr latch cleared after status read")
	}
}

func TestOAMDMAFromCPUPage(t *testing.T) {
	c := NewConsole()
	for i := 0; i < 256; i++ {
		c.writeCPU(uint16(0x0200+i), byte(i))
	}
	c.writeCPU(0x4014, 0x02)
	for i := 0; i < 256; i++ {
		if got := c.ppu.oam[i]; got != byte(i) {
			t.Fatalf("oam[%d]=0x%02X, want 0x%02X", i, got, byte(i))
		}
	}
}
