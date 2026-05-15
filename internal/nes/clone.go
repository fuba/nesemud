package nes

import "sync"

func (c *Cartridge) clone() *Cartridge {
	if c == nil {
		return nil
	}
	cp := *c
	cp.PRG = append([]byte(nil), c.PRG...)
	cp.CHR = append([]byte(nil), c.CHR...)
	cp.PRGRAM = append([]byte(nil), c.PRGRAM...)
	return &cp
}

func (p *ppu) clone() *ppu {
	if p == nil {
		return nil
	}
	cp := *p
	cp.frameRGB = append([]byte(nil), p.frameRGB...)
	cp.frameBGOpaq = append([]bool(nil), p.frameBGOpaq...)
	return &cp
}

func (c *Console) cloneLocked() *Console {
	cp := *c
	cp.mu = sync.RWMutex{}
	cp.cart = c.cart.clone()
	if c.cpu != nil {
		cpu := *c.cpu
		cp.cpu = &cpu
	}
	cp.ppu = c.ppu.clone()
	if c.apu != nil {
		apu := *c.apu
		cp.apu = &apu
	}
	cp.lastFrame = append([]byte(nil), c.lastFrame...)
	cp.audioSamples = append([]int16(nil), c.audioSamples...)
	cp.pendingPPUWrites = append([]ppuRegisterWrite(nil), c.pendingPPUWrites...)
	if c.replay != nil {
		replay := &Replay{Frames: append([]FrameInput(nil), c.replay.Frames...)}
		cp.replay = replay
	}
	return &cp
}

func (c *Console) Clone() *Console {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cloneLocked()
}
