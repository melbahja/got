package got

import (
	"context"
	"sync/atomic"
	"time"
)

type (

	// Progress can be used to show download progress to the user.
	Progress struct {
		Size int64
	}

	// ProgressFunc to show progress state, called based on Download interval.
	ProgressFunc func(size int64, total int64, d *Download)
)

// Run runs ProgressFunc based on interval if ProgressFunc set.
func (p *Progress) Run(ctx context.Context, d *Download) {
	if d.ProgressFunc != nil {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			d.ProgressFunc(atomic.LoadInt64(&p.Size), d.Info.Length, d)

			time.Sleep(time.Duration(d.Interval) * time.Millisecond)
		}
	}
}

func (p *Progress) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddInt64(&p.Size, int64(n))
	return n, nil
}
