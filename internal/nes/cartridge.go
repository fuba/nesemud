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

	flags6 := data[6]
	flags7 := data[7]
	prgBanks := int(data[4])
	chrBanks := int(data[5])
	mapper := decodeINESMapper(flags6, flags7, data[12:16])
	if !isSupportedMapper(mapper) {
		return nil, errors.New("only mapper 0, 1, 2, 3, 4, 5, 23, 25, 33, 66, 75, 87, 88 and 206 are currently supported")
	}

	offset := 16
	var trainer []byte
	if flags6&0x04 != 0 {
		if len(data) < offset+512 {
			return nil, errors.New("rom truncated")
		}
		trainer = append([]byte(nil), data[offset:offset+512]...)
		offset += 512
	}
	prgLen := prgBanks * 16 * 1024
	chrLen := chrBanks * 8 * 1024
	if len(data) < offset+prgLen+chrLen {
		return nil, errors.New("rom truncated")
	}

	mirroring := MirroringHorizontal
	if flags6&0x08 != 0 {
		mirroring = MirroringFourScreen
	} else if flags6&0x01 != 0 {
		mirroring = MirroringVertical
	}
	hasBattery := flags6&0x02 != 0
	prgRAMSize := inferINESPRGRAMSize(data[8], mapper, hasBattery, flags6&0x04 != 0)

	cart := &Cartridge{
		PRG:        append([]byte(nil), data[offset:offset+prgLen]...),
		Mapper:     mapper,
		PRGBanks:   prgBanks,
		CHRBanks:   max(1, chrBanks),
		HasBattery: hasBattery,
		HasTrainer: len(trainer) != 0,
		mirroring:  mirroring,
	}
	if prgRAMSize > 0 {
		cart.PRGRAM = make([]byte, prgRAMSize)
	}
	if len(trainer) != 0 {
		if len(cart.PRGRAM) < 8*1024 {
			prg := make([]byte, 8*1024)
			copy(prg, cart.PRGRAM)
			cart.PRGRAM = prg
		}
		copy(cart.PRGRAM[0x1000:], trainer)
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
	if mapper == 75 {
		cart.mapper75PRG[0] = 0
		cart.mapper75PRG[1] = 1
		cart.mapper75PRG[2] = byte(max(0, len(cart.PRG)/(8*1024)-2))
	}
	if mapper == 23 || mapper == 25 {
		cart.vrcPRG0 = 0
		cart.vrcPRG1 = 1
		cart.vrcIRQPrescaler = 341
		cart.mirroring = mirroring
	}
	if mapper == 5 {
		cart.mmc5PRGMode = 3
		cart.mmc5CHRMode = 0
		cart.mmc5FillTile = 0
		cart.mmc5FillAttr = 0
		cart.mmc5MulA = 0xFF
		cart.mmc5MulB = 0xFF
		num8k := max(1, len(cart.PRG)/(8*1024))
		cart.mmc5PRGBank[4] = byte(num8k - 1)
	}
	if mapper == 4 {
		cart.mmc3PRGRAMEnable = true
		cart.mmc3PRGWriteDeny = false
	}
	return cart, nil
}

func decodeINESMapper(flags6 byte, flags7 byte, trailing []byte) byte {
	mapper := flags6 >> 4
	if flags7&0x0C == 0x08 || inesHeaderTailIsZero(trailing) {
		mapper |= flags7 & 0xF0
	}
	return mapper
}

func inesHeaderTailIsZero(trailing []byte) bool {
	for _, b := range trailing {
		if b != 0 {
			return false
		}
	}
	return true
}

func inferINESPRGRAMSize(byte8 byte, mapper byte, hasBattery bool, hasTrainer bool) int {
	if byte8 != 0 {
		return int(byte8) * 8 * 1024
	}
	if hasTrainer {
		return 8 * 1024
	}
	// For common board families where iNES byte8 is frequently omitted,
	// use the usual 8 KiB default to keep mapper RAM behavior practical.
	switch mapper {
	case 1, 5:
		return 8 * 1024
	case 4:
		if hasBattery {
			return 8 * 1024
		}
		return 0
	case 23, 25:
		if hasBattery {
			return 8 * 1024
		}
		return 0
	default:
		return 0
	}
}

