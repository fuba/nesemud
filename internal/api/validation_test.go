package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nesemud/internal/nes"
	"nesemud/internal/streaming"
)

func TestReplayValidationEndpoint(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)
	body := map[string]any{
		"rom_content_base64": base64.StdEncoding.EncodeToString(buildValidationROM()),
		"frames":             10,
		"repeats":            2,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/validate/replay", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("deterministic")) {
		t.Fatalf("expected deterministic in response")
	}
}

func TestStreamStatsEndpoint(t *testing.T) {
	core := nes.NewConsole()
	hls := streaming.NewHLSStreamer()
	s := NewServer(core, hls, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/stream/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("running")) {
		t.Fatalf("expected running field")
	}
}

func buildValidationROM() []byte {
	header := []byte{'N', 'E', 'S', 0x1A, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	prg := make([]byte, 16*1024)
	prg[0] = 0xEA
	prg[1] = 0x4C
	prg[2] = 0x00
	prg[3] = 0x80
	prg[0x3FFC] = 0x00
	prg[0x3FFD] = 0x80
	chr := make([]byte, 8*1024)
	rom := append(header, prg...)
	rom = append(rom, chr...)
	return rom
}
