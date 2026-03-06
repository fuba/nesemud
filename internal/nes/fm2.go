package nes

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
)

func ParseFM2(data []byte) (*Replay, error) {
	s := bufio.NewScanner(bytes.NewReader(data))
	replay := &Replay{}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		p1, err := parseFM2Buttons(parts[2])
		if err != nil {
			return nil, err
		}
		p2, err := parseFM2Buttons(parts[3])
		if err != nil {
			return nil, err
		}
		replay.Frames = append(replay.Frames, FrameInput{P1: p1, P2: p2})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(replay.Frames) == 0 {
		return nil, errors.New("no frame data found in fm2")
	}
	return replay, nil
}

func parseFM2Buttons(field string) (Buttons, error) {
	field = strings.TrimSpace(field)
	if len(field) < 8 {
		return Buttons{}, errors.New("invalid fm2 button field")
	}
	field = field[:8]
	return Buttons{
		Right:  field[0] != '.',
		Left:   field[1] != '.',
		Down:   field[2] != '.',
		Up:     field[3] != '.',
		Start:  field[4] != '.',
		Select: field[5] != '.',
		B:      field[6] != '.',
		A:      field[7] != '.',
	}, nil
}
