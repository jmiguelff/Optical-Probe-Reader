package csv

import (
	"testing"
	"time"

	"optical-probe-reader/internal/parser"
)

func TestHeaderAndRowColumnsMatch(t *testing.T) {
	row := readingToRow(time.Now(), &parser.MeterReading{})
	if len(row) != len(csvHeaders) {
		t.Fatalf("row has %d columns, headers have %d", len(row), len(csvHeaders))
	}
}

func TestNewTariffColumnsAppendedAtEnd(t *testing.T) {
	want := []string{
		"energy_import_t3_kwh",
		"energy_import_t4_kwh",
		"energy_export_t3_kwh",
		"energy_export_t4_kwh",
	}
	tail := csvHeaders[len(csvHeaders)-len(want):]
	for i, w := range want {
		if tail[i] != w {
			t.Fatalf("csvHeaders tail[%d] = %q, want %q", i, tail[i], w)
		}
	}
}

func TestRowIncludesNewTariffValues(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	reading := &parser.MeterReading{
		EnergyImportT3: f(72590),
		EnergyImportT4: f(34175),
		EnergyExportT3: f(2917125),
		EnergyExportT4: f(1027411),
	}
	row := readingToRow(time.Now(), reading)
	got := row[len(row)-4:]
	want := []string{"72590", "34175", "2917125", "1027411"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row tail[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
