package nes

const maxScanlineStateSegments = 32

type ppu struct {
	ctrl        byte
	mask        byte
	status      byte
	oamAddr     byte
	oam         [256]byte
	vramAddr    uint16
	tempAddr    uint16
	fineX       byte
	addrLatch   bool
	readBuf     byte
	ntRAM       [4096]byte
	palette     [32]byte
	cycle       int
	scanline    int
	frameID     uint64
	lineState   [FrameHeight]scanlineRenderState
	lineSplits  [FrameHeight][maxScanlineStateSegments]scanlineStateSegment
	lineSplitN  [FrameHeight]int
	sprite0HitX [FrameHeight]int
	frameRGB    []byte
	frameBGOpaq []bool
}

var nesPaletteRGB = [64][3]byte{
	{124, 124, 124}, {0, 0, 252}, {0, 0, 188}, {68, 40, 188}, {148, 0, 132}, {168, 0, 32}, {168, 16, 0}, {136, 20, 0},
	{80, 48, 0}, {0, 120, 0}, {0, 104, 0}, {0, 88, 0}, {0, 64, 88}, {0, 0, 0}, {0, 0, 0}, {0, 0, 0},
	{188, 188, 188}, {0, 120, 248}, {0, 88, 248}, {104, 68, 252}, {216, 0, 204}, {228, 0, 88}, {248, 56, 0}, {228, 92, 16},
	{172, 124, 0}, {0, 184, 0}, {0, 168, 0}, {0, 168, 68}, {0, 136, 136}, {0, 0, 0}, {0, 0, 0}, {0, 0, 0},
	{248, 248, 248}, {60, 188, 252}, {104, 136, 252}, {152, 120, 248}, {248, 120, 248}, {248, 88, 152}, {248, 120, 88}, {252, 160, 68},
	{248, 184, 0}, {184, 248, 24}, {88, 216, 84}, {88, 248, 152}, {0, 232, 216}, {120, 120, 120}, {0, 0, 0}, {0, 0, 0},
	{252, 252, 252}, {164, 228, 252}, {184, 184, 248}, {216, 184, 248}, {248, 184, 248}, {248, 164, 192}, {240, 208, 176}, {252, 224, 168},
	{248, 216, 120}, {216, 248, 120}, {184, 248, 184}, {184, 248, 216}, {0, 252, 252}, {248, 216, 248}, {0, 0, 0}, {0, 0, 0},
}

func newPPU() *ppu {
	p := &ppu{
		frameRGB:    make([]byte, FrameSizeRGB),
		frameBGOpaq: make([]bool, FrameWidth*FrameHeight),
	}
	p.Reset()
	return p
}

func (p *ppu) Reset() {
	p.ctrl = 0
	p.mask = 0
	p.status = 0
	p.oamAddr = 0
	p.vramAddr = 0
	p.tempAddr = 0
	p.fineX = 0
	p.addrLatch = false
	p.readBuf = 0
	p.cycle = 0
	p.scanline = 0
	p.frameID = 0
	for i := range p.lineState {
		p.lineState[i] = scanlineRenderState{}
		p.lineSplitN[i] = 0
		p.sprite0HitX[i] = -1
	}
	for i := range p.frameRGB {
		p.frameRGB[i] = 0
	}
	for i := range p.frameBGOpaq {
		p.frameBGOpaq[i] = false
	}
}

func (p *ppu) invalidateLineState() {
	start := 0
	if p.scanline >= 0 && p.scanline < FrameHeight {
		start = p.scanline
		if p.cycle > 1 {
			start = p.scanline + 1
		}
	}
	if p.scanline >= FrameHeight {
		start = 0
	}
	if start < 0 {
		start = 0
	}
	if start > FrameHeight {
		start = FrameHeight
	}
	for i := start; i < len(p.lineState); i++ {
		p.lineState[i].valid = false
		p.lineSplitN[i] = 0
		p.sprite0HitX[i] = -1
	}
}

func (p *ppu) cpuReadRegister(c *Console, addr uint16) byte {
	switch addr & 0x2007 {
	case 0x2002:
		v := p.status
		p.status &^= 0x80
		p.addrLatch = false
		return v
	case 0x2004:
		return p.oam[p.oamAddr]
	case 0x2007:
		v := p.readData(c)
		p.incrementAddr()
		return v
	default:
		return 0
	}
}

func (p *ppu) cpuWriteRegister(c *Console, addr uint16, v byte) {
	if c != nil && c.ppuWriteTrace != nil {
		c.ppuWriteTrace(p.scanline, p.cycle, addr&0x2007, v, p.ctrl, p.mask)
	}
	switch addr & 0x2007 {
	case 0x2000:
		p.ctrl = v
		p.tempAddr = (p.tempAddr &^ 0x0C00) | (uint16(v&0x03) << 10)
		p.recordCurrentLineState(c, 8)
		p.invalidateLineState()
	case 0x2001:
		p.mask = v
		p.recordCurrentLineState(c, 0)
		p.invalidateLineState()
	case 0x2003:
		p.oamAddr = v
	case 0x2004:
		p.oam[p.oamAddr] = v
		p.oamAddr++
	case 0x2005:
		firstWrite := !p.addrLatch
		if firstWrite {
			p.fineX = v & 0x07
			p.tempAddr = (p.tempAddr &^ 0x001F) | uint16(v>>3)
			p.addrLatch = true
		} else {
			p.tempAddr = (p.tempAddr &^ 0x73E0) | (uint16(v&0x07) << 12) | (uint16(v&0xF8) << 2)
			p.addrLatch = false
		}
		p.recordCurrentLineState(c, 8)
		p.invalidateLineState()
	case 0x2006:
		firstWrite := !p.addrLatch
		if firstWrite {
			p.tempAddr = (p.tempAddr & 0x00FF) | (uint16(v&0x3F) << 8)
			p.addrLatch = true
		} else {
			p.tempAddr = (p.tempAddr & 0x7F00) | uint16(v)
			p.vramAddr = p.tempAddr
			p.addrLatch = false
			p.recordCurrentLineState(c, 16)
		}
		p.invalidateLineState()
	case 0x2007:
		p.writeData(c, v)
		p.incrementAddr()
	}
}

