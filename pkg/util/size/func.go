package size

import "fmt"

func ReadSummary(data []byte, size int) string {
	if len(data) < size {
		return string(data)
	}
	return fmt.Sprintf("%s ...%d", data[:size], len(data)-size)
}
