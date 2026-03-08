package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"nesemud/internal/nes"
	"nesemud/internal/streaming"
	"nesemud/internal/validation"
)

type MemoryResponse struct {
	Address uint16 `json:"address"`
	Bytes   []byte `json:"bytes"`
}

type MemoryWriteRequest struct {
	Value *int  `json:"value,omitempty"`
	Bytes []int `json:"bytes,omitempty"`
}

type ROMLoadRequest struct {
	ROMContentBase64 string `json:"rom_content_base64"`
}

type FM2LoadRequest struct {
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
}

type Server struct {
	core         *nes.Console
	hls          *streaming.HLSStreamer
	router       http.Handler
	validationMu sync.Mutex
}

func NewServer(core *nes.Console, hls *streaming.HLSStreamer) *Server {
	s := &Server{core: core, hls: hls}
	s.router = s.buildMux()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/web", s.handleWeb)
	mux.HandleFunc("/web/", s.handleWeb)
	mux.HandleFunc("/v1/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(openAPISpec))
	})
	mux.HandleFunc("/v1/state", s.handleState)
	mux.HandleFunc("/v1/rom/load", s.handleLoadROM)
	mux.HandleFunc("/v1/rom/upload", s.handleUploadROM)
	mux.HandleFunc("/v1/control/reset", s.handleReset)
	mux.HandleFunc("/v1/control/pause", s.handlePause)
	mux.HandleFunc("/v1/control/resume", s.handleResume)
	mux.HandleFunc("/v1/replay/fm2", s.handleFM2)
	mux.HandleFunc("/v1/memory/", s.handleMemory)
	mux.HandleFunc("/v1/input/player/", s.handleInput)
	mux.HandleFunc("/v1/retroarch/command", s.handleRetroArchCommand)
	mux.HandleFunc("/v1/retroarch/command/list", s.handleRetroArchCommandList)
	mux.HandleFunc("/v1/validate/replay", s.handleReplayValidation)
	mux.HandleFunc("/v1/validate/nestest", s.handleNESTestValidation)
	mux.HandleFunc("/v1/validate/suite", s.handleSuiteValidation)
	mux.HandleFunc("/v1/stream/stats", s.handleStreamStats)
	return mux
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jsonResponse(w, http.StatusOK, s.core.State())
}

