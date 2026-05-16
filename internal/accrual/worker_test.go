package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestProcess_ProcessedOrder(t *testing.T) {
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
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			updatedNumber = number
			updatedStatus = status
			updatedAccrual = accrual
			return nil
		},
	}

	worker := NewWorkerPool(repo, NewClient(server.URL))
	worker.process(context.Background(), "12345678903")

	if updatedNumber != "12345678903" {
		t.Errorf("updated number = %q, want %q", updatedNumber, "12345678903")
	}
	if updatedStatus != "PROCESSED" {
		t.Errorf("updated status = %q, want %q", updatedStatus, "PROCESSED")
	}
	if updatedAccrual == nil || *updatedAccrual != 50050 {
		t.Errorf("updated accrual = %v, want 50050 kopecks", updatedAccrual)
	}
}

func TestProcess_RegisteredOrder_NoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccrualResponse{Order: "12345678903", Status: "REGISTERED"})
	}))
	defer server.Close()

	repo := &mockRepo{
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			t.Error("UpdateOrderStatus should not be called for REGISTERED status")
			return nil
		},
	}

	worker := NewWorkerPool(repo, NewClient(server.URL))
	worker.process(context.Background(), "12345678903")
}

func TestProcess_NoContent_NoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	repo := &mockRepo{
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			t.Error("UpdateOrderStatus should not be called on 204")
			return nil
		},
	}

	worker := NewWorkerPool(repo, NewClient(server.URL))
	worker.process(context.Background(), "12345678903")
}

func TestProcess_InvalidStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccrualResponse{Order: "12345678903", Status: "INVALID"})
	}))
	defer server.Close()

	var updatedStatus string
	repo := &mockRepo{
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			updatedStatus = status
			return nil
		},
	}

	worker := NewWorkerPool(repo, NewClient(server.URL))
	worker.process(context.Background(), "12345678903")

	if updatedStatus != "INVALID" {
		t.Errorf("updated status = %q, want %q", updatedStatus, "INVALID")
	}
}

func TestProcess_429_SetsPause(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	worker := NewWorkerPool(&mockRepo{}, NewClient(server.URL))
	before := time.Now()
	worker.process(context.Background(), "12345678903")

	worker.pauseMu.Lock()
	until := worker.pauseUntil
	worker.pauseMu.Unlock()

	gotPause := until.Sub(before)
	// Допуск 1s в обе стороны — 30s Retry-After.
	if gotPause < 29*time.Second || gotPause > 31*time.Second {
		t.Errorf("pauseUntil offset = %v, want ~30s", gotPause)
	}
}

func TestWaitPause_BlocksThenReleases(t *testing.T) {
	worker := &WorkerPool{}
	worker.setPause(20 * time.Millisecond)

	start := time.Now()
	if err := worker.waitPause(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 20*time.Millisecond {
		t.Errorf("waitPause returned after %v, want >= 20ms", elapsed)
	}
}

func TestWaitPause_RespectsContextCancel(t *testing.T) {
	worker := &WorkerPool{}
	worker.setPause(time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := worker.waitPause(ctx)
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("waitPause didn't return promptly after cancel: %v", elapsed)
	}
}

func TestSetPause_TakesMax(t *testing.T) {
	worker := &WorkerPool{}
	worker.setPause(50 * time.Millisecond)
	first := worker.pauseUntil
	worker.setPause(10 * time.Millisecond) // короче — не должна сократить
	if !worker.pauseUntil.Equal(first) {
		t.Errorf("setPause shortened the pause: %v -> %v", first, worker.pauseUntil)
	}
	worker.setPause(200 * time.Millisecond) // длиннее — должна удлинить
	if !worker.pauseUntil.After(first) {
		t.Errorf("setPause did not extend the pause: %v -> %v", first, worker.pauseUntil)
	}
}

func TestRun_GracefulShutdown_NoOrders(t *testing.T) {
	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return nil, nil
		},
	}
	worker := &WorkerPool{
		repo:         repo,
		client:       NewClient("localhost:0"),
		workers:      3,
		pollInterval: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	// Даём пулу поработать, затем отменяем.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("Run did not return within 1s after ctx cancel")
	}
}

