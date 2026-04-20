package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"optical-probe-reader/internal/config"
	"optical-probe-reader/internal/csv"
	"optical-probe-reader/internal/output"
	"optical-probe-reader/internal/parser"
	"optical-probe-reader/internal/protocol"
	"optical-probe-reader/internal/transport"
)

// Harvester collects meter readings at regular intervals and writes them to a rotating CSV.
type Harvester struct {
	cfg       config.Config
	interval  time.Duration
	tr        transport.Transport
	csvWriter *csv.RotatingWriter
	engine    *protocol.Engine
}

// NewHarvester creates a new Harvester with the given config.
func NewHarvester(cfg config.Config) (*Harvester, error) {
	tr, err := transport.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create transport: %w", err)
	}

	csvWriter, err := csv.NewRotatingWriter(cfg.CSV.Directory, cfg.CSV.ArchiveDirectory)
	if err != nil {
		tr.Close()
		return nil, fmt.Errorf("create CSV writer: %w", err)
	}

	return &Harvester{
		cfg:       cfg,
		interval:  time.Duration(cfg.CSV.CollectionIntervalMs) * time.Millisecond,
		tr:        tr,
		csvWriter: csvWriter,
		engine:    protocol.NewEngine(),
	}, nil
}

// Run starts the collection loop, running until a shutdown signal is received.
// On each interval, it reads meter data, parses it, and writes a CSV row.
// Errors are logged to stderr but don't stop the loop.
func (h *Harvester) Run() error {
	defer h.Close()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "harvest: starting collection loop (interval: %v)\n", h.interval)

	for {
		select {
		case <-sigChan:
			fmt.Fprintf(os.Stderr, "harvest: shutdown signal received\n")
			return nil
		case <-ticker.C:
			if err := h.collectOnce(); err != nil {
				fmt.Fprintf(os.Stderr, "harvest: collection error: %v\n", err)
				// Continue; don't stop the loop
			}
		}
	}
}

// collectOnce performs a single meter read, parses it, and writes to CSV.
func (h *Harvester) collectOnce() error {
	machineTime := time.Now()

	overallTimeout := time.Duration(h.cfg.IEC62056.OverallTimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	raw, status, err := h.engine.ReadRaw(ctx, h.tr, h.cfg.IEC62056)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if len(raw) == 0 {
		return fmt.Errorf("read: empty payload (status: %s)", status)
	}

	// Parse the raw data
	reading := parser.Parse(raw)
	parsedFieldCount := reading.ParsedFieldCount()
	if parsedFieldCount == 0 {
		fmt.Fprintf(os.Stderr, "harvest: %s - parser extracted 0 fields; preview: %s\n", machineTime.Format("15:04:05"), buildRawPreview(raw, 4, 240))

		if h.cfg.Output.Directory != "" {
			savedPath, saveErr := output.WriteRawFile(h.cfg.Output.Directory, raw, machineTime)
			if saveErr != nil {
				fmt.Fprintf(os.Stderr, "harvest: %s - failed to save raw dump for parser failure: %v\n", machineTime.Format("15:04:05"), saveErr)
			} else {
				fmt.Fprintf(os.Stderr, "harvest: %s - saved parser-failure raw dump to %s\n", machineTime.Format("15:04:05"), savedPath)
			}
		}
	} else {
		timestampSuffix := ""
		if reading.TimestampEM != "" {
			timestampSuffix = fmt.Sprintf(" (timestamp_em=%s)", reading.TimestampEM)
		}
		fmt.Fprintf(os.Stderr, "harvest: %s - parser extracted %d fields%s\n", machineTime.Format("15:04:05"), parsedFieldCount, timestampSuffix)
	}

	// Write to CSV (with empty values for any unparseable fields)
	if err := h.csvWriter.WriteReading(machineTime, reading); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}

	fmt.Fprintf(os.Stderr, "harvest: %s - collected %d bytes (status: %s)\n", machineTime.Format("15:04:05"), len(raw), status)

	return nil
}

// Close cleans up resources: closes the transport and CSV writer.
func (h *Harvester) Close() error {
	h.tr.Close()
	if h.csvWriter != nil {
		if err := h.csvWriter.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "harvest: close CSV writer: %v\n", err)
		}
	}
	return nil
}

func buildRawPreview(raw []byte, maxLines int, maxChars int) string {
	previewLines := make([]string, 0, maxLines)

	for _, rawLine := range strings.Split(string(raw), "\n") {
		line := sanitizePreviewLine(rawLine)
		if line == "" {
			continue
		}

		previewLines = append(previewLines, line)
		if len(previewLines) == maxLines {
			break
		}
	}

	if len(previewLines) == 0 {
		return "<no printable preview>"
	}

	preview := strings.Join(previewLines, " | ")
	if len(preview) > maxChars {
		return preview[:maxChars] + "..."
	}

	return preview
}

func sanitizePreviewLine(rawLine string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
	if trimmed == "" {
		return ""
	}

	cleaned := strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, trimmed)

	return strings.TrimSpace(cleaned)
}