func (p *ppu) incrementAddr() {
	if p.ctrl&0x04 != 0 {
		p.vramAddr += 32
		return
	}
	p.vramAddr++
}

func (p *ppu) readData(c *Console) byte {
	addr := p.vramAddr & 0x3FFF
	if addr >= 0x3F00 {
		v := p.ppuRead(c, addr)
		p.readBuf = p.ppuRead(c, addr-0x1000)
		return v
	}
	v := p.readBuf
	p.readBuf = p.ppuRead(c, addr)
	return v
}

func (p *ppu) writeData(c *Console, v byte) {
	p.ppuWrite(c, p.vramAddr&0x3FFF, v)
}

func (p *ppu) ppuRead(c *Console, addr uint16) byte {
	a := addr & 0x3FFF
	switch {
	case a < 0x2000:
		if c.cart == nil {
			return 0
		}
		return c.cart.readCHR(a)
	case a < 0x3F00:
		idx := mirrorNametableAddr(c, a)
		return p.ntRAM[idx]
	default:
		idx := mirrorPaletteAddr(a)
		return p.palette[idx]
	}
}

func (p *ppu) ppuWrite(c *Console, addr uint16, v byte) {
	a := addr & 0x3FFF
	switch {
	case a < 0x2000:
		if c.cart == nil {
			return
		}
		c.cart.writeCHR(a, v)
	case a < 0x3F00:
		if c != nil && c.cart != nil && c.cart.Mapper == 5 {
			p.writeNametableMapper5(c.cart, a, v)
			return
		}
		idx := mirrorNametableAddr(c, a)
		p.ntRAM[idx] = v
	default:
		idx := mirrorPaletteAddr(a)
		p.palette[idx] = v
	}
}

func (p *ppu) writeNametableMapper5(cart *Cartridge, addr uint16, v byte) {
	off := int((addr - 0x2000) & 0x0FFF)
	table := (off / 0x400) & 0x03
	in := off & 0x03FF
	switch cart.mmc5NTMap[table] {
	case 0:
		p.ntRAM[in] = v
	case 1:
		p.ntRAM[0x400+in] = v
	case 2:
		if cart.mmc5ExRAMMode != 3 {
			cart.mmc5ExRAM[in] = v
		}
	case 3:
		// Fill mode ignores nametable writes.
	}
}

func (p *ppu) step(c *Console, cycles int) bool {
	nmi := false
	for i := 0; i < cycles*3; i++ {
		p.cycle++
		rendering := p.mask&0x18 != 0
		if p.shouldSkipOddFrameDot(rendering) {
			p.startNextFrame(c)
			continue
		}
		if p.scanline < 240 && p.cycle == 1 {
			start := p.scanline * FrameWidth
			end := start + FrameWidth
			for idx := start; idx < end; idx++ {
				p.frameBGOpaq[idx] = false
			}
		}
		if p.scanline < 240 && p.cycle >= 1 && p.cycle <= 256 {
			p.renderLiveBackgroundPixel(c, p.scanline, p.cycle-1)
			p.renderLiveSpritePixel(c, p.scanline, p.cycle-1)
		}
		if rendering && (p.scanline < 240 || p.scanline == 261) {
			if ((p.cycle >= 1 && p.cycle <= 256) || (p.cycle >= 321 && p.cycle <= 336)) && p.cycle%8 == 0 {
				p.incrementCoarseX()
			}
			if p.cycle == 256 {
				p.incrementY()
			}
			if p.cycle == 257 {
				p.copyHorizontalBits()
			}
			if p.scanline == 261 && p.cycle >= 280 && p.cycle <= 304 {
				p.copyVerticalBits()
			}
		}
		if p.cycle == 1 {
			switch {
			case p.scanline < 240:
				p.captureLineState(c, p.scanline)
				if p.countSpritesOnLine(c, p.scanline) > 8 {
					p.status |= 0x20
				}
				if c != nil && c.cart != nil && c.cart.Mapper == 5 {
					c.cart.mmc5ClockScanline()
				}
			case p.scanline == 241:
				if c != nil && c.cart != nil && len(c.lastFrame) >= FrameSizeRGB {
					// Capture completed visible frame before vblank-time register updates mutate line state.
					p.renderFrame(c, c.lastFrame)
					copy(p.frameRGB, c.lastFrame)
				}
				p.status |= 0x80
				if p.ctrl&0x80 != 0 {
					nmi = true
				}
			case p.scanline == 261:
				p.status &^= 0xE0
			}
		}
		if p.cycle == 260 && c != nil && c.cart != nil && c.cart.Mapper == 4 {
			if p.shouldClockMMC3IRQ(rendering) {
				c.cart.mmc3ClockIRQ()
			}
		}
		if p.cycle == 260 && c != nil && c.cart != nil && (c.cart.Mapper == 23 || c.cart.Mapper == 25) {
			if p.scanline < 240 {
				c.cart.vrcClockIRQ()
			}
		}
		if p.cycle >= 341 {
			p.cycle = 0
			p.scanline++
			if p.scanline >= 262 {
				p.startNextFrame(c)
			}
		}
	}
	return nmi
}

func (p *ppu) shouldSkipOddFrameDot(rendering bool) bool {
	return rendering && p.scanline == 261 && p.cycle == 340 && p.frameID%2 == 1
}

func (p *ppu) shouldClockMMC3IRQ(rendering bool) bool {
	if !rendering {
		return false
	}
	if p.scanline >= 240 && p.scanline != 261 {
		return false
	}
	bgFetchHigh := p.mask&0x08 != 0 && p.ctrl&0x10 != 0
	if bgFetchHigh {
		return true
	}
	if p.mask&0x10 == 0 {
		return false
	}
	if p.ctrl&0x20 != 0 {
		return true
	}
	return p.ctrl&0x08 != 0
}

