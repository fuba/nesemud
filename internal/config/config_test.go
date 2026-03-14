package config

import (
	"os"
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
