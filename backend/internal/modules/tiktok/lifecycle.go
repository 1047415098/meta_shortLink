package tiktok

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// Start is explicit and idempotent; disabled installations never create a worker.
func (s *Service) Start(parent context.Context) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if !s.Core.Config.TikTokEnabled || s.cancel != nil || s.closed {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.workers.Add(1)
	go s.runWorker(ctx)
}

func (s *Service) runWorker(ctx context.Context) {
	defer s.workers.Done()
	for ctx.Err() == nil {
		worked, err := s.ProcessEvent(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			// Only a coarse category is logged; payloads and visitor identifiers stay encrypted.
			slog.Error("TikTok background task failed", "task", "event_delivery", "category", deliveryErrorCategory(err))
		}
		if worked && err == nil {
			continue
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func deliveryErrorCategory(err error) string {
	var apiError *APIError
	switch {
	case errors.As(err, &apiError):
		return "api"
	case errors.Is(err, errEventLeaseLost):
		return "lease"
	case errors.Is(err, pgx.ErrNoRows):
		return "configuration"
	default:
		return "storage"
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
