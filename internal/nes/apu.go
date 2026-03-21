package nes

const ntscCPUHz = 1789773.0

var dutyTable = [4][8]byte{
	{0, 1, 0, 0, 0, 0, 0, 0},
	{0, 1, 1, 0, 0, 0, 0, 0},
	{0, 1, 1, 1, 1, 0, 0, 0},
	{1, 0, 0, 1, 1, 1, 1, 1},
}

var triangleTable = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8,
	7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7,
	8, 9, 10, 11, 12, 13, 14, 15,
}

var lengthTable = [32]byte{10, 254, 20, 2, 40, 4, 80, 6, 160, 8, 60, 10, 14, 12, 26, 14, 12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30}
var noisePeriodTable = [16]uint16{4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068}
var dmcRateTable = [16]uint16{428, 380, 340, 320, 286, 254, 226, 214, 190, 160, 142, 128, 106, 84, 72, 54}

type envelope struct {
	period   byte
	constant bool
	loop     bool
	start    bool
	divider  byte
	decay    byte
}

type sweepUnit struct {
	enabled bool
	period  byte
	negate  bool
	shift   byte
	reload  bool
	divider byte
}

type pulseChannel struct {
	enabled     bool
	duty        byte
	volume      byte
	timer       uint16
	timerValue  uint16
	sequencePos byte
	lengthHalt  bool
	lengthCount byte
	env         envelope
	sweep       sweepUnit
}

type triangleChannel struct {
	enabled      bool
	timer        uint16
	timerValue   uint16
	sequencePos  byte
	lengthHalt   bool
	lengthCount  byte
	control      bool
	linearReload byte
	linearCount  byte
	reloadFlag   bool
}

type noiseChannel struct {
	enabled     bool
	mode        byte
	periodIdx   byte
	volume      byte
	shiftReg    uint16
	timerValue  uint16
	lengthHalt  bool
	lengthCount byte
	env         envelope
}

type dmcChannel struct {
	enabled       bool
	irqPending    bool
	irqEnable     bool
	loop          bool
	rateIdx       byte
	outputLevel   byte
	sampleAddrReg byte
	sampleLenReg  byte
	currentAddr   uint16
	bytesRemain   uint16
	sampleBuffer  byte
	bufferEmpty   bool
	silence       bool
	shiftReg      byte
	bitsRemain    byte
	timerValue    uint16
	restartCount  uint64
}

type apu struct {
	pulse1                 pulseChannel
	pulse2                 pulseChannel
	triangle1              triangleChannel
	noise1                 noiseChannel
	dmc                    dmcChannel
	hp90X                  float64
	hp90Y                  float64
	hp440X                 float64
	hp440Y                 float64
	lp14kY                 float64
	lastWrite4008          byte
	lastWrite400B          byte
	lastWrite4015          byte
	lastWrite4017          byte
	writeCount4008         uint64
	writeCount400B         uint64
	writeCount4015         uint64
	writeCount4017         uint64
	frameCounterStep       int
	frameCounterCycle      int
	frameCounter5Step      bool
	frameIRQ               bool
	frameIRQInhibit        bool
	frameCounterWriteDelay int
	frameCounterWriteValue byte
	cpuCycleParity         bool
	pendingCPUStallCycles  int
	sampleCycleRemainder   float64
}

func newAPU() *apu {
	a := &apu{}
	a.Reset()
	return a
}

func (a *apu) Reset() {
	a.pulse1 = pulseChannel{}
	a.pulse2 = pulseChannel{}
	a.triangle1 = triangleChannel{}
	a.noise1 = noiseChannel{shiftReg: 1}
	a.dmc = dmcChannel{
		bufferEmpty: true,
		silence:     true,
		bitsRemain:  8,
	}
	a.hp90X = 0
	a.hp90Y = 0
	a.hp440X = 0
	a.hp440Y = 0
	a.lp14kY = 0
	a.lastWrite4008 = 0
	a.lastWrite400B = 0
	a.lastWrite4015 = 0
	a.lastWrite4017 = 0
	a.writeCount4008 = 0
	a.writeCount400B = 0
	a.writeCount4015 = 0
	a.writeCount4017 = 0
	a.frameCounterStep = 0
	a.frameCounterCycle = 0
	a.frameCounter5Step = false
	a.frameIRQ = false
	a.frameIRQInhibit = false
	a.frameCounterWriteDelay = 0
	a.frameCounterWriteValue = 0
	a.cpuCycleParity = false
	a.pendingCPUStallCycles = 0
	a.sampleCycleRemainder = 0
}

