package nes

import "testing"

func TestParseFM2(t *testing.T) {
	data := []byte("version 3\nemuVersion 22020\n|0|....S..A|........|\n|0|........|........|\n")
	replay, err := ParseFM2(data)
	if err != nil {
		t.Fatalf("ParseFM2 returned error: %v", err)
	}
	if len(replay.Frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(replay.Frames))
	}
	if !replay.Frames[0].P1.A || !replay.Frames[0].P1.Start {
		t.Fatalf("expected first frame to have A and Start pressed")
	}
	if replay.Frames[1].P1.A || replay.Frames[1].P1.Start {
		t.Fatalf("expected second frame to have no pressed buttons")
	}
}
