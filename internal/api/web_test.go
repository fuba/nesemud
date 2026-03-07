package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nesemud/internal/nes"
)

func TestWebPageServed(t *testing.T) {
	s := NewServer(nes.NewConsole(), nil)

	req := httptest.NewRequest(http.MethodGet, "/web", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "nesemud /web") {
		t.Fatalf("unexpected body")
	}
}

func TestWebPageMethodNotAllowed(t *testing.T) {
	s := NewServer(nes.NewConsole(), nil)

	req := httptest.NewRequest(http.MethodPost, "/web", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}
