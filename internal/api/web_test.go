package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nesemud/internal/nes"
)

func TestWebPageServed(t *testing.T) {
	s := NewServer(nes.NewConsole(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/web", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("cache-control = %q", cc)
	}
	if build := rec.Header().Get("X-NESD-Web-Build"); build != "webrtc-stats-v2" {
		t.Fatalf("build header = %q", build)
	}
	if body := rec.Body.String(); !strings.Contains(body, "nesemud /web") {
		t.Fatalf("unexpected body")
	}
	if body := rec.Body.String(); !strings.Contains(body, "/v1/webrtc/stats") {
		t.Fatalf("missing webrtc stats endpoint in page")
	}
	if body := rec.Body.String(); !strings.Contains(body, "WebRTC stats: idle") {
		t.Fatalf("missing webrtc stats status line")
	}
	if body := rec.Body.String(); !strings.Contains(body, "Receiver: idle") {
		t.Fatalf("missing receiver stats status line")
	}
	if body := rec.Body.String(); !strings.Contains(body, "HLS URL: /hls/index.m3u8") {
		t.Fatalf("missing hls url status line")
	}
	if body := rec.Body.String(); !strings.Contains(body, "<audio id=\"audio\" autoplay></audio>") {
		t.Fatalf("missing dedicated audio element")
	}
	if body := rec.Body.String(); !strings.Contains(body, "Use WebRTC") || !strings.Contains(body, "Use HLS") {
		t.Fatalf("missing stream switch controls")
	}
	if body := rec.Body.String(); !strings.Contains(body, "webrtc-stats-v2") {
		t.Fatalf("missing ui build marker")
	}
	if body := rec.Body.String(); strings.Contains(body, "const DEV_MODE") {
		t.Fatalf("unexpected dev mode override in web page")
	}
}

func TestWebPageMethodNotAllowed(t *testing.T) {
	s := NewServer(nes.NewConsole(), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/web", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}
