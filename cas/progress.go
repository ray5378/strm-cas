package cas

import "io"

type countingReader struct {
	reader io.Reader
	total  int64
	onRead func(total int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 {
		c.total += int64(n)
		if c.onRead != nil {
			c.onRead(c.total)
		}
	}
	return n, err
}

func contentTotal(contentLength, partialSize int64) int64 {
	if contentLength <= 0 {
		return 0
	}
	return partialSize + contentLength
}
