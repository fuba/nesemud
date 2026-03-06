package nes

import "testing"

func TestLoadINESMapper0(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 16*1024)
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)

	cart, err := LoadINES(rom)
	if err != nil {
		t.Fatalf("LoadINES returned error: %v", err)
	}
	if cart.Mapper != 0 {
		t.Fatalf("expected mapper 0, got %d", cart.Mapper)
	}
	if len(cart.PRG) != 16*1024 {
		t.Fatalf("expected 16KiB PRG, got %d", len(cart.PRG))
	}
}
