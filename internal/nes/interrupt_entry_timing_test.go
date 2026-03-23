package nes

import "testing"

func TestStepInstructionAdvancesSubsystemsDuringNMIEntryCycles(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{0xEA, 0xEA, 0xEA}) // NOPs
	c.cart.PRG[0x3FFA] = 0x00                // NMI vector -> $8000
	c.cart.PRG[0x3FFB] = 0x80
	c.Reset()

	c.ppu.ctrl = 0x80 // enable NMI on vblank
	c.ppu.scanline = 241
	c.ppu.cycle = 0

	startDots := c.ppu.scanline*341 + c.ppu.cycle
	if err := c.StepInstruction(); err != nil {
		t.Fatalf("StepInstruction returned error: %v", err)
	}
	afterFirst := c.ppu.scanline*341 + c.ppu.cycle
	if got, want := afterFirst-startDots, 2*3; got != want {
		t.Fatalf("first instruction ppu dots advanced=%d, want %d", got, want)
	}
	if got, want := int(c.cpu.Cycles), 2; got != want {
		t.Fatalf("cpu cycles after first instruction=%d, want %d", got, want)
	}

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("second StepInstruction returned error: %v", err)
	}
	afterSecond := c.ppu.scanline*341 + c.ppu.cycle
	if got, want := afterSecond-afterFirst, 2*3; got != want {
		t.Fatalf("second instruction ppu dots advanced=%d, want %d", got, want)
	}
	if got, want := int(c.cpu.Cycles), 4; got != want {
		t.Fatalf("cpu cycles after second instruction=%d, want %d", got, want)
	}

	if err := c.StepInstruction(); err != nil {
		t.Fatalf("third StepInstruction returned error: %v", err)
	}
	afterThird := c.ppu.scanline*341 + c.ppu.cycle
	if got, want := afterThird-afterSecond, (7+2)*3; got != want {
		t.Fatalf("third instruction ppu dots advanced=%d, want %d (NMI entry + instruction)", got, want)
	}
	if got, want := int(c.cpu.Cycles), 13; got != want {
		t.Fatalf("cpu cycles after third instruction=%d, want %d", got, want)
	}
}

func TestStepInstructionAdvancesSubsystemsDuringIRQEntryCycles(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{0xEA}) // NOP
	c.cart.PRG[0x3FFE] = 0x00                // IRQ vector -> $8000
	c.cart.PRG[0x3FFF] = 0x80
	c.Reset()

	c.cpu.setFlag(flagI, false) // allow IRQ
	c.apu.frameIRQ = true
	c.ppu.scanline = 0
	c.ppu.cycle = 0

	startDots := c.ppu.scanline*341 + c.ppu.cycle
	if err := c.StepInstruction(); err != nil {
		t.Fatalf("StepInstruction returned error: %v", err)
	}
	endDots := c.ppu.scanline*341 + c.ppu.cycle
	if got, want := endDots-startDots, (2+7)*3; got != want {
		t.Fatalf("ppu dots advanced=%d, want %d (instruction + IRQ entry)", got, want)
	}
	if got, want := int(c.cpu.Cycles), 9; got != want {
		t.Fatalf("cpu cycles=%d, want %d", got, want)
	}
}
