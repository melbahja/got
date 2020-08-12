package got

import (
	"context"
	"sync/atomic"
	"time"
)

type (

	// Progress can be used to show download progress to the user.
	Progress struct {
		ProgressFunc

		Size, TotalSize uint64
		Interval        int

		lastSize  uint64
		startedAt time.Time
	}

	// ProgressFunc to show progress state, called based on Download interval.
	ProgressFunc func(p *Progress, d *Download)
)

// Run runs ProgressFunc based on interval if ProgressFunc set.
func (p *Progress) Run(ctx context.Context, d *Download) {

	if p.ProgressFunc != nil {

		for {
			// Context cancelled
			if ctx.Err() != nil {
				return
			}

			// Run progress func.
			p.ProgressFunc(p, d)

			// Update last size
			atomic.StoreUint64(&p.lastSize, atomic.LoadUint64(&p.Size))

			time.Sleep(time.Duration(d.Interval) * time.Millisecond)
		}
	}
}

// Speed returns download speed.
func (p *Progress) Speed() uint64 {
	return uint64((atomic.LoadUint64(&p.Size) - atomic.LoadUint64(&p.lastSize)) / uint64(p.Interval) * 1000)
}

// AvgSpeed returns average download speed.
func (p *Progress) AvgSpeed() uint64 {

	if totalMills := p.TotalCost().Milliseconds(); totalMills > 0 {
		return uint64(atomic.LoadUint64(&p.Size) / uint64(totalMills) * 1000)
	}

	return 0
}

// TotalCost returns download duration.
func (p *Progress) TotalCost() time.Duration {
	return time.Now().Sub(p.startedAt)
}

// Write updates progress size.
func (p *Progress) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddUint64(&p.Size, uint64(n))
	return n, nil
}
