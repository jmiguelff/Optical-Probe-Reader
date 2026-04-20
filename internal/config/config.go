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
	CSV       CSVConfig       `yaml:"csv"`
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
	SilenceDurationMs  int    `yaml:"silence_duration_ms"`
	MaxSilenceWaitMs   int    `yaml:"max_silence_wait_ms"`
	CaptureIdleGapMs   int    `yaml:"capture_idle_gap_ms"`
	CaptureMaxTimeMs   int    `yaml:"capture_max_time_ms"`
	OverallTimeoutMs   int    `yaml:"overall_timeout_ms"`
}

type OutputConfig struct {
	Format    string `yaml:"format"`
	Pretty    bool   `yaml:"pretty"`
	Directory string `yaml:"directory"`
}

type CSVConfig struct {
	Enabled              bool   `yaml:"enabled"`
	Directory            string `yaml:"directory"`
	ArchiveDirectory     string `yaml:"archive_directory"`
	CollectionIntervalMs int    `yaml:"collection_interval_ms"`
}

type Overrides struct {
	TransportType     string
	TCPAddress        string
	SerialDevice      string
	SerialBaud        int
	ConnectTimeoutMs  int
	ReadTimeoutMs     int
	SilenceDurationMs int
	MaxSilenceWaitMs  int
	CaptureIdleGapMs  int
	CaptureMaxTimeMs  int
	OutputFormat      string
	OutputDirectory   string
	CSVEnabled        bool
	CSVDirectory      string
	CSVArchiveDir     string
	CSVIntervalMs     int
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
			SilenceDurationMs:  5000,
			MaxSilenceWaitMs:   30000,
			CaptureIdleGapMs:   5000,
			CaptureMaxTimeMs:   600000,
			OverallTimeoutMs:   660000,
		},
		Output: OutputConfig{
			Format:    "raw",
			Pretty:    true,
			Directory: "captures",
		},
		CSV: CSVConfig{
			Enabled:              false,
			Directory:            "csv",
			ArchiveDirectory:     "csv/archive",
			CollectionIntervalMs: 900000, // 15 minutes
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
	if overrides.SilenceDurationMs > 0 {
		cfg.IEC62056.SilenceDurationMs = overrides.SilenceDurationMs
	}
	if overrides.MaxSilenceWaitMs > 0 {
		cfg.IEC62056.MaxSilenceWaitMs = overrides.MaxSilenceWaitMs
	}
	if overrides.CaptureIdleGapMs > 0 {
		cfg.IEC62056.CaptureIdleGapMs = overrides.CaptureIdleGapMs
	}
	if overrides.CaptureMaxTimeMs > 0 {
		cfg.IEC62056.CaptureMaxTimeMs = overrides.CaptureMaxTimeMs
	}
	if overrides.OutputFormat != "" {
		cfg.Output.Format = overrides.OutputFormat
	}
	if overrides.OutputDirectory != "" {
		cfg.Output.Directory = overrides.OutputDirectory
	}
	if overrides.CSVEnabled {
		cfg.CSV.Enabled = true
	}
	if overrides.CSVDirectory != "" {
		cfg.CSV.Directory = overrides.CSVDirectory
	}
	if overrides.CSVArchiveDir != "" {
		cfg.CSV.ArchiveDirectory = overrides.CSVArchiveDir
	}
	if overrides.CSVIntervalMs > 0 {
		cfg.CSV.CollectionIntervalMs = overrides.CSVIntervalMs
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
	if cfg.IEC62056.SilenceDurationMs <= 0 {
		return fmt.Errorf("iec62056.silence_duration_ms must be > 0")
	}
	if cfg.IEC62056.MaxSilenceWaitMs <= 0 {
		return fmt.Errorf("iec62056.max_silence_wait_ms must be > 0")
	}
	if cfg.IEC62056.MaxSilenceWaitMs < cfg.IEC62056.SilenceDurationMs {
		return fmt.Errorf("iec62056.max_silence_wait_ms must be >= iec62056.silence_duration_ms")
	}
	if cfg.IEC62056.CaptureIdleGapMs <= 0 {
		return fmt.Errorf("iec62056.capture_idle_gap_ms must be > 0")
	}
	if cfg.IEC62056.CaptureMaxTimeMs <= 0 {
		return fmt.Errorf("iec62056.capture_max_time_ms must be > 0")
	}
	switch strings.ToLower(cfg.Output.Format) {
	case "raw", "hex", "raw-hex", "ascii", "raw-ascii", "text", "csv":
	default:
		return fmt.Errorf("output.format must be one of: raw, ascii, csv")
	}

	return nil
}