func (p *ppu) startNextFrame(c *Console) {
	if c != nil && c.cart != nil && c.cart.Mapper == 5 {
		c.cart.mmc5EndFrame()
	}
	p.scanline = 0
	p.cycle = 0
	p.frameID++
}

func (p *ppu) renderFrame(c *Console, dst []byte) {
	if len(dst) < FrameSizeRGB {
		return
	}
	bgOpaque := p.frameBGOpaq
	for i := range bgOpaque {
		bgOpaque[i] = false
	}
	for y := 0; y < FrameHeight; y++ {
		state, segmentStartX := p.renderSegmentForPixel(c, y, 0)
		showBG := state.mask&0x08 != 0
		for x := 0; x < FrameWidth; x++ {
			state, segmentStartX = p.renderSegmentForPixel(c, y, x)
			showBG = state.mask&0x08 != 0
			showBGLeft := state.mask&0x02 != 0
			bgPatternBase := uint16(0x0000)
			if state.ctrl&0x10 != 0 {
				bgPatternBase = 0x1000
			}
			renderAddr := state.vramAddr
			if state.prefetched {
				renderAddr = rewindHorizontalTiles(renderAddr, 2)
			}
			scrollX, scrollY := decodeScroll(renderAddr, state.fineX)
			worldY := scrollY
			ntY := (worldY / 240) & 0x01
			tileY := (worldY % 240) / 8
			fineY := worldY % 8
			o := (y*FrameWidth + x) * 3
			backdrop := nesPaletteRGB[p.palette[0]&0x3F]
			if !showBG || (x < 8 && !showBGLeft) {
				dst[o+0] = backdrop[0]
				dst[o+1] = backdrop[1]
				dst[o+2] = backdrop[2]
				continue
			}
			localX := x - segmentStartX
			if localX < 0 {
				localX = 0
			}
			worldX := localX + scrollX
			ntX := (worldX / 256) & 0x01
			tileX := (worldX % 256) / 8
			fineX := worldX % 8
			ntBase := uint16(0x2000 + ((ntY<<1 | ntX) * 0x400))

			ntIndex := uint16(tileY*32 + tileX)
			tileID := p.readNametableWithState(c, state, ntBase+ntIndex)
			ptAddr := bgPatternBase + uint16(tileID)*16 + uint16(fineY)
			lo := p.readCHRWithState(c, state, ptAddr)
			hi := p.readCHRWithState(c, state, ptAddr+8)
			shift := byte(7 - fineX)
			pix := ((hi >> shift) & 0x01) << 1
			pix |= (lo >> shift) & 0x01

			attrAddr := ntBase + 0x03C0 + uint16((tileY/4)*8+(tileX/4))
			attr := p.readNametableWithState(c, state, attrAddr)
			quad := ((tileY % 4) / 2) * 2
			quad += (tileX % 4) / 2
			palHi := (attr >> (quad * 2)) & 0x03

			var palIndex byte
			if pix == 0 {
				palIndex = p.palette[0] & 0x3F
			} else {
				palSlot := int(palHi*4 + pix)
				palIndex = p.palette[palSlot&0x1F] & 0x3F
				bgOpaque[y*FrameWidth+x] = true
			}
			rgb := applyPPUMaskRGB(state.mask, nesPaletteRGB[palIndex])
			dst[o+0] = rgb[0]
			dst[o+1] = rgb[1]
			dst[o+2] = rgb[2]
		}
	}
	p.renderSprites(c, dst, bgOpaque)
}

func (p *ppu) renderLiveBackgroundPixel(c *Console, y int, x int) {
	if y < 0 || y >= FrameHeight || x < 0 || x >= FrameWidth {
		return
	}
	o := (y*FrameWidth + x) * 3
	backdrop := nesPaletteRGB[p.palette[0]&0x3F]
	showBG := p.mask&0x08 != 0
	showBGLeft := p.mask&0x02 != 0
	if !showBG || (x < 8 && !showBGLeft) {
		p.frameRGB[o+0] = backdrop[0]
		p.frameRGB[o+1] = backdrop[1]
		p.frameRGB[o+2] = backdrop[2]
		return
	}

	bgPatternBase := uint16(0x0000)
	if p.ctrl&0x10 != 0 {
		bgPatternBase = 0x1000
	}
	renderAddr := rewindHorizontalTiles(p.vramAddr, 2)
	scrollX, scrollY := decodeScroll(renderAddr, p.fineX)
	worldX := scrollX + ((p.cycle - 1) & 0x07)
	worldY := scrollY
	ntY := (worldY / 240) & 0x01
	tileY := (worldY % 240) / 8
	fineY := worldY % 8
	ntX := (worldX / 256) & 0x01
	tileX := (worldX % 256) / 8
	fineX := worldX % 8
	ntBase := uint16(0x2000 + ((ntY<<1 | ntX) * 0x400))

	ntIndex := uint16(tileY*32 + tileX)
	tileID := p.readNametableCurrent(c, ntBase+ntIndex)
	ptAddr := bgPatternBase + uint16(tileID)*16 + uint16(fineY)
	lo := p.readCHRCurrent(c, ptAddr)
	hi := p.readCHRCurrent(c, ptAddr+8)
	shift := byte(7 - fineX)
	pix := ((hi >> shift) & 0x01) << 1
	pix |= (lo >> shift) & 0x01

	attrAddr := ntBase + 0x03C0 + uint16((tileY/4)*8+(tileX/4))
	attr := p.readNametableCurrent(c, attrAddr)
	quad := ((tileY % 4) / 2) * 2
	quad += (tileX % 4) / 2
	palHi := (attr >> (quad * 2)) & 0x03

	var palIndex byte
	if pix == 0 {
		palIndex = p.palette[0] & 0x3F
		p.frameBGOpaq[y*FrameWidth+x] = false
	} else {
		palSlot := int(palHi*4 + pix)
		palIndex = p.palette[palSlot&0x1F] & 0x3F
		p.frameBGOpaq[y*FrameWidth+x] = true
	}
	rgb := applyPPUMaskRGB(p.mask, nesPaletteRGB[palIndex])
	p.frameRGB[o+0] = rgb[0]
	p.frameRGB[o+1] = rgb[1]
	p.frameRGB[o+2] = rgb[2]
}

