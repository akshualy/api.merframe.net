package throttle

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := New(2)
		start := time.Now()

		for range 3 {
			if err := th.Wait(t.Context()); err != nil {
				t.Fatal(err)
			}
		}

		if got := time.Since(start); got != time.Second {
			t.Errorf("three permits at 2/s took %v, want 1s", got)
		}
	})
}

func TestWaitIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := New(1)
		if err := th.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		time.Sleep(10 * time.Second)
		start := time.Now()

		for range 2 {
			if err := th.Wait(t.Context()); err != nil {
				t.Fatal(err)
			}
		}

		if got := time.Since(start); got != time.Second {
			t.Errorf("two permits after an idle gap took %v, want 1s", got)
		}
	})
}

func TestHold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := New(10)
		th.Hold(5 * time.Second)
		th.Hold(time.Second)
		start := time.Now()

		if err := th.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		if got := time.Since(start); got != 5*time.Second {
			t.Errorf("the first permit after a hold came after %v, want 5s", got)
		}
	})
}

func TestEveryAndSleep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := Every(6 * time.Second)
		start := time.Now()

		for range 2 {
			if err := th.Wait(t.Context()); err != nil {
				t.Fatal(err)
			}
		}

		if got := time.Since(start); got != 6*time.Second {
			t.Errorf("two permits spaced 6s took %v, want 6s", got)
		}

		if err := Sleep(t.Context(), -time.Second); err != nil {
			t.Errorf("Sleep with a negative duration returned %v, want nil", err)
		}
	})
}

func TestWaitCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := New(0.1)
		if err := th.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		if err := th.Wait(ctx); !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})
}
