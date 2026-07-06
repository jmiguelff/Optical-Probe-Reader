package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var signedValuePattern = regexp.MustCompile(`^([A-Za-z]+)([+-]?\d+(?:\.\d+)?)$`)

// MeterReading contains all extracted OBIS fields from a meter dump.
type MeterReading struct {
	// Timestamps
	TimestampEM string // Derived from OBIS 12 + 11 (meter date/time)
	ReadTime    time.Time

	// Core energy fields (kWh)
	EnergyImportT1 *float64 // 8.1
	EnergyImportT2 *float64 // 8.2
	EnergyImportT3 *float64 // 8.3
	EnergyImportT4 *float64 // 8.4
	EnergyExportT1 *float64 // 9.1
	EnergyExportT2 *float64 // 9.2
	EnergyExportT3 *float64 // 9.3
	EnergyExportT4 *float64 // 9.4

	// Calculated totals (prefer meter's official register, else tariff sum)
	EnergyImportTotal *float64 // OBIS 20 (+A total), else 8.1+8.2[+8.3][+8.4]
	EnergyExportTotal *float64 // OBIS 21 (-A total), else 9.1+9.2[+9.3][+9.4]

	// Power
	ActivePowerKW     *float64 // P (kW)
	ReactivePowerKVAR *float64 // R (kVAR)
	FrequencyHz       *float64 // 74 (Hz)

	// Per-phase electrical
	VoltageL1V      *float64 // 72.1
	VoltageL2V      *float64 // 72.2
	VoltageL3V      *float64 // 72.3
	CurrentL1A      *float64 // 73.1
	CurrentL2A      *float64 // 73.2
	CurrentL3A      *float64 // 73.3
	CurrentNeutralA *float64 // 73.0

	// Max demand (kW)
	MaxDemand1KW *float64 // 6.1
	MaxDemand2KW *float64 // 6.2

	// Reactive quadrants (kVARh)
	ReactiveQuadrant381 *float64 // 38.1
	ReactiveQuadrant382 *float64 // 38.2
	ReactiveQuadrant391 *float64 // 39.1
	ReactiveQuadrant392 *float64 // 39.2
	ReactiveQuadrant481 *float64 // 48.1
	ReactiveQuadrant482 *float64 // 48.2
	ReactiveQuadrant491 *float64 // 49.1
	ReactiveQuadrant492 *float64 // 49.2
}