func (p *ppu) renderLiveSpritePixel(c *Console, y int, x int) {
	if y < 0 || y >= FrameHeight || x < 0 || x >= FrameWidth {
		return
	}
	if p.mask&0x10 == 0 {
		return
	}
	if x < 8 && p.mask&0x04 == 0 {
		return
	}
	spriteHeight := 8
	sprite8x16 := p.ctrl&0x20 != 0
	spritePatternBase := uint16(0x0000)
	if !sprite8x16 && p.ctrl&0x08 != 0 {
		spritePatternBase = 0x1000
	}
	if sprite8x16 {
		spriteHeight = 16
	}
	for i := 0; i < 64; i++ {
		oam := i * 4
		sy := int(p.oam[oam]) + 1
		if y < sy || y >= sy+spriteHeight {
			continue
		}
		sx := int(p.oam[oam+3])
		if x < sx || x >= sx+8 {
			continue
		}
		tile := p.oam[oam+1]
		attr := p.oam[oam+2]
		srcY := y - sy
		if attr&0x80 != 0 {
			srcY = spriteHeight - 1 - srcY
		}
		srcX := x - sx
		if attr&0x40 != 0 {
			srcX = 7 - srcX
		}
		addr := spritePatternBase + uint16(tile)*16 + uint16(srcY)
		if sprite8x16 {
			tileBase := tile &^ 0x01
			patternBase := uint16(tile&0x01) << 12
			tileOffset := byte(0)
			row := srcY
			if row >= 8 {
				tileOffset = 1
				row -= 8
			}
			addr = patternBase + uint16(tileBase+tileOffset)*16 + uint16(row)
		}
		lo := p.readCHRCurrent(c, addr)
		hi := p.readCHRCurrent(c, addr+8)
		shift := byte(7 - srcX)
		pix := ((hi >> shift) & 0x01) << 1
		pix |= (lo >> shift) & 0x01
		if pix == 0 {
			continue
		}
		if i == 0 && x < FrameWidth-1 && p.frameBGOpaq[y*FrameWidth+x] {
			p.status |= 0x40
		}
		if attr&0x20 != 0 && p.frameBGOpaq[y*FrameWidth+x] {
			return
		}
		pal := attr & 0x03
		palSlot := 0x10 + int(pal*4+pix)
		palIndex := p.palette[palSlot&0x1F] & 0x3F
		rgb := applyPPUMaskRGB(p.mask, nesPaletteRGB[palIndex])
		o := (y*FrameWidth + x) * 3
		p.frameRGB[o+0] = rgb[0]
		p.frameRGB[o+1] = rgb[1]
		p.frameRGB[o+2] = rgb[2]
		return
	}
}

func (p *ppu) captureLineState(c *Console, line int) {
	if line < 0 || line >= FrameHeight {
		return
	}
	st := scanlineRenderState{
		valid:      true,
		prefetched: p.cycle == 1,
		ctrl:       p.ctrl,
		mask:       p.mask,
		vramAddr:   p.vramAddr,
		fineX:      p.fineX,
		mirroring:  MirroringHorizontal,
	}
	if c != nil && c.cart != nil {
		st.mirroring = c.cart.mirroring
		st.mapper = c.cart.Mapper
		st.mapper3CHRSel = c.cart.mapper3CHRSel
		st.mapper33CHR = c.cart.mapper33CHR
		st.mapper66CHRSel = c.cart.mapper66CHRSel
		st.mapper75CHR = c.cart.mapper75CHR
		st.mapper87CHRSel = c.cart.mapper87CHRSel
		st.mmc5CHRMode = c.cart.mmc5CHRMode
		st.mmc5CHRBank = c.cart.mmc5CHRBank
		st.mmc5UpperCHR = c.cart.mmc5UpperCHR
		st.mmc5ExRAMMode = c.cart.mmc5ExRAMMode
		st.mmc5ExRAM = c.cart.mmc5ExRAM
		st.mmc5FillTile = c.cart.mmc5FillTile
		st.mmc5FillAttr = c.cart.mmc5FillAttr
		st.mmc5NTMap = c.cart.mmc5NTMap
		for i := range st.vrcCHR {
			st.vrcCHR[i] = c.cart.vrcCHRBank(uint16(i << 10))
		}
		st.vrcMirroring = c.cart.mirroring
		st.mmc1Control = c.cart.mmc1Control
		st.mmc1CHRBank0 = c.cart.mmc1CHRBank0
		st.mmc1CHRBank1 = c.cart.mmc1CHRBank1
		st.mmc3BankSelect = c.cart.mmc3BankSelect
		st.mmc3Regs = c.cart.mmc3Regs
	}
	p.lineState[line] = st
	p.lineSplitN[line] = 1
	p.lineSplits[line][0] = scanlineStateSegment{startX: 0, state: st}
}

func (p *ppu) renderStateForLine(c *Console, line int) scanlineRenderState {
	if line >= 0 && line < FrameHeight && p.lineState[line].valid {
		return p.lineState[line]
	}
	p.captureLineState(c, line)
	return p.lineState[line]
}

func (p *ppu) renderStateForPixel(c *Console, line int, x int) scanlineRenderState {
	state, _ := p.renderSegmentForPixel(c, line, x)
	return state
}

func (p *ppu) renderSegmentForPixel(c *Console, line int, x int) (scanlineRenderState, int) {
	if line < 0 || line >= FrameHeight {
		return scanlineRenderState{}, 0
	}
	if !p.lineState[line].valid {
		p.captureLineState(c, line)
	}
	count := p.lineSplitN[line]
	if count == 0 {
		return p.lineState[line], 0
	}
	state := p.lineSplits[line][0].state
	startX := p.lineSplits[line][0].startX
	for i := 1; i < count; i++ {
		if x < p.lineSplits[line][i].startX {
			break
		}
		state = p.lineSplits[line][i].state
		startX = p.lineSplits[line][i].startX
	}
	return state, startX
}

