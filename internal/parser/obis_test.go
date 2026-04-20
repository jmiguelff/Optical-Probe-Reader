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

func assertFloat(t *testing.T, got *float64, want float64, label string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s = nil, want %v", label, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, *got, want)
	}
}
