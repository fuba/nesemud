package nes

import (
	"os"
	"time"
)

func (c *Console) LoadROMFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
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
	const cyclesPerFrame = 29780
	startCycles := c.cpu.Cycles
	for c.cpu.Cycles-startCycles < cyclesPerFrame {
		prevCycles := c.cpu.Cycles
		if err := c.cpu.Step(c); err != nil {
			c.paused = true
			break
		}
		if c.ppu.step(c, int(c.cpu.Cycles-prevCycles)) {
			c.cpu.NMI(c)
		}
		if c.cart != nil && c.cart.consumeIRQ() {
			c.cpu.IRQ(c)
		}
	}
	c.frameCount++
	if c.cart != nil {
		c.ppu.renderFrame(c, c.lastFrame)
	} else {
		c.renderFallbackFrameLocked()
	}
	c.renderAudioFromAPULocked()
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
	return map[string]any{
		"paused":        c.paused,
		"frame_count":   c.frameCount,
		"rom_loaded":    c.cart != nil,
		"replay_active": c.replay != nil && c.replayCursor < len(c.replay.Frames),
		"replay_cursor": c.replayCursor,
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
			"status":   c.ppu.status,
			"ctrl":     c.ppu.ctrl,
			"mask":     c.ppu.mask,
		},
		"apu": map[string]any{
			"status":          c.apu.ReadStatus(),
			"pulse1_length":   c.apu.pulse1.lengthCount,
			"pulse1_env":      c.apu.pulse1.env.decay,
			"triangle_length": c.apu.triangle1.lengthCount,
			"noise_length":    c.apu.noise1.lengthCount,
			"noise_env":       c.apu.noise1.env.decay,
			"dmc_level":       c.apu.dmc.outputLevel,
		},
	}
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

func (c *Console) StepInstruction() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return nil
	}
	prev := c.cpu.Cycles
	if err := c.cpu.Step(c); err != nil {
		c.paused = true
		return err
	}
	if c.ppu.step(c, int(c.cpu.Cycles-prev)) {
		c.cpu.NMI(c)
	}
	if c.cart != nil && c.cart.consumeIRQ() {
		c.cpu.IRQ(c)
	}
	return nil
}