func (p *ppu) recordCurrentLineState(c *Console, delayPixels int) {
	if p.scanline < 0 || p.scanline >= FrameHeight {
		return
	}
	if p.cycle < 1 || p.cycle > 256 {
		return
	}
	st := p.captureState(c)
	line := p.scanline
	startX := p.cycle - 1 + delayPixels
	if startX < 0 {
		startX = 0
	}
	if delayPixels > 0 {
		startX = ((startX + 7) / 8) * 8
	}
	if startX > FrameWidth {
		startX = FrameWidth
	}
	if !p.lineState[line].valid {
		p.lineState[line] = st
		p.lineSplitN[line] = 1
		p.lineSplits[line][0] = scanlineStateSegment{startX: 0, state: st}
		return
	}
	n := p.lineSplitN[line]
	if n > 0 && p.lineSplits[line][n-1].startX == startX {
		p.lineSplits[line][n-1].state = st
		return
	}
	if n < len(p.lineSplits[line]) {
		p.lineSplits[line][n] = scanlineStateSegment{startX: startX, state: st}
		p.lineSplitN[line]++
		return
	}
	// Keep the newest state reachable on the right edge even when we hit segment capacity.
	last := len(p.lineSplits[line]) - 1
	if startX < p.lineSplits[line][last].startX {
		startX = p.lineSplits[line][last].startX
	}
	p.lineSplits[line][last] = scanlineStateSegment{startX: startX, state: st}
}

func (p *ppu) captureState(c *Console) scanlineRenderState {
	st := scanlineRenderState{
		valid:      true,
		prefetched: p.cycle == 1,
		ctrl:       p.ctrl,
		mask:       p.mask,
		vramAddr:   p.vramAddr,
		fineX:      p.fineX,
		mirroring:  MirroringHorizontal,
	}
	if c != nil && c.cart != nil {
		st.mirroring = c.cart.mirroring
		st.mapper = c.cart.Mapper
		st.mapper3CHRSel = c.cart.mapper3CHRSel
		st.mapper33CHR = c.cart.mapper33CHR
		st.mapper66CHRSel = c.cart.mapper66CHRSel
		st.mapper75CHR = c.cart.mapper75CHR
		st.mapper87CHRSel = c.cart.mapper87CHRSel
		st.mmc5CHRMode = c.cart.mmc5CHRMode
		st.mmc5CHRBank = c.cart.mmc5CHRBank
		st.mmc5UpperCHR = c.cart.mmc5UpperCHR
		st.mmc5ExRAMMode = c.cart.mmc5ExRAMMode
		st.mmc5ExRAM = c.cart.mmc5ExRAM
		st.mmc5FillTile = c.cart.mmc5FillTile
		st.mmc5FillAttr = c.cart.mmc5FillAttr
		st.mmc5NTMap = c.cart.mmc5NTMap
		for i := range st.vrcCHR {
			st.vrcCHR[i] = c.cart.vrcCHRBank(uint16(i << 10))
		}
		st.vrcMirroring = c.cart.mirroring
		st.mmc1Control = c.cart.mmc1Control
		st.mmc1CHRBank0 = c.cart.mmc1CHRBank0
		st.mmc1CHRBank1 = c.cart.mmc1CHRBank1
		st.mmc3BankSelect = c.cart.mmc3BankSelect
		st.mmc3Regs = c.cart.mmc3Regs
	}
	return st
}

func (p *ppu) incrementCoarseX() {
	if p.vramAddr&0x001F == 31 {
		p.vramAddr &^= 0x001F
		p.vramAddr ^= 0x0400
		return
	}
	p.vramAddr++
}

func (p *ppu) incrementY() {
	if p.vramAddr&0x7000 != 0x7000 {
		p.vramAddr += 0x1000
		return
	}
	p.vramAddr &^= 0x7000
	y := (p.vramAddr & 0x03E0) >> 5
	switch y {
	case 29:
		y = 0
		p.vramAddr ^= 0x0800
	case 31:
		y = 0
	default:
		y++
	}
	p.vramAddr = (p.vramAddr &^ 0x03E0) | (y << 5)
}

func (p *ppu) copyHorizontalBits() {
	p.vramAddr = (p.vramAddr &^ 0x041F) | (p.tempAddr & 0x041F)
}

func (p *ppu) copyVerticalBits() {
	p.vramAddr = (p.vramAddr &^ 0x7BE0) | (p.tempAddr & 0x7BE0)
}

func decodeScroll(vramAddr uint16, fineX byte) (int, int) {
	scrollX := int(vramAddr&0x001F)<<3 | int(fineX&0x07)
	scrollY := int((vramAddr>>5)&0x001F)<<3 | int((vramAddr>>12)&0x07)
	if vramAddr&0x0400 != 0 {
		scrollX += 256
	}
	if vramAddr&0x0800 != 0 {
		scrollY += 240
	}
	return scrollX, scrollY
}

func rewindHorizontalTiles(vramAddr uint16, tiles int) uint16 {
	for i := 0; i < tiles; i++ {
		if vramAddr&0x001F == 0 {
			vramAddr = (vramAddr &^ 0x001F) | 0x001F
			vramAddr ^= 0x0400
			continue
		}
		vramAddr--
	}
	return vramAddr
}

func (p *ppu) readNametableWithState(c *Console, st scanlineRenderState, addr uint16) byte {
	a := addr & 0x3FFF
	switch {
	case a < 0x2000:
		return p.readCHRWithState(c, st, a)
	case a < 0x3F00:
		if st.mapper == 5 {
			return p.readNametableMapper5(st, a)
		}
		idx := mirrorNametableAddrMode(st.mirroring, a)
		return p.ntRAM[idx]
	default:
		idx := mirrorPaletteAddr(a)
		return p.palette[idx]
	}
}

