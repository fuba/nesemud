package config

import (
	"encoding/json"
	"os"
	"strings"
)

type Config struct {
	ListenAddr string `json:"listen_addr"`
	LogFile    string `json:"log_file"`
	HLSDir     string `json:"hls_dir"`
}

func Default() Config {
	return Config{
		ListenAddr: ":18080",
		LogFile:    "./nesd.log",
		HLSDir:     "./hls",
	}
}

func Load(path string) (Config, error) {
	if path == "" {
		cfg := Default()
		applyEnvOverrides(&cfg)
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENV")), "DEVELOPMENT") {
		cfg.ListenAddr = ":18081"
	}
}
