package transport

import (
	"strings"
	"time"

	"optical-probe-reader/internal/config"

	"go.bug.st/serial"
)

type SerialTransport struct {
	port serial.Port
}

func NewSerial(cfg config.Config) (*SerialTransport, error) {
	mode := &serial.Mode{
		BaudRate: cfg.Serial.Baud,
		DataBits: cfg.Serial.DataBits,
		Parity:   serialParity(cfg.Serial.Parity),
		StopBits: serialStopBits(cfg.Serial.StopBits),
	}

	port, err := serial.Open(cfg.Serial.Device, mode)
	if err != nil {
		return nil, err
	}

	if cfg.Serial.ReadTimeoutMs > 0 {
		_ = port.SetReadTimeout(time.Duration(cfg.Serial.ReadTimeoutMs) * time.Millisecond)
	}

	return &SerialTransport{port: port}, nil
}

func (s *SerialTransport) Read(p []byte) (int, error) {
	return s.port.Read(p)
}

func (s *SerialTransport) Write(p []byte) (int, error) {
	return s.port.Write(p)
}

func (s *SerialTransport) SetReadDeadline(deadline time.Time) error {
	if deadline.IsZero() {
		return s.port.SetReadTimeout(serial.NoTimeout)
	}

	remaining := time.Until(deadline)
	if remaining < time.Millisecond {
		remaining = time.Millisecond
	}

	return s.port.SetReadTimeout(remaining)
}

func (s *SerialTransport) Close() error {
	return s.port.Close()
}

func serialParity(parity string) serial.Parity {
	switch strings.ToLower(parity) {
	case "odd":
		return serial.OddParity
	case "none":
		return serial.NoParity
	default:
		return serial.EvenParity
	}
}

func serialStopBits(stopBits int) serial.StopBits {
	if stopBits == 2 {
		return serial.TwoStopBits
	}
	return serial.OneStopBit
}
