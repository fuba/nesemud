package nes

import "testing"

func TestRenderFrameClearsBackgroundOpaqueBufferBeforeSpritePriority(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:    0,
		mirroring: MirroringHorizontal,
		PRG:       make([]byte, 16*1024),
		CHR:       make([]byte, 8*1024),
		PRGRAM:    make([]byte, 8*1024),
		PRGBanks:  1,
		CHRBanks:  1,
	}
	// Tile 0, row 0 -> left-most opaque pixel.
	c.cart.CHR[0] = 0x80

	st := scanlineRenderState{
		valid:     true,
		mask:      0x14, // sprites only + left-edge sprite enable
		mirroring: MirroringHorizontal,
	}
	c.ppu.lineState[1] = st
	c.ppu.lineSplitN[1] = 1
	c.ppu.lineSplits[1][0] = scanlineStateSegment{startX: 0, state: st}
	c.ppu.mask = st.mask

	// Sprite 0 at x=0, y=1, behind background.
	c.ppu.oam[0] = 0  // visible at scanline 1
	c.ppu.oam[1] = 0  // tile 0
	c.ppu.oam[2] = 32 // priority behind background
	c.ppu.oam[3] = 0

	// Palette index used by sprite pixel (pal=0, pix=1 => slot 0x11).
	c.ppu.palette[0] = 0
	c.ppu.palette[0x11] = 1

	// Simulate stale opaque flag from a previous frame. Render must clear it.
	c.ppu.frameBGOpaq[FrameWidth] = true

	dst := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, dst)

	o := FrameWidth * 3 // (x=0,y=1)
	want := nesPaletteRGB[1]
	if got := [3]byte{dst[o], dst[o+1], dst[o+2]}; got != want {
		t.Fatalf("sprite pixel = %v, want %v", got, want)
	}
}
