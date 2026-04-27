package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		name            string
		handler         http.HandlerFunc
		wantStatus      int
		wantResponse    *AccrualResponse
		wantRetryAfter  time.Duration
		wantErr         bool
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
			name: "429 with Retry-After header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantStatus:     http.StatusTooManyRequests,
			wantResponse:   nil,
			wantRetryAfter: 60 * time.Second,
		},
		{
			name: "429 without Retry-After defaults to 1 minute",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantStatus:     http.StatusTooManyRequests,
			wantResponse:   nil,
			wantRetryAfter: time.Minute,
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatus:   http.StatusInternalServerError,
			wantResponse: nil,
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
