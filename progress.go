package got

import (
	"time"
)

type (

	// Download progress.
	Progress struct {
		Length int64
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

			d.ProgressFunc(p.Length, d.Info.Length, d)

			time.Sleep(time.Duration(d.Interval) * time.Millisecond)
		}
	}
}

func (p *Progress) Write(b []byte) (int, error) {
	n := len(b)
	p.Length += int64(n)
	return n, nil
}
