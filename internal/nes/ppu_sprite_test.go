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
	c.ppu.mask = 0x14

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

func TestPPULiveSpriteRenderDuringStep(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	base := 16
	for r := 0; r < 8; r++ {
		c.cart.CHR[base+r] = 0xFF
		c.cart.CHR[base+8+r] = 0x00
	}
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[0x11] = 0x30
	c.ppu.mask = 0x14

	c.ppu.oam[0] = 9
	c.ppu.oam[1] = 1
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 10

	for c.ppu.scanline < 10 || (c.ppu.scanline == 10 && c.ppu.cycle <= 11) {
		_ = c.ppu.step(c, 1)
	}

	at := (10*FrameWidth + 10) * 3
	want := nesPaletteRGB[c.ppu.palette[0x11]&0x3F]
	if c.ppu.frameRGB[at] != want[0] || c.ppu.frameRGB[at+1] != want[1] || c.ppu.frameRGB[at+2] != want[2] {
		t.Fatalf("expected live sprite renderer to update frame buffer during step")
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
	c.ppu.mask = 0x14

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

func TestPPUSprite8x16RenderUsesBottomTile(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	c.ppu.ctrl = 0x20 // 8x16 sprites
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[0x11] = 0x30
	c.ppu.palette[0x12] = 0x16
	c.ppu.mask = 0x14

	// 8x16 sprite tile 0:
	// top half uses tile 0 -> color 1, bottom half uses tile 1 -> color 2.
	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
		c.cart.CHR[16+r] = 0x00
		c.cart.CHR[24+r] = 0xFF
	}

	c.ppu.oam[0] = 9
	c.ppu.oam[1] = 0
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 10

	buf := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, buf)

	top := (10*FrameWidth + 10) * 3
	bottom := (18*FrameWidth + 10) * 3
	wantTop := nesPaletteRGB[c.ppu.palette[0x11]&0x3F]
	wantBottom := nesPaletteRGB[c.ppu.palette[0x12]&0x3F]

	if buf[top] != wantTop[0] || buf[top+1] != wantTop[1] || buf[top+2] != wantTop[2] {
		t.Fatalf("expected top half of 8x16 sprite to use first tile")
	}
	if buf[bottom] != wantBottom[0] || buf[bottom+1] != wantBottom[1] || buf[bottom+2] != wantBottom[2] {
		t.Fatalf("expected bottom half of 8x16 sprite to use second tile")
	}
}

func TestPPUSprite0HitSetsStatusFlag(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	c.ppu.mask = 0x1E
	c.ppu.ntRAM[0] = 0
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30

	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
	}

	c.ppu.oam[0] = 0
	c.ppu.oam[1] = 0
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 0

	hitSeen := false
	for i := 0; i < 800; i++ {
		_ = c.ppu.step(c, 1)
		if c.ppu.status&0x40 != 0 {
			hitSeen = true
			break
		}
	}
	if !hitSeen {
		t.Fatalf("expected sprite 0 hit flag to be set")
	}
}

func TestPPUSprite0HitDoesNotSetAtX255(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	c.ppu.mask = 0x1E
	c.ppu.ntRAM[31] = 0
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30

	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
	}

	c.ppu.oam[0] = 0
	c.ppu.oam[1] = 0
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 255

	for i := 0; i < 1200; i++ {
		_ = c.ppu.step(c, 1)
	}
	if c.ppu.status&0x40 != 0 {
		t.Fatalf("did not expect sprite 0 hit at x=255")
	}
}

func TestPPUMaskHidesSpriteLeftEdge(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	for r := 0; r < 8; r++ {
		c.cart.CHR[16+r] = 0xFF
		c.cart.CHR[24+r] = 0x00
	}
	c.ppu.mask = 0x10
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[0x11] = 0x30

	c.ppu.oam[0] = 9
	c.ppu.oam[1] = 1
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 1

	buf := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, buf)

	left := buf[(10*FrameWidth+0)*3 : (10*FrameWidth+0)*3+3]
	visible := buf[(10*FrameWidth+8)*3 : (10*FrameWidth+8)*3+3]
	wantBackdrop := nesPaletteRGB[c.ppu.palette[0]&0x3F]
	wantSprite := nesPaletteRGB[c.ppu.palette[0x11]&0x3F]

	if left[0] != wantBackdrop[0] || left[1] != wantBackdrop[1] || left[2] != wantBackdrop[2] {
		t.Fatalf("expected left edge sprite pixel to be masked")
	}
	if visible[0] != wantSprite[0] || visible[1] != wantSprite[1] || visible[2] != wantSprite[2] {
		t.Fatalf("expected sprite after left edge to remain visible")
	}
}

func TestPPUSpriteOverflowSetsStatusFlag(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	c.ppu.mask = 0x18
	for i := 0; i < 9; i++ {
		o := i * 4
		c.ppu.oam[o+0] = 19
		c.ppu.oam[o+1] = 0
		c.ppu.oam[o+2] = 0
		c.ppu.oam[o+3] = byte(i * 8)
	}

	for c.ppu.scanline < 20 {
		_ = c.ppu.step(c, 1)
	}
	if c.ppu.status&0x20 == 0 {
		t.Fatalf("expected sprite overflow flag to be set")
	}
}

func TestSprite0HitUsesPerScanlineMaskState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	for r := 0; r < 8; r++ {
		c.cart.CHR[r] = 0xFF
		c.cart.CHR[8+r] = 0x00
	}
	for r := 0; r < 8; r++ {
		c.cart.CHR[16+r] = 0xFF
		c.cart.CHR[24+r] = 0x00
	}
	c.ppu.ntRAM[0] = 0
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30

	c.ppu.oam[0] = 9
	c.ppu.oam[1] = 1
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 0

	line := 10
	c.ppu.mask = 0x00
	c.ppu.lineState[line] = scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x1E,
		vramAddr:  0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}

	if hit := c.ppu.computeSprite0HitX(c, line); hit != 0 {
		t.Fatalf("sprite0 hit x = %d, want 0", hit)
	}
}

func TestPPUSpriteOverflowUsesPerScanlineSpriteHeight(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
		Mapper:   0,
		PRGBanks: 1,
	}

	for i := 0; i < 9; i++ {
		o := i * 4
		c.ppu.oam[o+0] = 10
		c.ppu.oam[o+1] = 0
		c.ppu.oam[o+2] = 0
		c.ppu.oam[o+3] = byte(i * 8)
	}

	c.ppu.lineState[25] = scanlineRenderState{
		valid: true,
		ctrl:  0x20,
		mask:  0x18,
	}

	if count := c.ppu.countSpritesOnLine(c, 25); count != 9 {
		t.Fatalf("countSpritesOnLine = %d, want 9 for 8x16 sprites", count)
	}
}