func (p *ppu) readNametableCurrent(c *Console, addr uint16) byte {
	a := addr & 0x3FFF
	switch {
	case a < 0x2000:
		return p.readCHRCurrent(c, a)
	case a < 0x3F00:
		if c != nil && c.cart != nil && c.cart.Mapper == 5 {
			st := p.captureState(c)
			return p.readNametableMapper5(st, a)
		}
		idx := mirrorNametableAddr(c, a)
		return p.ntRAM[idx]
	default:
		idx := mirrorPaletteAddr(a)
		return p.palette[idx]
	}
}

func (p *ppu) readCHRCurrent(c *Console, addr uint16) byte {
	if c == nil || c.cart == nil {
		return 0
	}
	return c.cart.readCHR(addr & 0x1FFF)
}

func (p *ppu) readCHRWithState(c *Console, st scanlineRenderState, addr uint16) byte {
	if c == nil || c.cart == nil || len(c.cart.CHR) == 0 {
		return 0
	}
	a := int(addr & 0x1FFF)
	switch st.mapper {
	case 1:
		if st.mmc1Control&0x10 == 0 {
			bank8k := int(st.mmc1CHRBank0&0x1E) % max(1, len(c.cart.CHR)/(8*1024))
			base := bank8k * 8 * 1024
			return c.cart.CHR[base+a]
		}
		if a < 0x1000 {
			bank4k := int(st.mmc1CHRBank0) % max(1, len(c.cart.CHR)/(4*1024))
			base := bank4k * 4 * 1024
			return c.cart.CHR[base+a]
		}
		bank4k := int(st.mmc1CHRBank1) % max(1, len(c.cart.CHR)/(4*1024))
		base := bank4k * 4 * 1024
		return c.cart.CHR[base+int(addr&0x0FFF)]
	case 3:
		bank := int(st.mapper3CHRSel) % max(1, c.cart.CHRBanks)
		base := bank * 8 * 1024
		return c.cart.CHR[base+a]
	case 4:
		return readCHRMapper4State(c.cart.CHR, st, a)
	case 33:
		return readCHRMapper33State(c.cart.CHR, st, a)
	case 66:
		bank := int(st.mapper66CHRSel) % max(1, len(c.cart.CHR)/(8*1024))
		return c.cart.CHR[bank*8*1024+a]
	case 75:
		return readCHRMapper75State(c.cart.CHR, st, a)
	case 87:
		bank := int(st.mapper87CHRSel) % max(1, len(c.cart.CHR)/(8*1024))
		return c.cart.CHR[bank*8*1024+a]
	case 88:
		return readCHRMapper206State(c.cart.CHR, st, a, true)
	case 206:
		return readCHRMapper206State(c.cart.CHR, st, a, false)
	case 5:
		return readCHRMapper5State(c.cart.CHR, st, a)
	case 23, 25:
		bank := int(st.vrcCHR[(a>>10)&0x07]) % max(1, len(c.cart.CHR)/1024)
		return c.cart.CHR[bank*1024+(a&0x03FF)]
	default:
		return c.cart.CHR[a%len(c.cart.CHR)]
	}
}

func (p *ppu) readNametableMapper5(st scanlineRenderState, addr uint16) byte {
	off := int((addr - 0x2000) & 0x0FFF)
	table := (off / 0x400) & 0x03
	in := off & 0x03FF
	switch st.mmc5NTMap[table] {
	case 0:
		return p.ntRAM[in]
	case 1:
		return p.ntRAM[0x400+in]
	case 2:
		if st.mmc5ExRAMMode != 0x02 && st.mmc5ExRAMMode != 0x01 {
			return 0
		}
		return st.mmc5ExRAM[in]
	default:
		if in >= 0x3C0 {
			return st.mmc5FillAttr * 0x55
		}
		return st.mmc5FillTile
	}
}

func readCHRMapper5State(chr []byte, st scanlineRenderState, addr int) byte {
	num1k := max(1, len(chr)/1024)
	slot := (addr >> 10) & 0x07
	bank := 0
	switch st.mmc5CHRMode & 0x03 {
	case 0:
		bank = int(st.mmc5UpperCHR)<<8 | int(st.mmc5CHRBank[7])
		bank += slot
	case 1:
		group := (slot / 4) * 4
		bank = int(st.mmc5UpperCHR)<<8 | int(st.mmc5CHRBank[group+3])
		bank += slot & 0x03
	case 2:
		group := (slot / 2) * 2
		bank = int(st.mmc5UpperCHR)<<8 | int(st.mmc5CHRBank[group+1])
		bank += slot & 0x01
	default:
		bank = int(st.mmc5UpperCHR)<<8 | int(st.mmc5CHRBank[slot])
	}
	bank %= num1k
	return chr[bank*1024+(addr&0x03FF)]
}

func readCHRMapper33State(chr []byte, st scanlineRenderState, addr int) byte {
	num1k := max(1, len(chr)/1024)
	var bank int
	switch {
	case addr < 0x0800:
		bank = int(st.mapper33CHR[0] &^ 1)
		bank += addr / 0x0400
	case addr < 0x1000:
		bank = int(st.mapper33CHR[1] &^ 1)
		bank += (addr - 0x0800) / 0x0400
	case addr < 0x1400:
		bank = int(st.mapper33CHR[2])
	case addr < 0x1800:
		bank = int(st.mapper33CHR[3])
	case addr < 0x1C00:
		bank = int(st.mapper33CHR[4])
	default:
		bank = int(st.mapper33CHR[5])
	}
	bank %= num1k
	return chr[bank*1024+(addr&0x03FF)]
}

