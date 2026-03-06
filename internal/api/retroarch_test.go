package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"nesemud/internal/nes"
)

func TestRetroArchCommandCompatibilityForms(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil)

	reqJSON := httptest.NewRequest(http.MethodPost, "/v1/retroarch/command", bytes.NewBufferString(`{"command":"PAUSE"}`))
	recJSON := httptest.NewRecorder()
	s.Handler().ServeHTTP(recJSON, reqJSON)
	if recJSON.Code != http.StatusAccepted {
		t.Fatalf("json form status=%d", recJSON.Code)
	}

	reqQuery := httptest.NewRequest(http.MethodPost, "/v1/retroarch/command?cmd=RESUME", nil)
	recQuery := httptest.NewRecorder()
	s.Handler().ServeHTTP(recQuery, reqQuery)
	if recQuery.Code != http.StatusAccepted {
		t.Fatalf("query form status=%d", recQuery.Code)
	}

	reqText := httptest.NewRequest(http.MethodPost, "/v1/retroarch/command", bytes.NewBufferString("RESET"))
	reqText.Header.Set("Content-Type", "text/plain")
	recText := httptest.NewRecorder()
	s.Handler().ServeHTTP(recText, reqText)
	if recText.Code != http.StatusAccepted {
		t.Fatalf("text form status=%d", recText.Code)
	}
}

func TestRetroArchCommandList(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/retroarch/command/list", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("RESET")) {
		t.Fatalf("expected RESET in list")
	}
}
