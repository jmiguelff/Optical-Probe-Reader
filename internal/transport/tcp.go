package transport

import (
	"net"
	"time"

	"optical-probe-reader/internal/config"
)

type TCPTransport struct {
	conn net.Conn
}

func NewTCP(cfg config.Config) (*TCPTransport, error) {
	dialer := net.Dialer{
		Timeout: time.Duration(cfg.TCP.ConnectTimeoutMs) * time.Millisecond,
	}

	conn, err := dialer.Dial("tcp", cfg.TCP.Address)
	if err != nil {
		return nil, err
	}

	if cfg.TCP.ReadTimeoutMs > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(time.Duration(cfg.TCP.ReadTimeoutMs) * time.Millisecond))
	}

	return &TCPTransport{conn: conn}, nil
}

func (t *TCPTransport) Read(p []byte) (int, error) {
	return t.conn.Read(p)
}

func (t *TCPTransport) Write(p []byte) (int, error) {
	return t.conn.Write(p)
}

func (t *TCPTransport) SetReadDeadline(deadline time.Time) error {
	return t.conn.SetReadDeadline(deadline)
}

func (t *TCPTransport) Close() error {
	return t.conn.Close()
}
