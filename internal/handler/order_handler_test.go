package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ustasjs/gopher-markt/internal/handler/mocks"
	"github.com/ustasjs/gopher-markt/internal/middleware"
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
