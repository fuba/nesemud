package nes

import "testing"

func TestStepInstructionDefersPPURegisterWriteUntilFinalCPUCycle(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{
		0xA9, 0x01, // LDA #$01
		0x8D, 0x06, 0x20, // STA $2006
	})
	c.cpu.Reset(c)

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("lda step failed: %v", err)
	}

	c.ppu.lineState[0] = scanlineRenderState{
		valid:     true,
		ctrl:      0x00,
		mask:      0x0A,
		vramAddr:  0,
		fineX:     0,
		mirroring: MirroringHorizontal,
		mapper:    0,
	}
	c.ppu.lineSplitN[0] = 1
	c.ppu.lineSplits[0][0] = scanlineStateSegment{startX: 0, state: c.ppu.lineState[0]}
	c.ppu.scanline = 0
	c.ppu.cycle = 9
	c.ppu.addrLatch = true

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("sta step failed: %v", err)
	}

	before := c.ppu.renderStateForPixel(c, 0, 39)
	after := c.ppu.renderStateForPixel(c, 0, 40)
	if before.vramAddr != 0 {
		t.Fatalf("state before deferred split = 0x%04X, want 0x0000", before.vramAddr)
	}
	if after.vramAddr != 1 {
		t.Fatalf("state after deferred split = 0x%04X, want 0x0001", after.vramAddr)
	}
}

func TestStepFrameAdvancesExactlyOnePPUFramePerCall(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{
		0xEA,             // NOP
		0x4C, 0x00, 0x80, // JMP $8000
	})
	c.cpu.Reset(c)

	var prev uint64
	for i := 0; i < 12; i++ {
		c.StepFrame()
		if c.paused {
			t.Fatalf("console paused unexpectedly at iteration %d: %s", i, c.lastCPUError)
		}
		if c.ppu.frameID != prev+1 {
			t.Fatalf("ppu frame advanced by %d, want 1 (iteration %d)", c.ppu.frameID-prev, i)
		}
		prev = c.ppu.frameID
	}
}
