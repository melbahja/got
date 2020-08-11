package got

import (
	"sync"
	"time"
)

type (

	// Progress can be used to show download progress to the user.
	Progress struct {
		TotalSize int64
		Size int64
		Speed int64
		AvgSpeed int64
		TotalCost time.Duration

		start time.Time
		lastSize int64
		mu   sync.Mutex
	}

	// ProgressFunc to show progress state, called based on Download interval.
	ProgressFunc func(p *Progress,  d *Download)
)

// Run runs ProgressFunc based on interval if ProgressFunc set.
func (p *Progress) Run(d *Download) {

	if d.ProgressFunc != nil {

		for {

			if d.StopProgress {
				break
			}

			p.CalculateSpeed(int64(d.Interval))

			d.ProgressFunc(p, d)

			time.Sleep(time.Duration(d.Interval) * time.Millisecond)
		}
	}
}

func (p *Progress) CalculateSpeed(interval int64) {
	p.Speed = (p.Size - p.lastSize) / interval * 1000
	p.lastSize = p.Size

	now := time.Now()
	p.TotalCost = now.Sub(p.start)
	totalMills := (now.Sub(p.start)).Milliseconds()
	if totalMills == 0 {
		p.AvgSpeed = 0
	} else {
		p.AvgSpeed = p.Size / totalMills * 1000
	}
}

func (p *Progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(b)
	p.Size += int64(n)
	return n, nil
}
