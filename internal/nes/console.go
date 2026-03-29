package nes

import (
	"math"
	"os"
	"time"
)

func (c *Console) LoadROMFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return c.LoadROMContent(data)
}

func (c *Console) LoadROMContent(data []byte) error {
	cart, err := LoadINES(data)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cart = cart
	c.resetLocked()
	return nil
}

func (c *Console) LoadFM2Content(content []byte) error {
	replay, err := ParseFM2(content)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetLocked()
	c.replay = replay
	c.replayCursor = 0
	return nil
}

func (c *Console) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetLocked()
}

func (c *Console) resetLocked() {
	for i := range c.ram {
		c.ram[i] = 0
	}
	c.frameCount = 0
	c.controllerP1 = Buttons{}
	c.controllerP2 = Buttons{}
	c.cpu.PowerOn()
	c.cpu.Reset(c)
	c.ppu.Reset()
	c.apu.Reset()
	c.lastCPUError = ""
	c.nmiPending = false
	c.nmiDelayInstr = 0
	c.irqDelayInstr = 0
	c.lastFrameTime = time.Now()
	c.renderFallbackFrameLocked()
}

func (c *Console) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = paused
}

func (c *Console) SetController(player int, buttons Buttons) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if player == 1 {
		c.controllerP1 = buttons
		return
	}
	if player == 2 {
		c.controllerP2 = buttons
	}
}

func (c *Console) Peek(addr uint16, n int) ([]byte, error) {
	if n <= 0 || n > 4096 {
		return nil, ErrInvalidRange
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	buf := make([]byte, n)
	for i := 0; i < n; i++ {
		buf[i] = c.readCPU(uint16(uint32(addr) + uint32(i)))
	}
	return buf, nil
}

func (c *Console) Poke(addr uint16, data []byte) error {
	if len(data) == 0 || len(data) > 4096 {
		return ErrInvalidRange
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := 0; i < len(data); i++ {
		c.writeCPU(uint16(uint32(addr)+uint32(i)), data[i])
	}
	return nil
}

func (c *Console) StepFrame() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return
	}
	if c.replay != nil && c.replayCursor < len(c.replay.Frames) {
		c.controllerP1 = c.replay.Frames[c.replayCursor].P1
		c.controllerP2 = c.replay.Frames[c.replayCursor].P2
		c.latchControllers()
		c.replayCursor++
	}
	const monoSamplesPerFrame = AudioRate / TargetFPS
	samplePeriodCPU := ntscCPUHz / float64(AudioRate)
	sampleIndex := 0
	nextSampleAt := samplePeriodCPU
	startCycles := c.cpu.Cycles
	startPPUFrame := c.ppu.frameID
	for c.ppu.frameID == startPPUFrame {
		c.serviceQueuedNMIIfReadyLocked(startCycles, &sampleIndex, &nextSampleAt, monoSamplesPerFrame)
		prevCycles := c.cpu.Cycles
		prevP := c.cpu.P
		c.beginDeferredPPUWritesLocked()
		if err := c.cpu.Step(c); err != nil {
			c.endDeferredPPUWritesLocked()
			c.lastCPUError = err.Error()
			c.paused = true
			break
		}
		c.endDeferredPPUWritesLocked()
		c.noteIRQUnmaskByInstructionLocked(prevP)
		cpuCycles := int(c.cpu.Cycles - prevCycles)
		c.advanceInstructionEffectsLocked(cpuCycles, startCycles, &sampleIndex, &nextSampleAt, monoSamplesPerFrame)
		c.finishInstructionBoundaryLocked()
		if c.paused {
			break
		}
	}
	if c.ppu.frameID != startPPUFrame {
		c.frameCount++
	}
	if c.cart == nil {
		c.renderFallbackFrameLocked()
	}
	for sampleIndex < monoSamplesPerFrame {
		s := c.apu.Sample(c.readCPU)
		c.audioSamples[sampleIndex*2] = s
		c.audioSamples[sampleIndex*2+1] = s
		sampleIndex++
	}
	c.lastFrameTime = time.Now()
}

