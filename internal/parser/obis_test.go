package parser

import (
	"math"
	"testing"
)

func TestParseStandardOBISFrame(t *testing.T) {
	raw := []byte("/ISK5\\2MT382-1000\r\n" +
		"0-0:1.0.0(260420145723S)\r\n" +
		"0-0:0.9.1(145723)\r\n" +
		"0-0:0.9.2(260420)\r\n" +
		"1-0:1.8.1(12345.678*kWh)\r\n" +
		"1-0:1.8.2(23456.789*kWh)\r\n" +
		"1-0:2.8.1(12.345*kWh)\r\n" +
		"1-0:2.8.2(67.890*kWh)\r\n" +
		"1-0:16.7.0(-0.321*kW)\r\n" +
		"1-0:14.7.0(49.98*Hz)\r\n" +
		"1-0:32.7.0(230.1*V)\r\n" +
		"1-0:52.7.0(229.4*V)\r\n" +
		"1-0:72.7.0(231.0*V)\r\n" +
		"1-0:31.7.0(1.23*A)\r\n" +
		"1-0:51.7.0(2.34*A)\r\n" +
		"1-0:71.7.0(3.45*A)\r\n" +
		"!\r\n\x03\x00")

	reading := Parse(raw)

	assertFloat(t, reading.EnergyImportT1, 12345.678, "EnergyImportT1")
	assertFloat(t, reading.EnergyImportT2, 23456.789, "EnergyImportT2")
	assertFloat(t, reading.EnergyExportT1, 12.345, "EnergyExportT1")
	assertFloat(t, reading.EnergyExportT2, 67.89, "EnergyExportT2")
	assertFloat(t, reading.EnergyImportTotal, 35802.467, "EnergyImportTotal")
	assertFloat(t, reading.EnergyExportTotal, 80.235, "EnergyExportTotal")
	assertFloat(t, reading.ActivePowerKW, -0.321, "ActivePowerKW")
	assertFloat(t, reading.FrequencyHz, 49.98, "FrequencyHz")
	assertFloat(t, reading.VoltageL1V, 230.1, "VoltageL1V")
	assertFloat(t, reading.VoltageL2V, 229.4, "VoltageL2V")
	assertFloat(t, reading.VoltageL3V, 231.0, "VoltageL3V")
	assertFloat(t, reading.CurrentL1A, 1.23, "CurrentL1A")
	assertFloat(t, reading.CurrentL2A, 2.34, "CurrentL2A")
	assertFloat(t, reading.CurrentL3A, 3.45, "CurrentL3A")

	if reading.TimestampEM != "260420_145723" {
		t.Fatalf("TimestampEM = %q, want %q", reading.TimestampEM, "260420_145723")
	}

	if got := reading.ParsedFieldCount(); got < 15 {
		t.Fatalf("ParsedFieldCount = %d, want at least 15", got)
	}
}

func TestParseLegacyShortCodes(t *testing.T) {
	raw := []byte("12(200401)\n11(003005)\n8.1(10.5)\n8.2(11.5)\nP+0.75\n74(50.0)\n")

	reading := Parse(raw)

	if reading.TimestampEM != "200401_003005" {
		t.Fatalf("TimestampEM = %q, want %q", reading.TimestampEM, "200401_003005")
	}
	assertFloat(t, reading.EnergyImportTotal, 22.0, "EnergyImportTotal")
	assertFloat(t, reading.ActivePowerKW, 0.75, "ActivePowerKW")
	assertFloat(t, reading.FrequencyHz, 50.0, "FrequencyHz")
}

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

func TestImportTotalPrefersOfficialWhenTariffMissing(t *testing.T) {
	// OBIS 20 must win even when a tariff is absent: with 8.3 missing the
	// tariff sum would be 35, but the official register says 100. This proves
	// the import total is immune to a tariff register missing from a dump.
	raw := []byte("8.1(10*kWh)\r\n8.2(20*kWh)\r\n8.4(5*kWh)\r\n20(100*kWh)\r\n")
	reading := Parse(raw)
	assertFloat(t, reading.EnergyImportTotal, 100, "EnergyImportTotal")
}

func TestTotalNilWhenSingleTariffAndNoOfficial(t *testing.T) {
	// Only T1 present, no T2 and no official register: the fallback requires
	// at least T1 and T2, so the total stays nil rather than reporting T1 alone.
	raw := []byte("8.1(10*kWh)\r\n")
	reading := Parse(raw)
	if reading.EnergyImportTotal != nil {
		t.Fatalf("EnergyImportTotal = %v, want nil", *reading.EnergyImportTotal)
	}
}

func assertFloat(t *testing.T, got *float64, want float64, label string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s = nil, want %v", label, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, *got, want)
	}
}
