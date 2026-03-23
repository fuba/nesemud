package nes

import (
	"strings"
	"testing"
)

func TestStateIncludesLastCPUErrorWhenPaused(t *testing.T) {
	c := NewConsole()
	c.cart = buildTestCartridge([]byte{0x8B}) // Unsupported opcode.
	c.cpu.Reset(c)

	err := c.StepInstruction()
	if err == nil {
		t.Fatalf("expected step error for unsupported opcode")
	}

	st := c.State()
	paused, _ := st["paused"].(bool)
	if !paused {
		t.Fatalf("expected console to be paused after CPU error")
	}
	lastErr, _ := st["last_cpu_error"].(string)
	if !strings.Contains(lastErr, "unsupported opcode") {
		t.Fatalf("unexpected last_cpu_error=%q", lastErr)
	}
}
