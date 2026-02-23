# Optical Probe Reader

Minimal Go CLI to read meter data over IEC 62056-21 using either:

- RS232/USB serial
- TCP (serial-to-ethernet gateway)

Current scope is intentionally small: load YAML config, apply CLI overrides, choose transport, run protocol stub, and print raw output.

## Features (current)

- YAML configuration loading (`config.example.yaml`)
- CLI overrides for common transport settings
- Transport abstraction with:
  - Serial transport
  - TCP transport
- IEC 62056-21 protocol stub (wakeup + identification request + read loop)
- Raw output mode

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
  --output=raw
```

Supported flags on `read`:

- `-c` path to YAML config
- `--transport=serial|tcp`
- `--serial=/dev/ttyUSB0`
- `--baud=300`
- `--addr=host:port`
- `--connect-timeout-ms=...`
- `--read-timeout-ms=...`
- `--output=raw`

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
  overall_timeout_ms: 8000

output:
  format: "raw"
  pretty: true
```

## Notes

- Output formats other than `raw` are not implemented yet.
- Protocol engine is a stub and does not yet perform full IEC mode negotiation/parsing.
- For serial settings, common IEC startup framing like 7E1 is supported via config.

## Next steps

- Implement full IEC 62056-21 handshake and mode selection
- Add structured parser + JSON output
- Add reconnect/backoff options for TCP gateways
