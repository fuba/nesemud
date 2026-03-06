package nes

const ntscCPUHz = 1789773.0

var dutyTable = [4][8]byte{
	{0, 1, 0, 0, 0, 0, 0, 0},
	{0, 1, 1, 0, 0, 0, 0, 0},
	{0, 1, 1, 1, 1, 0, 0, 0},
	{1, 0, 0, 1, 1, 1, 1, 1},
}

var lengthTable = [32]byte{10, 254, 20, 2, 40, 4, 80, 6, 160, 8, 60, 10, 14, 12, 26, 14, 12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30}
var noisePeriodTable = [16]uint16{4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068}
var dmcRateTable = [16]uint16{428, 380, 340, 320, 286, 254, 226, 214, 190, 160, 142, 128, 106, 85, 72, 54}

type envelope struct {
	period   byte
	constant bool
	loop     bool
	start    bool
	divider  byte
	decay    byte
}

type pulseChannel struct {
	enabled     bool
	duty        byte
	volume      byte
	timer       uint16
	phase       float64
	lengthHalt  bool
	lengthCount byte
	env         envelope
}

type triangleChannel struct {
	enabled     bool
	timer       uint16
	phase       float64
	lengthHalt  bool
	lengthCount byte
}

type noiseChannel struct {
	enabled     bool
	mode        byte
	periodIdx   byte
	volume      byte
	shiftReg    uint16
	phase       float64
	lengthHalt  bool
	lengthCount byte
	env         envelope
}

type dmcChannel struct {
	enabled       bool
	irqEnable     bool
	loop          bool
	rateIdx       byte
	outputLevel   byte
	sampleAddrReg byte
	sampleLenReg  byte
	currentAddr   uint16
	bytesRemain   uint16
	shiftReg      byte
	bitsRemain    byte
	phase         float64
}

type apu struct {
	pulse1    pulseChannel
	triangle1 triangleChannel
	noise1    noiseChannel
	dmc       dmcChannel
	status    byte
	seqPhase  float64
	seqStep   int
}

func newAPU() *apu {
	return &apu{}
}

func (a *apu) Reset() {
	a.pulse1 = pulseChannel{}
	a.triangle1 = triangleChannel{}
	a.noise1 = noiseChannel{shiftReg: 1}
	a.dmc = dmcChannel{}
	a.status = 0
	a.seqPhase = 0
	a.seqStep = 0
}

func (a *apu) ReadStatus() byte {
	return a.status & 0x1D
}

