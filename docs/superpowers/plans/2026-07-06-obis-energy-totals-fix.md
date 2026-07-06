# OBIS 4-Tariff Energy Totals Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Read all four energy tariffs per direction plus the meter's official grand-total registers (OBIS 20/21), and use those official totals for `energy_import_total_kwh` / `energy_export_total_kwh` instead of summing only two tariffs.

**Architecture:** The parser (`internal/parser/obis.go`) gains four tariff fields (T3/T4 for import and export) and reads the official total registers as locals. `calculateTotals` prefers the official register, falling back to the tariff sum (requiring T1+T2, preserving legacy 2-tariff behavior). The CSV writer (`internal/csv/writer.go`) appends four new tariff columns at the end so existing column positions are untouched.

**Tech Stack:** Go, standard library only (`encoding/csv`, `strconv`, `regexp`).

## Global Constraints

- Go module: `optical-probe-reader`.
- Existing CSV column **names and positions** must not change; new columns are appended at the **end** of `csvHeaders` and `readingToRow`.
- Totals prefer the official register (OBIS 20 = +A total, OBIS 21 = -A total); tariff sum is used only as fallback and requires at least T1 and T2 present.
- Aliases list the full IEC code first, then the meter's short code — matching the existing `extractFloat(observedValues, "1.8.1", "8.1")` pattern.
- Existing tests (`TestParseStandardOBISFrame`, `TestParseLegacyShortCodes`) must keep passing.
- Reference dump (2026-07-06): import 8.1..8.4 = 50714/34362/72590/34175 (OBIS 20 = 191841); export 9.1..9.4 = 1507421/1022150/2917125/1027411 (sum 6474107, OBIS 21 = **6474108**).

---

### Task 1: Parser — four tariffs and official-register totals

**Files:**
- Modify: `internal/parser/obis.go` (struct fields ~20-27, `Parse` reads ~69-72 and totals call ~102, `calculateTotals` ~250-261, `ParsedFieldCount` slice ~271-298)
- Test: `internal/parser/obis_test.go`

**Interfaces:**
- Consumes: existing `extractFloat(observedValues map[string]string, aliases ...string) *float64`, `scanObservedValues`.
- Produces: new `MeterReading` fields `EnergyImportT3, EnergyImportT4, EnergyExportT3, EnergyExportT4 *float64`; changed method signature `func (m *MeterReading) calculateTotals(importGrandTotal, exportGrandTotal *float64)`; new helper `func grandTotalOrTariffSum(grandTotal, t1, t2, t3, t4 *float64) *float64`.

- [ ] **Step 1: Write the failing test (real dump)**

Add to `internal/parser/obis_test.go`:

```go
func TestParseFourTariffRealDump(t *testing.T) {
	raw := []byte("/LGZ4\\2ZMD4054407.B31\r\n" +
		"8.1(00050714*kWh)\r\n" +
		"8.1*80(00050670)\r\n" + // historical snapshot, must be ignored
		"8.2(00034362*kWh)\r\n" +
		"8.3(00072590*kWh)\r\n" +
		"8.4(00034175*kWh)\r\n" +
		"9.1(01507421*kWh)\r\n" +
		"9.1*80(01501447)\r\n" + // historical snapshot, must be ignored
		"9.2(01022150*kWh)\r\n" +
		"9.3(02917125*kWh)\r\n" +
		"9.4(01027411*kWh)\r\n" +
		"20(00191841*kWh)\r\n" +
		"20*80(00191700)\r\n" +
		"21(06474108*kWh)\r\n" +
		"21*80(06454460)\r\n" +
		"11(11:18:22)\r\n" +
		"12(26-07-06)\r\n" +
		"!\r\n")

	reading := Parse(raw)

	assertFloat(t, reading.EnergyImportT1, 50714, "EnergyImportT1")
	assertFloat(t, reading.EnergyImportT2, 34362, "EnergyImportT2")
	assertFloat(t, reading.EnergyImportT3, 72590, "EnergyImportT3")
	assertFloat(t, reading.EnergyImportT4, 34175, "EnergyImportT4")
	assertFloat(t, reading.EnergyExportT1, 1507421, "EnergyExportT1")
	assertFloat(t, reading.EnergyExportT2, 1022150, "EnergyExportT2")
	assertFloat(t, reading.EnergyExportT3, 2917125, "EnergyExportT3")
	assertFloat(t, reading.EnergyExportT4, 1027411, "EnergyExportT4")

	// Totals must come from the official registers (OBIS 20 / 21), not the
	// tariff sum. Export is the discriminating case: sum 9.1..9.4 = 6474107,
	// but OBIS 21 = 6474108.
	assertFloat(t, reading.EnergyImportTotal, 191841, "EnergyImportTotal")
	assertFloat(t, reading.EnergyExportTotal, 6474108, "EnergyExportTotal")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/parser/ -run TestParseFourTariffRealDump`
