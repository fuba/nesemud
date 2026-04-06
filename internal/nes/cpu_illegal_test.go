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

func TestCPUIllegalANCAndALR(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0xFF, // LDA #$FF
		0x0B, 0x80, // ANC #$80 => A=$80, C=1
		0x4B, 0x0F, // ALR #$0F => A=$00, C=0
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 3; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	if c.cpu.A != 0x00 {
		t.Fatalf("A = 0x%02X, want 0x00", c.cpu.A)
	}
	if c.cpu.P&flagC != 0 {
		t.Fatalf("carry should be clear after ALR, P=0x%02X", c.cpu.P)
	}
}

func TestCPUIllegalARRAndSBX(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0xFF, // LDA #$FF
		0xAA,       // TAX
		0x38,       // SEC
		0x6B, 0xFF, // ARR #$FF
		0xCB, 0x10, // SBX #$10
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 5; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	if c.cpu.X == 0 {
		t.Fatalf("expected SBX to change X")
	}
}

func TestCPUIllegalStoreHighVariantsDontError(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0x55, // LDA #$55
		0xA2, 0xAA, // LDX #$AA
		0xA0, 0x11, // LDY #$11
		0x9B, 0x34, 0x12, // TAS $1234,Y
		0x9C, 0x00, 0x20, // SHY $2000,X
		0x9E, 0x00, 0x20, // SHX $2000,Y
		0x9F, 0x00, 0x20, // AHX $2000,Y
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 7; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
}

func TestCPUIllegalUSBCImmediateAlias(t *testing.T) {
	c := NewConsole()
	cart := buildTestCartridge([]byte{
		0xA9, 0x50, // LDA #$50
		0x38,       // SEC
		0xEB, 0x10, // USBC/SBC #$10 => A=$40
	})
	c.cart = cart
	c.cpu.Reset(c)
	for i := 0; i < 3; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	if c.cpu.A != 0x40 {
		t.Fatalf("A = 0x%02X, want 0x40", c.cpu.A)
	}
}