func (a *apu) ReadStatus() byte {
	v := a.PeekStatus()
	a.frameIRQ = false
	return v
}

func (a *apu) PeekStatus() byte {
	var v byte
	if a.pulse1.lengthCount > 0 {
		v |= 0x01
	}
	if a.pulse2.lengthCount > 0 {
		v |= 0x02
	}
	if a.triangle1.lengthCount > 0 {
		v |= 0x04
	}
	if a.noise1.lengthCount > 0 {
		v |= 0x08
	}
	if a.dmc.bytesRemain > 0 {
		v |= 0x10
	}
	if a.frameIRQ {
		v |= 0x40
	}
	if a.dmc.irqPending {
		v |= 0x80
	}
	return v
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
	case 0x4001:
		a.writeSweep(&a.pulse1.sweep, value)
	case 0x4002:
		a.pulse1.timer = (a.pulse1.timer & 0x0700) | uint16(value)
	case 0x4003:
		a.pulse1.timer = (a.pulse1.timer & 0x00FF) | (uint16(value&0x07) << 8)
		a.pulse1.timerValue = a.pulse1.timer
		a.pulse1.sequencePos = 0
		if a.pulse1.enabled {
			a.pulse1.lengthCount = lengthTable[(value>>3)&0x1F]
		}
		a.pulse1.env.start = true
		a.pulse1.sweep.reload = true

	case 0x4004:
		a.pulse2.duty = (value >> 6) & 0x03
		a.pulse2.lengthHalt = value&0x20 != 0
		a.pulse2.env.loop = value&0x20 != 0
		a.pulse2.env.constant = value&0x10 != 0
		a.pulse2.env.period = value & 0x0F
		a.pulse2.volume = value & 0x0F
	case 0x4005:
		a.writeSweep(&a.pulse2.sweep, value)
	case 0x4006:
		a.pulse2.timer = (a.pulse2.timer & 0x0700) | uint16(value)
	case 0x4007:
		a.pulse2.timer = (a.pulse2.timer & 0x00FF) | (uint16(value&0x07) << 8)
		a.pulse2.timerValue = a.pulse2.timer
		a.pulse2.sequencePos = 0
		if a.pulse2.enabled {
			a.pulse2.lengthCount = lengthTable[(value>>3)&0x1F]
		}
		a.pulse2.env.start = true
		a.pulse2.sweep.reload = true

	case 0x4008:
		a.lastWrite4008 = value
		a.writeCount4008++
		a.triangle1.control = value&0x80 != 0
		a.triangle1.lengthHalt = a.triangle1.control
		a.triangle1.linearReload = value & 0x7F
	case 0x400A:
		a.triangle1.timer = (a.triangle1.timer & 0x0700) | uint16(value)
	case 0x400B:
		a.lastWrite400B = value
		a.writeCount400B++
		a.triangle1.timer = (a.triangle1.timer & 0x00FF) | (uint16(value&0x07) << 8)
		a.triangle1.timerValue = a.triangle1.timer
		a.triangle1.sequencePos = 0
		if a.triangle1.enabled {
			a.triangle1.lengthCount = lengthTable[(value>>3)&0x1F]
		}
		a.triangle1.reloadFlag = true

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
		if a.noise1.enabled {
			a.noise1.lengthCount = lengthTable[(value>>3)&0x1F]
		}
		a.noise1.env.start = true
		if a.noise1.shiftReg == 0 {
			a.noise1.shiftReg = 1
		}

	case 0x4010:
		a.dmc.irqEnable = value&0x80 != 0
		if !a.dmc.irqEnable {
			a.dmc.irqPending = false
		}
		a.dmc.loop = value&0x40 != 0
		a.dmc.rateIdx = value & 0x0F
		a.dmc.timerValue = dmcRateTable[a.dmc.rateIdx]
	case 0x4011:
		a.dmc.outputLevel = value & 0x7F
	case 0x4012:
		a.dmc.sampleAddrReg = value
	case 0x4013:
		a.dmc.sampleLenReg = value

	case 0x4015:
		a.lastWrite4015 = value
		a.writeCount4015++
		a.frameIRQ = false
		a.dmc.irqPending = false
		a.pulse1.enabled = value&0x01 != 0
		if !a.pulse1.enabled {
			a.pulse1.lengthCount = 0
		}
		a.pulse2.enabled = value&0x02 != 0
		if !a.pulse2.enabled {
			a.pulse2.lengthCount = 0
		}
		a.triangle1.enabled = value&0x04 != 0
		if !a.triangle1.enabled {
			a.triangle1.lengthCount = 0
			a.triangle1.linearCount = 0
		}
		a.noise1.enabled = value&0x08 != 0
		if !a.noise1.enabled {
			a.noise1.lengthCount = 0
		}
		a.dmc.enabled = value&0x10 != 0
		if !a.dmc.enabled {
			a.dmc.bytesRemain = 0
		} else if a.dmc.bytesRemain == 0 {
			a.restartDMC()
		}
	case 0x4017:
		a.lastWrite4017 = value
		a.writeCount4017++
		if value&0x40 != 0 {
			a.frameIRQ = false
		}
		a.frameIRQInhibit = value&0x40 != 0
		a.frameCounterWriteValue = value
		a.frameCounterWriteDelay = 2
		if a.cpuCycleParity {
			a.frameCounterWriteDelay = 3
		}
	}
}