func (a *apu) WriteRegister(addr uint16, value byte) {
	switch addr {
	case 0x4000:
		a.pulse1.duty = (value >> 6) & 0x03
		a.pulse1.lengthHalt = value&0x20 != 0
		a.pulse1.env.loop = value&0x20 != 0
		a.pulse1.env.constant = value&0x10 != 0
		a.pulse1.env.period = value & 0x0F
		a.pulse1.volume = value & 0x0F
	case 0x4002:
		a.pulse1.timer = (a.pulse1.timer & 0x0700) | uint16(value)
	case 0x4003:
		a.pulse1.timer = (a.pulse1.timer & 0x00FF) | (uint16(value&0x07) << 8)
		a.pulse1.phase = 0
		a.pulse1.lengthCount = lengthTable[(value>>3)&0x1F]
		a.pulse1.env.start = true

	case 0x4008:
		a.triangle1.lengthHalt = value&0x80 != 0
	case 0x400A:
		a.triangle1.timer = (a.triangle1.timer & 0x0700) | uint16(value)
	case 0x400B:
		a.triangle1.timer = (a.triangle1.timer & 0x00FF) | (uint16(value&0x07) << 8)
		a.triangle1.phase = 0
		a.triangle1.lengthCount = lengthTable[(value>>3)&0x1F]

	case 0x400C:
		a.noise1.lengthHalt = value&0x20 != 0
		a.noise1.env.loop = value&0x20 != 0
		a.noise1.env.constant = value&0x10 != 0
		a.noise1.env.period = value & 0x0F
		a.noise1.volume = value & 0x0F
	case 0x400E:
		a.noise1.mode = (value >> 7) & 0x01
		a.noise1.periodIdx = value & 0x0F
	case 0x400F:
		a.noise1.lengthCount = lengthTable[(value>>3)&0x1F]
		a.noise1.env.start = true
		if a.noise1.shiftReg == 0 {
			a.noise1.shiftReg = 1
		}

	case 0x4010:
		a.dmc.irqEnable = value&0x80 != 0
		a.dmc.loop = value&0x40 != 0
		a.dmc.rateIdx = value & 0x0F
	case 0x4011:
		a.dmc.outputLevel = value & 0x7F
	case 0x4012:
		a.dmc.sampleAddrReg = value
	case 0x4013:
		a.dmc.sampleLenReg = value

	case 0x4015:
		a.status = value & 0x1D
		a.pulse1.enabled = a.status&0x01 != 0
		if !a.pulse1.enabled {
			a.pulse1.lengthCount = 0
		}
		a.triangle1.enabled = a.status&0x04 != 0
		if !a.triangle1.enabled {
			a.triangle1.lengthCount = 0
		}
		a.noise1.enabled = a.status&0x08 != 0
		if !a.noise1.enabled {
			a.noise1.lengthCount = 0
		}
		a.dmc.enabled = a.status&0x10 != 0
		if !a.dmc.enabled {
			a.dmc.bytesRemain = 0
		} else if a.dmc.bytesRemain == 0 {
			a.restartDMC()
		}
	}
}

func (a *apu) GenerateFrameSamples(monoSamples int, reader func(addr uint16) byte) []int16 {
	if monoSamples <= 0 {
		return nil
	}
	out := make([]int16, monoSamples*2)

	for i := 0; i < monoSamples; i++ {
		a.seqPhase += 240.0 / float64(AudioRate)
		for a.seqPhase >= 1.0 {
			a.seqPhase -= 1.0
			a.tickFrameSequencer()
		}

		pulse := a.samplePulseLevel()
		tri := a.sampleTriangleLevel()
		noi := a.sampleNoiseLevel()
		dmc := a.sampleDMCLevel(reader)
		sample := mixSample(pulse, tri, noi, dmc)

		out[i*2] = sample
		out[i*2+1] = sample
	}

	return out
}

func (a *apu) tickFrameSequencer() {
	a.clockEnvelope(&a.pulse1.env)
	a.clockEnvelope(&a.noise1.env)

	if a.seqStep%2 == 1 {
		a.clockLengthPulse(&a.pulse1)
		a.clockLengthNoise(&a.noise1)
		a.clockLengthTriangle(&a.triangle1)
	}
	a.seqStep = (a.seqStep + 1) & 0x03
}

func (a *apu) clockEnvelope(e *envelope) {
	if e.start {
		e.start = false
		e.decay = 15
		e.divider = e.period
		return
	}
	if e.divider == 0 {
		e.divider = e.period
		if e.decay == 0 {
			if e.loop {
				e.decay = 15
			}
		} else {
			e.decay--
		}
	} else {
		e.divider--
	}
}

func (a *apu) clockLengthPulse(ch *pulseChannel) {
	if ch.lengthHalt || ch.lengthCount == 0 {
		return
	}
	ch.lengthCount--
}

func (a *apu) clockLengthNoise(ch *noiseChannel) {
	if ch.lengthHalt || ch.lengthCount == 0 {
		return
	}
	ch.lengthCount--
}

func (a *apu) clockLengthTriangle(ch *triangleChannel) {
	if ch.lengthHalt || ch.lengthCount == 0 {
		return
	}
	ch.lengthCount--
}

