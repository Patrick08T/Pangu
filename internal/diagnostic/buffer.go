package diagnostic

import "bytes"

type Buffer struct {
	Data      bytes.Buffer
	Limit     int
	Truncated bool
}

func (b *Buffer) Write(data []byte) (int, error) {
	remaining := max(0, b.Limit-b.Data.Len())
	b.Truncated = b.Truncated || len(data) > remaining
	_, _ = b.Data.Write(data[:min(len(data), remaining)])
	return len(data), nil
}
