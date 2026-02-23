package transport

import (
	"strings"
	"time"

	"optical-probe-reader/internal/config"
)

type Transport interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	SetReadDeadline(t time.Time) error
	Close() error
}

func New(cfg config.Config) (Transport, error) {
	if strings.EqualFold(cfg.Transport.Type, "tcp") {
		return NewTCP(cfg)
	}
	return NewSerial(cfg)
}
