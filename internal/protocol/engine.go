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

const (
	wakeupSequence        = "\r\n"
	identificationRequest = "/?!\r\n"
	etxByte               = 0x03
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) ReadRaw(ctx context.Context, tr transport.Transport, cfg config.IEC62056Config) ([]byte, error) {
	if err := e.waitForSilence(ctx, tr, cfg); err != nil {
		return nil, err
	}

	if cfg.Wakeup {
		if _, err := tr.Write([]byte(wakeupSequence)); err != nil {
			return nil, err
		}

		if err := sleepWithContext(ctx, 200*time.Millisecond); err != nil {
			return nil, err
		}
	}

	if _, err := tr.Write([]byte(identificationRequest)); err != nil {
		return nil, err
	}

	buffer, err := e.captureRawData(ctx, tr, cfg)
	if err != nil {
		return nil, err
	}
	if len(buffer) == 0 {
		return nil, errors.New("no data captured")
	}

	return buffer, nil
}

func (e *Engine) waitForSilence(ctx context.Context, tr transport.Transport, cfg config.IEC62056Config) error {
	tmp := make([]byte, 256)
	start := time.Now()
	lastData := start
	maxWait := time.Duration(cfg.MaxSilenceWaitMs) * time.Millisecond
	silenceDuration := time.Duration(cfg.SilenceDurationMs) * time.Millisecond
	pollTimeout := time.Duration(cfg.InterCharTimeoutMs) * time.Millisecond
	stageDeadline := start.Add(maxWait)

	for {
		if time.Since(lastData) >= silenceDuration {
			return nil
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if time.Now().After(stageDeadline) {
			return errors.New("line did not become silent before max_silence_wait_ms")
		}

		if err := setReadDeadline(ctx, tr, pollTimeout, stageDeadline); err != nil {
			return err
		}

		n, err := tr.Read(tmp)
		if n > 0 {
			lastData = time.Now()
		}

		if err == nil || n > 0 {
			continue
		}

		if errors.Is(err, io.EOF) {
			continue
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			continue
		}

		return err
	}
}

func (e *Engine) captureRawData(ctx context.Context, tr transport.Transport, cfg config.IEC62056Config) ([]byte, error) {
	buffer := make([]byte, 0, 1024)
	tmp := make([]byte, 256)
	start := time.Now()
	lastData := start
	pollTimeout := time.Duration(cfg.InterCharTimeoutMs) * time.Millisecond
	idleGap := time.Duration(cfg.CaptureIdleGapMs) * time.Millisecond
	stageDeadline := start.Add(time.Duration(cfg.CaptureMaxTimeMs) * time.Millisecond)

	for {
		if frame, ok := frameUntilETXBCC(buffer); ok {
			return frame, nil
		}

		if err := ctx.Err(); err != nil {
			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, err
		}

		now := time.Now()
		if now.After(stageDeadline) {
			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, errors.New("capture_max_time_ms reached before any data was captured")
		}

		if len(buffer) > 0 && now.Sub(lastData) >= idleGap {
			return buffer, nil
		}

		if err := setReadDeadline(ctx, tr, pollTimeout, stageDeadline); err != nil {
			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, err
		}

		n, err := tr.Read(tmp)
		if n > 0 {
			buffer = append(buffer, tmp[:n]...)
			lastData = time.Now()
			continue
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(buffer) > 0 {
					return buffer, nil
				}
				return nil, err
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if len(buffer) > 0 && time.Since(lastData) >= idleGap {
					return buffer, nil
				}
				continue
			}

			if len(buffer) > 0 {
				return buffer, nil
			}
			return nil, err
		}

		if len(buffer) > 0 && time.Since(lastData) >= idleGap {
			return buffer, nil
		}
	}
}

func frameUntilETXBCC(buffer []byte) ([]byte, bool) {
	etxPos := bytes.IndexByte(buffer, etxByte)
	if etxPos == -1 || len(buffer) < etxPos+2 {
		return nil, false
	}

	frame := make([]byte, etxPos+2)
	copy(frame, buffer[:etxPos+2])
	return frame, true
}

func setReadDeadline(ctx context.Context, tr transport.Transport, pollTimeout time.Duration, stageDeadline time.Time) error {
	deadline := time.Now().Add(pollTimeout)
	if deadline.After(stageDeadline) {
		deadline = stageDeadline
	}

	if ctxDeadline, ok := ctx.Deadline(); ok && deadline.After(ctxDeadline) {
		deadline = ctxDeadline
	}

	return tr.SetReadDeadline(deadline)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