func (a *apu) samplePulse() int16 {
	if !a.pulse1.enabled || a.pulse1.timer < 8 || a.pulse1.lengthCount == 0 {
		return 0
	}
	freq := ntscCPUHz / (16.0 * float64(a.pulse1.timer+1))
	step := freq / float64(AudioRate)
	ampVol := a.pulse1.env.decay
	if a.pulse1.env.constant {
		ampVol = a.pulse1.volume
	}
	amp := int16((float64(ampVol) / 15.0) * 4000)
	seq := dutyTable[a.pulse1.duty]
	idx := int(a.pulse1.phase) & 0x07
	out := int16(0)
	if seq[idx] == 1 {
		out = amp
	}
	a.pulse1.phase += step * 8.0
	for a.pulse1.phase >= 8.0 {
		a.pulse1.phase -= 8.0
	}
	return out
}

func (a *apu) samplePulseLevel() float64 {
	if !a.pulse1.enabled || a.pulse1.timer < 8 || a.pulse1.lengthCount == 0 {
		return 0
	}
	freq := ntscCPUHz / (16.0 * float64(a.pulse1.timer+1))
	step := freq / float64(AudioRate)
	ampVol := a.pulse1.env.decay
	if a.pulse1.env.constant {
		ampVol = a.pulse1.volume
	}
	seq := dutyTable[a.pulse1.duty]
	idx := int(a.pulse1.phase) & 0x07
	out := 0.0
	if seq[idx] == 1 {
		out = float64(ampVol)
	}
	a.pulse1.phase += step * 8.0
	for a.pulse1.phase >= 8.0 {
		a.pulse1.phase -= 8.0
	}
	return out
}

func (a *apu) sampleTriangle() int16 {
	if !a.triangle1.enabled || a.triangle1.timer < 2 || a.triangle1.lengthCount == 0 {
		return 0
	}
	freq := ntscCPUHz / (32.0 * float64(a.triangle1.timer+1))
	step := freq / float64(AudioRate)
	idx := int(a.triangle1.phase) & 0x1F
	out := triangleStep(idx)
	a.triangle1.phase += step * 32.0
	for a.triangle1.phase >= 32.0 {
		a.triangle1.phase -= 32.0
	}
	return out
}

func (a *apu) sampleTriangleLevel() float64 {
	if !a.triangle1.enabled || a.triangle1.timer < 2 || a.triangle1.lengthCount == 0 {
		return 0
	}
	freq := ntscCPUHz / (32.0 * float64(a.triangle1.timer+1))
	step := freq / float64(AudioRate)
	idx := int(a.triangle1.phase) & 0x1F
	v := float64((idx ^ 0x1F) & 0x0F)
	a.triangle1.phase += step * 32.0
	for a.triangle1.phase >= 32.0 {
		a.triangle1.phase -= 32.0
	}
	return v
}

func (a *apu) sampleNoise() int16 {
	if !a.noise1.enabled || a.noise1.lengthCount == 0 {
		return 0
	}
	period := noisePeriodTable[a.noise1.periodIdx]
	bitRate := ntscCPUHz / float64(period)
	step := bitRate / float64(AudioRate)
	a.noise1.phase += step
	for a.noise1.phase >= 1.0 {
		a.noise1.phase -= 1.0
		bit0 := a.noise1.shiftReg & 0x01
		var tap uint16
		if a.noise1.mode == 1 {
			tap = (a.noise1.shiftReg >> 6) & 0x01
		} else {
			tap = (a.noise1.shiftReg >> 1) & 0x01
		}
		fb := bit0 ^ tap
		a.noise1.shiftReg >>= 1
		a.noise1.shiftReg |= fb << 14
	}
	if a.noise1.shiftReg&0x01 != 0 {
		return 0
	}
	ampVol := a.noise1.env.decay
	if a.noise1.env.constant {
		ampVol = a.noise1.volume
	}
	return int16((float64(ampVol) / 15.0) * 2500)
}