func readCHRMapper4State(chr []byte, st scanlineRenderState, addr int) byte {
	num1k := max(1, len(chr)/1024)
	chrMode := (st.mmc3BankSelect >> 7) & 1
	bank := 0
	a := addr & 0x1FFF

	if chrMode == 0 {
		switch {
		case a < 0x0800:
			bank = int(st.mmc3Regs[0] &^ 1)
			bank += a / 0x0400
		case a < 0x1000:
			bank = int(st.mmc3Regs[1] &^ 1)
			bank += (a - 0x0800) / 0x0400
		case a < 0x1400:
			bank = int(st.mmc3Regs[2])
		case a < 0x1800:
			bank = int(st.mmc3Regs[3])
		case a < 0x1C00:
			bank = int(st.mmc3Regs[4])
		default:
			bank = int(st.mmc3Regs[5])
		}
	} else {
		switch {
		case a < 0x0400:
			bank = int(st.mmc3Regs[2])
		case a < 0x0800:
			bank = int(st.mmc3Regs[3])
		case a < 0x0C00:
			bank = int(st.mmc3Regs[4])
		case a < 0x1000:
			bank = int(st.mmc3Regs[5])
		case a < 0x1800:
			bank = int(st.mmc3Regs[0] &^ 1)
			bank += (a - 0x1000) / 0x0400
		default:
			bank = int(st.mmc3Regs[1] &^ 1)
			bank += (a - 0x1800) / 0x0400
		}
	}
	bank %= num1k
	return chr[bank*1024+(a&0x03FF)]
}

func readCHRMapper75State(chr []byte, st scanlineRenderState, addr int) byte {
	num4k := max(1, len(chr)/(4*1024))
	if addr < 0x1000 {
		bank := int(st.mapper75CHR[0]) % num4k
		return chr[bank*4*1024+addr]
	}
	bank := int(st.mapper75CHR[1]) % num4k
	return chr[bank*4*1024+(addr&0x0FFF)]
}

func readCHRMapper206State(chr []byte, st scanlineRenderState, addr int, mapper88 bool) byte {
	num1k := max(1, len(chr)/1024)
	var bank int
	switch {
	case addr < 0x0800:
		bank = int(st.mmc3Regs[0] & 0x3E)
		bank += addr / 0x0400
	case addr < 0x1000:
		bank = int(st.mmc3Regs[1] & 0x3E)
		bank += (addr - 0x0800) / 0x0400
	case addr < 0x1400:
		bank = int(st.mmc3Regs[2])
	case addr < 0x1800:
		bank = int(st.mmc3Regs[3])
	case addr < 0x1C00:
		bank = int(st.mmc3Regs[4])
	default:
		bank = int(st.mmc3Regs[5])
	}
	if mapper88 && addr >= 0x1000 {
		bank |= 0x40
	}
	bank %= num1k
	return chr[bank*1024+(addr&0x03FF)]
}

func (p *ppu) renderSprites(c *Console, dst []byte, bgOpaque []bool) {
	for i := 63; i >= 0; i-- {
		o := i * 4
		sy := int(p.oam[o]) + 1
		tile := p.oam[o+1]
		attr := p.oam[o+2]
		sx := int(p.oam[o+3])
		flipV := attr&0x80 != 0
		flipH := attr&0x40 != 0
		behindBG := attr&0x20 != 0
		pal := attr & 0x03

		for py := 0; py < 16; py++ {
			y := sy + py
			if y < 0 || y >= FrameHeight {
				continue
			}
			state := p.renderStateForLine(c, y)
			if state.mask&0x10 == 0 {
				continue
			}
			spriteHeight := 8
			sprite8x16 := state.ctrl&0x20 != 0
			spritePatternBase := uint16(0x0000)
			if !sprite8x16 && state.ctrl&0x08 != 0 {
				spritePatternBase = 0x1000
			}
			if sprite8x16 {
				spriteHeight = 16
			}
			if py >= spriteHeight {
				continue
			}
			srcY := py
			if flipV {
				srcY = spriteHeight - 1 - py
			}
			addr := spritePatternBase + uint16(tile)*16 + uint16(srcY)
			if sprite8x16 {
				tileBase := tile &^ 0x01
				patternBase := uint16(tile&0x01) << 12
				tileOffset := byte(0)
				row := srcY
				if row >= 8 {
					tileOffset = 1
					row -= 8
				}
				addr = patternBase + uint16(tileBase+tileOffset)*16 + uint16(row)
			}
			lo := p.readCHRWithState(c, state, addr)
			hi := p.readCHRWithState(c, state, addr+8)
			for px := 0; px < 8; px++ {
				x := sx + px
				if x < 0 || x >= FrameWidth {
					continue
				}
				if x < 8 && state.mask&0x04 == 0 {
					continue
				}
				srcX := px
				if flipH {
					srcX = 7 - px
				}
				shift := byte(7 - srcX)
				pix := ((hi >> shift) & 0x01) << 1
				pix |= (lo >> shift) & 0x01
				if pix == 0 {
					continue
				}
				if behindBG && bgOpaque[y*FrameWidth+x] {
					continue
				}
				palSlot := 0x10 + int(pal*4+pix)
				palIndex := p.palette[palSlot&0x1F] & 0x3F
				rgb := applyPPUMaskRGB(state.mask, nesPaletteRGB[palIndex])
				di := (y*FrameWidth + x) * 3
				dst[di+0] = rgb[0]
				dst[di+1] = rgb[1]
				dst[di+2] = rgb[2]
			}
		}
	}
}

func (p *ppu) countSpritesOnLine(c *Console, line int) int {
	spriteHeight := 8
	state := p.renderStateForLine(c, line)
	if state.ctrl&0x20 != 0 {
		spriteHeight = 16
	}
	count := 0
	for i := 0; i < 64; i++ {
		y := int(p.oam[i*4]) + 1
		if line >= y && line < y+spriteHeight {
			count++
		}
	}
	return count
}

func applyPPUMaskRGB(mask byte, rgb [3]byte) [3]byte {
	out := rgb
	if mask&0x01 != 0 {
		luma := byte((uint16(out[0]) + uint16(out[1]) + uint16(out[2])) / 3)
		out[0], out[1], out[2] = luma, luma, luma
	}
	rFactor, gFactor, bFactor := emphasisFactors(mask)
	out[0] = scaleColor(out[0], rFactor)
	out[1] = scaleColor(out[1], gFactor)
	out[2] = scaleColor(out[2], bFactor)
	return out
}

