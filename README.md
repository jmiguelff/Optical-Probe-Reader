# Optical Probe Reader

Minimal Go CLI to read meter data over IEC 62056-21 using either:

- RS232/USB serial
- TCP (serial-to-ethernet gateway)

Current scope includes raw IEC 62056-21 capture and structured OBIS field extraction with CSV logging.

## Features (current)

- YAML configuration loading (`config.example.yaml`, `config.serial.yaml`, `config.tcp.yaml`, `config.csv.yaml`)
- CLI overrides for common transport settings
- Transport abstraction with:
  - Serial transport
  - TCP transport
- IEC 62056-21 raw capture flow (silence wait + wakeup + identification request + capture until ETX/BCC or timeout)
- Timestamped raw dump files named `metering_{timestamp}.txt`
- **OBIS parser**: extracts 24+ meter fields from IEC text-mode data (energy, power, frequency, voltage, current, demand, reactive quadrants)
- **CSV harvester**: collects meter readings at configurable intervals and writes to daily rotating CSV files with automatic archival

## Project structure

```text
.
├── main.go
├── config.example.yaml
├── config.serial.yaml
├── config.tcp.yaml
├── config.csv.yaml
├── optical-probe-reader.service    # Systemd service file
├── README.md
├── SYSTEMD_INSTALL.md              # Installation & deployment guide
└── internal
    ├── config
    ├── csv
    ├── output
    ├── parser
    ├── protocol
    ├── scheduler
    └── transport
```

## Requirements

- Go (module uses Go 1.24)

## Build

```bash
go mod tidy
go build -o meter .
```

## Build for Raspberry Pi (armv7)

Your target looks like: Linux `armv7l` (Raspberry Pi OS kernel `4.14.34-v7+`). For that, you want a Linux ARMv7 binary (`GOOS=linux`, `GOARCH=arm`, `GOARM=7`).

### Option A: Build on the Raspberry Pi (simplest)

1. Install Go on the Pi (use the distro package, or the official Go tarball).
2. Build:

```bash
go mod tidy
go build -o meter .
./meter help
```

### Option B: Cross-compile from another Linux/macOS machine

```bash
go mod tidy

# Build an ARMv7 Linux binary for Raspberry Pi
GOOS=linux GOARCH=arm GOARM=7 go build -o meter-rpi .

# (Optional) sanity check what you produced
file meter-rpi
```

Copy the binary to the Pi (example using scp):

```bash
scp ./meter-rpi pi@raspberrypi.local:~/meter
ssh pi@raspberrypi.local 'chmod +x ~/meter && ~/meter help'
```

Notes:

- If cross-compiling fails due to a dependency requiring `cgo` on your build host, build on the Pi (Option A) or set up an ARMv7 cross toolchain and build with `CGO_ENABLED=1`.

## Usage

Run with config file:

```bash
./meter read -c config.serial.yaml
```

Or directly with `go run`:

```bash
go run . read -c config.serial.yaml
```

### CLI overrides

```bash
./meter read -c config.tcp.yaml \
  --transport=tcp \
  --addr=192.168.1.50:10001 \
  --connect-timeout-ms=2000 \
  --read-timeout-ms=2000 \
  --output=ascii
```

Supported flags on `read`:

- `-c` path to YAML config
- `--transport=serial|tcp`
- `--serial=/dev/ttyUSB0`
- `--baud=300`
- `--addr=host:port`
- `--connect-timeout-ms=...`
- `--read-timeout-ms=...`
- `--output=raw|ascii`

### CSV Harvesting

Collect meter readings at regular intervals (default 15 min) and write to daily rotating CSV files:

```bash
./meter harvest -c config.csv.yaml
```

The harvester:
- Reads meter data every `collection_interval_ms` (default 900000 = 15 minutes)
- Parses OBIS codes to extract 24 meter fields
- Writes to daily CSV files in the working directory (e.g., `metering_2026-04-19.csv`)
- At midnight, closes the file and moves it to the archive directory
- If the app restarts mid-day, it appends to the existing CSV

Harvest overrides:

```bash
./meter harvest -c config.csv.yaml \
  --interval-ms=600000 \
  --csv-dir=csv \
  --csv-archive-dir=csv/archive
```

Supported flags:
- `-c` path to YAML config
- `--interval-ms=...` collection interval (milliseconds)
- `--csv-dir=...` working directory for current CSV
- `--csv-archive-dir=...` archive directory for completed files

**CSV Fields**: The CSV contains the following columns:
- `timestamp` — Linux machine time (ISO 8601 format)
- `timestamp_em` — Meter timestamp (derived from OBIS codes 12 + 11)
- Energy (kWh): `energy_import_t1`, `energy_import_t2`, `energy_import_total`, `energy_export_t1`, `energy_export_t2`, `energy_export_total`
- Power: `active_power_kw`, `reactive_power_kvar`, `frequency_hz`
- Voltage (V): `voltage_l1`, `voltage_l2`, `voltage_l3`
- Current (A): `current_l1`, `current_l2`, `current_l3`, `current_neutral`
- Max demand (kW): `max_demand_1`, `max_demand_2`
- Reactive quadrants (kVARh): `reactive_quadrant_38_1`, `reactive_quadrant_38_2`, `reactive_quadrant_39_1`, `reactive_quadrant_39_2`, `reactive_quadrant_48_1`, `reactive_quadrant_48_2`, `reactive_quadrant_49_1`, `reactive_quadrant_49_2`

