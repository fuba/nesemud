package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLoad_DefaultProductionPort(t *testing.T) {
	t.Setenv("ENV", "")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.ListenAddr != ":18080" {
		t.Fatalf("expected :18080 got %s", cfg.ListenAddr)
	}
}

func TestLoad_DevelopmentPortOverride(t *testing.T) {
	t.Setenv("ENV", "DEVELOPMENT")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.ListenAddr != ":18081" {
		t.Fatalf("expected :18081 got %s", cfg.ListenAddr)
	}
}

func TestLoad_DevelopmentPortOverrideWithFile(t *testing.T) {
	t.Setenv("ENV", "DEVELOPMENT")
	tmp, err := os.CreateTemp(t.TempDir(), "cfg-*.json")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer tmp.Close()
	if _, err := tmp.WriteString(`{"listen_addr":":9999"}`); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	cfg, err := Load(tmp.Name())
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.ListenAddr != ":18081" {
		t.Fatalf("expected :18081 got %s", cfg.ListenAddr)
	}
}

func TestLoad_ExplicitListenAddrOverrideWins(t *testing.T) {
	t.Setenv("ENV", "DEVELOPMENT")
	t.Setenv("NESD_LISTEN_ADDR", ":28081")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.ListenAddr != ":28081" {
		t.Fatalf("expected :28081 got %s", cfg.ListenAddr)
	}
}

func TestDefaultRFOutput(t *testing.T) {
	cfg := Default()
	if cfg.RFOutput.Enabled {
		t.Fatal("RF output must be disabled by default")
	}
	if cfg.RFOutput.Address != "127.0.0.1:23000" {
		t.Fatalf("RF output address=%q", cfg.RFOutput.Address)
	}
	if cfg.RFOutput.StreamID != 1 {
		t.Fatalf("RF output stream ID=%d", cfg.RFOutput.StreamID)
	}
	if cfg.RFOutput.RFCenterHz != 189_000_000 {
		t.Fatalf("RF center=%d", cfg.RFOutput.RFCenterHz)
	}
	if cfg.RFOutput.SamplesPerPacket != 356 {
		t.Fatalf("samples per packet=%d", cfg.RFOutput.SamplesPerPacket)
	}
}

func TestLoadRFOutputFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"rf_output": {
			"enabled": true,
			"address": "192.0.2.1:24000",
			"allow_remote": true,
			"stream_id": 27
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.RFOutput.Enabled || cfg.RFOutput.Address != "192.0.2.1:24000" || cfg.RFOutput.StreamID != 27 {
		t.Fatalf("RF output=%+v", cfg.RFOutput)
	}
}

func TestLoadRejectsRemoteRFOutputWithoutExplicitOptIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"rf_output": {
			"enabled": true,
			"address": "192.0.2.1:24000"
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected remote RF output to require explicit opt-in")
	}
}

func TestLoadRejectsRemoteJumboRFOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"rf_output": {
			"enabled": true,
			"address": "192.0.2.1:24000",
			"allow_remote": true,
			"samples_per_packet": 1820
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected SPP-1820 remote output to be rejected")
	}
}

func TestLoadRejectsHostnameRFOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"rf_output": {
			"enabled": true,
			"address": "localhost:24000"
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected hostname RF output to be rejected")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"unknown_setting":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unknown configuration field to be rejected")
	}
}

func TestLoadRejectsTrailingJSONValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestLoadRFOutputEnvironmentOverrides(t *testing.T) {
	t.Setenv("NESD_RF_OUTPUT_ENABLED", "true")
	t.Setenv("NESD_RF_OUTPUT_ADDRESS", "127.0.0.1:24001")
	t.Setenv("NESD_RF_OUTPUT_STREAM_ID", "1314149187")
	t.Setenv("NESD_RF_OUTPUT_RF_CENTER_HZ", "201000000")
	t.Setenv("NESD_RF_OUTPUT_SAMPLES_PER_PACKET", "1820")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.RFOutput.Enabled || cfg.RFOutput.Address != "127.0.0.1:24001" || cfg.RFOutput.StreamID != 1314149187 || cfg.RFOutput.RFCenterHz != 201_000_000 || cfg.RFOutput.SamplesPerPacket != 1820 {
		t.Fatalf("RF output=%+v", cfg.RFOutput)
	}
}

func TestLoadRejectsInvalidRFOutputEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		enableRF bool
	}{
		{name: "enabled", key: "NESD_RF_OUTPUT_ENABLED", value: "sometimes"},
		{name: "address", key: "NESD_RF_OUTPUT_ADDRESS", value: "missing-port", enableRF: true},
		{name: "stream ID negative", key: "NESD_RF_OUTPUT_STREAM_ID", value: "-1"},
		{name: "stream ID overflow", key: "NESD_RF_OUTPUT_STREAM_ID", value: "4294967296"},
		{name: "RF center negative", key: "NESD_RF_OUTPUT_RF_CENTER_HZ", value: "-1", enableRF: true},
		{name: "unsupported SPP", key: "NESD_RF_OUTPUT_SAMPLES_PER_PACKET", value: "1000", enableRF: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if tt.enableRF {
				t.Setenv("NESD_RF_OUTPUT_ENABLED", "true")
			}
			if _, err := Load(""); err == nil {
				t.Fatal("expected load error")
			}
		})
	}
}

func TestLoadRejectsInvalidEnabledRFOutputAddressFromFile(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{
		{name: "invalid address", address: "not-a-UDP-address"},
		{name: "empty host", address: ":23000"},
		{name: "zero port", address: "127.0.0.1:0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			contents := `{"rf_output":{"enabled":true,"address":` + strconv.Quote(tt.address) + `}}`
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected RF output validation error")
			}
		})
	}
}

func TestLoadAllowsInvalidRFOutputAddressWhenDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"rf_output":{"enabled":false,"address":"legacy-address"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("disabled RF output: %v", err)
	}
}

func TestLoadValidatesRFOutputAfterEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"rf_output":{"enabled":true,"address":"invalid"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NESD_RF_OUTPUT_ENABLED", "false")
	if _, err := Load(path); err != nil {
		t.Fatalf("environment disabled RF output: %v", err)
	}
}
