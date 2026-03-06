package config

import (
	"encoding/json"
	"os"
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
		return Default(), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