Unparseable fields are written as empty cells in the CSV. Failure to read a single record does not stop the harvester; it logs the error and continues on the next interval.

## Deployment as a Systemd Service

To run the meter harvester continuously on a Linux system (e.g., Raspberry Pi), install it as a systemd service:

**Quick install on Raspberry Pi:**

```bash
# 1. Copy binary to target system
scp ./meter-rpi pi@raspberrypi.local:/tmp/meter-rpi

# 2. SSH into the Pi and run the installer
ssh pi@raspberrypi.local << 'EOF'
  cd /tmp
  sudo cp meter-rpi /opt/optical-probe-reader/meter
  sudo chown optical-probe:optical-probe /opt/optical-probe-reader/meter
  sudo chmod 755 /opt/optical-probe-reader/meter
  sudo systemctl restart optical-probe-reader.service
EOF
```

**For full setup instructions**, see [SYSTEMD_INSTALL.md](SYSTEMD_INSTALL.md).

**Service features:**
- Starts automatically at boot (multi-user.target)
- Restarts on failure with backoff
- Logs to systemd journal (`journalctl`)
- Runs as non-root user (`optical-probe`)
- Secure defaults: read-only filesystem except CSV directories

**Common commands:**

```bash
# View status
sudo systemctl status optical-probe-reader.service

# View logs in real-time
sudo journalctl -u optical-probe-reader.service -f

# Restart service
sudo systemctl restart optical-probe-reader.service

# Stop/start
sudo systemctl stop optical-probe-reader.service
sudo systemctl start optical-probe-reader.service
```

## Configuration

Example configs:

Serial example (`config.serial.yaml`):

```yaml
transport:
  type: serial

serial:
  device: /dev/ttyUSB0
  baud: 300
  data_bits: 7
  parity: even
  stop_bits: 1
  read_timeout_ms: 2000

tcp:
  address: "192.168.1.50:10001"
  connect_timeout_ms: 2000
  read_timeout_ms: 2000

iec62056:
  mode: "A"
  wakeup: true
  inter_char_timeout_ms: 150
  silence_duration_ms: 5000
  max_silence_wait_ms: 30000
  capture_idle_gap_ms: 5000
  capture_max_time_ms: 600000
  overall_timeout_ms: 660000

output:
  format: "ascii"
  pretty: true
  directory: "captures"

TCP example (`config.tcp.yaml`):

```yaml
transport:
  type: tcp

tcp:
  address: "192.168.1.52:4001"
  connect_timeout_ms: 2000
  read_timeout_ms: 2000

iec62056:
  mode: "A"
  wakeup: true
  inter_char_timeout_ms: 150
  silence_duration_ms: 5000
  max_silence_wait_ms: 30000
  capture_idle_gap_ms: 5000
  capture_max_time_ms: 600000
  overall_timeout_ms: 660000

output:
  format: "ascii"
  pretty: true
  directory: "captures"
```

Harvest/CSV example (`config.csv.yaml`):

```yaml
transport:
  type: serial

serial:
  device: /dev/ttyUSB0
  baud: 300
  data_bits: 7
  parity: even
  stop_bits: 1
  read_timeout_ms: 2000

iec62056:
  mode: "A"
  wakeup: true
  inter_char_timeout_ms: 150
  silence_duration_ms: 5000
  max_silence_wait_ms: 30000
  capture_idle_gap_ms: 5000
  capture_max_time_ms: 600000
  overall_timeout_ms: 660000

output:
  format: "ascii"
  pretty: true
  directory: "captures"

csv:
  enabled: true
  directory: "csv"
  archive_directory: "csv/archive"
  collection_interval_ms: 900000
```

## Notes

- `output.format=raw` prints hex bytes for debugging.
- `output.format=ascii` prints direct text output from the meter.
- Protocol engine now sends `/?!\r\n`, reads identification, sends fixed ACK (`\x06000\r\n`), and keeps reading without changing baud.
- Full IEC mode negotiation/parsing is not implemented yet.
- For serial settings, common IEC startup framing like 7E1 is supported via config.
- CSV harvester extracts 24+ OBIS meter fields and writes daily rotating CSV files with timestamps.
- CSV files rotate at midnight; completed files are moved to the archive directory.

## Read completion status

After each `read`, the CLI writes a status line to stderr:

- `status: etx_bcc_reached` means the frame ended with ETX+BCC.
- `status: timeout_before_etx_bcc` means overall timeout hit before ETX+BCC.
- `status: partial_without_etx_bcc` means read ended without a complete ETX+BCC frame.

Exit codes:

- `0` complete frame (`etx_bcc_reached`)
- `3` timeout before ETX+BCC
- `4` partial/incomplete without ETX+BCC
- `1` other runtime errors

## Next steps

- Improve OBIS parser to handle more meter formats and profile variants
- Add JSON output format for harvested data
- Implement full IEC 62056-21 mode A/B/C handshake with negotiation
- Add reconnect/backoff options for TCP gateways and serial timeouts
- Add structured logging (debug/info/error levels)
- Add unit tests for OBIS parsing, CSV rotation, and harvester scheduling
