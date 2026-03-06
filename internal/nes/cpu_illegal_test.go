package nes

import "testing"

func TestCPUIllegalLAXAndSAX(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0x0F, // LDA #$0F
		0xAA,       // TAX
		0x87, 0x20, // SAX $20 => 0x0F
		0xA9, 0x00, // LDA #0
		0xA2, 0x00, // LDX #0
		0xA7, 0x20, // LAX $20 => A=X=0x0F
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 6; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	if c.cpu.A != 0x0F || c.cpu.X != 0x0F {
		t.Fatalf("A/X mismatch: A=0x%02X X=0x%02X", c.cpu.A, c.cpu.X)
	}
}

func TestCPUIllegalSLOAndRRA(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0x01, // LDA #1
		0x85, 0x30, // STA $30
		0x07, 0x30, // SLO $30 => mem=2, A|=2 => 3
		0x67, 0x30, // RRA $30
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 4; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	if c.cpu.A == 0 {
		t.Fatalf("expected A changed by illegal ops")
	}
}

func TestCPUIllegalNOPVariantsDontError(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0x80, 0x12, // NOP #$12
		0x04, 0x10, // NOP $10
		0x0C, 0x00, 0x80, // NOP $8000
		0x1A, // NOP
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 4; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("nop variant step %d failed: %v", i, err)
		}
	}
}
