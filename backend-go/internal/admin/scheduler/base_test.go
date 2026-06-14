package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestBaseSchedulerNextRunWithoutStartupDelay(t *testing.T) {
	// When StartupDelay=0, nextRun should be now + interval
	interval := 2 * time.Second
	beforeStart := time.Now()

	s := New(Config{
		Name:     "test",
		Interval: interval,
		Job: func(ctx context.Context) (*JobResult, error) {
			return &JobResult{Summary: "ok"}, nil
		},
	})

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	nextRun := s.getNextRun()
	if nextRun == nil {
		t.Fatal("nextRun is nil after Start()")
	}

	elapsed := nextRun.Sub(beforeStart)
	if elapsed < interval-time.Second || elapsed > interval+time.Second {
		t.Errorf("nextRun should be ~%v from start, got %v (diff: %v)", interval, elapsed, nextRun.Sub(beforeStart))
	}
}

func TestBaseSchedulerNextRunWithStartupDelay(t *testing.T) {
	startupDelay := 5 * time.Second
	interval := 60 * time.Second
	beforeStart := time.Now()

	s := New(Config{
		Name:         "test",
		Interval:     interval,
		StartupDelay: startupDelay,
		Job: func(ctx context.Context) (*JobResult, error) {
			return &JobResult{Summary: "ok"}, nil
		},
	})

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	nextRun := s.getNextRun()
	if nextRun == nil {
		t.Fatal("nextRun is nil after Start()")
	}

	elapsed := nextRun.Sub(beforeStart)
	if elapsed < startupDelay-time.Second || elapsed > startupDelay+time.Second {
		t.Errorf("nextRun should be ~%v from start, got %v (diff: %v)", startupDelay, elapsed, nextRun.Sub(beforeStart))
	}
}

func TestBaseSchedulerNextRunCallback(t *testing.T) {
	targetDelay := 100 * time.Millisecond
	var executed int32

	s := New(Config{
		Name: "test-nextrun",
		NextRun: func(now time.Time) time.Time {
			return now.Add(targetDelay)
		},
		Job: func(ctx context.Context) (*JobResult, error) {
			atomic.AddInt32(&executed, 1)
			return &JobResult{Summary: "ok"}, nil
		},
	})

	beforeStart := time.Now()
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	nextRun := s.getNextRun()
	if nextRun == nil {
		t.Fatal("nextRun is nil after Start()")
	}

	elapsed := nextRun.Sub(beforeStart)
	if elapsed < 0 || elapsed > targetDelay+50*time.Millisecond {
		t.Errorf("nextRun should be ~%v from start, got %v", targetDelay, elapsed)
	}

	// Wait for at least one execution
	time.Sleep(targetDelay + 200*time.Millisecond)
	if atomic.LoadInt32(&executed) < 1 {
		t.Error("expected at least one execution in NextRun mode")
	}
}

// getNextRun returns the nextRun value (for testing).
func (s *BaseScheduler) getNextRun() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextRun
}
