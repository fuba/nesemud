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
	PRG            []byte
	CHR            []byte
	Mapper         uint8
	PRGBanks       int
	CHRBanks       int
	CHRIsRAM       bool
	mapper2BankSel byte
	mapper3CHRSel  byte
	mirroring      MirroringMode
	mmc1Shift      byte
	mmc1Control    byte
	mmc1CHRBank0   byte
	mmc1CHRBank1   byte
	mmc1PRGBank    byte
	mmc3BankSelect byte
	mmc3Regs       [8]byte
	mmc3IRQLatch   byte
	mmc3IRQCounter byte
	mmc3IRQReload  bool
	mmc3IRQEnable  bool
	mmc3IRQPending bool
}

type MirroringMode uint8

const (
	MirroringOneScreenLow MirroringMode = iota
	MirroringOneScreenHigh
	MirroringVertical
	MirroringHorizontal
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
