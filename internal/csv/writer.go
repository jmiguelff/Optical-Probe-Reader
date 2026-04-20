package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"optical-probe-reader/internal/parser"
)

// RotatingWriter manages a daily CSV file with automatic midnight rotation.
// When the date changes, the current file is closed and moved to the archive directory,
// and a new file is created for the new day.
type RotatingWriter struct {
	workDir       string // Current working directory for today's file
	archiveDir    string // Final destination for closed daily files
	currentFile   *os.File
	csvWriter     *csv.Writer
	currentDate   string // YYYY-MM-DD of the open file
	headerWritten bool
}

var csvHeaders = []string{
	"timestamp",
	"timestamp_em",
	"energy_import_t1_kwh",
	"energy_import_t2_kwh",
	"energy_import_total_kwh",
	"energy_export_t1_kwh",
	"energy_export_t2_kwh",
	"energy_export_total_kwh",
	"active_power_kw",
	"reactive_power_kvar",
	"frequency_hz",
	"voltage_l1_v",
	"voltage_l2_v",
	"voltage_l3_v",
	"current_l1_a",
	"current_l2_a",
	"current_l3_a",
	"current_neutral_a",
	"max_demand_1_kw",
	"max_demand_2_kw",
	"reactive_quadrant_38_1_kvarh",
	"reactive_quadrant_38_2_kvarh",
	"reactive_quadrant_39_1_kvarh",
	"reactive_quadrant_39_2_kvarh",
	"reactive_quadrant_48_1_kvarh",
	"reactive_quadrant_48_2_kvarh",
	"reactive_quadrant_49_1_kvarh",
	"reactive_quadrant_49_2_kvarh",
}

// NewRotatingWriter creates a new rotating CSV writer.
// workDir: directory where the current day's file is created
// archiveDir: directory where completed daily files are moved
func NewRotatingWriter(workDir, archiveDir string) (*RotatingWriter, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("create work directory: %w", err)
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return nil, fmt.Errorf("create archive directory: %w", err)
	}

	rw := &RotatingWriter{
		workDir:     workDir,
		archiveDir:  archiveDir,
		currentDate: time.Now().Format("2006-01-02"),
	}

	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

// WriteReading writes a MeterReading as a CSV row.
// If the date has changed since the file was opened, rotation occurs:
// the current file is closed, moved to archive, and a new file is opened.
func (rw *RotatingWriter) WriteReading(machine_timestamp time.Time, reading *parser.MeterReading) error {
	// Check if rotation is needed
	now := time.Now()
	newDate := now.Format("2006-01-02")

	if newDate != rw.currentDate {
		// Date changed; rotate the file
		if err := rw.rotateFile(); err != nil {
			// Log error but continue; don't fail the write
			fmt.Fprintf(os.Stderr, "warning: file rotation failed: %v\n", err)
		}
		rw.currentDate = newDate
		if err := rw.openFile(); err != nil {
			return err
		}
	}

	// Write header on first write to a file
	if !rw.headerWritten {
		if err := rw.writeHeader(); err != nil {
			return err
		}
		rw.headerWritten = true
	}

	// Convert MeterReading to CSV row
	row := readingToRow(machine_timestamp, reading)
	rw.csvWriter.Write(row)
	rw.csvWriter.Flush()

	return rw.csvWriter.Error()
}

// Close flushes and closes the current CSV file.
func (rw *RotatingWriter) Close() error {
	if rw.csvWriter != nil {
		rw.csvWriter.Flush()
	}
	if rw.currentFile != nil {
		return rw.currentFile.Close()
	}
	return nil
}

// openFile opens or creates the daily CSV file for today's date.
func (rw *RotatingWriter) openFile() error {
	filename := fmt.Sprintf("metering_%s.csv", rw.currentDate)
	filePath := filepath.Join(rw.workDir, filename)

	// Try to open for append (file may exist from earlier today if app restarted)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open CSV file %s: %w", filePath, err)
	}

	rw.currentFile = file
	rw.csvWriter = csv.NewWriter(rw.currentFile)

	// Check if file is empty (new file); if not, we're appending and should skip header
	fi, err := file.Stat()
	if err == nil && fi.Size() > 0 {
		// File already has content; don't rewrite header
		rw.headerWritten = true
	} else {
		rw.headerWritten = false
	}

	return nil
}

