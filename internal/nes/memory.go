package nes

func (c *Console) read(addr uint16) byte {
	return c.readCPU(addr)
}

func (c *Console) write(addr uint16, value byte) {
	c.writeCPU(addr, value)
}

func (c *Console) readCPU(addr uint16) byte {
	switch {
	case addr < 0x2000:
		return c.ram[addr&0x07FF]
	case addr < 0x4000:
		return c.ppu.cpuReadRegister(c, 0x2000+(addr&0x0007))
	case addr == 0x4016:
		return c.readControllerPort(0)
	case addr == 0x4017:
		return c.readControllerPort(1)
	case addr == 0x4015:
		return c.apu.ReadStatus()
	case addr >= 0x8000:
		if c.cart == nil {
			return 0
		}
		return c.cart.readPRG(addr)
	default:
		return 0
	}
}

func (c *Console) writeCPU(addr uint16, value byte) {
	switch {
	case addr < 0x2000:
		c.ram[addr&0x07FF] = value
	case addr < 0x4000:
		c.ppu.cpuWriteRegister(c, 0x2000+(addr&0x0007), value)
	case addr == 0x4014:
		base := uint16(value) << 8
		for i := 0; i < 256; i++ {
			c.ppu.oam[i] = c.readCPU(base + uint16(i))
		}
	case addr == 0x4016:
		c.writeControllerStrobe(value)
	case addr >= 0x4000 && addr <= 0x4017:
		c.apu.WriteRegister(addr, value)
	case addr >= 0x8000:
		if c.cart != nil {
			c.cart.writePRG(addr, value)
		}
	}
}

func (c *Console) writeControllerStrobe(v byte) {
	on := v&1 == 1
	c.controllerStrobe = on
	c.latchControllers()
}

func (c *Console) latchControllers() {
	c.controllerShift[0] = buttonsToByte(c.controllerP1)
	c.controllerShift[1] = buttonsToByte(c.controllerP2)
}

func (c *Console) readControllerPort(idx int) byte {
	if idx < 0 || idx > 1 {
		return 0x40
	}
	var b byte
	if c.controllerStrobe {
		if idx == 0 {
			if c.controllerP1.A {
				b = 1
			}
		} else {
			if c.controllerP2.A {
				b = 1
			}
		}
		return 0x40 | b
	}
	b = c.controllerShift[idx] & 1
	c.controllerShift[idx] = (c.controllerShift[idx] >> 1) | 0x80
	return 0x40 | b
}

func buttonsToByte(btn Buttons) byte {
	var v byte
	if btn.A {
		v |= 1 << 0
	}
	if btn.B {
		v |= 1 << 1
	}
	if btn.Select {
		v |= 1 << 2
	}
	if btn.Start {
		v |= 1 << 3
	}
	if btn.Up {
		v |= 1 << 4
	}
	if btn.Down {
		v |= 1 << 5
	}
	if btn.Left {
		v |= 1 << 6
	}
	if btn.Right {
		v |= 1 << 7
	}
	return v
}
