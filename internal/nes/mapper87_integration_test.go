package nes

import (
	"os"
	"testing"
)

func TestKageNoDensetsuMapper87BankSwitchTrace(t *testing.T) {
	const romPath = "../../dont_upload_roms/Kage no Densetsu (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("mapper87 trace rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load rom: %v", err)
	}
	if c.cart == nil || c.cart.Mapper != 87 {
		t.Fatalf("expected mapper 87 rom")
	}

	lastSel := c.cart.mapper87CHRSel
	changes := 0
	visibleChanges := 0
	for i := 0; i < 200000; i++ {
		if err := c.StepInstruction(); err != nil {
			t.Fatalf("step instruction: %v", err)
		}
		if c.cart.mapper87CHRSel == lastSel {
			continue
		}
		changes++
		if c.ppu.scanline < 240 {
			visibleChanges++
		}
		if changes <= 8 {
			t.Logf("change %d: sel=%d a=0x%02X x=0x%02X y=0x%02X scanline=%d cycle=%d pc=0x%04X", changes, c.cart.mapper87CHRSel, c.cpu.A, c.cpu.X, c.cpu.Y, c.ppu.scanline, c.ppu.cycle, c.cpu.PC)
		}
		lastSel = c.cart.mapper87CHRSel
		if changes >= 8 {
			break
		}
	}

	if changes == 0 {
		t.Fatalf("expected mapper87 bank changes during trace window")
	}
	t.Logf("total changes observed=%d visible_changes=%d", changes, visibleChanges)
}

func TestKageNoDensetsuPPURegisterTimingTrace(t *testing.T) {
	const romPath = "../../dont_upload_roms/Kage no Densetsu (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("ppu trace rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load rom: %v", err)
	}

	lastCtrl := c.ppu.ctrl
	lastMask := c.ppu.mask
	lastTemp := c.ppu.tempAddr
	lastFineX := c.ppu.fineX
	changes := 0

	for i := 0; i < 300000; i++ {
		if err := c.StepInstruction(); err != nil {
			t.Fatalf("step instruction: %v", err)
		}
		if c.ppu.ctrl == lastCtrl && c.ppu.mask == lastMask && c.ppu.tempAddr == lastTemp && c.ppu.fineX == lastFineX {
			continue
		}
		if c.ppu.scanline < 240 {
			changes++
			if changes <= 12 {
				t.Logf("ppu change %d: scanline=%d cycle=%d pc=0x%04X ctrl=%02X->%02X mask=%02X->%02X temp=%04X->%04X fineX=%d->%d",
					changes,
					c.ppu.scanline,
					c.ppu.cycle,
					c.cpu.PC,
					lastCtrl, c.ppu.ctrl,
					lastMask, c.ppu.mask,
					lastTemp, c.ppu.tempAddr,
					lastFineX, c.ppu.fineX,
				)
			}
		}
		lastCtrl = c.ppu.ctrl
		lastMask = c.ppu.mask
		lastTemp = c.ppu.tempAddr
		lastFineX = c.ppu.fineX
		if changes >= 12 {
			break
		}
	}
	if changes == 0 {
		t.Fatalf("expected visible-region ppu register changes during trace window")
	}
}

func TestKageNoDensetsuPPUWriteTrace(t *testing.T) {
	const romPath = "../../dont_upload_roms/Kage no Densetsu (Japan).nes"

	data, err := os.ReadFile(romPath)
	if err != nil {
		t.Skipf("ppu write trace rom not available: %v", err)
	}

	c := NewConsole()
	if err := c.LoadROMContent(data); err != nil {
		t.Fatalf("load rom: %v", err)
	}

	type writeEvent struct {
		scanline int
		cycle    int
		addr     uint16
		value    byte
		ctrl     byte
		mask     byte
		pc       uint16
	}
	events := make([]writeEvent, 0, 64)
	c.ppuWriteTrace = func(scanline int, cycle int, addr uint16, value byte, ctrl byte, mask byte) {
		if scanline < 0 || scanline >= 240 {
			return
		}
		if len(events) >= cap(events) {
			return
		}
		events = append(events, writeEvent{
			scanline: scanline,
			cycle:    cycle,
			addr:     addr,
			value:    value,
			ctrl:     ctrl,
			mask:     mask,
			pc:       c.cpu.PC,
		})
	}

	for i := 0; i < 500000 && len(events) < cap(events); i++ {
		if err := c.StepInstruction(); err != nil {
			t.Fatalf("step instruction: %v", err)
		}
	}
	if len(events) == 0 {
		t.Fatalf("expected visible-region ppu writes during trace window")
	}
	for i, ev := range events {
		t.Logf("ppu write %d: scanline=%d cycle=%d pc=0x%04X addr=0x%04X value=0x%02X ctrl=0x%02X mask=0x%02X",
			i+1, ev.scanline, ev.cycle, ev.pc, ev.addr, ev.value, ev.ctrl, ev.mask)
	}
}
