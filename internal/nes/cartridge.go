package nes

import (
	"errors"
)

func LoadINES(data []byte) (*Cartridge, error) {
	if len(data) < 16 {
		return nil, errors.New("rom too short")
	}
	if data[0] != 'N' || data[1] != 'E' || data[2] != 'S' || data[3] != 0x1A {
		return nil, errors.New("invalid iNES signature")
	}

	prgBanks := int(data[4])
	chrBanks := int(data[5])
	mapper := (data[6]>>4 | (data[7] & 0xF0))
	if mapper != 0 && mapper != 1 && mapper != 2 && mapper != 3 && mapper != 4 {
		return nil, errors.New("only mapper 0, 1, 2, 3 and 4 are currently supported")
	}

	offset := 16
	if data[6]&0x04 != 0 {
		offset += 512
	}
	prgLen := prgBanks * 16 * 1024
	chrLen := chrBanks * 8 * 1024
	if len(data) < offset+prgLen+chrLen {
		return nil, errors.New("rom truncated")
	}

	mirroring := MirroringHorizontal
	if data[6]&0x01 != 0 {
		mirroring = MirroringVertical
	}

	cart := &Cartridge{
		PRG:       append([]byte(nil), data[offset:offset+prgLen]...),
		Mapper:    mapper,
		PRGBanks:  prgBanks,
		CHRBanks:  max(1, chrBanks),
		mirroring: mirroring,
	}
	offset += prgLen
	if chrLen > 0 {
		cart.CHR = append([]byte(nil), data[offset:offset+chrLen]...)
	} else {
		cart.CHR = make([]byte, 8*1024)
		cart.CHRIsRAM = true
	}
	if mapper == 1 {
		cart.mmc1Reset()
	}
	return cart, nil
}

func (c *Cartridge) readPRG(addr uint16) byte {
	if len(c.PRG) == 0 {
		return 0
	}
	switch c.Mapper {
	case 0:
		if len(c.PRG) == 16*1024 {
			return c.PRG[(addr-0x8000)&0x3FFF]
		}
		return c.PRG[(addr-0x8000)&0x7FFF]
	case 1:
		return c.readPRGMapper1(addr)
	case 2:
		if c.PRGBanks <= 1 {
			return c.PRG[(addr-0x8000)&0x3FFF]
		}
		if addr < 0xC000 {
			bank := int(c.mapper2BankSel) % c.PRGBanks
			base := bank * 16 * 1024
			return c.PRG[base+int(addr-0x8000)]
		}
		lastBank := c.PRGBanks - 1
		base := lastBank * 16 * 1024
		return c.PRG[base+int(addr-0xC000)]
	case 4:
		return c.readPRGMapper4(addr)
	default:
		if len(c.PRG) == 16*1024 {
			return c.PRG[(addr-0x8000)&0x3FFF]
		}
		return c.PRG[(addr-0x8000)&0x7FFF]
	}
}

func (c *Cartridge) writePRG(addr uint16, value byte) {
	switch c.Mapper {
	case 1:
		c.mmc1Write(addr, value)
	case 2:
		c.mapper2BankSel = value & 0x0F
	case 3:
		c.mapper3CHRSel = value & 0x03
	case 4:
		c.mmc3Write(addr, value)
	default:
		_ = addr
	}
}

func (c *Cartridge) readCHR(addr uint16) byte {
	if len(c.CHR) == 0 {
		return 0
	}
	switch c.Mapper {
	case 1:
		if c.mmc1Control&0x10 == 0 {
			bank8k := int(c.mmc1CHRBank0&0x1E) % max(1, len(c.CHR)/(8*1024))
			base := bank8k * 8 * 1024
			return c.CHR[base+int(addr&0x1FFF)]
		}
		if addr < 0x1000 {
			bank4k := int(c.mmc1CHRBank0) % max(1, len(c.CHR)/(4*1024))
			base := bank4k * 4 * 1024
			return c.CHR[base+int(addr&0x0FFF)]
		}
		bank4k := int(c.mmc1CHRBank1) % max(1, len(c.CHR)/(4*1024))
		base := bank4k * 4 * 1024
		return c.CHR[base+int(addr&0x0FFF)]
	case 3:
		bank := int(c.mapper3CHRSel) % max(1, c.CHRBanks)
		base := bank * 8 * 1024
		return c.CHR[base+int(addr&0x1FFF)]
	case 4:
		return c.readCHRMapper4(addr)
	default:
		return c.CHR[int(addr)%len(c.CHR)]
	}
}

func (c *Cartridge) writeCHR(addr uint16, value byte) {
	if !c.CHRIsRAM || len(c.CHR) == 0 {
		return
	}
	c.CHR[int(addr)%len(c.CHR)] = value
}

func (c *Cartridge) readPRGMapper1(addr uint16) byte {
	mode := (c.mmc1Control >> 2) & 0x03
	switch mode {
	case 0, 1:
		bank := int(c.mmc1PRGBank&0x0E) % max(1, c.PRGBanks)
		base := bank * 16 * 1024
		return c.PRG[base+int((addr-0x8000)&0x7FFF)]
	case 2:
		if addr < 0xC000 {
			return c.PRG[int(addr-0x8000)]
		}
		bank := int(c.mmc1PRGBank) % max(1, c.PRGBanks)
		base := bank * 16 * 1024
		return c.PRG[base+int(addr-0xC000)]
	default:
		if addr < 0xC000 {
			bank := int(c.mmc1PRGBank) % max(1, c.PRGBanks)
			base := bank * 16 * 1024
			return c.PRG[base+int(addr-0x8000)]
		}
		lastBank := max(1, c.PRGBanks) - 1
		base := lastBank * 16 * 1024
		return c.PRG[base+int(addr-0xC000)]
	}
}

