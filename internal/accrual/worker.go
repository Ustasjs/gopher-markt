package accrual

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ustasjs/gopher-markt/internal/logger"
	"github.com/ustasjs/gopher-markt/internal/money"
)

const (
	pollInterval = 3 * time.Second
	batchLimit   = 100
	numWorkers   = 5

	statusProcessing = "PROCESSING"
	statusProcessed  = "PROCESSED"
	statusInvalid    = "INVALID"
)

type orderRepository interface {
	GetPendingOrders(ctx context.Context, limit int) ([]string, error)
	UpdateOrderStatus(ctx context.Context, number, status string, accrual *int64) error
}

type WorkerPool struct {
	repo         orderRepository
	client       *Client
	workers      int
	pollInterval time.Duration

	pauseMu    sync.Mutex
	pauseUntil time.Time
}

func NewWorkerPool(repo orderRepository, client *Client) *WorkerPool {
	return &WorkerPool{
		repo:         repo,
		client:       client,
		workers:      numWorkers,
		pollInterval: pollInterval,
	}
}

func (w *WorkerPool) Run(ctx context.Context) {
	jobs := make(chan string)
	var wg sync.WaitGroup

	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.workerLoop(ctx, jobs)
		}()
	}

	w.dispatch(ctx, jobs)
	close(jobs)
	wg.Wait()
}

func (w *WorkerPool) dispatch(ctx context.Context, jobs chan<- string) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.waitPause(ctx); err != nil {
				return
			}
			numbers, err := w.repo.GetPendingOrders(ctx, batchLimit)
			if err != nil {
				logger.Log.Error("accrual worker: get pending orders", zap.Error(err))
				continue
			}
			for _, number := range numbers {
				select {
				case <-ctx.Done():
					return
				case jobs <- number:
				}
			}
		}
	}
}

func (w *WorkerPool) workerLoop(ctx context.Context, jobs <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case number, ok := <-jobs:
			if !ok {
				return
			}
			if err := w.waitPause(ctx); err != nil {
				return
			}
			w.process(ctx, number)
		}
	}
}

func (w *WorkerPool) process(ctx context.Context, number string) {
	resp, status, retryAfter, err := w.client.GetOrder(ctx, number)
	if err != nil {
		logger.Log.Error("accrual worker: get order", zap.String("number", number), zap.Error(err))
		return
	}

	switch status {
	case http.StatusOK:
		if err := w.updateOrder(ctx, resp); err != nil {
			logger.Log.Error("accrual worker: update order", zap.String("number", number), zap.Error(err))
		}
	case http.StatusNoContent:
		// заказ не зарегистрирован в accrual — оставляем NEW
	case http.StatusTooManyRequests:
		logger.Log.Info("accrual worker: rate limited", zap.Duration("retry_after", retryAfter))
		w.setPause(retryAfter)
	default:
		logger.Log.Error("accrual worker: unexpected status",
			zap.String("number", number),
			zap.Int("status", status),
		)
	}
}

func (w *WorkerPool) waitPause(ctx context.Context) error {
	w.pauseMu.Lock()
	until := w.pauseUntil
	w.pauseMu.Unlock()
	d := time.Until(until)
	if d <= 0 {
		return nil
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

func (w *WorkerPool) setPause(d time.Duration) {
	if d <= 0 {
		return
	}
	w.pauseMu.Lock()
	defer w.pauseMu.Unlock()
	until := time.Now().Add(d)
	if until.After(w.pauseUntil) {
		w.pauseUntil = until
	}
}

func (w *WorkerPool) updateOrder(ctx context.Context, resp *AccrualResponse) error {
	newStatus, shouldUpdate := mapStatus(resp.Status)
	if !shouldUpdate {
		return nil
	}

	var accrualKopecks *int64
	if resp.Accrual != nil {
		v := money.ToKopecks(*resp.Accrual)
		accrualKopecks = &v
	}

	return w.repo.UpdateOrderStatus(ctx, resp.Order, newStatus, accrualKopecks)
}

func mapStatus(accrualStatus string) (string, bool) {
	switch accrualStatus {
	case "PROCESSING":
		return statusProcessing, true
	case "PROCESSED":
		return statusProcessed, true
	case "INVALID":
		return statusInvalid, true
	default: // REGISTERED
		return "", false
	}
}
