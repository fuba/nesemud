package nes

import "testing"

func TestCloneIsIndependentForRAMAndCartridgeRAM(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{0xEA})
	c.cart.PRGRAM = []byte{1, 2, 3}
	if err := c.Poke(0x10, []byte{0x11}); err != nil {
		t.Fatalf("poke live: %v", err)
	}

	clone := c.Clone()
	if err := clone.Poke(0x10, []byte{0x22}); err != nil {
		t.Fatalf("poke clone: %v", err)
	}
	clone.cart.PRGRAM[0] = 0x99

	got, err := c.Peek(0x10, 1)
	if err != nil {
		t.Fatalf("peek live: %v", err)
	}
	if got[0] != 0x11 {
		t.Fatalf("live RAM mutated by clone: got 0x%02X", got[0])
	}
	if c.cart.PRGRAM[0] != 1 {
		t.Fatalf("live cartridge RAM mutated by clone: got 0x%02X", c.cart.PRGRAM[0])
	}
}

func TestSimulateInputSequencesDoesNotMutateLiveConsole(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{0xEA, 0x4C, 0x00, 0x80})
	c.cpu.Reset(c)
	if err := c.Poke(0x10, []byte{0x44}); err != nil {
		t.Fatalf("poke live: %v", err)
	}

	results, err := c.SimulateInputSequences(
		[][]byte{{0x80}, {0x40}},
		1,
		0x10,
		1,
	)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d", len(results))
	}

	got, err := c.Peek(0x10, 1)
	if err != nil {
		t.Fatalf("peek live: %v", err)
	}
	if got[0] != 0x44 {
		t.Fatalf("live RAM mutated by simulation: got 0x%02X", got[0])
	}
}