func (c *Cartridge) readPRGMapper4(addr uint16) byte {
	num8k := max(1, len(c.PRG)/(8*1024))
	bank6 := int(c.mmc3Regs[6]) % num8k
	bank7 := int(c.mmc3Regs[7]) % num8k
	last := num8k - 1
	secondLast := max(0, num8k-2)
	prgMode := (c.mmc3BankSelect >> 6) & 1

	var bank int
	switch {
	case addr >= 0x8000 && addr < 0xA000:
		if prgMode == 0 {
			bank = bank6
		} else {
			bank = secondLast
		}
	case addr >= 0xA000 && addr < 0xC000:
		bank = bank7
	case addr >= 0xC000 && addr < 0xE000:
		if prgMode == 0 {
			bank = secondLast
		} else {
			bank = bank6
		}
	default:
		bank = last
	}
	off := int(addr & 0x1FFF)
	return c.PRG[bank*8*1024+off]
}

func (c *Cartridge) readCHRMapper4(addr uint16) byte {
	num1k := max(1, len(c.CHR)/1024)
	chrMode := (c.mmc3BankSelect >> 7) & 1
	bank := 0
	a := int(addr & 0x1FFF)

	if chrMode == 0 {
		switch {
		case a < 0x0800:
			bank = int(c.mmc3Regs[0] &^ 1)
			bank += a / 0x0400
		case a < 0x1000:
			bank = int(c.mmc3Regs[1] &^ 1)
			bank += (a - 0x0800) / 0x0400
		case a < 0x1400:
			bank = int(c.mmc3Regs[2])
		case a < 0x1800:
			bank = int(c.mmc3Regs[3])
		case a < 0x1C00:
			bank = int(c.mmc3Regs[4])
		default:
			bank = int(c.mmc3Regs[5])
		}
	} else {
		switch {
		case a < 0x0400:
			bank = int(c.mmc3Regs[2])
		case a < 0x0800:
			bank = int(c.mmc3Regs[3])
		case a < 0x0C00:
			bank = int(c.mmc3Regs[4])
		case a < 0x1000:
			bank = int(c.mmc3Regs[5])
		case a < 0x1800:
			bank = int(c.mmc3Regs[0] &^ 1)
			bank += (a - 0x1000) / 0x0400
		default:
			bank = int(c.mmc3Regs[1] &^ 1)
			bank += (a - 0x1800) / 0x0400
		}
	}
	bank %= num1k
	return c.CHR[bank*1024+(a&0x03FF)]
}

func (c *Cartridge) mmc1Reset() {
	c.mmc1Shift = 0x10
	c.mmc1Control = 0x0C
	c.mmc1CHRBank0 = 0
	c.mmc1CHRBank1 = 0
	c.mmc1PRGBank = 0
	c.applyMMC1Mirroring()
}

func (c *Cartridge) mmc1Write(addr uint16, value byte) {
	if value&0x80 != 0 {
		c.mmc1Shift = 0x10
		c.mmc1Control |= 0x0C
		c.applyMMC1Mirroring()
		return
	}
	complete := c.mmc1Shift&0x01 == 1
	c.mmc1Shift >>= 1
	c.mmc1Shift |= (value & 0x01) << 4
	if !complete {
		return
	}
	regValue := c.mmc1Shift & 0x1F
	switch (addr >> 13) & 0x03 {
	case 0:
		c.mmc1Control = regValue
		c.applyMMC1Mirroring()
	case 1:
		c.mmc1CHRBank0 = regValue
	case 2:
		c.mmc1CHRBank1 = regValue
	case 3:
		c.mmc1PRGBank = regValue & 0x0F
	}
	c.mmc1Shift = 0x10
}

func (c *Cartridge) applyMMC1Mirroring() {
	switch c.mmc1Control & 0x03 {
	case 0:
		c.mirroring = MirroringOneScreenLow
	case 1:
		c.mirroring = MirroringOneScreenHigh
	case 2:
		c.mirroring = MirroringVertical
	case 3:
		c.mirroring = MirroringHorizontal
	}
}

func (c *Cartridge) mmc3Write(addr uint16, value byte) {
	switch {
	case addr >= 0x8000 && addr <= 0x9FFF:
		if addr&1 == 0 {
			c.mmc3BankSelect = value
		} else {
			c.mmc3Regs[c.mmc3BankSelect&0x07] = value
		}
	case addr >= 0xA000 && addr <= 0xBFFF:
		if addr&1 == 0 {
			if value&0x01 == 0 {
				c.mirroring = MirroringVertical
			} else {
				c.mirroring = MirroringHorizontal
			}
		}
	case addr >= 0xC000 && addr <= 0xDFFF:
		if addr&1 == 0 {
			c.mmc3IRQLatch = value
		} else {
			c.mmc3IRQReload = true
		}
	case addr >= 0xE000:
		if addr&1 == 0 {
			c.mmc3IRQEnable = false
			c.mmc3IRQPending = false
		} else {
			c.mmc3IRQEnable = true
		}
	}
}

func (c *Cartridge) mmc3ClockIRQ() {
	if c.mmc3IRQCounter == 0 || c.mmc3IRQReload {
		c.mmc3IRQCounter = c.mmc3IRQLatch
		c.mmc3IRQReload = false
	} else {
		c.mmc3IRQCounter--
	}
	if c.mmc3IRQCounter == 0 && c.mmc3IRQEnable {
		c.mmc3IRQPending = true
	}
}

func (c *Cartridge) consumeIRQ() bool {
	if c.mmc3IRQPending {
		c.mmc3IRQPending = false
		return true
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
