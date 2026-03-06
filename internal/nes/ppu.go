package nes

type ppu struct {
	ctrl      byte
	mask      byte
	status    byte
	oamAddr   byte
	oam       [256]byte
	vramAddr  uint16
	addrLatch bool
	scrollX   byte
	scrollY   byte
	readBuf   byte
	ntRAM     [2048]byte
	palette   [32]byte
	cycle     int
	scanline  int
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
	p := &ppu{}
	p.Reset()
	return p
}

func (p *ppu) Reset() {
	p.ctrl = 0
	p.mask = 0
	p.status = 0
	p.oamAddr = 0
	p.vramAddr = 0
	p.addrLatch = false
	p.scrollX = 0
	p.scrollY = 0
	p.readBuf = 0
	p.cycle = 0
	p.scanline = 0
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
	switch addr & 0x2007 {
	case 0x2000:
		p.ctrl = v
	case 0x2001:
		p.mask = v
	case 0x2003:
		p.oamAddr = v
	case 0x2004:
		p.oam[p.oamAddr] = v
		p.oamAddr++
	case 0x2005:
		if !p.addrLatch {
			p.scrollX = v
			p.addrLatch = true
		} else {
			p.scrollY = v
			p.addrLatch = false
		}
	case 0x2006:
		if !p.addrLatch {
			p.vramAddr = (uint16(v) << 8) | (p.vramAddr & 0x00FF)
			p.addrLatch = true
		} else {
			p.vramAddr = (p.vramAddr & 0xFF00) | uint16(v)
			p.addrLatch = false
		}
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
		idx := mirrorNametableAddr(c, a)
		p.ntRAM[idx] = v
	default:
		idx := mirrorPaletteAddr(a)
		p.palette[idx] = v
	}
}

func (p *ppu) step(c *Console, cycles int) bool {
	nmi := false
	for i := 0; i < cycles*3; i++ {
		p.cycle++
		if p.cycle == 260 && c != nil && c.cart != nil && c.cart.Mapper == 4 {
			if p.scanline < 240 || p.scanline == 261 {
				c.cart.mmc3ClockIRQ()
			}
		}
		if p.cycle >= 341 {
			p.cycle = 0
			p.scanline++
			if p.scanline == 241 {
				p.status |= 0x80
				if p.ctrl&0x80 != 0 {
					nmi = true
				}
			}
			if p.scanline >= 262 {
				p.scanline = 0
				p.status &^= 0x80
			}
		}
	}
	return nmi
}

func (p *ppu) renderFrame(c *Console, dst []byte) {
	if len(dst) < FrameSizeRGB {
		return
	}
	bgOpaque := make([]bool, FrameWidth*FrameHeight)
	baseNT := uint16(0x2000 | (uint16(p.ctrl&0x03) * 0x400))
	bgPatternBase := uint16(0x0000)
	if p.ctrl&0x10 != 0 {
		bgPatternBase = 0x1000
	}

	for y := 0; y < FrameHeight; y++ {
		worldY := (y + int(p.scrollY)) & 0x1FF
		tileY := (worldY / 8) % 30
		fineY := worldY % 8
		for x := 0; x < FrameWidth; x++ {
			worldX := (x + int(p.scrollX)) & 0x1FF
			tileX := (worldX / 8) % 32
			fineX := worldX % 8

			ntIndex := uint16(tileY*32 + tileX)
			tileID := p.ppuRead(c, baseNT+ntIndex)
			ptAddr := bgPatternBase + uint16(tileID)*16 + uint16(fineY)
			lo := p.ppuRead(c, ptAddr)
			hi := p.ppuRead(c, ptAddr+8)
			shift := byte(7 - fineX)
			pix := ((hi >> shift) & 0x01) << 1
			pix |= (lo >> shift) & 0x01

			attrAddr := baseNT + 0x03C0 + uint16((tileY/4)*8+(tileX/4))
			attr := p.ppuRead(c, attrAddr)
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
			rgb := nesPaletteRGB[palIndex]
			o := (y*FrameWidth + x) * 3
			dst[o+0] = rgb[0]
			dst[o+1] = rgb[1]
			dst[o+2] = rgb[2]
		}
	}
	p.renderSprites(c, dst, bgOpaque)
}

func (p *ppu) renderSprites(c *Console, dst []byte, bgOpaque []bool) {
	spritePatternBase := uint16(0x0000)
	if p.ctrl&0x08 != 0 {
		spritePatternBase = 0x1000
	}
	for i := 0; i < 64; i++ {
		o := i * 4
		sy := int(p.oam[o]) + 1
		tile := p.oam[o+1]
		attr := p.oam[o+2]
		sx := int(p.oam[o+3])
		flipV := attr&0x80 != 0
		flipH := attr&0x40 != 0
		behindBG := attr&0x20 != 0
		pal := attr & 0x03

		for py := 0; py < 8; py++ {
			y := sy + py
			if y < 0 || y >= FrameHeight {
				continue
			}
			srcY := py
			if flipV {
				srcY = 7 - py
			}
			addr := spritePatternBase + uint16(tile)*16 + uint16(srcY)
			lo := p.ppuRead(c, addr)
			hi := p.ppuRead(c, addr+8)
			for px := 0; px < 8; px++ {
				x := sx + px
				if x < 0 || x >= FrameWidth {
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
				rgb := nesPaletteRGB[palIndex]
				di := (y*FrameWidth + x) * 3
				dst[di+0] = rgb[0]
				dst[di+1] = rgb[1]
				dst[di+2] = rgb[2]
			}
		}
	}
}

func mirrorNametableAddr(c *Console, addr uint16) int {
	off := (addr - 0x2000) & 0x0FFF
	table := (off / 0x400) & 0x03
	in := off & 0x03FF
	mode := MirroringHorizontal
	if c != nil && c.cart != nil {
		mode = c.cart.mirroring
	}
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
