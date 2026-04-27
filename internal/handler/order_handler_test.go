package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ustasjs/gopher-markt/internal/handler/mocks"
	"github.com/ustasjs/gopher-markt/internal/middleware"
	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey, userID)
	return r.WithContext(ctx)
}

func TestUploadOrder(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		userID           string
		mockUploadFunc   func(ctx context.Context, userID, orderNumber string) error
		expectedStatus   int
	}{
		{
			name:   "Success",
			body:   "12345678903",
			userID: "user-1",
			mockUploadFunc: func(ctx context.Context, userID, orderNumber string) error {
				return nil
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:   "AlreadyUploadedBySameUser",
			body:   "12345678903",
			userID: "user-1",
			mockUploadFunc: func(ctx context.Context, userID, orderNumber string) error {
				return storage.ErrOrderConflictSameUser
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "AlreadyUploadedByOtherUser",
			body:   "12345678903",
			userID: "user-1",
			mockUploadFunc: func(ctx context.Context, userID, orderNumber string) error {
				return storage.ErrOrderConflictOtherUser
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:   "InvalidOrderNumber",
			body:   "00000000000",
			userID: "user-1",
			mockUploadFunc: func(ctx context.Context, userID, orderNumber string) error {
				return service.ErrInvalidOrderNumber
			},
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "EmptyBody",
			body:           "",
			userID:         "user-1",
			mockUploadFunc: nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "WhitespaceOnlyBody",
			body:           "   ",
			userID:         "user-1",
			mockUploadFunc: nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "ServiceError",
			body:   "12345678903",
			userID: "user-1",
			mockUploadFunc: func(ctx context.Context, userID, orderNumber string) error {
				return errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockOrderService{
				UploadOrderFunc: tt.mockUploadFunc,
			}

			h := NewOrderHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(tt.body))
			req = withUserID(req, tt.userID)
			rec := httptest.NewRecorder()

			h.UploadOrder(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestGetOrders(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name           string
		userID         string
		mockGetFunc    func(ctx context.Context, userID string) ([]model.Order, error)
		expectedStatus int
		checkBody      func(t *testing.T, body string)
	}{
		{
			name:   "SuccessWithOrders",
			userID: "user-1",
			mockGetFunc: func(ctx context.Context, userID string) ([]model.Order, error) {
				accrual := int64(500)
				return []model.Order{
					{Number: "9278923470", Status: "PROCESSED", Accrual: &accrual, UploadedAt: now},
					{Number: "12345678903", Status: "PROCESSING", Accrual: nil, UploadedAt: now.Add(-time.Minute)},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp []orderResponse
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(resp) != 2 {
					t.Fatalf("expected 2 orders, got %d", len(resp))
				}
				if resp[0].Number != "9278923470" {
					t.Errorf("expected first order number 9278923470, got %s", resp[0].Number)
				}
				if resp[0].Accrual == nil || *resp[0].Accrual != 5.0 {
					t.Errorf("expected accrual 5.0, got %v", resp[0].Accrual)
				}
				if resp[1].Accrual != nil {
					t.Errorf("expected no accrual for PROCESSING order, got %v", *resp[1].Accrual)
				}
			},
		},
		{
			name:   "NoOrders",
			userID: "user-1",
			mockGetFunc: func(ctx context.Context, userID string) ([]model.Order, error) {
				return []model.Order{}, nil
			},
			expectedStatus: http.StatusNoContent,
			checkBody:      nil,
		},
		{
			name:   "ServiceError",
			userID: "user-1",
			mockGetFunc: func(ctx context.Context, userID string) ([]model.Order, error) {
				return nil, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkBody:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockOrderService{
				GetOrdersFunc: tt.mockGetFunc,
			}

			h := NewOrderHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			req = withUserID(req, tt.userID)
			rec := httptest.NewRecorder()

			h.GetOrders(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body.String())
			}
		})
	}
}
