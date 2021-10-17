package got

import (
	"io"
	"sync"
)

type OffsetWriter struct {
	io.WriterAt
	offset int64
}

func (dst *OffsetWriter) Write(b []byte) (n int, err error) {
	n, err = dst.WriteAt(b, dst.offset)
	dst.offset += int64(n)
	return
}

// Chunk represents the partial content range
type Chunk struct {
	Start, End uint64
}

// ChunkPool helps in multi *Download files.
var ChunkPool = &sync.Pool{
	New: func() interface{} {
		return new(Chunk)
	},
}
