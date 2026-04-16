package protocol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"

	"optical-probe-reader/internal/config"
	"optical-probe-reader/internal/transport"
)

type Engine struct{}

type ReadStatus int

const (
	ReadStatusComplete ReadStatus = iota
	ReadStatusPartialTimeout
	ReadStatusPartialNoTerminator
)

const (
	identificationRequest = "/?!\r\n"
	etxByte               = byte(0x03)
	stxByte               = byte(0x02)
)

func NewEngine() *Engine {
	return &Engine{}
}

func (s ReadStatus) String() string {
	switch s {
	case ReadStatusComplete:
		return "etx_bcc_reached"
	case ReadStatusPartialTimeout:
		return "timeout_before_etx_bcc"
	case ReadStatusPartialNoTerminator:
		return "partial_without_etx_bcc"
	default:
		return "unknown"
	}
}

func (e *Engine) ReadRaw(ctx context.Context, tr transport.Transport, cfg config.IEC62056Config) ([]byte, ReadStatus, error) {
	if cfg.Wakeup {
		_, _ = tr.Write([]byte("\r\n"))
		time.Sleep(200 * time.Millisecond)
	}

	if _, err := tr.Write([]byte(identificationRequest)); err != nil {
		return nil, ReadStatusPartialNoTerminator, err
	}

	interCharTimeout := time.Duration(cfg.InterCharTimeoutMs) * time.Millisecond
	payload, reachedTerminator, timedOut, err := readUntilSilence(ctx, tr, interCharTimeout)
	if err != nil {
		if len(payload) > 0 {
			if timedOut {
				return payload, ReadStatusPartialTimeout, nil
			}
			return payload, ReadStatusPartialNoTerminator, nil
		}
		return nil, ReadStatusPartialNoTerminator, err
	}

	if reachedTerminator {
		return payload, ReadStatusComplete, nil
	}
	if timedOut {
		return payload, ReadStatusPartialTimeout, nil
	}
	return payload, ReadStatusPartialNoTerminator, nil
}

func readUntilSilence(ctx context.Context, tr transport.Transport, interCharTimeout time.Duration) ([]byte, bool, bool, error) {
	buffer := make([]byte, 0, 1024)
	tmp := make([]byte, 256)

	for {
		select {
		case <-ctx.Done():
			if end, ok := findETXBCCEnd(buffer); ok {
				frame := append([]byte(nil), buffer[:end]...)
				return frame, true, false, nil
			}
			if len(buffer) > 0 {
				return buffer, false, true, nil
			}
			return nil, false, true, ctx.Err()
		default:
		}

		_ = tr.SetReadDeadline(time.Now().Add(interCharTimeout))
		n, err := tr.Read(tmp)

		if n > 0 {
			buffer = append(buffer, tmp[:n]...)
			if end, ok := findETXBCCEnd(buffer); ok {
				frame := append([]byte(nil), buffer[:end]...)
				return frame, true, false, nil
			}
			continue
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return buffer, false, false, nil
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if end, ok := findETXBCCEnd(buffer); ok {
					frame := append([]byte(nil), buffer[:end]...)
					return frame, true, false, nil
				}
				continue
			}

			if len(buffer) > 0 {
				return buffer, false, false, nil
			}
			return nil, false, false, err
		}

		if len(buffer) > 0 {
			return buffer, false, false, nil
		}
	}
}

func findETXBCCEnd(buffer []byte) (int, bool) {
	// Prefer IEC text-mode end marker: !\r\n ETX BCC
	if idx := bytes.Index(buffer, []byte{'!', '\r', '\n', etxByte}); idx >= 0 {
		bccPos := idx + 4
		if bccPos < len(buffer) {
			return bccPos + 1, true
		}
	}

	for i := 0; i+1 < len(buffer); i++ {
		if buffer[i] != etxByte {
			continue
		}

		// Validate BCC against STX..ETX only when the frame actually starts with STX.
		if stxPos := bytes.IndexByte(buffer[:i], stxByte); stxPos == 0 {
			expected := xorBytes(buffer[stxPos : i+1])
			if expected != buffer[i+1] {
				continue
			}
		}

		return i + 2, true
	}

	return 0, false
}

func xorBytes(data []byte) byte {
	var bcc byte
	for _, b := range data {
		bcc ^= b
	}
	return bcc
}