Expected: FAIL — compile error, `reading.EnergyImportT3` (and the other new fields) undefined.

- [ ] **Step 3: Add the four tariff fields to the struct**

In `internal/parser/obis.go`, replace the core energy + calculated totals block (currently lines ~20-27):

```go
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
```

- [ ] **Step 4: Read the new tariffs and the official totals in `Parse`**

Replace the four energy reads (currently lines ~69-72):

```go
	reading.EnergyImportT1 = extractFloat(observedValues, "1.8.1", "8.1")
	reading.EnergyImportT2 = extractFloat(observedValues, "1.8.2", "8.2")
	reading.EnergyImportT3 = extractFloat(observedValues, "1.8.3", "8.3")
	reading.EnergyImportT4 = extractFloat(observedValues, "1.8.4", "8.4")
	reading.EnergyExportT1 = extractFloat(observedValues, "2.8.1", "9.1")
	reading.EnergyExportT2 = extractFloat(observedValues, "2.8.2", "9.2")
	reading.EnergyExportT3 = extractFloat(observedValues, "2.8.3", "9.3")
	reading.EnergyExportT4 = extractFloat(observedValues, "2.8.4", "9.4")
```

Then replace the `// Calculate totals` block (currently lines ~101-102):

```go
	// Read official grand-total registers (OBIS 20 = +A total, OBIS 21 = -A total)
	importGrandTotal := extractFloat(observedValues, "1.8.0", "20")
	exportGrandTotal := extractFloat(observedValues, "2.8.0", "21")

	// Calculate totals
	reading.calculateTotals(importGrandTotal, exportGrandTotal)
```

- [ ] **Step 5: Rewrite `calculateTotals` and add the helper**

Replace the existing `calculateTotals` method (currently lines ~250-261):

```go
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
```

- [ ] **Step 6: Add the four new fields to `ParsedFieldCount`**

In the `values := []*float64{...}` slice inside `ParsedFieldCount` (currently ~271-298), add the four new fields alongside the existing energy fields:

```go
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
```

- [ ] **Step 7: Run the real-dump test to verify it passes**

Run: `go test ./internal/parser/ -run TestParseFourTariffRealDump -v`
Expected: PASS.

- [ ] **Step 8: Add edge-case tests (prefer-official and fallback)**

Add to `internal/parser/obis_test.go`:

```go
func TestGrandTotalPreferredWhenDiffersFromSum(t *testing.T) {
	// Tariff sum = 30, but the official register says 100. Must use 100.
	raw := []byte("9.1(10*kWh)\r\n9.2(20*kWh)\r\n21(100*kWh)\r\n")
	reading := Parse(raw)
	assertFloat(t, reading.EnergyExportTotal, 100, "EnergyExportTotal")
}

func TestTotalsFallBackToTariffSum(t *testing.T) {
	// No OBIS 20 present: sum the available tariffs (10+20+5+1 = 36).
	raw := []byte("8.1(10*kWh)\r\n8.2(20*kWh)\r\n8.3(5*kWh)\r\n8.4(1*kWh)\r\n")
	reading := Parse(raw)
	assertFloat(t, reading.EnergyImportTotal, 36, "EnergyImportTotal")
}
```

