package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ustasjs/gopher-markt/internal/model"
)

type mockRepo struct {
	getPendingOrdersFn  func(ctx context.Context, limit int) ([]string, error)
	updateOrderStatusFn func(ctx context.Context, number, status string, accrual *int64) error
}

func (m *mockRepo) GetPendingOrders(ctx context.Context, limit int) ([]string, error) {
	return m.getPendingOrdersFn(ctx, limit)
}
func (m *mockRepo) UpdateOrderStatus(ctx context.Context, number, status string, accrual *int64) error {
	return m.updateOrderStatusFn(ctx, number, status, accrual)
}
func (m *mockRepo) CreateUser(ctx context.Context, login, passwordHash string) (string, error) {
	return "", nil
}
func (m *mockRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	return nil, nil
}
func (m *mockRepo) CreateOrder(ctx context.Context, userID, number string) error { return nil }
func (m *mockRepo) GetOrderByNumber(ctx context.Context, number string) (*model.Order, error) {
	return nil, nil
}
func (m *mockRepo) GetOrdersByUserID(ctx context.Context, userID string) ([]model.Order, error) {
	return nil, nil
}
func (m *mockRepo) GetBalance(ctx context.Context, userID string) (int64, int64, error) {
	return 0, 0, nil
}
func (m *mockRepo) CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum int64) error {
	return nil
}
func (m *mockRepo) GetWithdrawalsByUserID(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	return nil, nil
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		accrualStatus string
		wantStatus    string
		wantUpdate    bool
	}{
		{"PROCESSING", "PROCESSING", true},
		{"PROCESSED", "PROCESSED", true},
		{"INVALID", "INVALID", true},
		{"REGISTERED", "", false},
		{"UNKNOWN", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.accrualStatus, func(t *testing.T) {
			got, shouldUpdate := mapStatus(tt.accrualStatus)
			if got != tt.wantStatus {
				t.Errorf("mapStatus(%q) status = %q, want %q", tt.accrualStatus, got, tt.wantStatus)
			}
			if shouldUpdate != tt.wantUpdate {
				t.Errorf("mapStatus(%q) shouldUpdate = %v, want %v", tt.accrualStatus, shouldUpdate, tt.wantUpdate)
			}
		})
	}
}

func TestProcessBatch_NoPendingOrders(t *testing.T) {
	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			t.Error("UpdateOrderStatus should not be called when no pending orders")
			return nil
		},
	}

	worker := NewWorker(repo, NewClient("localhost:3000"))
	worker.processBatch(context.Background())
}

func TestProcessBatch_ProcessedOrder(t *testing.T) {
	accrualVal := 500.5
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccrualResponse{
			Order:   "12345678903",
			Status:  "PROCESSED",
			Accrual: &accrualVal,
		})
	}))
	defer server.Close()

	var updatedNumber, updatedStatus string
	var updatedAccrual *int64

	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"12345678903"}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			updatedNumber = number
			updatedStatus = status
			updatedAccrual = accrual
			return nil
		},
	}

	worker := NewWorker(repo, NewClient(server.URL))
	worker.processBatch(context.Background())

	if updatedNumber != "12345678903" {
		t.Errorf("updated number = %q, want %q", updatedNumber, "12345678903")
	}
	if updatedStatus != "PROCESSED" {
		t.Errorf("updated status = %q, want %q", updatedStatus, "PROCESSED")
	}
	if updatedAccrual == nil {
		t.Fatal("expected accrual to be set, got nil")
	}
	if *updatedAccrual != 50050 { // 500.5 рублей = 50050 копеек
		t.Errorf("updated accrual = %d kopecks, want 50050", *updatedAccrual)
	}
}

func TestProcessBatch_RegisteredOrder_NoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccrualResponse{Order: "12345678903", Status: "REGISTERED"})
	}))
	defer server.Close()

	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"12345678903"}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			t.Error("UpdateOrderStatus should not be called for REGISTERED status")
			return nil
		},
	}

	worker := NewWorker(repo, NewClient(server.URL))
	worker.processBatch(context.Background())
}

func TestProcessBatch_NoContent_NoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"12345678903"}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			t.Error("UpdateOrderStatus should not be called on 204")
			return nil
		},
	}

	worker := NewWorker(repo, NewClient(server.URL))
	worker.processBatch(context.Background())
}

func TestProcessBatch_RateLimit_StopsBatch(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"111", "222", "333"}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			t.Error("UpdateOrderStatus should not be called after 429")
			return nil
		},
	}

	worker := NewWorker(repo, NewClient(server.URL))

	// Контекст с таймаутом — разблокирует select после 429
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	worker.processBatch(ctx)

	// Должен отправить ровно 1 запрос и остановиться
	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Errorf("expected 1 request before rate limit stop, got %d", got)
	}
}

func TestProcessBatch_InvalidStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccrualResponse{Order: "12345678903", Status: "INVALID"})
	}))
	defer server.Close()

	var updatedStatus string
	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"12345678903"}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			updatedStatus = status
			return nil
		},
	}

	worker := NewWorker(repo, NewClient(server.URL))
	worker.processBatch(context.Background())

	if updatedStatus != "INVALID" {
		t.Errorf("updated status = %q, want %q", updatedStatus, "INVALID")
	}
}