// rotateFile closes the current file and moves it to the archive directory.
func (rw *RotatingWriter) rotateFile() error {
	if rw.currentFile != nil {
		rw.csvWriter.Flush()
		if err := rw.currentFile.Close(); err != nil {
			return err
		}
	}

	// Move file from workDir to archiveDir
	oldFilename := fmt.Sprintf("metering_%s.csv", rw.currentDate)
	oldPath := filepath.Join(rw.workDir, oldFilename)
	newPath := filepath.Join(rw.archiveDir, oldFilename)

	if _, err := os.Stat(oldPath); err == nil {
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("move %s to %s: %w", oldPath, newPath, err)
		}
	}

	return nil
}

// writeHeader writes the CSV header row.
func (rw *RotatingWriter) writeHeader() error {
	rw.csvWriter.Write(csvHeaders)
	rw.csvWriter.Flush()
	return rw.csvWriter.Error()
}

// WriteSingleReadingFile writes a single two-row CSV file (header + one data row).
func WriteSingleReadingFile(dir string, machineTime time.Time, reading *parser.MeterReading) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create CSV directory: %w", err)
	}

	filename := fmt.Sprintf("metering_%s.csv", machineTime.Format("20060102_150405_000"))
	filePath := filepath.Join(dir, filename)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("create CSV file %s: %w", filePath, err)
	}

	writer := csv.NewWriter(file)
	writer.Write(csvHeaders)
	writer.Write(readingToRow(machineTime, reading))
	writer.Flush()

	if csvErr := writer.Error(); csvErr != nil {
		_ = file.Close()
		return "", fmt.Errorf("write CSV file %s: %w", filePath, csvErr)
	}

	if closeErr := file.Close(); closeErr != nil {
		return "", fmt.Errorf("close CSV file %s: %w", filePath, closeErr)
	}

	return filePath, nil
}

// readingToRow converts a MeterReading to a CSV row ([]string).
// Nil values are written as empty strings.
func readingToRow(machineTime time.Time, reading *parser.MeterReading) []string {
	return []string{
		machineTime.Format("2006-01-02 15:04:05"),
		reading.TimestampEM,
		floatOrEmpty(reading.EnergyImportT1),
		floatOrEmpty(reading.EnergyImportT2),
		floatOrEmpty(reading.EnergyImportTotal),
		floatOrEmpty(reading.EnergyExportT1),
		floatOrEmpty(reading.EnergyExportT2),
		floatOrEmpty(reading.EnergyExportTotal),
		floatOrEmpty(reading.ActivePowerKW),
		floatOrEmpty(reading.ReactivePowerKVAR),
		floatOrEmpty(reading.FrequencyHz),
		floatOrEmpty(reading.VoltageL1V),
		floatOrEmpty(reading.VoltageL2V),
		floatOrEmpty(reading.VoltageL3V),
		floatOrEmpty(reading.CurrentL1A),
		floatOrEmpty(reading.CurrentL2A),
		floatOrEmpty(reading.CurrentL3A),
		floatOrEmpty(reading.CurrentNeutralA),
		floatOrEmpty(reading.MaxDemand1KW),
		floatOrEmpty(reading.MaxDemand2KW),
		floatOrEmpty(reading.ReactiveQuadrant381),
		floatOrEmpty(reading.ReactiveQuadrant382),
		floatOrEmpty(reading.ReactiveQuadrant391),
		floatOrEmpty(reading.ReactiveQuadrant392),
		floatOrEmpty(reading.ReactiveQuadrant481),
		floatOrEmpty(reading.ReactiveQuadrant482),
		floatOrEmpty(reading.ReactiveQuadrant491),
		floatOrEmpty(reading.ReactiveQuadrant492),
	}
}

// floatOrEmpty returns the string representation of a float64 pointer,
// or an empty string if the pointer is nil.
func floatOrEmpty(val *float64) string {
	if val == nil {
		return ""
	}
	return strconv.FormatFloat(*val, 'f', -1, 64)
}
