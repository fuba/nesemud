package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type RFOutputConfig struct {
	Enabled          bool   `json:"enabled"`
	Address          string `json:"address"`
	AllowRemote      bool   `json:"allow_remote"`
	StreamID         uint32 `json:"stream_id"`
	RFCenterHz       int64  `json:"rf_center_hz"`
	SamplesPerPacket int    `json:"samples_per_packet"`
}

type Config struct {
	ListenAddr string         `json:"listen_addr"`
	LogFile    string         `json:"log_file"`
	HLSDir     string         `json:"hls_dir"`
	RFOutput   RFOutputConfig `json:"rf_output"`
}

func Default() Config {
	return Config{
		ListenAddr: ":18080",
		LogFile:    "./nesd.log",
		HLSDir:     "./hls",
		RFOutput: RFOutputConfig{
			Enabled:          false,
			Address:          "127.0.0.1:23000",
			StreamID:         1,
			RFCenterHz:       189_000_000,
			SamplesPerPacket: 356,
		},
	}
}

func Load(path string) (Config, error) {
	if path == "" {
		cfg := Default()
		if err := applyEnvOverrides(&cfg); err != nil {
			return Config{}, err
		}
		if err := validate(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if len(b) > 64<<10 {
		return Config{}, errors.New("configuration exceeds 64 KiB")
	}
	cfg := Default()
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("configuration contains trailing JSON values")
		}
		return Config{}, err
	}
	if err := applyEnvOverrides(&cfg); err != nil {
		return Config{}, err
	}
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENV")), "DEVELOPMENT") {
		cfg.ListenAddr = ":18081"
	}
	if v := strings.TrimSpace(os.Getenv("NESD_LISTEN_ADDR")); v != "" {
		cfg.ListenAddr = v
	}
	if raw, ok := os.LookupEnv("NESD_RF_OUTPUT_ENABLED"); ok {
		value := strings.TrimSpace(raw)
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse NESD_RF_OUTPUT_ENABLED: %w", err)
		}
		cfg.RFOutput.Enabled = enabled
	}
	if raw, ok := os.LookupEnv("NESD_RF_OUTPUT_ADDRESS"); ok {
		cfg.RFOutput.Address = strings.TrimSpace(raw)
	}
	if raw, ok := os.LookupEnv("NESD_RF_OUTPUT_ALLOW_REMOTE"); ok {
		allowRemote, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parse NESD_RF_OUTPUT_ALLOW_REMOTE: %w", err)
		}
		cfg.RFOutput.AllowRemote = allowRemote
	}
	if raw, ok := os.LookupEnv("NESD_RF_OUTPUT_STREAM_ID"); ok {
		value := strings.TrimSpace(raw)
		streamID, err := strconv.ParseUint(value, 0, 32)
		if err != nil {
			return fmt.Errorf("parse NESD_RF_OUTPUT_STREAM_ID: %w", err)
		}
		cfg.RFOutput.StreamID = uint32(streamID)
	}
	if raw, ok := os.LookupEnv("NESD_RF_OUTPUT_RF_CENTER_HZ"); ok {
		value := strings.TrimSpace(raw)
		rfCenterHz, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("parse NESD_RF_OUTPUT_RF_CENTER_HZ: %w", err)
		}
		cfg.RFOutput.RFCenterHz = rfCenterHz
	}
	if raw, ok := os.LookupEnv("NESD_RF_OUTPUT_SAMPLES_PER_PACKET"); ok {
		value := strings.TrimSpace(raw)
		samplesPerPacket, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse NESD_RF_OUTPUT_SAMPLES_PER_PACKET: %w", err)
		}
		cfg.RFOutput.SamplesPerPacket = samplesPerPacket
	}
	return nil
}

func validate(cfg Config) error {
	if !cfg.RFOutput.Enabled {
		return nil
	}
	ip, err := validateUDPAddress(cfg.RFOutput.Address)
	if err != nil {
		return fmt.Errorf("validate RF output address: %w", err)
	}
	if !ip.IsLoopback() && !cfg.RFOutput.AllowRemote {
		return errors.New("validate RF output address: non-loopback destination requires allow_remote=true")
	}
	if cfg.RFOutput.RFCenterHz <= 0 || cfg.RFOutput.RFCenterHz > (1<<43)-1 {
		return errors.New("validate RF center: must fit positive signed Q44.20 Hz")
	}
	switch cfg.RFOutput.SamplesPerPacket {
	case 356, 360, 1820:
	default:
		return errors.New("validate RF samples per packet: must be 356, 360, or 1820")
	}
	if cfg.RFOutput.SamplesPerPacket == 1820 && !ip.IsLoopback() {
		return errors.New("validate RF samples per packet: SPP-1820 is restricted to loopback transport")
	}
	return nil
}

func validateUDPAddress(address string) (net.IP, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("UDP host is empty")
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return nil, errors.New("UDP host must be an IPv4 literal")
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.Equal(net.IPv4bcast) {
		return nil, errors.New("UDP host must be a specific unicast address")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid UDP port: %w", err)
	}
	if portNumber == 0 {
		return nil, errors.New("UDP port must be between 1 and 65535")
	}
	return ip.To4(), nil
}