func (a *apu) GenerateFrameSamples(monoSamples int, reader func(addr uint16) byte) []int16 {
	if monoSamples <= 0 {
		return nil
	}
	out := make([]int16, monoSamples*2)
	cyclesPerSample := ntscCPUHz / float64(AudioRate)

	for i := 0; i < monoSamples; i++ {
		a.sampleCycleRemainder += cyclesPerSample
		stepCycles := int(a.sampleCycleRemainder)
		a.sampleCycleRemainder -= float64(stepCycles)
		a.StepCycles(stepCycles, reader)
		sample := a.Sample(reader)
		out[i*2] = sample
		out[i*2+1] = sample
	}

	return out
}

func (a *apu) GenerateSample(reader func(addr uint16) byte) int16 {
	a.StepCycles(1, reader)
	return a.Sample(reader)
}

func (a *apu) Sample(reader func(addr uint16) byte) int16 {
	pulse := a.samplePulseLevel(&a.pulse1, false)
	pulse += a.samplePulseLevel(&a.pulse2, true)
	tri := a.sampleTriangleLevel()
	noi := a.sampleNoiseLevel()
	dmc := a.sampleDMCLevel()
	return a.applyOutputFilter(mixSample(pulse, tri, noi, dmc))
}

func (a *apu) StepCycles(cycles int, reader func(addr uint16) byte) {
	for i := 0; i < cycles; i++ {
		a.stepCycle(reader)
	}
}

func (a *apu) stepCycle(reader func(addr uint16) byte) {
	a.frameCounterCycle++
	a.serviceDMCReader(reader)
	a.clockFrameCounter()
	a.clockTriangleTimer()
	a.clockDMCTimer(reader)
	a.clockFrameCounterWriteDelay()
	a.cpuCycleParity = !a.cpuCycleParity
	if a.cpuCycleParity {
		a.clockPulseTimer(&a.pulse1)
		a.clockPulseTimer(&a.pulse2)
		a.clockNoiseTimer()
	}
}

func (a *apu) clockFrameCounterWriteDelay() {
	if a.frameCounterWriteDelay == 0 {
		return
	}
	a.frameCounterWriteDelay--
	if a.frameCounterWriteDelay != 0 {
		return
	}
	a.frameCounter5Step = a.frameCounterWriteValue&0x80 != 0
	a.frameCounterStep = 0
	a.frameCounterCycle = 0
	if a.frameCounter5Step {
		a.tickQuarterFrame()
		a.tickHalfFrame()
	}
}

func (a *apu) tickFrameSequencer() {
	if a.frameCounter5Step {
		switch a.frameCounterStep {
		case 0, 1, 2, 3:
			a.tickQuarterFrame()
		}
		switch a.frameCounterStep {
		case 1, 3:
			a.tickHalfFrame()
		}
		a.frameCounterStep = (a.frameCounterStep + 1) % 5
		return
	}
	a.tickQuarterFrame()
	if a.frameCounterStep%2 == 1 {
		a.tickHalfFrame()
	}
	a.frameCounterStep = (a.frameCounterStep + 1) & 0x03
}