func TestRun_ProcessesPendingOrders(t *testing.T) {
	accrualVal := 100.0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccrualResponse{
			Order:   r.URL.Path[len("/api/orders/"):],
			Status:  "PROCESSED",
			Accrual: &accrualVal,
		})
	}))
	defer server.Close()

	var fetched atomic.Bool
	var updated int32
	updateDone := make(chan struct{}, 3)

	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			if fetched.CompareAndSwap(false, true) {
				return []string{"111", "222", "333"}, nil
			}
			return nil, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			atomic.AddInt32(&updated, 1)
			updateDone <- struct{}{}
			return nil
		},
	}

	worker := &WorkerPool{
		repo:         repo,
		client:       NewClient(server.URL),
		workers:      3,
		pollInterval: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	for i := 0; i < 3; i++ {
		select {
		case <-updateDone:
		case <-time.After(time.Second):
			t.Fatalf("update %d not received within 1s", i+1)
		}
	}

	cancel()
	<-done

	if got := atomic.LoadInt32(&updated); got != 3 {
		t.Errorf("updates = %d, want 3", got)
	}
}

func TestRun_429_PausesAllWorkersThenResumes(t *testing.T) {
	// Окно паузы — в течение этого времени любой запрос получит 429 с Retry-After: 1.
	// После окна — 200. Так гарантируем, что все воркеры успеют попасть в паузу
	// до того, как кто-то получит успешный ответ.
	const pauseWindow = 500 * time.Millisecond

	var (
		mu        sync.Mutex
		firstCall time.Time
		okCall    time.Time
	)
	startedAt := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		mu.Lock()
		if firstCall.IsZero() {
			firstCall = now
		}
		mu.Unlock()

		if time.Since(startedAt) < pauseWindow {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		mu.Lock()
		if okCall.IsZero() {
			okCall = now
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		accrualVal := 1.0
		json.NewEncoder(w).Encode(AccrualResponse{
			Order:   r.URL.Path[len("/api/orders/"):],
			Status:  "PROCESSED",
			Accrual: &accrualVal,
		})
	}))
	defer server.Close()

	// Бесконечный поток одинаковых заказов.
	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			return []string{"111", "222", "333"}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			return nil
		},
	}

	worker := &WorkerPool{
		repo:         repo,
		client:       NewClient(server.URL),
		workers:      3,
		pollInterval: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	// Ждём, пока сервер ответит первым 200 — это значит, пауза в воркере истекла
	// и воркеры возобновили работу.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := !okCall.IsZero()
		mu.Unlock()
		if got {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	mu.Lock()
	first := firstCall
	ok := okCall
	mu.Unlock()

	if ok.IsZero() {
		t.Fatal("server did not see a successful request — pool never resumed")
	}
	// Между первым запросом и первым успешным должно пройти >= 1s (Retry-After).
	if gap := ok.Sub(first); gap < time.Second {
		t.Errorf("gap between first request and first 200 = %v, want >= 1s (Retry-After respected)", gap)
	}
}

func TestRun_429_NoMoreThanOneExtraRequestPerWorker(t *testing.T) {
	// Сервер всегда отвечает 429. Если пауза работает корректно, после первого 429
	// каждый воркер ещё отправит максимум 1 in-flight запрос, и далее они уснут.
	// Итого: не более workers запросов после первого 429 в течение паузы.
	var reqCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.Header().Set("Retry-After", "10") // долгая пауза — на время теста не дойдём до неё
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	var fetched atomic.Bool
	repo := &mockRepo{
		getPendingOrdersFn: func(ctx context.Context, limit int) ([]string, error) {
			if fetched.CompareAndSwap(false, true) {
				// 50 заказов на 3 воркера — если паузы нет, все 50 уйдут.
				numbers := make([]string, 50)
				for i := range numbers {
					numbers[i] = "order"
				}
				return numbers, nil
			}
			return nil, nil
		},
		updateOrderStatusFn: func(ctx context.Context, number, status string, accrual *int64) error {
			return nil
		},
	}

	const workers = 3
	worker := &WorkerPool{
		repo:         repo,
		client:       NewClient(server.URL),
		workers:      workers,
		pollInterval: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	// Даём время на: 1) первый 429 → setPause; 2) другие воркеры могут отправить
	// по одному запросу; 3) все уходят в паузу.
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	// Допустимая верхняя граница: число воркеров (все могли быть in-flight на момент setPause).
	if got := atomic.LoadInt32(&reqCount); got > workers {
		t.Errorf("requests sent = %d, want <= %d (max one in-flight per worker after 429)", got, workers)
	}
	if got := atomic.LoadInt32(&reqCount); got < 1 {
		t.Errorf("requests sent = %d, want at least 1", got)
	}
}