func (s *Server) handleLoadROM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ROMLoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ROMContentBase64) == "" {
		http.Error(w, "rom_content_base64 is required", http.StatusBadRequest)
		return
	}
	b, err := base64.StdEncoding.DecodeString(req.ROMContentBase64)
	if err != nil {
		http.Error(w, "invalid rom_content_base64", http.StatusBadRequest)
		return
	}
	if err := s.core.LoadROMContent(b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleUploadROM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	f, _, err := r.FormFile("rom")
	if err != nil {
		http.Error(w, "rom file is required", http.StatusBadRequest)
		return
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(b) == 0 {
		http.Error(w, "empty rom", http.StatusBadRequest)
		return
	}
	if err := s.core.LoadROMContent(b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.core.Reset()
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.core.SetPaused(true)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.core.SetPaused(false)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleFM2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req FM2LoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Path) != "" {
		http.Error(w, "path input is disabled; use content", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}
	if err := s.core.LoadFM2Content([]byte(req.Content)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	addrStr := strings.TrimPrefix(r.URL.Path, "/v1/memory/")
	v, err := strconv.ParseUint(addrStr, 0, 16)
	if err != nil {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	addr := uint16(v)
	switch r.Method {
	case http.MethodGet:
		length := 1
		if l := r.URL.Query().Get("len"); l != "" {
			n, err := strconv.Atoi(l)
			if err != nil {
				http.Error(w, "invalid len", http.StatusBadRequest)
				return
			}
			length = n
		}
		b, err := s.core.Peek(addr, length)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResponse(w, http.StatusOK, MemoryResponse{Address: addr, Bytes: b})
	case http.MethodPut:
		var req MemoryWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		buf := make([]byte, 0, len(req.Bytes)+1)
		if req.Value != nil {
			buf = append(buf, byte(*req.Value))
		}
		for _, x := range req.Bytes {
			buf = append(buf, byte(x))
		}
		if len(buf) == 0 {
			http.Error(w, "value or bytes is required", http.StatusBadRequest)
			return
		}
		if err := s.core.Poke(addr, buf); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/v1/input/player/")
	id, err := strconv.Atoi(idStr)
	if err != nil || (id != 1 && id != 2) {
		http.Error(w, "invalid player id", http.StatusBadRequest)
		return
	}
	var b nes.Buttons
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.core.SetController(id, b)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRetroArchCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	command := strings.TrimSpace(r.URL.Query().Get("cmd"))
	if command == "" {
		ct := r.Header.Get("Content-Type")
		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		trimmed := bytes.TrimSpace(body)
		isJSON := strings.HasPrefix(ct, "application/json") || (len(trimmed) > 0 && trimmed[0] == '{')
		if isJSON {
			var req struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(trimmed, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			command = req.Command
		} else {
			command = strings.TrimSpace(string(trimmed))
		}
	}
	if command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}
	switch strings.ToUpper(command) {
	case "RESET", "SOFT_RESET":
		s.core.Reset()
	case "PAUSE":
		s.core.SetPaused(true)
	case "RESUME", "UNPAUSE":
		s.core.SetPaused(false)
	default:
		http.Error(w, "unsupported command", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleRetroArchCommandList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"commands": []string{
			"RESET",
			"SOFT_RESET",
			"PAUSE",
			"RESUME",
			"UNPAUSE",
		},
	})
}

func (s *Server) handleReplayValidation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.tryLockValidation() {
		http.Error(w, "validation is busy", http.StatusTooManyRequests)
		return
	}
	defer s.validationMu.Unlock()
	var req validation.ReplayValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ROMPath) != "" || strings.TrimSpace(req.FM2Path) != "" {
		http.Error(w, "path input is disabled; use content fields", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ROMContentBase64) == "" {
		http.Error(w, "rom_content_base64 is required", http.StatusBadRequest)
		return
	}
	res, err := validation.RunReplayValidation(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func (s *Server) handleNESTestValidation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.tryLockValidation() {
		http.Error(w, "validation is busy", http.StatusTooManyRequests)
		return
	}
	defer s.validationMu.Unlock()
	var req validation.NESTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ROMPath) != "" || strings.TrimSpace(req.ExpectedLogPath) != "" {
		http.Error(w, "path input is disabled; use content fields", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ROMContentBase64) == "" || strings.TrimSpace(req.ExpectedLogContent) == "" {
		http.Error(w, "rom_content_base64 and expected_log_content are required", http.StatusBadRequest)
		return
	}
	res, err := validation.RunNESTest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func (s *Server) handleSuiteValidation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.tryLockValidation() {
		http.Error(w, "validation is busy", http.StatusTooManyRequests)
		return
	}
	defer s.validationMu.Unlock()
	var req struct {
		Suite  string `json:"suite"`
		Frames int    `json:"frames"`
		ROMs   []struct {
			Name          string `json:"name"`
			ContentBase64 string `json:"content_base64"`
		} `json:"roms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Suite) == "" || len(req.ROMs) == 0 {
		http.Error(w, "suite and roms are required", http.StatusBadRequest)
		return
	}
	roms := make([]validation.ROMInput, 0, len(req.ROMs))
	for i, r := range req.ROMs {
		if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.ContentBase64) == "" {
			http.Error(w, fmt.Sprintf("roms[%d] name and content_base64 are required", i), http.StatusBadRequest)
			return
		}
		b, err := base64.StdEncoding.DecodeString(r.ContentBase64)
		if err != nil {
			http.Error(w, fmt.Sprintf("roms[%d] invalid content_base64", i), http.StatusBadRequest)
			return
		}
		roms = append(roms, validation.ROMInput{Name: r.Name, Data: b})
	}
	res, err := validation.RunSuiteByROMInputs(req.Suite, roms, req.Frames)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func (s *Server) handleStreamStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.hls == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"running": false})
		return
	}
	st := s.hls.Stats()
	jsonResponse(w, http.StatusOK, st)
}

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) tryLockValidation() bool {
	return s.validationMu.TryLock()
}
