package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"optical-probe-reader/internal/config"
	"optical-probe-reader/internal/output"
	"optical-probe-reader/internal/protocol"
	"optical-probe-reader/internal/transport"
)

var (
	errReadTimedOut   = errors.New("read incomplete: timeout before ETX+BCC")
	errReadIncomplete = errors.New("read incomplete: ETX+BCC not found")
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "read":
		err = runRead(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		printUsage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		switch {
		case errors.Is(err, errReadTimedOut):
			os.Exit(3)
		case errors.Is(err, errReadIncomplete):
			os.Exit(4)
		default:
			os.Exit(1)
		}
	}
}

func runRead(args []string) error {
	flags := flag.NewFlagSet("read", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	configPath := flags.String("c", "", "Path to YAML config file")

	transportType := flags.String("transport", "", "Transport type override: serial|tcp")
	addr := flags.String("addr", "", "TCP address override, e.g. 192.168.1.52:4001")
	serialDevice := flags.String("serial", "", "Serial device override, e.g. /dev/ttyUSB0")
	baud := flags.Int("baud", 0, "Serial baud override")

	connectTimeoutMs := flags.Int("connect-timeout-ms", 0, "TCP connect timeout override")
	readTimeoutMs := flags.Int("read-timeout-ms", 0, "Read timeout override for active transport")
	silenceDurationMs := flags.Int("silence-duration-ms", 0, "Required silent line time before /?! is sent")
	maxSilenceWaitMs := flags.Int("max-silence-wait-ms", 0, "Maximum wait for the line to become silent")
	captureIdleGapMs := flags.Int("capture-idle-gap-ms", 0, "Idle gap that ends capture after data has started")
	captureMaxTimeMs := flags.Int("capture-max-time-ms", 0, "Maximum time allowed for the capture stage")
	outputFormat := flags.String("output", "", "Output format override (currently supported: raw)")
	outputDir := flags.String("output-dir", "", "Directory for metering_{timestamp}.txt dumps")

	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg := config.Default()
	if *configPath != "" {
		loadedConfig, err := config.LoadYAML(*configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		cfg = loadedConfig
	}

	config.ApplyOverrides(&cfg, config.Overrides{
		TransportType:     *transportType,
		TCPAddress:        *addr,
		SerialDevice:      *serialDevice,
		SerialBaud:        *baud,
		ConnectTimeoutMs:  *connectTimeoutMs,
		ReadTimeoutMs:     *readTimeoutMs,
		SilenceDurationMs: *silenceDurationMs,
		MaxSilenceWaitMs:  *maxSilenceWaitMs,
		CaptureIdleGapMs:  *captureIdleGapMs,
		CaptureMaxTimeMs:  *captureMaxTimeMs,
		OutputFormat:      *outputFormat,
		OutputDirectory:   *outputDir,
	})

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	tr, err := transport.New(cfg)
	if err != nil {
		return fmt.Errorf("create transport: %w", err)
	}
	defer tr.Close()

	overallTimeout := time.Duration(cfg.IEC62056.OverallTimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	engine := protocol.NewEngine()
	raw, status, err := engine.ReadRaw(ctx, tr, cfg.IEC62056)
	if err != nil {
		return fmt.Errorf("protocol read: %w", err)
	}

	var writeErr error
	switch strings.ToLower(cfg.Output.Format) {
	case "ascii", "raw-ascii", "text":
		writeErr = output.WriteRawASCII(os.Stdout, raw)
	case "raw", "hex", "raw-hex":
		writeErr = output.WriteRawHex(os.Stdout, raw)
	default:
		return errors.New("unsupported output format")
	}
	if writeErr != nil {
		return writeErr
	}

	if cfg.Output.Directory != "" {
		savedPath, err := output.WriteRawFile(cfg.Output.Directory, raw, time.Now())
		if err != nil {
			return fmt.Errorf("write raw file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "saved: %s\n", savedPath)
	}

	fmt.Fprintf(os.Stderr, "status: %s\n", status)
	switch status {
	case protocol.ReadStatusComplete:
		return nil
	case protocol.ReadStatusPartialTimeout:
		return errReadTimedOut
	default:
		return errReadIncomplete
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  meter read -c config.yaml [overrides]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Overrides:")
	fmt.Fprintln(os.Stderr, "  --transport=serial|tcp")
	fmt.Fprintln(os.Stderr, "  --serial=/dev/ttyUSB0 --baud=300")
	fmt.Fprintln(os.Stderr, "  --addr=192.168.1.52:4001")
	fmt.Fprintln(os.Stderr, "  --connect-timeout-ms=2000 --read-timeout-ms=2000")
	fmt.Fprintln(os.Stderr, "  --output=raw|ascii")
}
