package output

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func WriteRawFile(dir string, data []byte, now time.Time) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("output directory is required")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("cannot write empty raw dump")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("metering_%s.txt", now.Format("20060102_150405_000"))
	filePath := filepath.Join(dir, fileName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}

	if err := file.Close(); err != nil {
		return "", err
	}

	return filePath, nil
}
