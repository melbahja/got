package got

import (
	"time"
	"sync"
)

type (

	// Download progress.
	Progress struct {
		Size int64
		mu sync.RWMutex
	}

	// Progress report func.
	ProgressFunc func(size int64, total int64, d *Download)
)

func (p *Progress) Run(d *Download) {

	if d.ProgressFunc != nil {

		for {

			if d.StopProgress {
				break
			}

			d.ProgressFunc(p.Size, d.Info.Length, d)

			time.Sleep(time.Duration(d.Interval) * time.Millisecond)
		}
	}
}

func (p *Progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(b)
	p.Size += int64(n)
	return n, nil
}
