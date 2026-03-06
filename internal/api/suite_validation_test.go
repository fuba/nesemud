package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nes_recorder/internal/nes"
)

func TestSuiteValidationEndpoint(t *testing.T) {
	d := t.TempDir()
	romPath := filepath.Join(d, "ppu_case.nes")
	if err := os.WriteFile(romPath, buildValidationROM(), 0o644); err != nil {
		t.Fatalf("write rom: %v", err)
	}

	core := nes.NewConsole()
	s := NewServer(core, nil)
	body := map[string]any{"suite": "ppu", "rom_dir": d, "frames": 5}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/validate/suite", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("rom_count")) {
		t.Fatalf("expected rom_count in response")
	}
}
