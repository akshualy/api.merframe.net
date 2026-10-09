package throttle

import (
	"context"
	"sync"
	"time"
)

type Throttle struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func New(perSecond float64) *Throttle {
	return Every(time.Duration(float64(time.Second) / perSecond))
}

func Every(interval time.Duration) *Throttle {
	return &Throttle{interval: interval}
}

func (t *Throttle) Wait(ctx context.Context) error {
	t.mu.Lock()

	due := t.next
	if now := time.Now(); due.Before(now) {
		due = now
	}

	t.next = due.Add(t.interval)
	t.mu.Unlock()
	return Sleep(ctx, time.Until(due))
}

// Hold pushes every pending and future permit back by at least d.
func (t *Throttle) Hold(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if resume := time.Now().Add(d); resume.After(t.next) {
		t.next = resume
	}
}

func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
