package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_NormalizesAddress(t *testing.T) {
	tests := []struct {
		input   string
		wantURL string
	}{
		{"localhost:3000", "http://localhost:3000"},
		{"http://localhost:3000", "http://localhost:3000"},
		{"https://localhost:3000", "https://localhost:3000"},
		{"localhost:3000/", "http://localhost:3000"},
	}

	for _, tt := range tests {
		c := NewClient(tt.input)
		if c.baseURL != tt.wantURL {
			t.Errorf("NewClient(%q).baseURL = %q, want %q", tt.input, c.baseURL, tt.wantURL)
		}
	}
}

func TestGetOrder(t *testing.T) {
	accrualVal := 500.5

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		wantStatus     int
		wantResponse   *AccrualResponse
		wantRetryAfter time.Duration
		wantErr        bool
	}{
		{
			name: "200 with accrual",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(AccrualResponse{
					Order:   "12345678903",
					Status:  "PROCESSED",
					Accrual: &accrualVal,
				})
			},
			wantStatus:   http.StatusOK,
			wantResponse: &AccrualResponse{Order: "12345678903", Status: "PROCESSED", Accrual: &accrualVal},
		},
		{
			name: "200 without accrual",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(AccrualResponse{Order: "12345678903", Status: "PROCESSING"})
			},
			wantStatus:   http.StatusOK,
			wantResponse: &AccrualResponse{Order: "12345678903", Status: "PROCESSING", Accrual: nil},
		},
		{
			name: "204 no content",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantStatus:   http.StatusNoContent,
			wantResponse: nil,
		},
		{
			name: "404 not retryable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantStatus:   http.StatusNotFound,
			wantResponse: nil,
		},
		{
			name: "429 returns retry-after to caller",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "5")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantStatus:     http.StatusTooManyRequests,
			wantResponse:   nil,
			wantRetryAfter: 5 * time.Second,
		},
		{
			name: "429 without header defaults to 1 minute",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantStatus:     http.StatusTooManyRequests,
			wantResponse:   nil,
			wantRetryAfter: time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client := NewClient(server.URL)
			resp, status, retryAfter, err := client.GetOrder(context.Background(), "12345678903")

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if retryAfter != tt.wantRetryAfter {
				t.Errorf("retryAfter = %v, want %v", retryAfter, tt.wantRetryAfter)
			}
			if tt.wantResponse == nil && resp != nil {
				t.Errorf("expected nil response, got %+v", resp)
			}
			if tt.wantResponse != nil {
				if resp == nil {
					t.Fatal("expected response, got nil")
				}
				if resp.Order != tt.wantResponse.Order {
					t.Errorf("Order = %q, want %q", resp.Order, tt.wantResponse.Order)
				}
				if resp.Status != tt.wantResponse.Status {
					t.Errorf("Status = %q, want %q", resp.Status, tt.wantResponse.Status)
				}
				if tt.wantResponse.Accrual == nil && resp.Accrual != nil {
					t.Errorf("expected nil Accrual, got %v", *resp.Accrual)
				}
				if tt.wantResponse.Accrual != nil {
					if resp.Accrual == nil {
						t.Fatal("expected Accrual, got nil")
					}
					if *resp.Accrual != *tt.wantResponse.Accrual {
						t.Errorf("Accrual = %v, want %v", *resp.Accrual, *tt.wantResponse.Accrual)
					}
				}
			}
		})
	}
}

type stubDoer struct {
	calls int32
	fn    func(req *http.Request) (*http.Response, error)
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&s.calls, 1)
	return s.fn(req)
}

func respWithStatus(status int, headers http.Header) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	return &http.Response{StatusCode: status, Header: headers, Body: http.NoBody}
}

func newTestRequest(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	return req
}

func TestRetryingDoer_SuccessAfterTransient5xx(t *testing.T) {
	stub := &stubDoer{}
	stub.fn = func(req *http.Request) (*http.Response, error) {
		// stub.calls уже инкрементирован в Do() — на первом вызове он равен 1.
		if atomic.LoadInt32(&stub.calls) == 1 {
			return respWithStatus(http.StatusBadGateway, nil), nil
		}
		return respWithStatus(http.StatusOK, nil), nil
	}

	doer := newRetryingDoer(stub, 3, time.Millisecond)
	resp, err := doer.Do(newTestRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestRetryingDoer_ExhaustsAttemptsOn5xx(t *testing.T) {
	stub := &stubDoer{fn: func(req *http.Request) (*http.Response, error) {
		return respWithStatus(http.StatusBadGateway, nil), nil
	}}

	doer := newRetryingDoer(stub, 3, time.Millisecond)
	resp, err := doer.Do(newTestRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestRetryingDoer_NoRetryOn4xx(t *testing.T) {
	stub := &stubDoer{fn: func(req *http.Request) (*http.Response, error) {
		return respWithStatus(http.StatusNotFound, nil), nil
	}}

	doer := newRetryingDoer(stub, 3, time.Millisecond)
	resp, err := doer.Do(newTestRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestRetryingDoer_NoRetryOn429(t *testing.T) {
	stub := &stubDoer{fn: func(req *http.Request) (*http.Response, error) {
		h := http.Header{}
		h.Set("Retry-After", "60")
		return respWithStatus(http.StatusTooManyRequests, h), nil
	}}

	doer := newRetryingDoer(stub, 3, time.Millisecond)
	resp, err := doer.Do(newTestRequest(t, context.Background()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Errorf("calls = %d, want 1 (429 не ретраится в обёртке)", got)
	}
}

func TestRetryingDoer_RetriesTransportError(t *testing.T) {
	netErr := errors.New("connection refused")
	stub := &stubDoer{fn: func(req *http.Request) (*http.Response, error) {
		return nil, netErr
	}}

	doer := newRetryingDoer(stub, 3, time.Millisecond)
	_, err := doer.Do(newTestRequest(t, context.Background()))
	if !errors.Is(err, netErr) {
		t.Fatalf("err = %v, want %v", err, netErr)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestRetryingDoer_StopsOnContextCancel(t *testing.T) {
	stub := &stubDoer{fn: func(req *http.Request) (*http.Response, error) {
		return nil, context.Canceled
	}}

	doer := newRetryingDoer(stub, 3, time.Millisecond)
	_, err := doer.Do(newTestRequest(t, context.Background()))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want %v", err, context.Canceled)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Errorf("calls = %d, want 1 (context cancel is not retryable)", got)
	}
}

func TestRetryingDoer_StopsWhenContextCancelledBetweenRetries(t *testing.T) {
	stub := &stubDoer{fn: func(req *http.Request) (*http.Response, error) {
		return respWithStatus(http.StatusBadGateway, nil), nil
	}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу — первая попытка пройдёт, на ожидании select выйдем

	doer := newRetryingDoer(stub, 3, 10*time.Millisecond)
	_, err := doer.Do(newTestRequest(t, ctx))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want %v", err, context.Canceled)
	}
	if got := atomic.LoadInt32(&stub.calls); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		header string
		want   time.Duration
	}{
		{"", time.Minute},
		{"abc", time.Minute},
		{"0", time.Minute},
		{"-5", time.Minute},
		{"3", 3 * time.Second},
		{strconv.Itoa(60), 60 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.header, func(t *testing.T) {
			h := http.Header{}
			if c.header != "" {
				h.Set("Retry-After", c.header)
			}
			got := parseRetryAfter(&http.Response{Header: h})
			if got != c.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", c.header, got, c.want)
			}
		})
	}
}
