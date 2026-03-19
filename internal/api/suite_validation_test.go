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

func TestSuiteValidationEndpoint(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)
	body := map[string]any{
		"suite":  "ppu",
		"frames": 5,
		"roms": []map[string]any{
			{
				"name":           "ppu_case.nes",
				"content_base64": base64.StdEncoding.EncodeToString(buildValidationROM()),
			},
		},
	}
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

func TestSuiteValidationEndpointNESTestRequiresLogAndMatches(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)
	body := map[string]any{
		"suite":  "nestest",
		"frames": 2,
		"roms": []map[string]any{
			{
				"name":                 "nestest.nes",
				"content_base64":       base64.StdEncoding.EncodeToString(buildValidationROM()),
				"expected_log_content": "8000  EA        NOP                             A:00 X:00 Y:00 P:24 SP:FA\n8001  4C 00 80  JMP $8000                       A:00 X:00 Y:00 P:24 SP:FA\n",
			},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/validate/suite", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"passed":1`)) {
		t.Fatalf("expected suite pass, body=%s", rec.Body.String())
	}
}

func TestSuiteValidationEndpointNESTestWithoutLogFails(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)
	body := map[string]any{
		"suite":  "nestest",
		"frames": 2,
		"roms": []map[string]any{
			{
				"name":           "nestest.nes",
				"content_base64": base64.StdEncoding.EncodeToString(buildValidationROM()),
			},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/validate/suite", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"failed":1`)) {
		t.Fatalf("expected suite failure without log, body=%s", rec.Body.String())
	}
}
