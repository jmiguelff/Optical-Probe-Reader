# Project State

## Overview

Optical Probe Reader is a Go CLI that connects to a meter over serial or TCP and captures a raw IEC 62056-21 data dump.

Current behavior is focused on reliable raw capture and file persistence, not full protocol parsing.

## Current Scope

Implemented:

- YAML configuration loading with CLI overrides.
- Transport abstraction with serial and TCP backends.
- Raw protocol flow:
  - wait for line silence,
  - optional wakeup write,
  - send identification request `/?!\r\n`,
  - capture until ETX plus BCC, or until idle/timeout ends capture.
- Raw dump persistence to timestamped files named `metering_{timestamp}.txt` in a configured output directory.
- Linux ARMv7 cross-compile instructions for Raspberry Pi.

## Key Files

- main entry and command handling: `main.go`
- config model/defaults/validation: `internal/config/config.go`
- protocol engine and capture flow: `internal/protocol/engine.go`
- transport abstraction and implementations:
  - `internal/transport/transport.go`
  - `internal/transport/serial.go`
  - `internal/transport/tcp.go`
- raw output writer: `internal/output/raw.go`
- user documentation and usage: `README.md`

## Configuration Status

Important IEC timing fields currently available:

- `inter_char_timeout_ms`
- `silence_duration_ms`
- `max_silence_wait_ms`
- `capture_idle_gap_ms`
- `capture_max_time_ms`
- `overall_timeout_ms`

Output settings currently used:

- `output.format` (raw only)
- `output.directory` (target directory for capture files)

## What Is Not Implemented Yet

- Full IEC 62056-21 handshake/mode negotiation.
- Structured parsing of meter payload (for example OBIS extraction).
- Alternate output formats (JSON/CSV/etc).
- Debug flags for detailed drain/capture logging.
- Automated tests for drain/capture edge cases.

## Known Behavior Notes

- If silence cannot be achieved within `max_silence_wait_ms`, the read fails.
- Capture returns partial data if the idle gap/timeout is reached after data has started.
- A successful run writes a file; raw bytes are not streamed to stdout.

## Build Status

- Local native build succeeds.
- Cross-compile for Raspberry Pi ARMv7 succeeds with:

```bash
GOOS=linux GOARCH=arm GOARM=7 go build -o meter-rpi .
```

## Suggested Next Milestones

1. Add debug logging flags for protocol drain and capture stages.
2. Add tests for silence wait, ETX/BCC detection, and partial capture behavior.
3. Implement protocol-level validation/parsing on top of the raw frame.
4. Add structured output mode while preserving raw dump support.
