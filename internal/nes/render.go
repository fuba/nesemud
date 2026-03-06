package nes

func (c *Console) renderFallbackFrameLocked() {
	for y := 0; y < FrameHeight; y++ {
		for x := 0; x < FrameWidth; x++ {
			o := (y*FrameWidth + x) * 3
			seed := byte((int(c.frameCount) + x + y) & 0xFF)
			c.lastFrame[o+0] = seed
			c.lastFrame[o+1] = byte(x)
			c.lastFrame[o+2] = byte(y)
		}
	}
}

func (c *Console) renderSilenceAudioLocked() {
	for i := range c.audioSamples {
		c.audioSamples[i] = 0
	}
}

func (c *Console) renderAudioFromAPULocked() {
	mono := len(c.audioSamples) / 2
	frame := c.apu.GenerateFrameSamples(mono, c.readCPU)
	if len(frame) != len(c.audioSamples) {
		c.renderSilenceAudioLocked()
		return
	}
	copy(c.audioSamples, frame)
}