// Parse extracts OBIS fields from raw IEC 62056-21 meter data.
// Input is typically raw ASCII text from the meter dump.
// Returns a MeterReading with any successfully parsed fields; unparseable fields remain nil.
func Parse(rawData []byte) *MeterReading {
	text := string(rawData)
	observedValues := scanObservedValues(text)
	reading := &MeterReading{
		ReadTime: time.Now(),
	}

	// Parse all OBIS codes
	reading.EnergyImportT1 = extractFloat(observedValues, "1.8.1", "8.1")
	reading.EnergyImportT2 = extractFloat(observedValues, "1.8.2", "8.2")
	reading.EnergyImportT3 = extractFloat(observedValues, "1.8.3", "8.3")
	reading.EnergyImportT4 = extractFloat(observedValues, "1.8.4", "8.4")
	reading.EnergyExportT1 = extractFloat(observedValues, "2.8.1", "9.1")
	reading.EnergyExportT2 = extractFloat(observedValues, "2.8.2", "9.2")
	reading.EnergyExportT3 = extractFloat(observedValues, "2.8.3", "9.3")
	reading.EnergyExportT4 = extractFloat(observedValues, "2.8.4", "9.4")

	reading.ActivePowerKW = extractFloat(observedValues, "16.7.0", "15.7.0", "1.7.0", "P")
	reading.ReactivePowerKVAR = extractFloat(observedValues, "R")
	reading.FrequencyHz = extractFloat(observedValues, "14.7.0", "74")

	reading.VoltageL1V = extractFloat(observedValues, "32.7.0", "72.1")
	reading.VoltageL2V = extractFloat(observedValues, "52.7.0", "72.2")
	reading.VoltageL3V = extractFloat(observedValues, "72.7.0", "72.3")
	reading.CurrentNeutralA = extractFloat(observedValues, "91.7.0", "73.0")
	reading.CurrentL1A = extractFloat(observedValues, "31.7.0", "73.1")
	reading.CurrentL2A = extractFloat(observedValues, "51.7.0", "73.2")
	reading.CurrentL3A = extractFloat(observedValues, "71.7.0", "73.3")

	reading.MaxDemand1KW = extractFloat(observedValues, "1.6.1", "6.1")
	reading.MaxDemand2KW = extractFloat(observedValues, "1.6.2", "6.2")

	reading.ReactiveQuadrant381 = extractFloat(observedValues, "5.8.1", "38.1")
	reading.ReactiveQuadrant382 = extractFloat(observedValues, "5.8.2", "38.2")
	reading.ReactiveQuadrant391 = extractFloat(observedValues, "6.8.1", "39.1")
	reading.ReactiveQuadrant392 = extractFloat(observedValues, "6.8.2", "39.2")
	reading.ReactiveQuadrant481 = extractFloat(observedValues, "7.8.1", "48.1")
	reading.ReactiveQuadrant482 = extractFloat(observedValues, "7.8.2", "48.2")
	reading.ReactiveQuadrant491 = extractFloat(observedValues, "8.8.1", "49.1")
	reading.ReactiveQuadrant492 = extractFloat(observedValues, "8.8.2", "49.2")

	// Parse timestamp from OBIS 12 and 11
	reading.TimestampEM = extractTimestamp(observedValues)

	// Read official grand-total registers (OBIS 20 = +A total, OBIS 21 = -A total)
	importGrandTotal := extractFloat(observedValues, "1.8.0", "20")
	exportGrandTotal := extractFloat(observedValues, "2.8.0", "21")

	// Calculate totals
	reading.calculateTotals(importGrandTotal, exportGrandTotal)

	return reading
}

func scanObservedValues(text string) map[string]string {
	observed := make(map[string]string)

	for _, rawLine := range strings.Split(text, "\n") {
		line := sanitizeLine(rawLine)
		if line == "" {
			continue
		}

		if code, value, ok := parseParenthesizedLine(line); ok {
			if _, exists := observed[code]; !exists {
				observed[code] = value
			}
			continue
		}

		if code, value, ok := parseSignedLine(line); ok {
			if _, exists := observed[code]; !exists {
				observed[code] = value
			}
		}
	}

	return observed
}

func sanitizeLine(rawLine string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
	if trimmed == "" {
		return ""
	}

	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, trimmed)
}

func parseParenthesizedLine(line string) (string, string, bool) {
	openIdx := strings.IndexByte(line, '(')
	closeIdx := strings.IndexByte(line, ')')
	if openIdx <= 0 || closeIdx <= openIdx+1 {
		return "", "", false
	}

	code := normalizeOBISCode(line[:openIdx])
	if code == "" {
		return "", "", false
	}

	value := strings.TrimSpace(line[openIdx+1 : closeIdx])
	if unitIdx := strings.IndexByte(value, '*'); unitIdx >= 0 {
		value = strings.TrimSpace(value[:unitIdx])
	}
	if value == "" {
		return "", "", false
	}

	return code, value, true
}

func parseSignedLine(line string) (string, string, bool) {
	matches := signedValuePattern.FindStringSubmatch(strings.TrimSpace(line))
	if matches == nil {
		return "", "", false
	}

	code := normalizeOBISCode(matches[1])
	if code == "" {
		return "", "", false
	}

	return code, strings.TrimSpace(matches[2]), true
}

func normalizeOBISCode(token string) string {
	normalized := strings.TrimSpace(token)
	if normalized == "" {
		return ""
	}

	if idx := strings.LastIndex(normalized, ":"); idx >= 0 {
		normalized = normalized[idx+1:]
	}
	if idx := strings.IndexByte(normalized, '*'); idx >= 0 {
		normalized = normalized[:idx]
	}

	return strings.TrimSpace(normalized)
}