func (a *apu) sampleNoiseLevel() float64 {
	if !a.noise1.enabled || a.noise1.lengthCount == 0 {
		return 0
	}
	period := noisePeriodTable[a.noise1.periodIdx]
	bitRate := ntscCPUHz / float64(period)
	step := bitRate / float64(AudioRate)
	a.noise1.phase += step
	for a.noise1.phase >= 1.0 {
		a.noise1.phase -= 1.0
		bit0 := a.noise1.shiftReg & 0x01
		var tap uint16
		if a.noise1.mode == 1 {
			tap = (a.noise1.shiftReg >> 6) & 0x01
		} else {
			tap = (a.noise1.shiftReg >> 1) & 0x01
		}
		fb := bit0 ^ tap
		a.noise1.shiftReg >>= 1
		a.noise1.shiftReg |= fb << 14
	}
	if a.noise1.shiftReg&0x01 != 0 {
		return 0
	}
	ampVol := a.noise1.env.decay
	if a.noise1.env.constant {
		ampVol = a.noise1.volume
	}
	return float64(ampVol)
}

func (a *apu) sampleDMC(reader func(addr uint16) byte) int16 {
	if !a.dmc.enabled {
		return 0
	}
	bitPeriod := dmcRateTable[a.dmc.rateIdx]
	bitRate := ntscCPUHz / float64(bitPeriod)
	step := bitRate / float64(AudioRate)
	a.dmc.phase += step
	for a.dmc.phase >= 1.0 {
		a.dmc.phase -= 1.0
		a.tickDMC(reader)
	}
	return int16(int(a.dmc.outputLevel)-64) * 40
}

func (a *apu) sampleDMCLevel(reader func(addr uint16) byte) float64 {
	if !a.dmc.enabled {
		return 0
	}
	bitPeriod := dmcRateTable[a.dmc.rateIdx]
	bitRate := ntscCPUHz / float64(bitPeriod)
	step := bitRate / float64(AudioRate)
	a.dmc.phase += step
	for a.dmc.phase >= 1.0 {
		a.dmc.phase -= 1.0
		a.tickDMC(reader)
	}
	return float64(a.dmc.outputLevel)
}

func (a *apu) tickDMC(reader func(addr uint16) byte) {
	if a.dmc.bitsRemain == 0 {
		if a.dmc.bytesRemain == 0 {
			if a.dmc.loop {
				a.restartDMC()
			} else {
				return
			}
		}
		if reader == nil {
			return
		}
		a.dmc.shiftReg = reader(a.dmc.currentAddr)
		a.dmc.bitsRemain = 8
		a.dmc.currentAddr++
		if a.dmc.currentAddr < 0x8000 {
			a.dmc.currentAddr = 0x8000
		}
		a.dmc.bytesRemain--
	}
	bit := a.dmc.shiftReg & 0x01
	a.dmc.shiftReg >>= 1
	a.dmc.bitsRemain--
	if bit == 1 {
		if a.dmc.outputLevel <= 125 {
			a.dmc.outputLevel += 2
		}
	} else {
		if a.dmc.outputLevel >= 2 {
			a.dmc.outputLevel -= 2
		}
	}
}

func (a *apu) restartDMC() {
	a.dmc.currentAddr = 0xC000 + uint16(a.dmc.sampleAddrReg)*64
	a.dmc.bytesRemain = uint16(a.dmc.sampleLenReg)*16 + 1
	a.dmc.bitsRemain = 0
}

func triangleStep(idx int) int16 {
	if idx < 16 {
		return int16((15-idx)*180 - 1350)
	}
	return int16((idx-16)*180 - 1350)
}

func mixSample(pulse1, tri, noise, dmc float64) int16 {
	pulseOut := 0.0
	if pulse1 > 0 {
		pulseOut = 95.88 / ((8128.0 / pulse1) + 100.0)
	}
	tndIn := tri/8227.0 + noise/12241.0 + dmc/22638.0
	tndOut := 0.0
	if tndIn > 0 {
		tndOut = 159.79 / ((1.0 / tndIn) + 100.0)
	}
	v := (pulseOut + tndOut) * 32767.0 * 0.8
	if v > 32767 {
		v = 32767
	}
	if v < -32768 {
		v = -32768
	}
	return int16(v)
}
