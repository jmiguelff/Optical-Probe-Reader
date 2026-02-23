package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Transport TransportConfig `yaml:"transport"`
	Serial    SerialConfig    `yaml:"serial"`
	TCP       TCPConfig       `yaml:"tcp"`
	IEC62056  IEC62056Config  `yaml:"iec62056"`
	Output    OutputConfig    `yaml:"output"`
}

type TransportConfig struct {
	Type string `yaml:"type"`
}

type SerialConfig struct {
	Device        string `yaml:"device"`
	Baud          int    `yaml:"baud"`
	DataBits      int    `yaml:"data_bits"`
	Parity        string `yaml:"parity"`
	StopBits      int    `yaml:"stop_bits"`
	ReadTimeoutMs int    `yaml:"read_timeout_ms"`
}

type TCPConfig struct {
	Address          string `yaml:"address"`
	ConnectTimeoutMs int    `yaml:"connect_timeout_ms"`
	ReadTimeoutMs    int    `yaml:"read_timeout_ms"`
}

type IEC62056Config struct {
	Mode               string `yaml:"mode"`
	Wakeup             bool   `yaml:"wakeup"`
	InterCharTimeoutMs int    `yaml:"inter_char_timeout_ms"`
	OverallTimeoutMs   int    `yaml:"overall_timeout_ms"`
}

type OutputConfig struct {
	Format string `yaml:"format"`
	Pretty bool   `yaml:"pretty"`
}

type Overrides struct {
	TransportType    string
	TCPAddress       string
	SerialDevice     string
	SerialBaud       int
	ConnectTimeoutMs int
	ReadTimeoutMs    int
	OutputFormat     string
}

func Default() Config {
	return Config{
		Transport: TransportConfig{Type: "serial"},
		Serial: SerialConfig{
			Device:        "/dev/ttyUSB0",
			Baud:          300,
			DataBits:      7,
			Parity:        "even",
			StopBits:      1,
			ReadTimeoutMs: 2000,
		},
		TCP: TCPConfig{
			Address:          "192.168.1.50:10001",
			ConnectTimeoutMs: 2000,
			ReadTimeoutMs:    2000,
		},
		IEC62056: IEC62056Config{
			Mode:               "A",
			Wakeup:             true,
			InterCharTimeoutMs: 150,
			OverallTimeoutMs:   8000,
		},
		Output: OutputConfig{
			Format: "raw",
			Pretty: true,
		},
	}
}

func LoadYAML(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func ApplyOverrides(cfg *Config, overrides Overrides) {
	if overrides.TransportType != "" {
		cfg.Transport.Type = overrides.TransportType
	}
	if overrides.TCPAddress != "" {
		cfg.TCP.Address = overrides.TCPAddress
	}
	if overrides.SerialDevice != "" {
		cfg.Serial.Device = overrides.SerialDevice
	}
	if overrides.SerialBaud > 0 {
		cfg.Serial.Baud = overrides.SerialBaud
	}
	if overrides.ConnectTimeoutMs > 0 {
		cfg.TCP.ConnectTimeoutMs = overrides.ConnectTimeoutMs
	}
	if overrides.ReadTimeoutMs > 0 {
		switch strings.ToLower(cfg.Transport.Type) {
		case "tcp":
			cfg.TCP.ReadTimeoutMs = overrides.ReadTimeoutMs
		default:
			cfg.Serial.ReadTimeoutMs = overrides.ReadTimeoutMs
		}
	}
	if overrides.OutputFormat != "" {
		cfg.Output.Format = overrides.OutputFormat
	}
}

func (cfg Config) Validate() error {
	switch strings.ToLower(cfg.Transport.Type) {
	case "serial":
		if cfg.Serial.Device == "" {
			return fmt.Errorf("serial.device is required when transport.type=serial")
		}
		if cfg.Serial.Baud <= 0 {
			return fmt.Errorf("serial.baud must be > 0")
		}
	case "tcp":
		if cfg.TCP.Address == "" {
			return fmt.Errorf("tcp.address is required when transport.type=tcp")
		}
	default:
		return fmt.Errorf("transport.type must be serial or tcp")
	}

	if cfg.IEC62056.OverallTimeoutMs <= 0 {
		return fmt.Errorf("iec62056.overall_timeout_ms must be > 0")
	}
	if cfg.IEC62056.InterCharTimeoutMs <= 0 {
		return fmt.Errorf("iec62056.inter_char_timeout_ms must be > 0")
	}

	return nil
}
