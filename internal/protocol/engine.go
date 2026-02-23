package protocol

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"optical-probe-reader/internal/config"
	"optical-probe-reader/internal/transport"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) ReadRaw(ctx context.Context, tr transport.Transport, cfg config.IEC62056Config) ([]byte, error) {
	if cfg.Wakeup {
		_, _ = tr.Write([]byte("\r\n"))
		time.Sleep(200 * time.Millisecond)
	}

	if _, err := tr.Write([]byte("/?!\r\n")); err != nil {
		return nil, err
	}

	buffer := make([]byte, 0, 1024)
	tmp := make([]byte, 256)
	interCharTimeout := time.Duration(cfg.InterCharTimeoutMs) * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, ctx.Err()
		default:
		}

		_ = tr.SetReadDeadline(time.Now().Add(interCharTimeout))
		n, err := tr.Read(tmp)

		if n > 0 {
			buffer = append(buffer, tmp[:n]...)
			continue
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return buffer, nil
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if len(buffer) > 0 {
					return buffer, nil
				}
				continue
			}

			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, err
		}

		if len(buffer) > 0 {
			return buffer, nil
		}
	}
}