func (a *apu) clockFrameCounter() {
	if a.frameCounter5Step {
		switch a.frameCounterCycle {
		case 3729, 7457, 11186, 18641:
			a.tickQuarterFrame()
		}
		switch a.frameCounterCycle {
		case 7457, 18641:
			a.tickHalfFrame()
		}
		if a.frameCounterCycle >= 18641 {
			a.frameCounterCycle = 0
		}
		return
	}

	switch a.frameCounterCycle {
	case 3729, 7457, 11186, 14915:
		a.tickQuarterFrame()
	}
	switch a.frameCounterCycle {
	case 7457, 14915:
		a.tickHalfFrame()
	}
	if a.frameCounterCycle == 14915 && !a.frameIRQInhibit {
		a.frameIRQ = true
	}
	if a.frameCounterCycle >= 14915 {
		a.frameCounterCycle = 0
	}
}

func (a *apu) tickQuarterFrame() {
	a.clockEnvelope(&a.pulse1.env)
	a.clockEnvelope(&a.pulse2.env)
	a.clockEnvelope(&a.noise1.env)
	a.clockTriangleLinear(&a.triangle1)
}

func (a *apu) tickHalfFrame() {
	a.clockLengthPulse(&a.pulse1)
	a.clockLengthPulse(&a.pulse2)
	a.clockLengthNoise(&a.noise1)
	a.clockLengthTriangle(&a.triangle1)
	a.clockSweep(&a.pulse1, false)
	a.clockSweep(&a.pulse2, true)
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

func (a *apu) clockTriangleLinear(ch *triangleChannel) {
	if ch.reloadFlag {
		ch.linearCount = ch.linearReload
	} else if ch.linearCount > 0 {
		ch.linearCount--
	}
	if !ch.control {
		ch.reloadFlag = false
	}
}

func (a *apu) writeSweep(sw *sweepUnit, value byte) {
	sw.enabled = value&0x80 != 0
	sw.period = (value >> 4) & 0x07
	sw.negate = value&0x08 != 0
	sw.shift = value & 0x07
	sw.reload = true
}

func (a *apu) clockSweep(ch *pulseChannel, onesComplement bool) {
	if ch.sweep.divider == 0 && ch.sweep.enabled && ch.sweep.shift > 0 && ch.timer >= 8 {
		delta := ch.timer >> ch.sweep.shift
		target := int(ch.timer)
		if ch.sweep.negate {
			target -= int(delta)
			if onesComplement {
				target--
			}
		} else {
			target += int(delta)
		}
		if target >= 0 && target <= 0x7FF {
			ch.timer = uint16(target)
		}
	}
	if ch.sweep.reload || ch.sweep.divider == 0 {
		ch.sweep.divider = ch.sweep.period
		ch.sweep.reload = false
	} else {
		ch.sweep.divider--
	}
}

func (a *apu) clockPulseTimer(ch *pulseChannel) {
	if ch.timerValue == 0 {
		ch.timerValue = ch.timer
		ch.sequencePos = (ch.sequencePos + 1) & 0x07
		return
	}
	ch.timerValue--
}

func (a *apu) clockTriangleTimer() {
	if a.triangle1.timerValue == 0 {
		a.triangle1.timerValue = a.triangle1.timer
		if a.triangle1.enabled && a.triangle1.timer >= 2 && a.triangle1.lengthCount > 0 && a.triangle1.linearCount > 0 {
			a.triangle1.sequencePos = (a.triangle1.sequencePos + 1) & 0x1F
		}
		return
	}
	a.triangle1.timerValue--
}

func (a *apu) clockNoiseTimer() {
	if a.noise1.timerValue == 0 {
		a.noise1.timerValue = noisePeriodTable[a.noise1.periodIdx]
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
		return
	}
	a.noise1.timerValue--
}

func (a *apu) clockDMCTimer(reader func(addr uint16) byte) {
	if a.dmc.timerValue == 0 {
		a.dmc.timerValue = dmcRateTable[a.dmc.rateIdx]
		a.tickDMC(reader)
		return
	}
	a.dmc.timerValue--
}

func (a *apu) samplePulseLevel(ch *pulseChannel, onesComplement bool) float64 {
	if !ch.enabled || ch.timer < 8 || ch.lengthCount == 0 {
		return 0
	}
	if ch.sweep.shift > 0 {
		delta := ch.timer >> ch.sweep.shift
		target := ch.timer + delta
		if ch.sweep.negate {
			target = ch.timer - delta
			if onesComplement {
				target--
			}
		}
		if target > 0x7FF {
			return 0
		}
	}
	ampVol := ch.env.decay
	if ch.env.constant {
		ampVol = ch.volume
	}
	if dutyTable[ch.duty][ch.sequencePos] == 0 {
		return 0
	}
	return float64(ampVol)
}

func (a *apu) sampleTriangleLevel() float64 {
	if !a.triangle1.enabled || a.triangle1.timer < 2 || a.triangle1.lengthCount == 0 || a.triangle1.linearCount == 0 {
		return 0
	}
	return float64(triangleTable[a.triangle1.sequencePos&0x1F])
}

func (a *apu) sampleNoiseLevel() float64 {
	if !a.noise1.enabled || a.noise1.lengthCount == 0 {
		return 0
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

func (a *apu) sampleDMCLevel() float64 {
	return float64(a.dmc.outputLevel)
}

func (a *apu) tickDMC(reader func(addr uint16) byte) {
	if !a.dmc.silence {
		bit := a.dmc.shiftReg & 0x01
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
	a.dmc.shiftReg >>= 1
	if a.dmc.bitsRemain > 0 {
		a.dmc.bitsRemain--
	}
	if a.dmc.bitsRemain == 0 {
		a.dmc.bitsRemain = 8
		if a.dmc.bufferEmpty {
			a.dmc.silence = true
		} else {
			a.dmc.silence = false
			a.dmc.shiftReg = a.dmc.sampleBuffer
			a.dmc.bufferEmpty = true
		}
	}
	a.serviceDMCReader(reader)
}

func (a *apu) serviceDMCReader(reader func(addr uint16) byte) {
	if reader == nil || a.dmc.bytesRemain == 0 || !a.dmc.bufferEmpty {
		return
	}
	a.dmc.sampleBuffer = reader(a.dmc.currentAddr)
	a.dmc.bufferEmpty = false
	a.pendingCPUStallCycles += 4
	a.dmc.currentAddr++
	if a.dmc.currentAddr == 0x0000 {
		a.dmc.currentAddr = 0x8000
	}
	a.dmc.bytesRemain--
	if a.dmc.bytesRemain != 0 {
		return
	}
	if a.dmc.loop {
		a.restartDMC()
		return
	}
	if a.dmc.irqEnable {
		a.dmc.irqPending = true
	}
}

func (a *apu) restartDMC() {
	a.dmc.currentAddr = 0xC000 + uint16(a.dmc.sampleAddrReg)*64
	a.dmc.bytesRemain = uint16(a.dmc.sampleLenReg)*16 + 1
	a.dmc.restartCount++
}

func (a *apu) consumeCPUStallCycles() int {
	stall := a.pendingCPUStallCycles
	a.pendingCPUStallCycles = 0
	return stall
}

func (a *apu) irqPending() bool {
	return a.frameIRQ || a.dmc.irqPending
}

func triangleStep(idx int) int16 {
	return int16(int(triangleTable[idx&0x1F])*180 - 1350)
}

func (a *apu) applyOutputFilter(sample int16) int16 {
	x := float64(sample)
	y1 := highPassFilter(x, &a.hp90X, &a.hp90Y, 90.0)
	y2 := highPassFilter(y1, &a.hp440X, &a.hp440Y, 440.0)
	y3 := lowPassFilter(y2, &a.lp14kY, 14000.0)
	if y3 > 32767 {
		y3 = 32767
	}
	if y3 < -32768 {
		y3 = -32768
	}
	return int16(y3)
}

func highPassFilter(x float64, prevX *float64, prevY *float64, cutoff float64) float64 {
	alpha := filterAlpha(cutoff)
	y := alpha * (*prevY + x - *prevX)
	*prevX = x
	*prevY = y
	return y
}

func lowPassFilter(x float64, prevY *float64, cutoff float64) float64 {
	alpha := lowPassAlpha(cutoff)
	y := *prevY + alpha*(x-*prevY)
	*prevY = y
	return y
}

func filterAlpha(cutoff float64) float64 {
	rc := 1.0 / (2.0 * 3.141592653589793 * cutoff)
	dt := 1.0 / float64(AudioRate)
	return rc / (rc + dt)
}

func lowPassAlpha(cutoff float64) float64 {
	rc := 1.0 / (2.0 * 3.141592653589793 * cutoff)
	dt := 1.0 / float64(AudioRate)
	return dt / (rc + dt)
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
