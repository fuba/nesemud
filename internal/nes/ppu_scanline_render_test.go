package nes

import "testing"

func TestRenderFrameUsesPerScanlineMMC3CHRState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:    4,
		CHRBanks:  1,
		mirroring: MirroringHorizontal,
		CHR:       make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.ppu.ntRAM[i] = 0
	}
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16

	for i := 0; i < 8; i++ {
		c.cart.CHR[i] = 0xFF
		c.cart.CHR[8+i] = 0x00
	}
	base := 2 * 1024
	for i := 0; i < 8; i++ {
		c.cart.CHR[base+i] = 0x00
		c.cart.CHR[base+8+i] = 0xFF
	}

	top := scanlineRenderState{
		valid:          true,
		ctrl:           0x00,
		mask:           0x0A,
		mirroring:      MirroringHorizontal,
		mapper:         4,
		mmc3BankSelect: 0x00,
	}
	top.mmc3Regs[0] = 0
	bottom := top
	bottom.mmc3Regs[0] = 2

	for y := 0; y < FrameHeight; y++ {
		c.ppu.lineState[y] = top
		if y >= FrameHeight/2 {
			c.ppu.lineState[y] = bottom
		}
	}

	frame := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, frame)

	topPixel := frame[0:3]
	bottomOffset := ((FrameHeight/2)*FrameWidth + 0) * 3
	bottomPixel := frame[bottomOffset : bottomOffset+3]

	if topPixel[0] != nesPaletteRGB[c.ppu.palette[1]][0] ||
		topPixel[1] != nesPaletteRGB[c.ppu.palette[1]][1] ||
		topPixel[2] != nesPaletteRGB[c.ppu.palette[1]][2] {
		t.Fatalf("top pixel = %v, want palette[1] color", topPixel)
	}
	if bottomPixel[0] != nesPaletteRGB[c.ppu.palette[2]][0] ||
		bottomPixel[1] != nesPaletteRGB[c.ppu.palette[2]][1] ||
		bottomPixel[2] != nesPaletteRGB[c.ppu.palette[2]][2] {
		t.Fatalf("bottom pixel = %v, want palette[2] color", bottomPixel)
	}
}

func TestRenderFrameUsesPerScanlineSpriteMaskState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:   0,
		PRGBanks: 1,
		CHRBanks: 1,
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.cart.CHR[16+i] = 0xFF
		c.cart.CHR[24+i] = 0x00
	}
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[0x11] = 0x30

	c.ppu.oam[0] = byte(FrameHeight/2 - 1)
	c.ppu.oam[1] = 1
	c.ppu.oam[2] = 0
	c.ppu.oam[3] = 16

	top := scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x00,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	bottom := top
	bottom.mask = 0x10

	for y := 0; y < FrameHeight; y++ {
		c.ppu.lineState[y] = top
		if y >= FrameHeight/2 {
			c.ppu.lineState[y] = bottom
		}
	}

	frame := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, frame)

	topOffset := ((FrameHeight/2-1)*FrameWidth + 16) * 3
	bottomOffset := ((FrameHeight/2)*FrameWidth + 16) * 3
	topPixel := frame[topOffset : topOffset+3]
	bottomPixel := frame[bottomOffset : bottomOffset+3]
	backdrop := nesPaletteRGB[c.ppu.palette[0]&0x3F]
	sprite := nesPaletteRGB[c.ppu.palette[0x11]&0x3F]

	if topPixel[0] != backdrop[0] || topPixel[1] != backdrop[1] || topPixel[2] != backdrop[2] {
		t.Fatalf("top pixel = %v, want backdrop while sprite rendering disabled", topPixel)
	}
	if bottomPixel[0] != sprite[0] || bottomPixel[1] != sprite[1] || bottomPixel[2] != sprite[2] {
		t.Fatalf("bottom pixel = %v, want sprite color while sprite rendering enabled", bottomPixel)
	}
}

func TestRenderFrameUsesPerScanlineScrollState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:    0,
		PRGBanks:  1,
		CHRBanks:  1,
		mirroring: MirroringHorizontal,
		PRG:       make([]byte, 16*1024),
		CHR:       make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.cart.CHR[i] = 0xFF
		c.cart.CHR[8+i] = 0x00
		c.cart.CHR[16+i] = 0x00
		c.cart.CHR[24+i] = 0xFF
	}
	c.ppu.ntRAM[0] = 0
	c.ppu.ntRAM[1] = 1
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16

	top := scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	bottom := top
	bottom.vramAddr = 1

	for y := 0; y < FrameHeight; y++ {
		c.ppu.lineState[y] = top
		if y >= FrameHeight/2 {
			c.ppu.lineState[y] = bottom
		}
	}

	frame := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, frame)

	topPixel := frame[0:3]
	bottomOffset := ((FrameHeight/2)*FrameWidth + 0) * 3
	bottomPixel := frame[bottomOffset : bottomOffset+3]

	if topPixel[0] != nesPaletteRGB[c.ppu.palette[1]][0] ||
		topPixel[1] != nesPaletteRGB[c.ppu.palette[1]][1] ||
		topPixel[2] != nesPaletteRGB[c.ppu.palette[1]][2] {
		t.Fatalf("top pixel = %v, want palette[1] color", topPixel)
	}
	if bottomPixel[0] != nesPaletteRGB[c.ppu.palette[2]][0] ||
		bottomPixel[1] != nesPaletteRGB[c.ppu.palette[2]][1] ||
		bottomPixel[2] != nesPaletteRGB[c.ppu.palette[2]][2] {
		t.Fatalf("bottom pixel = %v, want palette[2] color after scroll split", bottomPixel)
	}
}

