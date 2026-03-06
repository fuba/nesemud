package nes

import "testing"

func TestCPUIndirectAddressingModes(t *testing.T) {
	c := NewConsole()
	program := make([]byte, 0x60)
	copy(program, []byte{
		0xA9, 0x40, // LDA #$40
		0x85, 0x00, // STA $00
		0xA9, 0x80, // LDA #$80
		0x85, 0x01, // STA $01
		0xA0, 0x00, // LDY #$00
		0xB1, 0x00, // LDA ($00),Y
		0x85, 0x10, // STA $10
		0xA2, 0x00, // LDX #$00
		0xA1, 0x00, // LDA ($00,X)
		0x85, 0x11, // STA $11
	})
	program[0x40] = 0x99
	cart := buildTestCartridge(program)
	c.cart = cart
	c.cpu.Reset(c)

	for i := 0; i < 10; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}
	if got := c.readCPU(0x10); got != 0x99 {
		t.Fatalf("ram[0x10]=0x%02X, want 0x99", got)
	}
	if got := c.readCPU(0x11); got != 0x99 {
		t.Fatalf("ram[0x11]=0x%02X, want 0x99", got)
	}
}

func TestCPUShiftRotateAndCompare(t *testing.T) {
	c := NewConsole()
	program := []byte{
		0xA9, 0x81, // LDA #$81
		0x0A,       // ASL A => 0x02 carry=1
		0x2A,       // ROL A => 0x05
		0x85, 0x30, // STA $30
		0xA9, 0x01, // LDA #$01
		0x4A,       // LSR A => 0x00 carry=1
		0x6A,       // ROR A => 0x80
		0x85, 0x31, // STA $31
		0xA9, 0x01, // LDA #$01
		0x85, 0x32, // STA $32
		0x06, 0x32, // ASL $32 => 0x02
		0xA9, 0x03, // LDA #$03
		0x85, 0x33, // STA $33
		0x46, 0x33, // LSR $33 => 0x01
		0xA9, 0x05, // LDA #$05
		0xC9, 0x06, // CMP #$06 => C=0,N=1
		0x90, 0x02, // BCC +2 (taken)
		0xA9, 0xFF, // LDA #$FF (skip)
		0x85, 0x34, // STA $34 (A stays 0x05)
	}
	cart := buildTestCartridge(program)
	c.cart = cart
	c.cpu.Reset(c)

	for i := 0; i < 20; i++ {
		if err := c.cpu.Step(c); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}

	if got := c.readCPU(0x30); got != 0x05 {
		t.Fatalf("ram[0x30]=0x%02X, want 0x05", got)
	}
	if got := c.readCPU(0x31); got != 0x80 {
		t.Fatalf("ram[0x31]=0x%02X, want 0x80", got)
	}
	if got := c.readCPU(0x32); got != 0x02 {
		t.Fatalf("ram[0x32]=0x%02X, want 0x02", got)
	}
	if got := c.readCPU(0x33); got != 0x01 {
		t.Fatalf("ram[0x33]=0x%02X, want 0x01", got)
	}
	if got := c.readCPU(0x34); got != 0x05 {
		t.Fatalf("ram[0x34]=0x%02X, want 0x05", got)
	}
}
