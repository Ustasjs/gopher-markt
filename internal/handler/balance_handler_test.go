package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ustasjs/gopher-markt/internal/handler/mocks"
	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/service"
)

func TestGetBalance(t *testing.T) {
	tests := []struct {
		name            string
		userID          string
		mockGetBalance  func(ctx context.Context, userID string) (int64, int64, error)
		expectedStatus  int
		expectedCurrent float64
		expectedWithdrawn float64
	}{
		{
			name:   "Success",
			userID: "user-1",
			mockGetBalance: func(ctx context.Context, userID string) (int64, int64, error) {
				return 50050, 4200, nil
			},
			expectedStatus:    http.StatusOK,
			expectedCurrent:   500.50,
			expectedWithdrawn: 42.00,
		},
		{
			name:   "ZeroBalance",
			userID: "user-1",
			mockGetBalance: func(ctx context.Context, userID string) (int64, int64, error) {
				return 0, 0, nil
			},
			expectedStatus:    http.StatusOK,
			expectedCurrent:   0,
			expectedWithdrawn: 0,
		},
		{
			name:   "ServiceError",
			userID: "user-1",
			mockGetBalance: func(ctx context.Context, userID string) (int64, int64, error) {
				return 0, 0, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockBalanceService{
				GetBalanceFunc: tt.mockGetBalance,
			}

			h := NewBalanceHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
			req = withUserID(req, tt.userID)
			rec := httptest.NewRecorder()

			h.GetBalance(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp balanceResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Current != tt.expectedCurrent {
					t.Errorf("expected current %.2f, got %.2f", tt.expectedCurrent, resp.Current)
				}
				if resp.Withdrawn != tt.expectedWithdrawn {
					t.Errorf("expected withdrawn %.2f, got %.2f", tt.expectedWithdrawn, resp.Withdrawn)
				}
			}
		})
	}
}

func TestWithdraw(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		body           string
		mockWithdraw   func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error
		expectedStatus int
		expectedSum    int64
	}{
		{
			name:   "Success",
			userID: "user-1",
			body:   `{"order":"2377225624","sum":751}`,
			mockWithdraw: func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
				if sumKopecks != 75100 {
					t.Errorf("expected sumKopecks 75100, got %d", sumKopecks)
				}
				return nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "FractionalSum",
			userID: "user-1",
			body:   `{"order":"2377225624","sum":500.50}`,
			mockWithdraw: func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
				if sumKopecks != 50050 {
					t.Errorf("expected sumKopecks 50050, got %d", sumKopecks)
				}
				return nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "InvalidJSON",
			userID:         "user-1",
			body:           `{invalid}`,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "EmptyOrder",
			userID:         "user-1",
			body:           `{"order":"","sum":100}`,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ZeroSum",
			userID:         "user-1",
			body:           `{"order":"2377225624","sum":0}`,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "InvalidOrderNumber",
			userID: "user-1",
			body:   `{"order":"0000000000","sum":100}`,
			mockWithdraw: func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
				return service.ErrInvalidOrderNumber
			},
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:   "InsufficientBalance",
			userID: "user-1",
			body:   `{"order":"2377225624","sum":9999}`,
			mockWithdraw: func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
				return service.ErrInsufficientBalance
			},
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name:   "ServiceError",
			userID: "user-1",
			body:   `{"order":"2377225624","sum":100}`,
			mockWithdraw: func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
				return errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockBalanceService{
				WithdrawFunc: tt.mockWithdraw,
			}

			h := NewBalanceHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBufferString(tt.body))
			req = withUserID(req, tt.userID)
			rec := httptest.NewRecorder()

			h.Withdraw(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestGetWithdrawals(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name               string
		userID             string
		mockGetWithdrawals func(ctx context.Context, userID string) ([]model.Withdrawal, error)
		expectedStatus     int
		checkBody          func(t *testing.T, body string)
	}{
		{
			name:   "SuccessWithWithdrawals",
			userID: "user-1",
			mockGetWithdrawals: func(ctx context.Context, userID string) ([]model.Withdrawal, error) {
				return []model.Withdrawal{
					{OrderNumber: "2377225624", Sum: 50000, ProcessedAt: now},
					{OrderNumber: "9278923470", Sum: 10050, ProcessedAt: now.Add(-time.Hour)},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp []withdrawalResponse
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(resp) != 2 {
					t.Fatalf("expected 2 withdrawals, got %d", len(resp))
				}
				if resp[0].Order != "2377225624" {
					t.Errorf("expected order 2377225624, got %s", resp[0].Order)
				}
				if resp[0].Sum != 500.0 {
					t.Errorf("expected sum 500.0, got %v", resp[0].Sum)
				}
				if resp[1].Sum != 100.50 {
					t.Errorf("expected sum 100.50, got %v", resp[1].Sum)
				}
			},
		},
		{
			name:   "NoWithdrawals",
			userID: "user-1",
			mockGetWithdrawals: func(ctx context.Context, userID string) ([]model.Withdrawal, error) {
				return []model.Withdrawal{}, nil
			},
			expectedStatus: http.StatusNoContent,
			checkBody:      nil,
		},
		{
			name:   "ServiceError",
			userID: "user-1",
			mockGetWithdrawals: func(ctx context.Context, userID string) ([]model.Withdrawal, error) {
				return nil, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkBody:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockBalanceService{
				GetWithdrawalsFunc: tt.mockGetWithdrawals,
			}

			h := NewBalanceHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
			req = withUserID(req, tt.userID)
			rec := httptest.NewRecorder()

			h.GetWithdrawals(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body.String())
			}
		})
	}
}
