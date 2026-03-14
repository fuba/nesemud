package nes

import "testing"

func TestAPUMixFormulaBoundedOutput(t *testing.T) {
	v := mixSample(15, 15, 15, 127)
	if v <= 0 {
		t.Fatalf("expected positive mixed output")
	}
	if v > 32767 {
		t.Fatalf("overflow output: %d", v)
	}
}

func TestAPUMixProducesSilenceForZeroInput(t *testing.T) {
	v := mixSample(0, 0, 0, 0)
	if v != 0 {
		t.Fatalf("expected zero output, got %d", v)
	}
}

func TestAPUOutputFilterAttenuatesDCOffset(t *testing.T) {
	a := newAPU()
	var last int16
	for i := 0; i < 2000; i++ {
		last = a.applyOutputFilter(12000)
	}
	if last >= 4000 {
		t.Fatalf("expected output filter to attenuate sustained DC, got %d", last)
	}
}
