package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nesemud/internal/nes"
)

func TestNESTestValidationEndpoint(t *testing.T) {
	line := "8000  EA        NOP                             A:00 X:00 Y:00 P:24 SP:FA\n"
	line += "8001  4C 00 80  JMP $8000                       A:00 X:00 Y:00 P:24 SP:FA\n"

	core := nes.NewConsole()
	s := NewServer(core, nil)
	body := map[string]any{
		"rom_content_base64":   base64.StdEncoding.EncodeToString(buildValidationROM()),
		"expected_log_content": line,
		"instructions":         2,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/validate/nestest", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("\"matched\":true")) {
		t.Fatalf("expected matched true: %s", rec.Body.String())
	}
}