func (c *Console) SnapshotFrame() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]byte(nil), c.lastFrame...)
}

func (c *Console) SnapshotAudio() []int16 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make([]int16, len(c.audioSamples))
	copy(cp, c.audioSamples)
	return cp
}

func (c *Console) State() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	audioPeak, audioRMS, audioActive := summarizeAudio(c.audioSamples)
	return map[string]any{
		"paused":         c.paused,
		"last_cpu_error": c.lastCPUError,
		"frame_count":    c.frameCount,
		"rom_loaded":     c.cart != nil,
		"replay_active":  c.replay != nil && c.replayCursor < len(c.replay.Frames),
		"replay_cursor":  c.replayCursor,
		"cpu": map[string]any{
			"pc":     c.cpu.PC,
			"a":      c.cpu.A,
			"x":      c.cpu.X,
			"y":      c.cpu.Y,
			"sp":     c.cpu.SP,
			"p":      c.cpu.P,
			"cycles": c.cpu.Cycles,
		},
		"ppu": map[string]any{
			"scanline": c.ppu.scanline,
			"cycle":    c.ppu.cycle,
			"frame_id": c.ppu.frameID,
			"status":   c.ppu.status,
			"ctrl":     c.ppu.ctrl,
			"mask":     c.ppu.mask,
		},
		"apu": map[string]any{
			"status":                 c.apu.PeekStatus(),
			"frame_counter_cycle":    c.apu.frameCounterCycle,
			"frame_counter_5step":    c.apu.frameCounter5Step,
			"frame_irq":              c.apu.frameIRQ,
			"last_write_4008":        c.apu.lastWrite4008,
			"last_write_400b":        c.apu.lastWrite400B,
			"last_write_4015":        c.apu.lastWrite4015,
			"last_write_4017":        c.apu.lastWrite4017,
			"write_count_4008":       c.apu.writeCount4008,
			"write_count_400b":       c.apu.writeCount400B,
			"write_count_4015":       c.apu.writeCount4015,
			"write_count_4017":       c.apu.writeCount4017,
			"pulse1_enabled":         c.apu.pulse1.enabled,
			"pulse1_length":          c.apu.pulse1.lengthCount,
			"pulse1_env":             c.apu.pulse1.env.decay,
			"pulse1_timer":           c.apu.pulse1.timer,
			"pulse2_enabled":         c.apu.pulse2.enabled,
			"pulse2_length":          c.apu.pulse2.lengthCount,
			"pulse2_env":             c.apu.pulse2.env.decay,
			"pulse2_timer":           c.apu.pulse2.timer,
			"triangle_enabled":       c.apu.triangle1.enabled,
			"triangle_control":       c.apu.triangle1.control,
			"triangle_length":        c.apu.triangle1.lengthCount,
			"triangle_linear":        c.apu.triangle1.linearCount,
			"triangle_linear_reload": c.apu.triangle1.linearReload,
			"triangle_reload_flag":   c.apu.triangle1.reloadFlag,
			"triangle_timer":         c.apu.triangle1.timer,
			"noise_enabled":          c.apu.noise1.enabled,
			"noise_length":           c.apu.noise1.lengthCount,
			"noise_env":              c.apu.noise1.env.decay,
			"noise_period":           c.apu.noise1.periodIdx,
			"dmc_enabled":            c.apu.dmc.enabled,
			"dmc_level":              c.apu.dmc.outputLevel,
			"dmc_bytes":              c.apu.dmc.bytesRemain,
		},
		"audio": map[string]any{
			"peak_abs":       audioPeak,
			"rms":            audioRMS,
			"active_samples": audioActive,
			"sample_count":   len(c.audioSamples),
		},
		"controllers": map[string]any{
			"strobe":   c.controllerStrobe,
			"p1_shift": c.controllerShift[0],
			"p2_shift": c.controllerShift[1],
			"p1": map[string]any{
				"a":      c.controllerP1.A,
				"b":      c.controllerP1.B,
				"select": c.controllerP1.Select,
				"start":  c.controllerP1.Start,
				"up":     c.controllerP1.Up,
				"down":   c.controllerP1.Down,
				"left":   c.controllerP1.Left,
				"right":  c.controllerP1.Right,
			},
			"p2": map[string]any{
				"a":      c.controllerP2.A,
				"b":      c.controllerP2.B,
				"select": c.controllerP2.Select,
				"start":  c.controllerP2.Start,
				"up":     c.controllerP2.Up,
				"down":   c.controllerP2.Down,
				"left":   c.controllerP2.Left,
				"right":  c.controllerP2.Right,
			},
		},
	}
}

