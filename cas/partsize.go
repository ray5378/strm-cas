package cas

import "math"

type Mode string

const (
	Mode189PC Mode = "189pc"
)

func ChunkSize(mode Mode, size int64) int64 {
	switch mode {
	case "", Mode189PC:
		return partSize189PC(size)
	default:
		return partSize189PC(size)
	}
}

func partSize189PC(size int64) int64 {
	const defaultSize = int64(1024 * 1024 * 10) // 10 MiB
	if size > defaultSize*2*999 {
		return int64(math.Max(math.Ceil((float64(size)/1999)/float64(defaultSize)), 5) * float64(defaultSize))
	}
	if size > defaultSize*999 {
		return defaultSize * 2
	}
	return defaultSize
}
