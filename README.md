# Optical Probe Reader

Minimal Go CLI to read meter data over IEC 62056-21 using either:

- RS232/USB serial
- TCP (serial-to-ethernet gateway)

Current scope is intentionally small: load YAML config, apply CLI overrides, choose transport, drain the line, capture a raw meter dump, and store it in a timestamped text file.

## Features (current)

- YAML configuration loading (`config.example.yaml`)
- CLI overrides for common transport settings
- Transport abstraction with:
  - Serial transport
  - TCP transport
- IEC 62056-21 raw capture flow (silence wait + wakeup + identification request + capture until ETX/BCC or timeout)
- Timestamped raw dump files named `metering_{timestamp}.txt`

## Project structure

```text
.
├── main.go
├── config.example.yaml
└── internal
    ├── config
    ├── output
    ├── protocol
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
./meter read -c config.example.yaml
```

Or directly with `go run`:

```bash
go run . read -c config.example.yaml
```

### CLI overrides

```bash
./meter read -c config.example.yaml \
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

## Configuration

Example config:

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
```

## Notes

- `output.format=raw` prints hex bytes for debugging.
- `output.format=ascii` prints direct text output from the meter.
- Protocol engine now sends `/?!\r\n`, reads identification, sends fixed ACK (`\x06000\r\n`), and keeps reading without changing baud.
- Full IEC mode negotiation/parsing is not implemented yet.
- For serial settings, common IEC startup framing like 7E1 is supported via config.

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

- Implement full IEC 62056-21 handshake and mode selection
- Add structured parser + JSON output
- Add reconnect/backoff options for TCP gateways
