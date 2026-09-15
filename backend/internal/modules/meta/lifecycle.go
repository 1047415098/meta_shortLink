package meta

import (
	"context"
	"log/slog"
	"time"
)

// Start is explicit: constructing an app for a request or test never sends events.
func (s *Service) Start(parent context.Context) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.cancel != nil || s.closed {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	// Only CAPI delivery runs in the background; Insights synchronization was removed.
	s.workers.Add(1)
	go s.runWorker(ctx, "consultation", s.ProcessEvent, 2*time.Second)
}

func (s *Service) runWorker(ctx context.Context, name string, run func(context.Context) (bool, error), idle time.Duration) {
	defer s.workers.Done()
	for ctx.Err() == nil {
		worked, err := run(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			slog.Error("Meta background task failed", "task", name, "error", cleanMessage(err.Error()))
		}
		if worked && err == nil {
			continue
		}
		timer := time.NewTimer(idle)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Service) Close() {
	s.lifecycle.Lock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.lifecycle.Unlock()
	s.workers.Wait()
}
