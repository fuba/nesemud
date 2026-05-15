package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nesemud/internal/nes"
)

func TestMemoryPeekPokeAndFM2Reset(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)

	pokeBody := []byte(`{"value":171}`)
	pokeReq := httptest.NewRequest(http.MethodPut, "/v1/memory/16", bytes.NewReader(pokeBody))
	pokeRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(pokeRec, pokeReq)
	if pokeRec.Code != http.StatusNoContent {
		t.Fatalf("poke status = %d", pokeRec.Code)
	}

	peekReq := httptest.NewRequest(http.MethodGet, "/v1/memory/16?len=1", nil)
	peekRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(peekRec, peekReq)
	if peekRec.Code != http.StatusOK {
		t.Fatalf("peek status = %d", peekRec.Code)
	}

	var mr MemoryResponse
	if err := json.Unmarshal(peekRec.Body.Bytes(), &mr); err != nil {
		t.Fatalf("unmarshal peek response: %v", err)
	}
	if len(mr.Bytes) != 1 || mr.Bytes[0] != 171 {
		t.Fatalf("unexpected bytes: %+v", mr.Bytes)
	}

	_ = core.Poke(0x20, []byte{0x55})
	fm2 := "version 3\n|0|A.......|........|\n"
	fm2ReqBody := FM2LoadRequest{Content: fm2}
	b, _ := json.Marshal(fm2ReqBody)
	fm2Req := httptest.NewRequest(http.MethodPost, "/v1/replay/fm2", bytes.NewReader(b))
	fm2Rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(fm2Rec, fm2Req)
	if fm2Rec.Code != http.StatusAccepted {
		t.Fatalf("fm2 status = %d", fm2Rec.Code)
	}

	peekAfterReq := httptest.NewRequest(http.MethodGet, "/v1/memory/32?len=1", nil)
	peekAfterRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(peekAfterRec, peekAfterReq)
	if peekAfterRec.Code != http.StatusOK {
		t.Fatalf("peek-after status = %d", peekAfterRec.Code)
	}
	if err := json.Unmarshal(peekAfterRec.Body.Bytes(), &mr); err != nil {
		t.Fatalf("unmarshal after response: %v", err)
	}
	if mr.Bytes[0] != 0 {
		t.Fatalf("expected reset memory to zero, got %d", mr.Bytes[0])
	}
}

func TestInputEndpointUpdatesControllerState(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/v1/input/player/1", bytes.NewBufferString(`{"start":true,"right":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("input status = %d", rec.Code)
	}

	state := core.State()
	controllers, ok := state["controllers"].(map[string]any)
	if !ok {
		t.Fatalf("controllers missing from state: %#v", state)
	}
	p1, ok := controllers["p1"].(map[string]any)
	if !ok {
		t.Fatalf("p1 missing from controllers: %#v", controllers)
	}
	if got, _ := p1["start"].(bool); !got {
		t.Fatalf("expected start=true, got %#v", p1["start"])
	}
	if got, _ := p1["right"].(bool); !got {
		t.Fatalf("expected right=true, got %#v", p1["right"])
	}
}

func TestSimulationEndpointReturnsMemoryWithoutMutatingLiveCore(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)
	if err := core.Poke(0x20, []byte{0x66}); err != nil {
		t.Fatalf("poke live: %v", err)
	}

	body := []byte(`{
		"sequences": [[128], [64]],
		"frames_per_input": 1,
		"memory_address": 32,
		"memory_length": 1
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/simulate/sequences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("simulate status = %d body=%s", rec.Code, rec.Body.String())
	}

	var sr SimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sr); err != nil {
		t.Fatalf("unmarshal simulate response: %v", err)
	}
	if len(sr.Results) != 2 || len(sr.Results[0].Bytes) != 1 {
		t.Fatalf("unexpected simulate response: %+v", sr)
	}

	got, err := core.Peek(0x20, 1)
	if err != nil {
		t.Fatalf("peek live: %v", err)
	}
	if got[0] != 0x66 {
		t.Fatalf("live core mutated by simulation: got 0x%02X", got[0])
	}
}

func TestStateIncludesAudioStats(t *testing.T) {
	core := nes.NewConsole()
	state := core.State()
	audioState, ok := state["audio"].(map[string]any)
	if !ok {
		t.Fatalf("audio missing from state: %#v", state)
	}
	if _, ok := audioState["peak_abs"]; !ok {
		t.Fatalf("expected peak_abs in audio state: %#v", audioState)
	}
}

func TestWebRTCOfferUnavailableWithoutStreamer(t *testing.T) {
	s := NewServer(nes.NewConsole(), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/webrtc/offer", bytes.NewBufferString(`{"type":"offer","sdp":"v=0"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