func isSupportedMapper(mapper byte) bool {
	switch mapper {
	case 0, 1, 2, 3, 4, 5, 23, 25, 33, 66, 75, 87, 88, 206:
		return true
	default:
		return false
	}
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
	case 5:
		return c.readPRGMapper5(addr)
	case 23, 25:
		return c.readPRGMapperVRC(addr)
	case 33:
		return c.readPRGMapper33(addr)
	case 66:
		return c.readPRGMapper66(addr)
	case 75:
		return c.readPRGMapper75(addr)
	case 88, 206:
		return c.readPRGMapper206(addr)
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
		c.mapper3CHRSel = c.mapper3ResolveBusConflict(addr, value) & 0x03
	case 4:
		c.mmc3Write(addr, value)
	case 5:
		c.mmc5WritePRG(addr, value)
	case 23, 25:
		c.vrcWrite(addr, value)
	case 33:
		c.mapper33Write(addr, value)
	case 66:
		c.mapper66PRGSel = (value >> 4) & 0x03
		c.mapper66CHRSel = value & 0x03
	case 75:
		c.mapper75Write(addr, value)
	case 87:
		if addr >= 0x6000 && addr < 0x8000 {
			c.mapper87CHRSel = ((value & 0x01) << 1) | ((value >> 1) & 0x01)
		}
	case 88, 206:
		c.mapper206Write(addr, value)
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
	case 5:
		return c.readCHRMapper5(addr)
	case 23, 25:
		return c.readCHRMapperVRC(addr)
	case 33:
		return c.readCHRMapper33(addr)
	case 66:
		return c.readCHRMapper66(addr)
	case 75:
		return c.readCHRMapper75(addr)
	case 87:
		return c.readCHRMapper87(addr)
	case 88:
		return c.readCHRMapper88(addr)
	case 206:
		return c.readCHRMapper206(addr)
	default:
		return c.CHR[int(addr)%len(c.CHR)]
	}
}

func (c *Cartridge) mapper3ResolveBusConflict(addr uint16, value byte) byte {
	if len(c.PRG) == 0 || addr < 0x8000 {
		return value
	}
	return value & c.readPRG(addr)
}

