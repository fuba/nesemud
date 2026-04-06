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

func TestLoadINESSupportsExpandedMapperSet(t *testing.T) {
	for _, mapper := range []byte{5, 23, 25, 33, 66, 75, 87, 88, 206} {
		header := []byte{'N', 'E', 'S', 0x1A, 2, 1, byte(mapper << 4), byte(mapper & 0xF0), 0, 0, 0, 0, 0, 0, 0, 0}
		prg := make([]byte, 2*16*1024)
		chr := make([]byte, 8*1024)
		rom := append(header, prg...)
		rom = append(rom, chr...)

		cart, err := LoadINES(rom)
		if err != nil {
			t.Fatalf("mapper %d rejected: %v", mapper, err)
		}
		if cart.Mapper != mapper {
			t.Fatalf("expected mapper %d, got %d", mapper, cart.Mapper)
		}
	}
}

func TestLoadINESFourScreenAndPRGRAM(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 2, 1, 0x08, 0x00, 0x02, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 2*16*1024)
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)

	cart, err := LoadINES(rom)
	if err != nil {
		t.Fatalf("LoadINES returned error: %v", err)
	}
	if cart.mirroring != MirroringFourScreen {
		t.Fatalf("expected four-screen mirroring, got %d", cart.mirroring)
	}
	if got := len(cart.PRGRAM); got != 16*1024 {
		t.Fatalf("expected 16KiB PRG-RAM, got %d", got)
	}
}

func TestLoadINESDefaultsPRGRAMTo8KiBWhenHeaderByte8IsZero(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 2, 1, 0x10, 0x00, 0x00, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 2*16*1024)
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)

	cart, err := LoadINES(rom)
	if err != nil {
		t.Fatalf("LoadINES returned error: %v", err)
	}
	if got := len(cart.PRGRAM); got != 8*1024 {
		t.Fatalf("expected default 8KiB PRG-RAM, got %d bytes", got)
	}
}

func TestLoadINESMapper25BatteryDefaultsPRGRAMTo8KiB(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 8, 16, 0x92, 0x10, 0x00, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 8*16*1024)
	chr := make([]byte, 16*8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)

	cart, err := LoadINES(rom)
	if err != nil {
		t.Fatalf("LoadINES returned error: %v", err)
	}
	if cart.Mapper != 25 {
		t.Fatalf("expected mapper 25, got %d", cart.Mapper)
	}
	if got := len(cart.PRGRAM); got != 8*1024 {
		t.Fatalf("expected mapper25 battery default 8KiB PRG-RAM, got %d bytes", got)
	}
}

func TestLoadINESMapper4BatteryDefaultsPRGRAMTo8KiB(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 16, 32, 0x42, 0x00, 0x00, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 16*16*1024)
	chr := make([]byte, 32*8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)

	cart, err := LoadINES(rom)
	if err != nil {
		t.Fatalf("LoadINES returned error: %v", err)
	}
	if cart.Mapper != 4 {
		t.Fatalf("expected mapper 4, got %d", cart.Mapper)
	}
	if got := len(cart.PRGRAM); got != 8*1024 {
		t.Fatalf("expected mapper4 battery default 8KiB PRG-RAM, got %d bytes", got)
	}
}

func TestLoadINESLegacyDirtyHeaderMasksMapperHighNibble(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0x10, 0x40, 0, 0, 0, 0, 'D', 'u', 'd', 'e'}
	prg := make([]byte, 16*1024)
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)

	cart, err := LoadINES(rom)
	if err != nil {
		t.Fatalf("LoadINES returned error: %v", err)
	}
	if cart.Mapper != 1 {
		t.Fatalf("expected legacy dirty header to keep mapper 1, got %d", cart.Mapper)
	}
}

func TestTrainerLoadsIntoCPU7000Window(t *testing.T) {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0x04, 0x00, 0, 0, 0, 0, 0, 0, 0, 0}
	trainer := make([]byte, 512)
	trainer[0] = 0x5A
	trainer[len(trainer)-1] = 0xA5
	prg := make([]byte, 16*1024)
	chr := make([]byte, 8*1024)
	rom := append(header, trainer...)
	rom = append(rom, prg...)
	rom = append(rom, chr...)

	c := NewConsole()
	if err := c.LoadROMContent(rom); err != nil {
		t.Fatalf("LoadROMContent returned error: %v", err)
	}
	if got := c.readCPU(0x7000); got != 0x5A {
		t.Fatalf("trainer first byte = 0x%02X, want 0x5A", got)
	}
	if got := c.readCPU(0x71FF); got != 0xA5 {
		t.Fatalf("trainer last byte = 0x%02X, want 0xA5", got)
	}
}