func TestMidFramePPUWritePreservesPreviouslyCapturedLineState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:   0,
		PRGBanks: 1,
		CHRBanks: 1,
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.cart.CHR[i] = 0xFF
		c.cart.CHR[8+i] = 0x00
		c.cart.CHR[16+i] = 0x00
		c.cart.CHR[24+i] = 0xFF
	}
	c.ppu.ntRAM[0] = 0
	c.ppu.ntRAM[1] = 1
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16

	c.ppu.lineState[0] = scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	c.ppu.lineState[2] = scanlineRenderState{valid: true}
	c.ppu.scanline = 1
	c.ppu.cycle = 40
	c.writeCPU(0x2005, 8)

	if !c.ppu.lineState[0].valid {
		t.Fatalf("expected previously captured line state to stay valid")
	}
	if c.ppu.lineState[2].valid {
		t.Fatalf("expected future line state to be invalidated")
	}
}

func TestMidFramePPUWriteCreatesSameLineSplitState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:   0,
		PRGBanks: 1,
		CHRBanks: 1,
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.cart.CHR[i] = 0xFF
		c.cart.CHR[8+i] = 0x00
		c.cart.CHR[16+i] = 0x00
		c.cart.CHR[24+i] = 0xFF
	}
	c.ppu.ntRAM[0] = 0
	c.ppu.ntRAM[1] = 1
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16

	c.ppu.lineState[0] = scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	c.ppu.lineSplitN[0] = 1
	c.ppu.lineSplits[0][0] = scanlineStateSegment{startX: 0, state: c.ppu.lineState[0]}

	c.ppu.scanline = 0
	c.ppu.cycle = 9
	c.ppu.addrLatch = true
	c.writeCPU(0x2006, 0x01)

	if c.ppu.lineSplitN[0] < 2 {
		t.Fatalf("expected same-line split segment to be recorded")
	}

	leftState := c.ppu.renderStateForPixel(c, 0, 23)
	rightState := c.ppu.renderStateForPixel(c, 0, 24)
	if leftState.vramAddr != 0 {
		t.Fatalf("left state vramAddr = 0x%04X, want 0x0000 before split", leftState.vramAddr)
	}
	if rightState.vramAddr != 1 {
		t.Fatalf("right state vramAddr = 0x%04X, want 0x0001 after split", rightState.vramAddr)
	}
}

func TestMidFramePPUCTRLWriteCreatesSameLineSplitState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:   0,
		PRGBanks: 1,
		CHRBanks: 1,
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
	}

	base := scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	c.ppu.lineState[0] = base
	c.ppu.lineSplitN[0] = 1
	c.ppu.lineSplits[0][0] = scanlineStateSegment{startX: 0, state: base}

	c.ppu.scanline = 0
	c.ppu.cycle = 9
	c.writeCPU(0x2000, 0x10)

	if c.ppu.lineSplitN[0] < 2 {
		t.Fatalf("expected same-line split segment after mid-frame PPUCTRL write")
	}
	startX := c.ppu.lineSplits[0][1].startX
	if startX <= 0 || startX >= FrameWidth {
		t.Fatalf("unexpected split startX=%d", startX)
	}
	before := c.ppu.renderStateForPixel(c, 0, startX-1)
	after := c.ppu.renderStateForPixel(c, 0, startX)
	if before.ctrl != 0x00 {
		t.Fatalf("before split ctrl=0x%02X, want 0x00", before.ctrl)
	}
	if after.ctrl != 0x10 {
		t.Fatalf("after split ctrl=0x%02X, want 0x10", after.ctrl)
	}
}

