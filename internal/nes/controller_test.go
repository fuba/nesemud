package nes

import "testing"

func TestControllerShiftRegisterRead(t *testing.T) {
	c := NewConsole()
	c.SetController(1, Buttons{A: true, Start: true, Right: true})

	c.writeCPU(0x4016, 1)
	c.writeCPU(0x4016, 0)

	want := []byte{1, 0, 0, 1, 0, 0, 0, 1}
	for i := 0; i < 8; i++ {
		got := c.readCPU(0x4016) & 1
		if got != want[i] {
			t.Fatalf("bit %d = %d, want %d", i, got, want[i])
		}
	}
}