func summarizeAudio(samples []int16) (int, float64, int) {
	peak := 0
	sumSquares := 0.0
	active := 0
	for _, s := range samples {
		v := int(s)
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
		if s != 0 {
			active++
		}
		sumSquares += float64(s) * float64(s)
	}
	if len(samples) == 0 {
		return 0, 0, 0
	}
	return peak, math.Sqrt(sumSquares / float64(len(samples))), active
}

func (c *Console) SnapshotCPU() CPUState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CPUState{
		PC:     c.cpu.PC,
		A:      c.cpu.A,
		X:      c.cpu.X,
		Y:      c.cpu.Y,
		SP:     c.cpu.SP,
		P:      c.cpu.P,
		Cycles: c.cpu.Cycles,
	}
}

func (c *Console) SetCPUState(st CPUState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cpu.PC = st.PC
	c.cpu.A = st.A
	c.cpu.X = st.X
	c.cpu.Y = st.Y
	c.cpu.SP = st.SP
	c.cpu.P = st.P
	c.cpu.Cycles = st.Cycles
}

func (c *Console) StepInstruction() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return nil
	}
	c.serviceQueuedNMIIfReadyLocked(c.cpu.Cycles, nil, nil, 0)
	prev := c.cpu.Cycles
	prevP := c.cpu.P
	c.beginDeferredPPUWritesLocked()
	if err := c.cpu.Step(c); err != nil {
		c.endDeferredPPUWritesLocked()
		c.lastCPUError = err.Error()
		c.paused = true
		return err
	}
	c.endDeferredPPUWritesLocked()
	c.noteIRQUnmaskByInstructionLocked(prevP)
	cpuCycles := int(c.cpu.Cycles - prev)
	c.advanceInstructionEffectsLocked(cpuCycles, c.cpu.Cycles, nil, nil, 0)
	c.finishInstructionBoundaryLocked()
	return nil
}

func (c *Console) beginDeferredPPUWritesLocked() {
	c.deferPPUWrites = true
	c.pendingPPUWrites = c.pendingPPUWrites[:0]
}

func (c *Console) endDeferredPPUWritesLocked() {
	c.deferPPUWrites = false
}

