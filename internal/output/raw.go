package output

import "io"

func WriteRaw(writer io.Writer, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	_, err := writer.Write(data)
	return err
}