func (c *Cartridge) writeCHR(addr uint16, value byte) {
	if !c.CHRIsRAM || len(c.CHR) == 0 {
		return
	}
	switch c.Mapper {
	case 1:
		if c.mmc1Control&0x10 == 0 {
			bank8k := int(c.mmc1CHRBank0&0x1E) % max(1, len(c.CHR)/(8*1024))
			base := bank8k * 8 * 1024
			c.CHR[base+int(addr&0x1FFF)] = value
			return
		}
		if addr < 0x1000 {
			bank4k := int(c.mmc1CHRBank0) % max(1, len(c.CHR)/(4*1024))
			base := bank4k * 4 * 1024
			c.CHR[base+int(addr&0x0FFF)] = value
			return
		}
		bank4k := int(c.mmc1CHRBank1) % max(1, len(c.CHR)/(4*1024))
		base := bank4k * 4 * 1024
		c.CHR[base+int(addr&0x0FFF)] = value
		return
	default:
		c.CHR[int(addr)%len(c.CHR)] = value
	}
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
	bank6 := int(c.mmc3Regs[6]&0x3F) % num8k
	bank7 := int(c.mmc3Regs[7]&0x3F) % num8k
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

func (c *Cartridge) readPRGMapper33(addr uint16) byte {
	num8k := max(1, len(c.PRG)/(8*1024))
	last := num8k - 1
	secondLast := max(0, num8k-2)
	var bank int
	switch {
	case addr < 0xA000:
		bank = int(c.mapper33PRG[0]) % num8k
	case addr < 0xC000:
		bank = int(c.mapper33PRG[1]) % num8k
	case addr < 0xE000:
		bank = secondLast
	default:
		bank = last
	}
	return c.PRG[bank*8*1024+int(addr&0x1FFF)]
}

func (c *Cartridge) readPRGMapper66(addr uint16) byte {
	num32k := max(1, len(c.PRG)/(32*1024))
	bank := int(c.mapper66PRGSel) % num32k
	return c.PRG[bank*32*1024+int((addr-0x8000)&0x7FFF)]
}

func (c *Cartridge) readPRGMapper75(addr uint16) byte {
	num8k := max(1, len(c.PRG)/(8*1024))
	last := num8k - 1
	var bank int
	switch {
	case addr < 0xA000:
		bank = int(c.mapper75PRG[0]) % num8k
	case addr < 0xC000:
		bank = int(c.mapper75PRG[1]) % num8k
	case addr < 0xE000:
		bank = int(c.mapper75PRG[2]) % num8k
	default:
		bank = last
	}
	return c.PRG[bank*8*1024+int(addr&0x1FFF)]
}

func (c *Cartridge) readPRGMapper5(addr uint16) byte {
	rom, base, off := c.mmc5ResolvePRGWindow(addr)
	if rom {
		if len(c.PRG) == 0 {
			return 0
		}
		return c.PRG[(base+off)%len(c.PRG)]
	}
	return c.mmc5PRGRAM[(base+off)%len(c.mmc5PRGRAM)]
}

func (c *Cartridge) readPRGMapperVRC(addr uint16) byte {
	num8k := max(1, len(c.PRG)/(8*1024))
	last := num8k - 1
	secondLast := max(0, num8k-2)
	var bank int
	switch {
	case addr < 0xA000:
		if c.vrcSwapMode {
			bank = secondLast
		} else {
			bank = int(c.vrcPRG0) % num8k
		}
	case addr < 0xC000:
		bank = int(c.vrcPRG1) % num8k
	case addr < 0xE000:
		if c.vrcSwapMode {
			bank = int(c.vrcPRG0) % num8k
		} else {
			bank = secondLast
		}
	default:
		bank = last
	}
	return c.PRG[bank*8*1024+int(addr&0x1FFF)]
}

func (c *Cartridge) readPRGMapper206(addr uint16) byte {
	num8k := max(1, len(c.PRG)/(8*1024))
	last := num8k - 1
	secondLast := max(0, num8k-2)
	bank6 := int(c.mmc3Regs[6]&0x3F) % num8k
	bank7 := int(c.mmc3Regs[7]&0x3F) % num8k
	var bank int
	switch {
	case addr < 0xA000:
		bank = bank6
	case addr < 0xC000:
		bank = bank7
	case addr < 0xE000:
		bank = secondLast
	default:
		bank = last
	}
	return c.PRG[bank*8*1024+int(addr&0x1FFF)]
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

func (c *Cartridge) readCHRMapper33(addr uint16) byte {
	a := int(addr & 0x1FFF)
	num1k := max(1, len(c.CHR)/1024)
	var bank int
	switch {
	case a < 0x0800:
		bank = int(c.mapper33CHR[0] &^ 1)
		bank += a / 0x0400
	case a < 0x1000:
		bank = int(c.mapper33CHR[1] &^ 1)
		bank += (a - 0x0800) / 0x0400
	case a < 0x1400:
		bank = int(c.mapper33CHR[2])
	case a < 0x1800:
		bank = int(c.mapper33CHR[3])
	case a < 0x1C00:
		bank = int(c.mapper33CHR[4])
	default:
		bank = int(c.mapper33CHR[5])
	}
	bank %= num1k
	return c.CHR[bank*1024+(a&0x03FF)]
}

func (c *Cartridge) readCHRMapper66(addr uint16) byte {
	num8k := max(1, len(c.CHR)/(8*1024))
	bank := int(c.mapper66CHRSel) % num8k
	return c.CHR[bank*8*1024+int(addr&0x1FFF)]
}

func (c *Cartridge) readCHRMapper75(addr uint16) byte {
	num4k := max(1, len(c.CHR)/(4*1024))
	a := int(addr & 0x1FFF)
	if a < 0x1000 {
		bank := int(c.mapper75CHR[0]) % num4k
		return c.CHR[bank*4*1024+a]
	}
	bank := int(c.mapper75CHR[1]) % num4k
	return c.CHR[bank*4*1024+int(addr&0x0FFF)]
}

func (c *Cartridge) readCHRMapper5(addr uint16) byte {
	num1k := max(1, len(c.CHR)/1024)
	slot := int((addr >> 10) & 0x07)
	bank := 0
	switch c.mmc5CHRMode & 0x03 {
	case 0:
		bank = int(c.mmc5UpperCHR)<<8 | int(c.mmc5CHRBank[7])
		bank += slot
	case 1:
		group := (slot / 4) * 4
		bank = int(c.mmc5UpperCHR)<<8 | int(c.mmc5CHRBank[group+3])
		bank += slot & 0x03
	case 2:
		group := (slot / 2) * 2
		bank = int(c.mmc5UpperCHR)<<8 | int(c.mmc5CHRBank[group+1])
		bank += slot & 0x01
	default:
		bank = int(c.mmc5UpperCHR)<<8 | int(c.mmc5CHRBank[slot])
	}
	bank %= num1k
	return c.CHR[bank*1024+int(addr&0x03FF)]
}

func (c *Cartridge) readCHRMapperVRC(addr uint16) byte {
	num1k := max(1, len(c.CHR)/1024)
	bank := int(c.vrcCHRBank(addr)) % num1k
	return c.CHR[bank*1024+int(addr&0x03FF)]
}

func (c *Cartridge) readCHRMapper87(addr uint16) byte {
	num8k := max(1, len(c.CHR)/(8*1024))
	bank := int(c.mapper87CHRSel) % num8k
	return c.CHR[bank*8*1024+int(addr&0x1FFF)]
}

func (c *Cartridge) vrcCHRBank(addr uint16) byte {
	slot := int((addr >> 10) & 0x07)
	return (c.vrcCHRHigh[slot] << 4) | (c.vrcCHRLow[slot] & 0x0F)
}

func (c *Cartridge) readCHRMapper88(addr uint16) byte {
	return c.readCHRMapper206Banked(addr, true)
}

func (c *Cartridge) readCHRMapper206(addr uint16) byte {
	return c.readCHRMapper206Banked(addr, false)
}

func (c *Cartridge) readCHRMapper206Banked(addr uint16, mapper88 bool) byte {
	a := int(addr & 0x1FFF)
	num1k := max(1, len(c.CHR)/1024)
	var bank int
	switch {
	case a < 0x0800:
		bank = int(c.mmc3Regs[0] & 0x3E)
		bank += a / 0x0400
	case a < 0x1000:
		bank = int(c.mmc3Regs[1] & 0x3E)
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
	if mapper88 && a >= 0x1000 {
		bank |= 0x40
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

func (c *Cartridge) mapper33Write(addr uint16, value byte) {
	switch {
	case addr >= 0x8000 && addr <= 0x8003:
		switch addr & 0x03 {
		case 0:
			if value&0x40 != 0 {
				c.mirroring = MirroringHorizontal
			} else {
				c.mirroring = MirroringVertical
			}
			c.mapper33PRG[0] = value & 0x3F
		case 1:
			c.mapper33PRG[1] = value & 0x3F
		case 2:
			c.mapper33CHR[0] = value &^ 1
		case 3:
			c.mapper33CHR[1] = value &^ 1
		}
	case addr >= 0xA000 && addr <= 0xA003:
		c.mapper33CHR[2+(addr&0x03)] = value
	}
}

func (c *Cartridge) mapper75Write(addr uint16, value byte) {
	switch addr & 0xF000 {
	case 0x8000:
		c.mapper75PRG[0] = value & 0x0F
	case 0x9000:
		if value&0x01 != 0 {
			c.mirroring = MirroringHorizontal
		} else {
			c.mirroring = MirroringVertical
		}
		c.mapper75CHR[0] = (c.mapper75CHR[0] & 0x0F) | ((value & 0x02) << 3)
		c.mapper75CHR[1] = (c.mapper75CHR[1] & 0x0F) | ((value & 0x04) << 2)
	case 0xA000:
		c.mapper75PRG[1] = value & 0x0F
	case 0xC000:
		c.mapper75PRG[2] = value & 0x0F
	case 0xE000:
		c.mapper75CHR[0] = (c.mapper75CHR[0] & 0x10) | (value & 0x0F)
	case 0xF000:
		c.mapper75CHR[1] = (c.mapper75CHR[1] & 0x10) | (value & 0x0F)
	}
}

func (c *Cartridge) mapper206Write(addr uint16, value byte) {
	if addr < 0x8000 || addr > 0x9FFF {
		return
	}
	if addr&1 == 0 {
		c.mmc3BankSelect = value & 0x07
		return
	}
	c.mmc3Regs[c.mmc3BankSelect&0x07] = value
}

func (c *Cartridge) vrcWrite(addr uint16, value byte) {
	reg := c.vrcRegister(addr)
	switch reg {
	case 0x8000:
		c.vrcPRG0 = value & 0x1F
	case 0x9000:
		switch value & 0x03 {
		case 0:
			c.mirroring = MirroringVertical
		case 1:
			c.mirroring = MirroringHorizontal
		case 2:
			c.mirroring = MirroringOneScreenLow
		default:
			c.mirroring = MirroringOneScreenHigh
		}
		c.vrcSwapMode = value&0x02 != 0
	case 0xA000:
		c.vrcPRG1 = value & 0x1F
	case 0xB000, 0xB001, 0xB002, 0xB003,
		0xC000, 0xC001, 0xC002, 0xC003,
		0xD000, 0xD001, 0xD002, 0xD003,
		0xE000, 0xE001, 0xE002, 0xE003:
		slot := int((reg - 0xB000) / 0x1000)
		part := int(reg & 0x0003)
		idx := slot*2 + (part >> 1)
		if idx >= 0 && idx < 8 {
			if part&0x01 == 0 {
				c.vrcCHRLow[idx] = value & 0x0F
			} else {
				c.vrcCHRHigh[idx] = value & 0x1F
			}
		}
	case 0xF000:
		c.vrcIRQLatch = value
	case 0xF001:
		c.vrcIRQEnable = value&0x02 != 0
		c.vrcIRQEnableAck = value&0x01 != 0
		if c.vrcIRQEnable {
			c.vrcIRQCounter = c.vrcIRQLatch
			c.vrcIRQPrescaler = 341
		}
		c.vrcIRQPending = false
	case 0xF002:
		c.vrcIRQEnable = c.vrcIRQEnableAck
		c.vrcIRQPending = false
		c.vrcIRQPrescaler = 341
	case 0xF003:
		c.vrcIRQEnable = c.vrcIRQEnableAck
		c.vrcIRQPending = false
		c.vrcIRQPrescaler = 341
	}
}

func (c *Cartridge) readRegister(addr uint16) byte {
	if c.Mapper != 5 {
		return 0
	}
	switch addr {
	case 0x5204:
		var status byte
		if c.mmc5IRQPending {
			status |= 0x80
		}
		if c.mmc5InFrame {
			status |= 0x40
		}
		c.mmc5IRQPending = false
		return status
	case 0x5205:
		product := uint16(c.mmc5MulA) * uint16(c.mmc5MulB)
		return byte(product & 0x00FF)
	case 0x5206:
		product := uint16(c.mmc5MulA) * uint16(c.mmc5MulB)
		return byte(product >> 8)
	default:
		if addr >= 0x5C00 && addr <= 0x5FFF {
			return c.mmc5ExRAM[(addr-0x5C00)&0x03FF]
		}
		return 0
	}
}

func (c *Cartridge) writeRegister(addr uint16, value byte) {
	if c.Mapper != 5 {
		return
	}
	switch {
	case addr == 0x5100:
		c.mmc5PRGMode = value & 0x03
	case addr == 0x5101:
		c.mmc5CHRMode = value & 0x03
	case addr == 0x5102:
		c.mmc5RAMProtect1 = value
	case addr == 0x5103:
		c.mmc5RAMProtect2 = value
	case addr == 0x5104:
		c.mmc5ExRAMMode = value & 0x03
	case addr == 0x5105:
		c.mmc5SetNametableMapping(value)
	case addr == 0x5106:
		c.mmc5FillTile = value
	case addr == 0x5107:
		c.mmc5FillAttr = value & 0x03
	case addr >= 0x5113 && addr <= 0x5117:
		c.mmc5PRGBank[addr-0x5113] = value
	case addr == 0x5120:
		c.mmc5CHRBank[0] = value
	case addr == 0x5121:
		c.mmc5CHRBank[1] = value
	case addr == 0x5122:
		c.mmc5CHRBank[2] = value
	case addr == 0x5123:
		c.mmc5CHRBank[3] = value
	case addr == 0x5124:
		c.mmc5CHRBank[4] = value
	case addr == 0x5125:
		c.mmc5CHRBank[5] = value
	case addr == 0x5126:
		c.mmc5CHRBank[6] = value
	case addr == 0x5127:
		c.mmc5CHRBank[7] = value
	case addr == 0x5128:
		c.mmc5CHRBank[8] = value
	case addr == 0x5129:
		c.mmc5CHRBank[9] = value
	case addr == 0x512A:
		c.mmc5CHRBank[10] = value
	case addr == 0x512B:
		c.mmc5CHRBank[11] = value
	case addr == 0x5130:
		c.mmc5UpperCHR = value & 0x03
	case addr == 0x5203:
		c.mmc5IRQLatch = value
	case addr == 0x5204:
		c.mmc5IRQEnable = value&0x80 != 0
		if !c.mmc5IRQEnable {
			c.mmc5IRQPending = false
		}
	case addr == 0x5205:
		c.mmc5MulA = value
	case addr == 0x5206:
		c.mmc5MulB = value
	case addr >= 0x5C00 && addr <= 0x5FFF:
		if c.mmc5ExRAMMode != 3 {
			c.mmc5ExRAM[(addr-0x5C00)&0x03FF] = value
		}
	}
}

func (c *Cartridge) readPRGRAM(addr uint16) byte {
	if c.Mapper == 5 {
		base := c.mmc5PRGRAMBankBase(c.mmc5PRGBank[0])
		off := int((addr - 0x6000) & 0x1FFF)
		return c.mmc5PRGRAM[(base+off)%len(c.mmc5PRGRAM)]
	}
	if len(c.PRGRAM) == 0 {
		return 0
	}
	if c.Mapper == 4 && !c.mmc3PRGRAMEnable {
		return 0
	}
	return c.PRGRAM[int(addr-0x6000)%len(c.PRGRAM)]
}

func (c *Cartridge) writePRGRAM(addr uint16, value byte) {
	if c.Mapper != 5 {
		if c.Mapper == 4 {
			if len(c.PRGRAM) == 0 {
				return
			}
			if !c.mmc3PRGRAMEnable || c.mmc3PRGWriteDeny {
				return
			}
			c.PRGRAM[int(addr-0x6000)%len(c.PRGRAM)] = value
			return
		}
		if c.Mapper == 87 {
			c.writePRG(addr, value)
			return
		}
		if len(c.PRGRAM) == 0 {
			return
		}
		c.PRGRAM[int(addr-0x6000)%len(c.PRGRAM)] = value
		return
	}
	if c.mmc5RAMProtect1 != 0x02 || c.mmc5RAMProtect2 != 0x01 {
		return
	}
	base := c.mmc5PRGRAMBankBase(c.mmc5PRGBank[0])
	off := int((addr - 0x6000) & 0x1FFF)
	c.mmc5PRGRAM[(base+off)%len(c.mmc5PRGRAM)] = value
}

func (c *Cartridge) mmc5WritePRG(addr uint16, value byte) {
	if c.mmc5RAMProtect1 != 0x02 || c.mmc5RAMProtect2 != 0x01 {
		return
	}
	rom, base, off := c.mmc5ResolvePRGWindow(addr)
	if rom {
		return
	}
	c.mmc5PRGRAM[(base+off)%len(c.mmc5PRGRAM)] = value
}

func (c *Cartridge) mmc5ResolvePRGWindow(addr uint16) (bool, int, int) {
	numROM8k := max(1, len(c.PRG)/(8*1024))
	numRAM8k := max(1, len(c.mmc5PRGRAM)/(8*1024))
	mode := c.mmc5PRGMode & 0x03

	switch mode {
	case 0:
		base := mmc5ROMBase8k(c.mmc5PRGBank[4], 4, numROM8k)
		return true, base * 8 * 1024, int((addr - 0x8000) & 0x7FFF)
	case 1:
		if addr < 0xC000 {
			return c.mmc5ResolveBankedPRG(c.mmc5PRGBank[2], 2, numROM8k, numRAM8k, int((addr-0x8000)&0x3FFF))
		}
		base := mmc5ROMBase8k(c.mmc5PRGBank[4], 2, numROM8k)
		return true, base * 8 * 1024, int((addr - 0xC000) & 0x3FFF)
	case 2:
		switch {
		case addr < 0xC000:
			return c.mmc5ResolveBankedPRG(c.mmc5PRGBank[2], 2, numROM8k, numRAM8k, int((addr-0x8000)&0x3FFF))
		case addr < 0xE000:
			return c.mmc5ResolveBankedPRG(c.mmc5PRGBank[3], 1, numROM8k, numRAM8k, int((addr-0xC000)&0x1FFF))
		default:
			base := mmc5ROMBase8k(c.mmc5PRGBank[4], 1, numROM8k)
			return true, base * 8 * 1024, int((addr - 0xE000) & 0x1FFF)
		}
	default:
		switch {
		case addr < 0xA000:
			return c.mmc5ResolveBankedPRG(c.mmc5PRGBank[1], 1, numROM8k, numRAM8k, int((addr-0x8000)&0x1FFF))
		case addr < 0xC000:
			return c.mmc5ResolveBankedPRG(c.mmc5PRGBank[2], 1, numROM8k, numRAM8k, int((addr-0xA000)&0x1FFF))
		case addr < 0xE000:
			return c.mmc5ResolveBankedPRG(c.mmc5PRGBank[3], 1, numROM8k, numRAM8k, int((addr-0xC000)&0x1FFF))
		default:
			base := mmc5ROMBase8k(c.mmc5PRGBank[4], 1, numROM8k)
			return true, base * 8 * 1024, int((addr - 0xE000) & 0x1FFF)
		}
	}
}

func (c *Cartridge) mmc5ResolveBankedPRG(value byte, size8k int, numROM8k int, numRAM8k int, off int) (bool, int, int) {
	if value&0x80 != 0 {
		base := mmc5ROMBase8k(value, size8k, numROM8k)
		return true, base * 8 * 1024, off
	}
	base := mmc5RAMBase8k(value, size8k, numRAM8k)
	return false, base * 8 * 1024, off
}

func mmc5ROMBase8k(value byte, size8k int, total8k int) int {
	switch size8k {
	case 4:
		groups := max(1, total8k/4)
		return (((int(value) & 0x7C) >> 2) % groups) * 4
	case 2:
		groups := max(1, total8k/2)
		return (((int(value) & 0x7E) >> 1) % groups) * 2
	default:
		return (int(value) & 0x7F) % total8k
	}
}

func mmc5RAMBase8k(value byte, size8k int, total8k int) int {
	switch size8k {
	case 2:
		groups := max(1, total8k/2)
		return ((int(value&0x06) >> 1) % groups) * 2
	default:
		return int(value&0x07) % total8k
	}
}

func (c *Cartridge) mmc5PRGRAMBankBase(value byte) int {
	return mmc5RAMBase8k(value, 1, max(1, len(c.mmc5PRGRAM)/(8*1024))) * 8 * 1024
}

func (c *Cartridge) mmc5SetNametableMapping(value byte) {
	for i := 0; i < 4; i++ {
		c.mmc5NTMap[i] = (value >> (i * 2)) & 0x03
	}
	switch c.mmc5NTMap[0] {
	case 0:
		c.mirroring = MirroringVertical
	case 1:
		c.mirroring = MirroringHorizontal
	case 2:
		c.mirroring = MirroringOneScreenLow
	default:
		c.mirroring = MirroringOneScreenHigh
	}
}

func (c *Cartridge) vrcRegister(addr uint16) uint16 {
	base := addr & 0xF000
	switch c.Mapper {
	case 23:
		a0 := (addr >> 0) & 1
		a1 := (addr >> 1) & 1
		return base | (a1 << 1) | a0
	case 25:
		a0 := (addr >> 1) & 1
		a1 := (addr >> 0) & 1
		return base | (a1 << 1) | a0
	default:
		return base | (addr & 0x0003)
	}
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
		} else {
			c.mmc3PRGRAMEnable = value&0x80 != 0
			c.mmc3PRGWriteDeny = value&0x40 != 0
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
	if c.mmc5IRQPending {
		c.mmc5IRQPending = false
		return true
	}
	if c.vrcIRQPending {
		c.vrcIRQPending = false
		return true
	}
	return false
}

func (c *Cartridge) irqPending() bool {
	return c.mmc3IRQPending || c.mmc5IRQPending || c.vrcIRQPending
}

func (c *Cartridge) mmc5ClockScanline() {
	if c.Mapper != 5 {
		return
	}
	c.mmc5InFrame = true
	if c.mmc5Scanline == c.mmc5IRQLatch && c.mmc5IRQEnable {
		c.mmc5IRQPending = true
	}
	c.mmc5Scanline++
}

func (c *Cartridge) mmc5EndFrame() {
	if c.Mapper != 5 {
		return
	}
	c.mmc5InFrame = false
	c.mmc5Scanline = 0
}

func (c *Cartridge) vrcClockIRQ() {
	if !c.vrcIRQEnable {
		return
	}
	c.vrcIRQPrescaler -= 3
	for c.vrcIRQPrescaler <= 0 {
		c.vrcIRQPrescaler += 341
		if c.vrcIRQCounter == 0xFF {
			c.vrcIRQCounter = c.vrcIRQLatch
			c.vrcIRQPending = true
		} else {
			c.vrcIRQCounter++
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