func extractFloat(observedValues map[string]string, aliases ...string) *float64 {
	rawValue, ok := lookupValue(observedValues, aliases...)
	if !ok {
		return nil
	}

	parsedValue, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return nil
	}

	return &parsedValue
}

func lookupValue(observedValues map[string]string, aliases ...string) (string, bool) {
	for _, alias := range aliases {
		if value, ok := observedValues[alias]; ok {
			return value, true
		}
	}

	return "", false
}

// extractTimestamp builds a timestamp string from observed OBIS date and time values.
// Format depends on meter; typically: 0.9.2(DDMMYY) and 0.9.1(HHMMSS) or legacy 12/11.
func extractTimestamp(observedValues map[string]string) string {
	dateStr, hasDate := lookupValue(observedValues, "0.9.2", "12")
	timeStr, hasTime := lookupValue(observedValues, "0.9.1", "11")
	if !hasDate || !hasTime {
		return ""
	}

	dateStr = padDigits(dateStr, 6)
	timeStr = padDigits(timeStr, 6)

	return fmt.Sprintf("%s_%s", dateStr, timeStr)
}

func padDigits(value string, width int) string {
	trimmed := strings.TrimSpace(value)
	for len(trimmed) < width {
		trimmed = "0" + trimmed
	}
	return trimmed
}

// calculateTotals derives energy_import_total and energy_export_total.
// It prefers the meter's official grand-total register (OBIS 20 for +A,
// OBIS 21 for -A) when present: that value is authoritative and immune to a
// tariff register missing from a single dump. Otherwise it falls back to
// summing the available tariff registers (requiring at least T1 and T2,
// preserving legacy 2-tariff behavior).
func (m *MeterReading) calculateTotals(importGrandTotal, exportGrandTotal *float64) {
	m.EnergyImportTotal = grandTotalOrTariffSum(
		importGrandTotal,
		m.EnergyImportT1, m.EnergyImportT2, m.EnergyImportT3, m.EnergyImportT4,
	)
	m.EnergyExportTotal = grandTotalOrTariffSum(
		exportGrandTotal,
		m.EnergyExportT1, m.EnergyExportT2, m.EnergyExportT3, m.EnergyExportT4,
	)
}

// grandTotalOrTariffSum returns a copy of grandTotal when present. Otherwise,
// when both t1 and t2 are present, it returns their sum plus any present
// t3/t4. Returns nil when neither an official total nor T1+T2 are available.
func grandTotalOrTariffSum(grandTotal, t1, t2, t3, t4 *float64) *float64 {
	if grandTotal != nil {
		v := *grandTotal
		return &v
	}
	if t1 == nil || t2 == nil {
		return nil
	}
	sum := *t1 + *t2
	if t3 != nil {
		sum += *t3
	}
	if t4 != nil {
		sum += *t4
	}
	return &sum
}

// ParsedFieldCount returns how many parser output fields are populated.
func (m *MeterReading) ParsedFieldCount() int {
	count := 0

	if m.TimestampEM != "" {
		count++
	}

	values := []*float64{
		m.EnergyImportT1,
		m.EnergyImportT2,
		m.EnergyImportT3,
		m.EnergyImportT4,
		m.EnergyExportT1,
		m.EnergyExportT2,
		m.EnergyExportT3,
		m.EnergyExportT4,
		m.EnergyImportTotal,
		m.EnergyExportTotal,
		m.ActivePowerKW,
		m.ReactivePowerKVAR,
		m.FrequencyHz,
		m.VoltageL1V,
		m.VoltageL2V,
		m.VoltageL3V,
		m.CurrentL1A,
		m.CurrentL2A,
		m.CurrentL3A,
		m.CurrentNeutralA,
		m.MaxDemand1KW,
		m.MaxDemand2KW,
		m.ReactiveQuadrant381,
		m.ReactiveQuadrant382,
		m.ReactiveQuadrant391,
		m.ReactiveQuadrant392,
		m.ReactiveQuadrant481,
		m.ReactiveQuadrant482,
		m.ReactiveQuadrant491,
		m.ReactiveQuadrant492,
	}

	for _, value := range values {
		if value != nil {
			count++
		}
	}

	return count
}