func (c *Console) advanceInstructionEffectsLocked(cpuCycles int, startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	if cpuCycles <= 0 {
		return
	}
	if len(c.pendingPPUWrites) == 0 || cpuCycles == 1 {
		c.advanceSubsystemsLocked(cpuCycles, startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
		c.applyAPUDrivenStallLocked()
		return
	}
	c.advanceSubsystemsLocked(cpuCycles-1, startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
	for _, write := range c.pendingPPUWrites {
		c.ppu.cpuWriteRegister(c, write.addr, write.value)
	}
	c.pendingPPUWrites = c.pendingPPUWrites[:0]
	c.advanceSubsystemsLocked(1, startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
	c.applyAPUDrivenStallLocked()
}

func (c *Console) advanceSubsystemsLocked(cpuCycles int, startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	if cpuCycles <= 0 {
		return
	}
	c.apu.StepCycles(cpuCycles, c.readCPU)
	if c.ppu.step(c, cpuCycles) {
		c.queueNMIInterruptLocked()
	}
	if (c.cart != nil && c.cart.irqPending()) || c.apu.irqPending() {
		c.serviceQueuedIRQIfReadyLocked(startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
	}
	if sampleIndex == nil || nextSampleAt == nil {
		return
	}
	elapsed := float64(c.cpu.Cycles - startCycles)
	for *sampleIndex < monoSamplesPerFrame && elapsed >= *nextSampleAt {
		s := c.apu.Sample(c.readCPU)
		c.audioSamples[*sampleIndex*2] = s
		c.audioSamples[*sampleIndex*2+1] = s
		*sampleIndex = *sampleIndex + 1
		*nextSampleAt += ntscCPUHz / float64(AudioRate)
	}
}

func (c *Console) applyAPUDrivenStallLocked() {
	for {
		stallCycles := c.apu.consumeCPUStallCycles()
		if stallCycles == 0 {
			return
		}
		c.cpu.Cycles += uint64(stallCycles)
		c.apu.StepCycles(stallCycles, c.readCPU)
		if c.ppu.step(c, stallCycles) {
			c.queueNMIInterruptLocked()
		}
		if (c.cart != nil && c.cart.irqPending()) || c.apu.irqPending() {
			c.serviceQueuedIRQIfReadyLocked(0, nil, nil, 0)
		}
	}
}

func (c *Console) serviceNMIInterruptLocked(startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	prevCycles := c.cpu.Cycles
	c.cpu.NMI(c)
	c.advanceInterruptEntryCyclesLocked(int(c.cpu.Cycles-prevCycles), startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
}

func (c *Console) serviceIRQInterruptLocked(startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	prevCycles := c.cpu.Cycles
	c.cpu.IRQ(c)
	c.advanceInterruptEntryCyclesLocked(int(c.cpu.Cycles-prevCycles), startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
}

func (c *Console) queueNMIInterruptLocked() {
	c.nmiPending = true
	if c.nmiDelayInstr < 1 {
		c.nmiDelayInstr = 1
	}
}

func (c *Console) serviceQueuedNMIIfReadyLocked(startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	if !c.nmiPending {
		return
	}
	if c.nmiDelayInstr > 0 {
		c.nmiDelayInstr--
		return
	}
	c.nmiPending = false
	c.serviceNMIInterruptLocked(startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
}

func (c *Console) serviceQueuedIRQIfReadyLocked(startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	if c.irqDelayInstr > 0 {
		return
	}
	c.serviceIRQInterruptLocked(startCycles, sampleIndex, nextSampleAt, monoSamplesPerFrame)
}

func (c *Console) noteIRQUnmaskByInstructionLocked(prevP byte) {
	if prevP&flagI == 0 || c.cpu.P&flagI != 0 {
		return
	}
	if c.irqDelayInstr < 1 {
		c.irqDelayInstr = 1
	}
}

func (c *Console) finishInstructionBoundaryLocked() {
	if c.irqDelayInstr > 0 {
		c.irqDelayInstr--
	}
}

func (c *Console) advanceInterruptEntryCyclesLocked(cpuCycles int, startCycles uint64, sampleIndex *int, nextSampleAt *float64, monoSamplesPerFrame int) {
	if cpuCycles <= 0 {
		return
	}
	c.apu.StepCycles(cpuCycles, c.readCPU)
	_ = c.ppu.step(c, cpuCycles)
	if sampleIndex == nil || nextSampleAt == nil {
		return
	}
	elapsed := float64(c.cpu.Cycles - startCycles)
	for *sampleIndex < monoSamplesPerFrame && elapsed >= *nextSampleAt {
		s := c.apu.Sample(c.readCPU)
		c.audioSamples[*sampleIndex*2] = s
		c.audioSamples[*sampleIndex*2+1] = s
		*sampleIndex = *sampleIndex + 1
		*nextSampleAt += ntscCPUHz / float64(AudioRate)
	}
}
