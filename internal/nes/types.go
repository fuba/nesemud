package nes

import (
	"errors"
	"sync"
	"time"
)

const (
	FrameWidth   = 256
	FrameHeight  = 240
	AudioRate    = 48000
	TargetFPS    = 60
	FrameSizeRGB = FrameWidth * FrameHeight * 3
)

var (
	ErrInvalidRange = errors.New("invalid memory range")
)

type Buttons struct {
	A      bool `json:"a"`
	B      bool `json:"b"`
	Select bool `json:"select"`
	Start  bool `json:"start"`
	Up     bool `json:"up"`
	Down   bool `json:"down"`
	Left   bool `json:"left"`
	Right  bool `json:"right"`
}

type FrameInput struct {
	P1 Buttons
	P2 Buttons
}

type Replay struct {
	Frames []FrameInput
}

type Cartridge struct {
	PRG             []byte
	CHR             []byte
	PRGRAM          []byte
	Mapper          uint8
	PRGBanks        int
	CHRBanks        int
	CHRIsRAM        bool
	HasBattery      bool
	HasTrainer      bool
	mapper2BankSel  byte
	mapper3CHRSel   byte
	mapper33PRG     [2]byte
	mapper33CHR     [6]byte
	mapper66PRGSel  byte
	mapper66CHRSel  byte
	mapper75PRG     [3]byte
	mapper75CHR     [2]byte
	mapper87CHRSel  byte
	mmc5PRGMode     byte
	mmc5CHRMode     byte
	mmc5PRGBank     [5]byte
	mmc5CHRBank     [12]byte
	mmc5UpperCHR    byte
	mmc5ExRAMMode   byte
	mmc5ExRAM       [1024]byte
	mmc5FillTile    byte
	mmc5FillAttr    byte
	mmc5NTMap       [4]byte
	mmc5PRGRAM      [64 * 1024]byte
	mmc5RAMProtect1 byte
	mmc5RAMProtect2 byte
	mmc5MulA        byte
	mmc5MulB        byte
	mmc5IRQLatch    byte
	mmc5IRQEnable   bool
	mmc5IRQPending  bool
	mmc5InFrame     bool
	mmc5Scanline    byte
	vrcPRG0         byte
	vrcPRG1         byte
	vrcSwapMode     bool
	vrcCHRLow       [8]byte
	vrcCHRHigh      [8]byte
	vrcIRQCounter   byte
	vrcIRQLatch     byte
	vrcIRQEnable    bool
	vrcIRQEnableAck bool
	vrcIRQPrescaler int
	vrcIRQPending   bool
	mirroring       MirroringMode
	mmc1Shift       byte
	mmc1Control     byte
	mmc1CHRBank0    byte
	mmc1CHRBank1    byte
	mmc1PRGBank     byte
	mmc3BankSelect  byte
	mmc3Regs        [8]byte
	mmc3IRQLatch    byte
	mmc3IRQCounter  byte
	mmc3IRQReload   bool
	mmc3IRQEnable   bool
	mmc3IRQPending  bool
}

type scanlineRenderState struct {
	valid          bool
	prefetched     bool
	ctrl           byte
	mask           byte
	vramAddr       uint16
	fineX          byte
	mirroring      MirroringMode
	mapper         uint8
	mapper3CHRSel  byte
	mapper33CHR    [6]byte
	mapper66CHRSel byte
	mapper75CHR    [2]byte
	mapper87CHRSel byte
	mmc5CHRMode    byte
	mmc5CHRBank    [12]byte
	mmc5UpperCHR   byte
	mmc5ExRAMMode  byte
	mmc5ExRAM      [1024]byte
	mmc5FillTile   byte
	mmc5FillAttr   byte
	mmc5NTMap      [4]byte
	vrcCHR         [8]byte
	vrcMirroring   MirroringMode
	mmc1Control    byte
	mmc1CHRBank0   byte
	mmc1CHRBank1   byte
	mmc3BankSelect byte
	mmc3Regs       [8]byte
}

type scanlineStateSegment struct {
	startX int
	state  scanlineRenderState
}

type ppuRegisterWrite struct {
	addr  uint16
	value byte
}

type ppuWriteTraceFunc func(scanline int, cycle int, addr uint16, value byte, ctrl byte, mask byte)

type MirroringMode uint8

const (
	MirroringOneScreenLow MirroringMode = iota
	MirroringOneScreenHigh
	MirroringVertical
	MirroringHorizontal
	MirroringFourScreen
)

type Console struct {
	mu               sync.RWMutex
	ram              [2048]byte
	cart             *Cartridge
	cpu              *cpu6502
	ppu              *ppu
	apu              *apu
	frameCount       uint64
	paused           bool
	lastFrame        []byte
	audioSamples     []int16
	controllerP1     Buttons
	controllerP2     Buttons
	controllerStrobe bool
	controllerShift  [2]byte
	deferPPUWrites   bool
	pendingPPUWrites []ppuRegisterWrite
	ppuWriteTrace    ppuWriteTraceFunc
	replay           *Replay
	replayCursor     int
	lastFrameTime    time.Time
}

type CPUState struct {
	PC     uint16 `json:"pc"`
	A      byte   `json:"a"`
	X      byte   `json:"x"`
	Y      byte   `json:"y"`
	SP     byte   `json:"sp"`
	P      byte   `json:"p"`
	Cycles uint64 `json:"cycles"`
}

func NewConsole() *Console {
	c := &Console{
		lastFrame:     make([]byte, FrameSizeRGB),
		audioSamples:  make([]int16, AudioRate/TargetFPS*2),
		lastFrameTime: time.Now(),
		cpu:           newCPU(),
		ppu:           newPPU(),
		apu:           newAPU(),
	}
	c.renderFallbackFrameLocked()
	return c
}
