package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"nesemud/internal/nes"
)

func TestROMUpload(t *testing.T) {
	core := nes.NewConsole()
	s := NewServer(core, nil, nil)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("rom", "test.nes")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(minimalINES()); err != nil {
		t.Fatalf("write rom: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/rom/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
	}

	stateReq := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	stateRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(stateRec, stateReq)
	if stateRec.Code != http.StatusOK {
		t.Fatalf("state status = %d", stateRec.Code)
	}
	var st map[string]any
	if err := json.Unmarshal(stateRec.Body.Bytes(), &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	romLoaded, ok := st["rom_loaded"].(bool)
	if !ok || !romLoaded {
		t.Fatalf("rom_loaded = %v", st["rom_loaded"])
	}
}

func minimalINES() []byte {
	b := make([]byte, 16+16384+8192)
	b[0] = 'N'
	b[1] = 'E'
	b[2] = 'S'
	b[3] = 0x1A
	b[4] = 1
	b[5] = 1
	return b
}