func emphasisFactors(mask byte) (float64, float64, float64) {
	rFactor, gFactor, bFactor := 1.0, 1.0, 1.0
	if mask&0x20 != 0 {
		rFactor *= 1.12
		gFactor *= 0.92
		bFactor *= 0.92
	}
	if mask&0x40 != 0 {
		rFactor *= 0.92
		gFactor *= 1.12
		bFactor *= 0.92
	}
	if mask&0x80 != 0 {
		rFactor *= 0.92
		gFactor *= 0.92
		bFactor *= 1.12
	}
	return rFactor, gFactor, bFactor
}

func scaleColor(v byte, factor float64) byte {
	scaled := float64(v) * factor
	if scaled < 0 {
		return 0
	}
	if scaled > 255 {
		return 255
	}
	return byte(scaled)
}

func (p *ppu) computeSprite0HitX(c *Console, line int) int {
	if c == nil || c.cart == nil || line < 0 || line >= FrameHeight {
		return -1
	}
	state := p.renderStateForLine(c, line)
	if state.mask&0x18 != 0x18 {
		return -1
	}
	spriteY := int(p.oam[0]) + 1
	spriteTile := p.oam[1]
	spriteAttr := p.oam[2]
	spriteX := int(p.oam[3])
	spriteHeight := 8
	sprite8x16 := state.ctrl&0x20 != 0
	if sprite8x16 {
		spriteHeight = 16
	}
	if line < spriteY || line >= spriteY+spriteHeight {
		return -1
	}
	spriteRow := line - spriteY
	if spriteAttr&0x80 != 0 {
		spriteRow = spriteHeight - 1 - spriteRow
	}
	renderAddr := state.vramAddr
	if state.prefetched {
		renderAddr = rewindHorizontalTiles(renderAddr, 2)
	}
	scrollX, scrollY := decodeScroll(renderAddr, state.fineX)
	worldY := scrollY
	ntY := (worldY / 240) & 0x01
	tileY := (worldY % 240) / 8
	fineY := worldY % 8

	for px := 0; px < 8; px++ {
		x := spriteX + px
		if x < 0 || x >= FrameWidth-1 {
			continue
		}
		if x < 8 && (state.mask&0x06) != 0x06 {
			continue
		}
		srcX := px
		if spriteAttr&0x40 != 0 {
			srcX = 7 - px
		}
		if p.spritePixelOpaque(c, state, spriteTile, spriteRow, srcX, sprite8x16) && p.backgroundPixelOpaque(c, state, x, ntY, tileY, fineY, scrollX) {
			return x
		}
	}
	return -1
}

func (p *ppu) backgroundPixelOpaque(c *Console, st scanlineRenderState, x int, ntY int, tileY int, fineY int, scrollX int) bool {
	worldX := x + scrollX
	ntX := (worldX / 256) & 0x01
	tileX := (worldX % 256) / 8
	fineX := worldX % 8
	ntBase := uint16(0x2000 + ((ntY<<1 | ntX) * 0x400))
	ntIndex := uint16(tileY*32 + tileX)
	tileID := p.readNametableWithState(c, st, ntBase+ntIndex)
	bgPatternBase := uint16(0x0000)
	if st.ctrl&0x10 != 0 {
		bgPatternBase = 0x1000
	}
	ptAddr := bgPatternBase + uint16(tileID)*16 + uint16(fineY)
	lo := p.readCHRWithState(c, st, ptAddr)
	hi := p.readCHRWithState(c, st, ptAddr+8)
	shift := byte(7 - fineX)
	pix := ((hi >> shift) & 0x01) << 1
	pix |= (lo >> shift) & 0x01
	return pix != 0
}

func (p *ppu) spritePixelOpaque(c *Console, st scanlineRenderState, tile byte, spriteRow int, srcX int, sprite8x16 bool) bool {
	addr := uint16(tile)*16 + uint16(spriteRow)
	if sprite8x16 {
		tileBase := tile &^ 0x01
		patternBase := uint16(tile&0x01) << 12
		tileOffset := byte(0)
		row := spriteRow
		if row >= 8 {
			tileOffset = 1
			row -= 8
		}
		addr = patternBase + uint16(tileBase+tileOffset)*16 + uint16(row)
	} else if st.ctrl&0x08 != 0 {
		addr += 0x1000
	}
	lo := p.readCHRWithState(c, st, addr)
	hi := p.readCHRWithState(c, st, addr+8)
	shift := byte(7 - srcX)
	pix := ((hi >> shift) & 0x01) << 1
	pix |= (lo >> shift) & 0x01
	return pix != 0
}

func mirrorNametableAddr(c *Console, addr uint16) int {
	mode := MirroringHorizontal
	if c != nil && c.cart != nil {
		mode = c.cart.mirroring
	}
	return mirrorNametableAddrMode(mode, addr)
}

func mirrorNametableAddrMode(mode MirroringMode, addr uint16) int {
	off := (addr - 0x2000) & 0x0FFF
	table := (off / 0x400) & 0x03
	in := off & 0x03FF
	switch mode {
	case MirroringVertical:
		if table == 0 || table == 2 {
			return int(in)
		}
		return int(0x400 + in)
	case MirroringOneScreenLow:
		return int(in)
	case MirroringOneScreenHigh:
		return int(0x400 + in)
	case MirroringFourScreen:
		return int(off)
	default: // Horizontal
		if table == 0 || table == 1 {
			return int(in)
		}
		return int(0x400 + in)
	}
}

func mirrorPaletteAddr(addr uint16) int {
	i := int((addr - 0x3F00) & 0x1F)
	switch i {
	case 0x10:
		return 0x00
	case 0x14:
		return 0x04
	case 0x18:
		return 0x08
	case 0x1C:
		return 0x0C
	default:
		return i
	}
}
