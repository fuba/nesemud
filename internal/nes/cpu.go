package nes

import "fmt"

const (
	flagC byte = 1 << 0
	flagZ byte = 1 << 1
	flagI byte = 1 << 2
	flagD byte = 1 << 3
	flagB byte = 1 << 4
	flagU byte = 1 << 5
	flagV byte = 1 << 6
	flagN byte = 1 << 7
)

type cpu6502 struct {
	A, X, Y byte
	SP      byte
	P       byte
	PC      uint16
	Cycles  uint64
}

func newCPU() *cpu6502 {
	c := &cpu6502{}
	c.PowerOn()
	return c
}

func (c *cpu6502) PowerOn() {
	c.A = 0
	c.X = 0
	c.Y = 0
	c.SP = 0xFD
	c.P = flagU | flagI
	c.PC = 0
	c.Cycles = 0
}

func (c *cpu6502) Reset(bus cpuBus) {
	c.SP -= 3
	c.setFlag(flagI, true)
	c.PC = c.read16(bus, 0xFFFC)
}

type cpuBus interface {
	read(addr uint16) byte
	write(addr uint16, v byte)
}

func (c *cpu6502) Step(bus cpuBus) error {
	op := c.fetch8(bus)
	switch op {
	case 0xA9:
		c.A = c.fetch8(bus)
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0xA5:
		c.A = bus.read(c.addrZP(bus))
		c.updateNZ(c.A)
		c.Cycles += 3
	case 0xB5:
		c.A = bus.read(c.addrZPX(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0xAD:
		c.A = bus.read(c.addrABS(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0xBD:
		addr, crossed := c.addrABSX(bus)
		c.A = bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xB9:
		addr, crossed := c.addrABSY(bus)
		c.A = bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xA1:
		c.A = bus.read(c.addrINDX(bus))
		c.updateNZ(c.A)
		c.Cycles += 6
	case 0xB1:
		addr, crossed := c.addrINDY(bus)
		c.A = bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 5 + boolToCycle(crossed)

	case 0xA2:
		c.X = c.fetch8(bus)
		c.updateNZ(c.X)
		c.Cycles += 2
	case 0xA6:
		c.X = bus.read(c.addrZP(bus))
		c.updateNZ(c.X)
		c.Cycles += 3
	case 0xB6:
		c.X = bus.read(c.addrZPY(bus))
		c.updateNZ(c.X)
		c.Cycles += 4
	case 0xAE:
		c.X = bus.read(c.addrABS(bus))
		c.updateNZ(c.X)
		c.Cycles += 4
	case 0xBE:
		addr, crossed := c.addrABSY(bus)
		c.X = bus.read(addr)
		c.updateNZ(c.X)
		c.Cycles += 4 + boolToCycle(crossed)

	case 0xA0:
		c.Y = c.fetch8(bus)
		c.updateNZ(c.Y)
		c.Cycles += 2
	case 0xA4:
		c.Y = bus.read(c.addrZP(bus))
		c.updateNZ(c.Y)
		c.Cycles += 3
	case 0xB4:
		c.Y = bus.read(c.addrZPX(bus))
		c.updateNZ(c.Y)
		c.Cycles += 4
	case 0xAC:
		c.Y = bus.read(c.addrABS(bus))
		c.updateNZ(c.Y)
		c.Cycles += 4
	case 0xBC:
		addr, crossed := c.addrABSX(bus)
		c.Y = bus.read(addr)
		c.updateNZ(c.Y)
		c.Cycles += 4 + boolToCycle(crossed)

	case 0x85:
		bus.write(c.addrZP(bus), c.A)
		c.Cycles += 3
	case 0x95:
		bus.write(c.addrZPX(bus), c.A)
		c.Cycles += 4
	case 0x8D:
		bus.write(c.addrABS(bus), c.A)
		c.Cycles += 4
	case 0x9D:
		addr, _ := c.addrABSX(bus)
		bus.write(addr, c.A)
		c.Cycles += 5
	case 0x99:
		addr, _ := c.addrABSY(bus)
		bus.write(addr, c.A)
		c.Cycles += 5
	case 0x81:
		bus.write(c.addrINDX(bus), c.A)
		c.Cycles += 6
	case 0x91:
		addr, _ := c.addrINDY(bus)
		bus.write(addr, c.A)
		c.Cycles += 6

	case 0x86:
		bus.write(c.addrZP(bus), c.X)
		c.Cycles += 3
	case 0x96:
		bus.write(c.addrZPY(bus), c.X)
		c.Cycles += 4
	case 0x8E:
		bus.write(c.addrABS(bus), c.X)
		c.Cycles += 4

	case 0x84:
		bus.write(c.addrZP(bus), c.Y)
		c.Cycles += 3
	case 0x94:
		bus.write(c.addrZPX(bus), c.Y)
		c.Cycles += 4
	case 0x8C:
		bus.write(c.addrABS(bus), c.Y)
		c.Cycles += 4

	case 0xAA:
		c.X = c.A
		c.updateNZ(c.X)
		c.Cycles += 2
	case 0xA8:
		c.Y = c.A
		c.updateNZ(c.Y)
		c.Cycles += 2
	case 0x8A:
		c.A = c.X
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x98:
		c.A = c.Y
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0xBA:
		c.X = c.SP
		c.updateNZ(c.X)
		c.Cycles += 2
	case 0x9A:
		c.SP = c.X
		c.Cycles += 2

	case 0x29:
		c.A &= c.fetch8(bus)
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x25:
		c.A &= bus.read(c.addrZP(bus))
		c.updateNZ(c.A)
		c.Cycles += 3
	case 0x35:
		c.A &= bus.read(c.addrZPX(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x2D:
		c.A &= bus.read(c.addrABS(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x3D:
		addr, crossed := c.addrABSX(bus)
		c.A &= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x39:
		addr, crossed := c.addrABSY(bus)
		c.A &= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x21:
		c.A &= bus.read(c.addrINDX(bus))
		c.updateNZ(c.A)
		c.Cycles += 6
	case 0x31:
		addr, crossed := c.addrINDY(bus)
		c.A &= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 5 + boolToCycle(crossed)

	case 0x09:
		c.A |= c.fetch8(bus)
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x05:
		c.A |= bus.read(c.addrZP(bus))
		c.updateNZ(c.A)
		c.Cycles += 3
	case 0x15:
		c.A |= bus.read(c.addrZPX(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x0D:
		c.A |= bus.read(c.addrABS(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x1D:
		addr, crossed := c.addrABSX(bus)
		c.A |= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x19:
		addr, crossed := c.addrABSY(bus)
		c.A |= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x01:
		c.A |= bus.read(c.addrINDX(bus))
		c.updateNZ(c.A)
		c.Cycles += 6
	case 0x11:
		addr, crossed := c.addrINDY(bus)
		c.A |= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 5 + boolToCycle(crossed)

	case 0x49:
		c.A ^= c.fetch8(bus)
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x45:
		c.A ^= bus.read(c.addrZP(bus))
		c.updateNZ(c.A)
		c.Cycles += 3
	case 0x55:
		c.A ^= bus.read(c.addrZPX(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x4D:
		c.A ^= bus.read(c.addrABS(bus))
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x5D:
		addr, crossed := c.addrABSX(bus)
		c.A ^= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x59:
		addr, crossed := c.addrABSY(bus)
		c.A ^= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x41:
		c.A ^= bus.read(c.addrINDX(bus))
		c.updateNZ(c.A)
		c.Cycles += 6
	case 0x51:
		addr, crossed := c.addrINDY(bus)
		c.A ^= bus.read(addr)
		c.updateNZ(c.A)
		c.Cycles += 5 + boolToCycle(crossed)

	case 0x69:
		c.adc(c.fetch8(bus))
		c.Cycles += 2
	case 0x65:
		c.adc(bus.read(c.addrZP(bus)))
		c.Cycles += 3
	case 0x75:
		c.adc(bus.read(c.addrZPX(bus)))
		c.Cycles += 4
	case 0x6D:
		c.adc(bus.read(c.addrABS(bus)))
		c.Cycles += 4
	case 0x7D:
		addr, crossed := c.addrABSX(bus)
		c.adc(bus.read(addr))
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x79:
		addr, crossed := c.addrABSY(bus)
		c.adc(bus.read(addr))
		c.Cycles += 4 + boolToCycle(crossed)
	case 0x61:
		c.adc(bus.read(c.addrINDX(bus)))
		c.Cycles += 6
	case 0x71:
		addr, crossed := c.addrINDY(bus)
		c.adc(bus.read(addr))
		c.Cycles += 5 + boolToCycle(crossed)

	case 0xE9:
		c.sbc(c.fetch8(bus))
		c.Cycles += 2
	case 0xEB: // USBC/SBC #imm (illegal alias)
		c.sbc(c.fetch8(bus))
		c.Cycles += 2
	case 0xE5:
		c.sbc(bus.read(c.addrZP(bus)))
		c.Cycles += 3
	case 0xF5:
		c.sbc(bus.read(c.addrZPX(bus)))
		c.Cycles += 4
	case 0xED:
		c.sbc(bus.read(c.addrABS(bus)))
		c.Cycles += 4
	case 0xFD:
		addr, crossed := c.addrABSX(bus)
		c.sbc(bus.read(addr))
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xF9:
		addr, crossed := c.addrABSY(bus)
		c.sbc(bus.read(addr))
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xE1:
		c.sbc(bus.read(c.addrINDX(bus)))
		c.Cycles += 6
	case 0xF1:
		addr, crossed := c.addrINDY(bus)
		c.sbc(bus.read(addr))
		c.Cycles += 5 + boolToCycle(crossed)

	case 0xC9:
		c.compare(c.A, c.fetch8(bus))
		c.Cycles += 2
	case 0xC5:
		c.compare(c.A, bus.read(c.addrZP(bus)))
		c.Cycles += 3
	case 0xD5:
		c.compare(c.A, bus.read(c.addrZPX(bus)))
		c.Cycles += 4
	case 0xCD:
		c.compare(c.A, bus.read(c.addrABS(bus)))
		c.Cycles += 4
	case 0xDD:
		addr, crossed := c.addrABSX(bus)
		c.compare(c.A, bus.read(addr))
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xD9:
		addr, crossed := c.addrABSY(bus)
		c.compare(c.A, bus.read(addr))
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xC1:
		c.compare(c.A, bus.read(c.addrINDX(bus)))
		c.Cycles += 6
	case 0xD1:
		addr, crossed := c.addrINDY(bus)
		c.compare(c.A, bus.read(addr))
		c.Cycles += 5 + boolToCycle(crossed)

	case 0xE0:
		c.compare(c.X, c.fetch8(bus))
		c.Cycles += 2
	case 0xE4:
		c.compare(c.X, bus.read(c.addrZP(bus)))
		c.Cycles += 3
	case 0xEC:
		c.compare(c.X, bus.read(c.addrABS(bus)))
		c.Cycles += 4
	case 0xC0:
		c.compare(c.Y, c.fetch8(bus))
		c.Cycles += 2
	case 0xC4:
		c.compare(c.Y, bus.read(c.addrZP(bus)))
		c.Cycles += 3
	case 0xCC:
		c.compare(c.Y, bus.read(c.addrABS(bus)))
		c.Cycles += 4

	case 0xE6:
		addr := c.addrZP(bus)
		v := bus.read(addr) + 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 5
	case 0xF6:
		addr := c.addrZPX(bus)
		v := bus.read(addr) + 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 6
	case 0xEE:
		addr := c.addrABS(bus)
		v := bus.read(addr) + 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 6
	case 0xFE:
		addr, _ := c.addrABSX(bus)
		v := bus.read(addr) + 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 7

	case 0xC6:
		addr := c.addrZP(bus)
		v := bus.read(addr) - 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 5
	case 0xD6:
		addr := c.addrZPX(bus)
		v := bus.read(addr) - 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 6
	case 0xCE:
		addr := c.addrABS(bus)
		v := bus.read(addr) - 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 6
	case 0xDE:
		addr, _ := c.addrABSX(bus)
		v := bus.read(addr) - 1
		bus.write(addr, v)
		c.updateNZ(v)
		c.Cycles += 7

	case 0x0A:
		c.setFlag(flagC, c.A&0x80 != 0)
		c.A <<= 1
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x06:
		c.aslMem(bus, c.addrZP(bus), 5)
	case 0x16:
		c.aslMem(bus, c.addrZPX(bus), 6)
	case 0x0E:
		c.aslMem(bus, c.addrABS(bus), 6)
	case 0x1E:
		addr, _ := c.addrABSX(bus)
		c.aslMem(bus, addr, 7)

	case 0x4A:
		c.setFlag(flagC, c.A&0x01 != 0)
		c.A >>= 1
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x46:
		c.lsrMem(bus, c.addrZP(bus), 5)
	case 0x56:
		c.lsrMem(bus, c.addrZPX(bus), 6)
	case 0x4E:
		c.lsrMem(bus, c.addrABS(bus), 6)
	case 0x5E:
		addr, _ := c.addrABSX(bus)
		c.lsrMem(bus, addr, 7)

	case 0x2A:
		carryIn := byte(0)
		if c.hasFlag(flagC) {
			carryIn = 1
		}
		c.setFlag(flagC, c.A&0x80 != 0)
		c.A = (c.A << 1) | carryIn
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x26:
		c.rolMem(bus, c.addrZP(bus), 5)
	case 0x36:
		c.rolMem(bus, c.addrZPX(bus), 6)
	case 0x2E:
		c.rolMem(bus, c.addrABS(bus), 6)
	case 0x3E:
		addr, _ := c.addrABSX(bus)
		c.rolMem(bus, addr, 7)

	case 0x6A:
		carryIn := byte(0)
		if c.hasFlag(flagC) {
			carryIn = 0x80
		}
		c.setFlag(flagC, c.A&0x01 != 0)
		c.A = (c.A >> 1) | carryIn
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x66:
		c.rorMem(bus, c.addrZP(bus), 5)
	case 0x76:
		c.rorMem(bus, c.addrZPX(bus), 6)
	case 0x6E:
		c.rorMem(bus, c.addrABS(bus), 6)
	case 0x7E:
		addr, _ := c.addrABSX(bus)
		c.rorMem(bus, addr, 7)

	case 0x24:
		c.bit(bus.read(c.addrZP(bus)))
		c.Cycles += 3
	case 0x2C:
		c.bit(bus.read(c.addrABS(bus)))
		c.Cycles += 4

	case 0xE8:
		c.X++
		c.updateNZ(c.X)
		c.Cycles += 2
	case 0xC8:
		c.Y++
		c.updateNZ(c.Y)
		c.Cycles += 2
	case 0xCA:
		c.X--
		c.updateNZ(c.X)
		c.Cycles += 2
	case 0x88:
		c.Y--
		c.updateNZ(c.Y)
		c.Cycles += 2

	case 0x4C:
		c.PC = c.addrABS(bus)
		c.Cycles += 3
	case 0x6C:
		ptr := c.addrABS(bus)
		c.PC = c.read16Bug(bus, ptr)
		c.Cycles += 5
	case 0x20:
		addr := c.addrABS(bus)
		ret := c.PC - 1
		c.push(bus, byte(ret>>8))
		c.push(bus, byte(ret))
		c.PC = addr
		c.Cycles += 6
	case 0x60:
		lo := uint16(c.pop(bus))
		hi := uint16(c.pop(bus))
		c.PC = (hi<<8 | lo) + 1
		c.Cycles += 6
	case 0x40:
		c.P = (c.pop(bus) | flagU) &^ flagB
		lo := uint16(c.pop(bus))
		hi := uint16(c.pop(bus))
		c.PC = lo | (hi << 8)
		c.Cycles += 6

	case 0x10:
		c.branch(bus, !c.hasFlag(flagN))
	case 0x30:
		c.branch(bus, c.hasFlag(flagN))
	case 0x50:
		c.branch(bus, !c.hasFlag(flagV))
	case 0x70:
		c.branch(bus, c.hasFlag(flagV))
	case 0x90:
		c.branch(bus, !c.hasFlag(flagC))
	case 0xB0:
		c.branch(bus, c.hasFlag(flagC))
	case 0xD0:
		c.branch(bus, !c.hasFlag(flagZ))
	case 0xF0:
		c.branch(bus, c.hasFlag(flagZ))

	case 0x18:
		c.setFlag(flagC, false)
		c.Cycles += 2
	case 0x38:
		c.setFlag(flagC, true)
		c.Cycles += 2
	case 0x58:
		c.setFlag(flagI, false)
		c.Cycles += 2
	case 0x78:
		c.setFlag(flagI, true)
		c.Cycles += 2
	case 0xB8:
		c.setFlag(flagV, false)
		c.Cycles += 2
	case 0xD8:
		c.setFlag(flagD, false)
		c.Cycles += 2
	case 0xF8:
		c.setFlag(flagD, true)
		c.Cycles += 2

	case 0x48:
		c.push(bus, c.A)
		c.Cycles += 3
	case 0x68:
		c.A = c.pop(bus)
		c.updateNZ(c.A)
		c.Cycles += 4
	case 0x08:
		c.push(bus, c.P|flagB|flagU)
		c.Cycles += 3
	case 0x28:
		c.P = (c.pop(bus) | flagU) &^ flagB
		c.Cycles += 4

	case 0xEA:
		c.Cycles += 2
	case 0x02, 0x12, 0x22, 0x32, 0x42, 0x52, 0x62, 0x72, 0x92, 0xB2, 0xD2, 0xF2:
		// KIL/JAM opcodes are treated as inert 1-byte NOPs for compatibility.
		c.Cycles += 2
	case 0x1A, 0x3A, 0x5A, 0x7A, 0xDA, 0xFA: // NOP (illegal, implied)
		c.Cycles += 2
	case 0x0B, 0x2B: // ANC #imm
		c.A &= c.fetch8(bus)
		c.updateNZ(c.A)
		c.setFlag(flagC, c.A&0x80 != 0)
		c.Cycles += 2
	case 0x4B: // ALR #imm
		c.A &= c.fetch8(bus)
		c.setFlag(flagC, c.A&0x01 != 0)
		c.A >>= 1
		c.updateNZ(c.A)
		c.Cycles += 2
	case 0x6B: // ARR #imm
		c.A &= c.fetch8(bus)
		carryIn := byte(0)
		if c.hasFlag(flagC) {
			carryIn = 0x80
		}
		c.A = (c.A >> 1) | carryIn
		c.updateNZ(c.A)
		c.setFlag(flagC, c.A&0x40 != 0)
		c.setFlag(flagV, ((c.A>>6)&1)^((c.A>>5)&1) != 0)
		c.Cycles += 2
	case 0xCB: // SBX #imm
		v := c.fetch8(bus)
		ax := c.A & c.X
		res := uint16(ax) - uint16(v)
		c.X = byte(res)
		c.setFlag(flagC, ax >= v)
		c.updateNZ(c.X)
		c.Cycles += 2
	case 0x80, 0x82, 0x89, 0xC2, 0xE2: // NOP #imm
		_ = c.fetch8(bus)
		c.Cycles += 2
	case 0x04, 0x44, 0x64: // NOP zp
		_ = c.addrZP(bus)
		c.Cycles += 3
	case 0x14, 0x34, 0x54, 0x74, 0xD4, 0xF4: // NOP zpx
		_ = c.addrZPX(bus)
		c.Cycles += 4
	case 0x0C: // NOP abs
		_ = c.addrABS(bus)
		c.Cycles += 4
	case 0x1C, 0x3C, 0x5C, 0x7C, 0xDC, 0xFC: // NOP absx
		_, crossed := c.addrABSX(bus)
		c.Cycles += 4 + boolToCycle(crossed)

	case 0xA7: // LAX zp
		v := bus.read(c.addrZP(bus))
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 3
	case 0xB7: // LAX zpy
		v := bus.read(c.addrZPY(bus))
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 4
	case 0xAF: // LAX abs
		v := bus.read(c.addrABS(bus))
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 4
	case 0xBF: // LAX absy
		addr, crossed := c.addrABSY(bus)
		v := bus.read(addr)
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 4 + boolToCycle(crossed)
	case 0xA3: // LAX (ind,x)
		v := bus.read(c.addrINDX(bus))
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 6
	case 0xB3: // LAX (ind),y
		addr, crossed := c.addrINDY(bus)
		v := bus.read(addr)
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 5 + boolToCycle(crossed)
	case 0xAB: // LAX #imm
		v := c.fetch8(bus)
		c.A, c.X = v, v
		c.updateNZ(v)
		c.Cycles += 2

	case 0x87: // SAX zp
		bus.write(c.addrZP(bus), c.A&c.X)
		c.Cycles += 3
	case 0x97: // SAX zpy
		bus.write(c.addrZPY(bus), c.A&c.X)
		c.Cycles += 4
	case 0x8F: // SAX abs
		bus.write(c.addrABS(bus), c.A&c.X)
		c.Cycles += 4
	case 0x83: // SAX (ind,x)
		bus.write(c.addrINDX(bus), c.A&c.X)
		c.Cycles += 6
	case 0x93: // AHX (ind),y
		addr, _ := c.addrINDY(bus)
		bus.write(addr, c.illegalStoreHighValue(addr, c.A&c.X))
		c.Cycles += 6
	case 0x9B: // TAS abs,y
		addr, _ := c.addrABSY(bus)
		c.SP = c.A & c.X
		bus.write(addr, c.illegalStoreHighValue(addr, c.SP))
		c.Cycles += 5
	case 0x9C: // SHY abs,x
		addr, _ := c.addrABSX(bus)
		bus.write(addr, c.illegalStoreHighValue(addr, c.Y))
		c.Cycles += 5
	case 0x9E: // SHX abs,y
		addr, _ := c.addrABSY(bus)
		bus.write(addr, c.illegalStoreHighValue(addr, c.X))
		c.Cycles += 5
	case 0x9F: // AHX abs,y
		addr, _ := c.addrABSY(bus)
		bus.write(addr, c.illegalStoreHighValue(addr, c.A&c.X))
		c.Cycles += 5
	case 0xBB: // LAS abs,y
		addr, crossed := c.addrABSY(bus)
		v := bus.read(addr) & c.SP
		c.A = v
		c.X = v
		c.SP = v
		c.updateNZ(v)
		c.Cycles += 4 + boolToCycle(crossed)

	case 0xC7, 0xD7, 0xCF, 0xDF, 0xDB, 0xC3, 0xD3: // DCP
		addr, cycles := c.illegalAddr(bus, op)
		v := bus.read(addr) - 1
		bus.write(addr, v)
		c.compare(c.A, v)
		c.Cycles += cycles
	case 0xE7, 0xF7, 0xEF, 0xFF, 0xFB, 0xE3, 0xF3: // ISB
		addr, cycles := c.illegalAddr(bus, op)
		v := bus.read(addr) + 1
		bus.write(addr, v)
		c.sbc(v)
		c.Cycles += cycles
	case 0x07, 0x17, 0x0F, 0x1F, 0x1B, 0x03, 0x13: // SLO
		addr, cycles := c.illegalAddr(bus, op)
		v := bus.read(addr)
		c.setFlag(flagC, v&0x80 != 0)
		v <<= 1
		bus.write(addr, v)
		c.A |= v
		c.updateNZ(c.A)
		c.Cycles += cycles
	case 0x27, 0x37, 0x2F, 0x3F, 0x3B, 0x23, 0x33: // RLA
		addr, cycles := c.illegalAddr(bus, op)
		v := bus.read(addr)
		carryIn := byte(0)
		if c.hasFlag(flagC) {
			carryIn = 1
		}
		c.setFlag(flagC, v&0x80 != 0)
		v = (v << 1) | carryIn
		bus.write(addr, v)
		c.A &= v
		c.updateNZ(c.A)
		c.Cycles += cycles
	case 0x47, 0x57, 0x4F, 0x5F, 0x5B, 0x43, 0x53: // SRE
		addr, cycles := c.illegalAddr(bus, op)
		v := bus.read(addr)
		c.setFlag(flagC, v&0x01 != 0)
		v >>= 1
		bus.write(addr, v)
		c.A ^= v
		c.updateNZ(c.A)
		c.Cycles += cycles
	case 0x67, 0x77, 0x6F, 0x7F, 0x7B, 0x63, 0x73: // RRA
		addr, cycles := c.illegalAddr(bus, op)
		v := bus.read(addr)
		carryIn := byte(0)
		if c.hasFlag(flagC) {
			carryIn = 0x80
		}
		c.setFlag(flagC, v&0x01 != 0)
		v = (v >> 1) | carryIn
		bus.write(addr, v)
		c.adc(v)
		c.Cycles += cycles
	case 0x00:
		c.PC++
		c.push(bus, byte(c.PC>>8))
		c.push(bus, byte(c.PC))
		c.push(bus, c.P|flagB|flagU)
		c.setFlag(flagI, true)
		c.PC = c.read16(bus, 0xFFFE)
		c.Cycles += 7

	default:
		return fmt.Errorf("unsupported opcode 0x%02X at 0x%04X", op, c.PC-1)
	}
	return nil
}

func (c *cpu6502) illegalAddr(bus cpuBus, op byte) (uint16, uint64) {
	switch op {
	case 0xC7, 0xE7, 0x07, 0x27, 0x47, 0x67:
		return c.addrZP(bus), 5
	case 0xD7, 0xF7, 0x17, 0x37, 0x57, 0x77:
		return c.addrZPX(bus), 6
	case 0xCF, 0xEF, 0x0F, 0x2F, 0x4F, 0x6F:
		return c.addrABS(bus), 6
	case 0xDF, 0xFF, 0x1F, 0x3F, 0x5F, 0x7F:
		addr, _ := c.addrABSX(bus)
		return addr, 7
	case 0xDB, 0xFB, 0x1B, 0x3B, 0x5B, 0x7B:
		addr, _ := c.addrABSY(bus)
		return addr, 7
	case 0xC3, 0xE3, 0x03, 0x23, 0x43, 0x63:
		return c.addrINDX(bus), 8
	default:
		addr, _ := c.addrINDY(bus)
		return addr, 8
	}
}

func (c *cpu6502) illegalStoreHighValue(addr uint16, value byte) byte {
	return value & byte((addr>>8)+1)
}

func (c *cpu6502) NMI(bus cpuBus) {
	c.push(bus, byte(c.PC>>8))
	c.push(bus, byte(c.PC))
	c.push(bus, (c.P&^flagB)|flagU)
	c.setFlag(flagI, true)
	c.PC = c.read16(bus, 0xFFFA)
	c.Cycles += 7
}

func (c *cpu6502) IRQ(bus cpuBus) {
	if c.hasFlag(flagI) {
		return
	}
	c.enterIRQ(bus)
}

func (c *cpu6502) IRQForced(bus cpuBus) {
	c.enterIRQ(bus)
}

func (c *cpu6502) enterIRQ(bus cpuBus) {
	c.push(bus, byte(c.PC>>8))
	c.push(bus, byte(c.PC))
	c.push(bus, (c.P&^flagB)|flagU)
	c.setFlag(flagI, true)
	c.PC = c.read16(bus, 0xFFFE)
	c.Cycles += 7
}

func (c *cpu6502) fetch8(bus cpuBus) byte {
	v := bus.read(c.PC)
	c.PC++
	return v
}

func (c *cpu6502) fetch16(bus cpuBus) uint16 {
	lo := uint16(c.fetch8(bus))
	hi := uint16(c.fetch8(bus))
	return lo | (hi << 8)
}

func (c *cpu6502) read16(bus cpuBus, addr uint16) uint16 {
	lo := uint16(bus.read(addr))
	hi := uint16(bus.read(addr + 1))
	return lo | (hi << 8)
}

func (c *cpu6502) read16Bug(bus cpuBus, addr uint16) uint16 {
	lo := uint16(bus.read(addr))
	hiAddr := (addr & 0xFF00) | uint16(byte(addr+1))
	hi := uint16(bus.read(hiAddr))
	return lo | (hi << 8)
}

func (c *cpu6502) addrZP(bus cpuBus) uint16 {
	return uint16(c.fetch8(bus))
}

func (c *cpu6502) addrZPX(bus cpuBus) uint16 {
	return uint16(byte(c.fetch8(bus) + c.X))
}

func (c *cpu6502) addrZPY(bus cpuBus) uint16 {
	return uint16(byte(c.fetch8(bus) + c.Y))
}

func (c *cpu6502) addrABS(bus cpuBus) uint16 {
	return c.fetch16(bus)
}

func (c *cpu6502) addrABSX(bus cpuBus) (uint16, bool) {
	base := c.fetch16(bus)
	addr := base + uint16(c.X)
	return addr, pageCrossed(base, addr)
}

func (c *cpu6502) addrABSY(bus cpuBus) (uint16, bool) {
	base := c.fetch16(bus)
	addr := base + uint16(c.Y)
	return addr, pageCrossed(base, addr)
}

func (c *cpu6502) addrINDX(bus cpuBus) uint16 {
	zp := byte(c.fetch8(bus) + c.X)
	lo := uint16(bus.read(uint16(zp)))
	hi := uint16(bus.read(uint16(byte(zp + 1))))
	return lo | (hi << 8)
}

func (c *cpu6502) addrINDY(bus cpuBus) (uint16, bool) {
	zp := c.fetch8(bus)
	baseLo := uint16(bus.read(uint16(zp)))
	baseHi := uint16(bus.read(uint16(byte(zp + 1))))
	base := baseLo | (baseHi << 8)
	addr := base + uint16(c.Y)
	return addr, pageCrossed(base, addr)
}

func (c *cpu6502) push(bus cpuBus, v byte) {
	bus.write(0x0100|uint16(c.SP), v)
	c.SP--
}

func (c *cpu6502) pop(bus cpuBus) byte {
	c.SP++
	return bus.read(0x0100 | uint16(c.SP))
}

func (c *cpu6502) setFlag(mask byte, on bool) {
	if on {
		c.P |= mask
		return
	}
	c.P &^= mask
}

func (c *cpu6502) hasFlag(mask byte) bool {
	return c.P&mask != 0
}

func (c *cpu6502) updateNZ(v byte) {
	c.setFlag(flagZ, v == 0)
	c.setFlag(flagN, v&0x80 != 0)
}

func (c *cpu6502) adc(v byte) {
	carry := byte(0)
	if c.hasFlag(flagC) {
		carry = 1
	}
	sum := uint16(c.A) + uint16(v) + uint16(carry)
	res := byte(sum)
	c.setFlag(flagC, sum > 0xFF)
	c.setFlag(flagV, ((^(c.A ^ v))&(c.A^res)&0x80) != 0)
	c.A = res
	c.updateNZ(c.A)
}

func (c *cpu6502) sbc(v byte) {
	c.adc(^v)
}

func (c *cpu6502) compare(reg, val byte) {
	res := reg - val
	c.setFlag(flagC, reg >= val)
	c.setFlag(flagZ, reg == val)
	c.setFlag(flagN, res&0x80 != 0)
}

func (c *cpu6502) bit(v byte) {
	res := c.A & v
	c.setFlag(flagZ, res == 0)
	c.setFlag(flagV, v&0x40 != 0)
	c.setFlag(flagN, v&0x80 != 0)
}

func (c *cpu6502) branch(bus cpuBus, cond bool) {
	off := int8(c.fetch8(bus))
	c.Cycles += 2
	if !cond {
		return
	}
	oldPC := c.PC
	c.PC = uint16(int32(c.PC) + int32(off))
	c.Cycles++
	if pageCrossed(oldPC, c.PC) {
		c.Cycles++
	}
}

func (c *cpu6502) aslMem(bus cpuBus, addr uint16, cycles uint64) {
	v := bus.read(addr)
	c.setFlag(flagC, v&0x80 != 0)
	v <<= 1
	bus.write(addr, v)
	c.updateNZ(v)
	c.Cycles += cycles
}

func (c *cpu6502) lsrMem(bus cpuBus, addr uint16, cycles uint64) {
	v := bus.read(addr)
	c.setFlag(flagC, v&0x01 != 0)
	v >>= 1
	bus.write(addr, v)
	c.updateNZ(v)
	c.Cycles += cycles
}

func (c *cpu6502) rolMem(bus cpuBus, addr uint16, cycles uint64) {
	v := bus.read(addr)
	carryIn := byte(0)
	if c.hasFlag(flagC) {
		carryIn = 1
	}
	c.setFlag(flagC, v&0x80 != 0)
	v = (v << 1) | carryIn
	bus.write(addr, v)
	c.updateNZ(v)
	c.Cycles += cycles
}

func (c *cpu6502) rorMem(bus cpuBus, addr uint16, cycles uint64) {
	v := bus.read(addr)
	carryIn := byte(0)
	if c.hasFlag(flagC) {
		carryIn = 0x80
	}
	c.setFlag(flagC, v&0x01 != 0)
	v = (v >> 1) | carryIn
	bus.write(addr, v)
	c.updateNZ(v)
	c.Cycles += cycles
}

func pageCrossed(a, b uint16) bool {
	return (a & 0xFF00) != (b & 0xFF00)
}

func boolToCycle(on bool) uint64 {
	if on {
		return 1
	}
	return 0
}