func TestMidFramePPUSCROLLWriteCreatesSameLineSplitState(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:   0,
		PRGBanks: 1,
		CHRBanks: 1,
		PRG:      make([]byte, 16*1024),
		CHR:      make([]byte, 8*1024),
	}

	base := scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	c.ppu.lineState[0] = base
	c.ppu.lineSplitN[0] = 1
	c.ppu.lineSplits[0][0] = scanlineStateSegment{startX: 0, state: base}

	c.ppu.scanline = 0
	c.ppu.cycle = 9
	c.ppu.addrLatch = false
	c.writeCPU(0x2005, 0x05)

	if c.ppu.lineSplitN[0] < 2 {
		t.Fatalf("expected same-line split segment after mid-frame PPUSCROLL write")
	}
	startX := c.ppu.lineSplits[0][1].startX
	if startX <= 0 || startX >= FrameWidth {
		t.Fatalf("unexpected split startX=%d", startX)
	}
	before := c.ppu.renderStateForPixel(c, 0, startX-1)
	after := c.ppu.renderStateForPixel(c, 0, startX)
	if before.fineX != 0 {
		t.Fatalf("before split fineX=%d, want 0", before.fineX)
	}
	if after.fineX != 5 {
		t.Fatalf("after split fineX=%d, want 5", after.fineX)
	}
}

func TestRenderFrameUsesSameLineSegmentOrigin(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:    0,
		PRGBanks:  1,
		CHRBanks:  1,
		mirroring: MirroringHorizontal,
		PRG:       make([]byte, 16*1024),
		CHR:       make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.cart.CHR[i] = 0xFF
		c.cart.CHR[8+i] = 0x00
		c.cart.CHR[16+i] = 0x00
		c.cart.CHR[24+i] = 0xFF
	}
	c.ppu.ntRAM[0] = 0
	c.ppu.ntRAM[1] = 1
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16

	base := scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	split := base
	split.vramAddr = 1

	c.ppu.lineState[0] = base
	c.ppu.lineSplitN[0] = 2
	c.ppu.lineSplits[0][0] = scanlineStateSegment{startX: 0, state: base}
	c.ppu.lineSplits[0][1] = scanlineStateSegment{startX: 8, state: split}

	frame := make([]byte, FrameSizeRGB)
	c.ppu.renderFrame(c, frame)

	left := frame[0:3]
	rightOffset := 8 * 3
	right := frame[rightOffset : rightOffset+3]

	if left[0] != nesPaletteRGB[c.ppu.palette[1]][0] ||
		left[1] != nesPaletteRGB[c.ppu.palette[1]][1] ||
		left[2] != nesPaletteRGB[c.ppu.palette[1]][2] {
		t.Fatalf("left pixel = %v, want palette[1] color before split", left)
	}
	if right[0] != nesPaletteRGB[c.ppu.palette[2]][0] ||
		right[1] != nesPaletteRGB[c.ppu.palette[2]][1] ||
		right[2] != nesPaletteRGB[c.ppu.palette[2]][2] {
		t.Fatalf("right pixel = %v, want palette[2] color at split origin", right)
	}
}

func TestLiveBackgroundRenderingKeepsPreviouslyDrawnPixelsAfterVRAMWrite(t *testing.T) {
	c := NewConsole()
	c.cart = &Cartridge{
		Mapper:    0,
		PRGBanks:  1,
		CHRBanks:  1,
		mirroring: MirroringHorizontal,
		PRG:       make([]byte, 16*1024),
		CHR:       make([]byte, 8*1024),
	}

	for i := 0; i < 8; i++ {
		c.cart.CHR[i] = 0xFF
		c.cart.CHR[8+i] = 0x00
		c.cart.CHR[16+i] = 0x00
		c.cart.CHR[24+i] = 0xFF
	}
	c.ppu.palette[0] = 0x0F
	c.ppu.palette[1] = 0x30
	c.ppu.palette[2] = 0x16
	c.ppu.mask = 0x0A
	c.ppu.ctrl = 0x00
	c.ppu.fineX = 0
	c.ppu.vramAddr = 2
	c.ppu.scanline = 0
	c.ppu.ntRAM[0] = 0

	c.ppu.cycle = 1
	c.ppu.renderLiveBackgroundPixel(c, 0, 0)
	c.ppu.ntRAM[0] = 1
	c.ppu.cycle = 2
	c.ppu.renderLiveBackgroundPixel(c, 0, 1)

	left := c.ppu.frameRGB[0:3]
	next := c.ppu.frameRGB[3:6]
	if left[0] != nesPaletteRGB[c.ppu.palette[1]][0] ||
		left[1] != nesPaletteRGB[c.ppu.palette[1]][1] ||
		left[2] != nesPaletteRGB[c.ppu.palette[1]][2] {
		t.Fatalf("left pixel = %v, want original tile color before VRAM write", left)
	}
	if next[0] != nesPaletteRGB[c.ppu.palette[2]][0] ||
		next[1] != nesPaletteRGB[c.ppu.palette[2]][1] ||
		next[2] != nesPaletteRGB[c.ppu.palette[2]][2] {
		t.Fatalf("next pixel = %v, want updated tile color after VRAM write", next)
	}
}
