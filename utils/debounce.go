package utils

import (
	"sync"
	"time"
)

func NewDebouncer(after time.Duration, f func()) *Debouncer {
	d := &Debouncer{after: after, f: f}

	return d
}

type Debouncer struct {
	mu    sync.Mutex
	after time.Duration
	timer *time.Timer
	f     func()
}

func (d *Debouncer) F() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.after, d.f)
}
