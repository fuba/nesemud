package nes

type SimulationResult struct {
	Index int               `json:"index"`
	Bytes []byte            `json:"bytes"`
	Trace []SimulationTrace `json:"trace,omitempty"`
}

type SimulationTrace struct {
	InputIndex int    `json:"input_index"`
	Bytes      []byte `json:"bytes"`
}

func ButtonsFromBitmask(v byte) Buttons {
	return Buttons{
		A:      v&0x01 != 0,
		B:      v&0x02 != 0,
		Select: v&0x04 != 0,
		Start:  v&0x08 != 0,
		Up:     v&0x10 != 0,
		Down:   v&0x20 != 0,
		Left:   v&0x40 != 0,
		Right:  v&0x80 != 0,
	}
}

func (c *Console) SimulateInputSequences(
	sequences [][]byte,
	framesPerInput int,
	addr uint16,
	length int,
	traceEveryInput bool,
) ([]SimulationResult, error) {
	if framesPerInput <= 0 || framesPerInput > 30 {
		return nil, ErrInvalidRange
	}
	if len(sequences) == 0 || len(sequences) > 256 {
		return nil, ErrInvalidRange
	}
	if length <= 0 || length > 4096 {
		return nil, ErrInvalidRange
	}

	c.mu.RLock()
	base := c.cloneLocked()
	c.mu.RUnlock()
	base.replay = nil
	results := make([]SimulationResult, 0, len(sequences))
	for i, sequence := range sequences {
		if len(sequence) == 0 || len(sequence) > 16 {
			return nil, ErrInvalidRange
		}
		sim := base.Clone()
		sim.replay = nil
		sim.simulationFast = true
		trace := make([]SimulationTrace, 0, len(sequence))
		for _, bitmask := range sequence {
			buttons := ButtonsFromBitmask(bitmask)
			for frame := 0; frame < framesPerInput; frame++ {
				sim.SetController(1, buttons)
				sim.StepFrame()
			}
			if traceEveryInput {
				b, err := sim.Peek(addr, length)
				if err != nil {
					return nil, err
				}
				trace = append(trace, SimulationTrace{
					InputIndex: len(trace),
					Bytes:      b,
				})
			}
		}
		b, err := sim.Peek(addr, length)
		if err != nil {
			return nil, err
		}
		results = append(results, SimulationResult{Index: i, Bytes: b, Trace: trace})
	}
	return results, nil
}