- [ ] **Step 9: Run the full parser test suite**

Run: `go test ./internal/parser/ -v`
Expected: PASS — including `TestParseStandardOBISFrame` and `TestParseLegacyShortCodes` (they have no OBIS 20/21, so they exercise the sum fallback and their expected totals are unchanged).

- [ ] **Step 10: Commit**

```bash
git add internal/parser/obis.go internal/parser/obis_test.go
git commit -m "$(cat <<'EOF'
fix(parser): read 4 tariffs and use official OBIS 20/21 totals

The meter has 4 active-energy tariffs (8.1-8.4 / 9.1-9.4) and exposes
official grand-total registers (OBIS 20 = +A total, OBIS 21 = -A total).
The parser only read T1/T2 and summed them, undercounting the totals.

Read T3/T4 both directions, and compute the totals from OBIS 20/21 when
present, falling back to the tariff sum (T1+T2[+T3][+T4]) otherwise.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: CSV writer — append four tariff columns

**Files:**
- Modify: `internal/csv/writer.go` (`csvHeaders` ~26-55, `readingToRow` ~217-248)
- Test: `internal/csv/writer_test.go` (create)

**Interfaces:**
- Consumes: `MeterReading.EnergyImportT3/T4`, `EnergyExportT3/T4` from Task 1; existing unexported `csvHeaders []string`, `readingToRow(machineTime time.Time, reading *parser.MeterReading) []string`, `floatOrEmpty(*float64) string`.
- Produces: four appended CSV columns `energy_import_t3_kwh`, `energy_import_t4_kwh`, `energy_export_t3_kwh`, `energy_export_t4_kwh`.

- [ ] **Step 1: Write the failing tests**

Create `internal/csv/writer_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/csv/ -v`
Expected: `TestNewTariffColumnsAppendedAtEnd` and `TestRowIncludesNewTariffValues` FAIL (columns not present / row tail holds reactive-quadrant values). `TestHeaderAndRowColumnsMatch` currently passes (28 == 28) — it guards the invariant while we add columns.

- [ ] **Step 3: Append the four headers**

In `internal/csv/writer.go`, change the end of `csvHeaders` (currently `"reactive_quadrant_49_2_kvarh",` then `}`):

```go
	"reactive_quadrant_49_2_kvarh",
	"energy_import_t3_kwh",
	"energy_import_t4_kwh",
	"energy_export_t3_kwh",
	"energy_export_t4_kwh",
}
```

- [ ] **Step 4: Append the four row values**

In `readingToRow`, change the end of the returned slice (currently `floatOrEmpty(reading.ReactiveQuadrant492),` then `}`):

```go
		floatOrEmpty(reading.ReactiveQuadrant492),
		floatOrEmpty(reading.EnergyImportT3),
		floatOrEmpty(reading.EnergyImportT4),
		floatOrEmpty(reading.EnergyExportT3),
		floatOrEmpty(reading.EnergyExportT4),
	}
```

- [ ] **Step 5: Run the CSV test suite**

Run: `go test ./internal/csv/ -v`
Expected: PASS — all three tests, including the column-count invariant (32 == 32).

- [ ] **Step 6: Commit**

```bash
git add internal/csv/writer.go internal/csv/writer_test.go
git commit -m "$(cat <<'EOF'
feat(csv): add T3/T4 tariff columns at end of output

Append energy_import_t3/t4_kwh and energy_export_t3/t4_kwh after the
existing columns so positions of current columns are unchanged. The
existing total columns now carry OBIS 20/21 (from the parser change).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Full build and test verification

**Files:** none (verification only)

- [ ] **Step 1: Build the whole module**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 2: Run the entire test suite**

Run: `go test ./...`
Expected: all packages PASS (`ok` for `internal/parser` and `internal/csv`).

- [ ] **Step 3: Vet**

Run: `go vet ./...`
Expected: no output, exit 0.
```
