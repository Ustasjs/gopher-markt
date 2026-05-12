package accrual

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/ustasjs/gopher-markt/internal/logger"
	"github.com/ustasjs/gopher-markt/internal/money"
)

const (
	pollInterval = 3 * time.Second
	batchLimit   = 100

	statusProcessing = "PROCESSING"
	statusProcessed  = "PROCESSED"
	statusInvalid    = "INVALID"
)

type orderRepository interface {
	GetPendingOrders(ctx context.Context, limit int) ([]string, error)
	UpdateOrderStatus(ctx context.Context, number, status string, accrual *int64) error
}

type Worker struct {
	repo   orderRepository
	client *Client
}

func NewWorker(repo orderRepository, client *Client) *Worker {
	return &Worker{repo: repo, client: client}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	numbers, err := w.repo.GetPendingOrders(ctx, batchLimit)
	if err != nil {
		logger.Log.Error("accrual worker: get pending orders", zap.Error(err))
		return
	}

	for _, number := range numbers {
		if ctx.Err() != nil {
			return
		}

		resp, status, retryAfter, err := w.client.GetOrder(ctx, number)
		if err != nil {
			logger.Log.Error("accrual worker: get order", zap.String("number", number), zap.Error(err))
			continue
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
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryAfter):
			}
			return // прерываем батч, следующий тик заберёт заказы заново
		default:
			logger.Log.Error("accrual worker: unexpected status",
				zap.String("number", number),
				zap.Int("status", status),
			)
		}
	}
}

func (w *Worker) updateOrder(ctx context.Context, resp *AccrualResponse) error {
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
