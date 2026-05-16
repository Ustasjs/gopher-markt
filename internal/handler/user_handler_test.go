package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ustasjs/gopher-markt/internal/handler/mocks"
	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

func TestRegister(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        string
		mockRegisterFunc   func(ctx context.Context, login, password string) (string, error)
		expectedStatus     int
		expectedAuthHeader string
	}{
		{
			name:        "Success",
			requestBody: `{"login":"testuser","password":"testpass"}`,
			mockRegisterFunc: func(ctx context.Context, login, password string) (string, error) {
				return "test-jwt-token", nil
			},
			expectedStatus:     http.StatusOK,
			expectedAuthHeader: "Bearer test-jwt-token",
		},
		{
			name:               "EmptyLogin",
			requestBody:        `{"login":"","password":"testpass"}`,
			mockRegisterFunc:   nil,
			expectedStatus:     http.StatusBadRequest,
			expectedAuthHeader: "",
		},
		{
			name:               "EmptyPassword",
			requestBody:        `{"login":"testuser","password":""}`,
			mockRegisterFunc:   nil,
			expectedStatus:     http.StatusBadRequest,
			expectedAuthHeader: "",
		},
		{
			name:               "InvalidJSON",
			requestBody:        `{invalid json}`,
			mockRegisterFunc:   nil,
			expectedStatus:     http.StatusBadRequest,
			expectedAuthHeader: "",
		},
		{
			name:        "LoginConflict",
			requestBody: `{"login":"existing","password":"testpass"}`,
			mockRegisterFunc: func(ctx context.Context, login, password string) (string, error) {
				return "", storage.ErrLoginConflict
			},
			expectedStatus:     http.StatusConflict,
			expectedAuthHeader: "",
		},
		{
			name:        "ServiceError",
			requestBody: `{"login":"testuser","password":"testpass"}`,
			mockRegisterFunc: func(ctx context.Context, login, password string) (string, error) {
				return "", errors.New("database error")
			},
			expectedStatus:     http.StatusInternalServerError,
			expectedAuthHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockUserService{
				RegisterFunc: tt.mockRegisterFunc,
			}

			handler := NewUserHandler(mockService)

			body := bytes.NewBufferString(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/user/register", body)
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			authHeader := rec.Header().Get("Authorization")
			if authHeader != tt.expectedAuthHeader {
				t.Errorf("expected Authorization header %q, got %q", tt.expectedAuthHeader, authHeader)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        string
		mockLoginFunc      func(ctx context.Context, login, password string) (string, error)
		expectedStatus     int
		expectedAuthHeader string
	}{
		{
			name:        "Success",
			requestBody: `{"login":"testuser","password":"testpass"}`,
			mockLoginFunc: func(ctx context.Context, login, password string) (string, error) {
				return "test-jwt-token", nil
			},
			expectedStatus:     http.StatusOK,
			expectedAuthHeader: "Bearer test-jwt-token",
		},
		{
			name:               "EmptyLogin",
			requestBody:        `{"login":"","password":"testpass"}`,
			mockLoginFunc:      nil,
			expectedStatus:     http.StatusBadRequest,
			expectedAuthHeader: "",
		},
		{
			name:               "EmptyPassword",
			requestBody:        `{"login":"testuser","password":""}`,
			mockLoginFunc:      nil,
			expectedStatus:     http.StatusBadRequest,
			expectedAuthHeader: "",
		},
		{
			name:               "InvalidJSON",
			requestBody:        `{invalid json}`,
			mockLoginFunc:      nil,
			expectedStatus:     http.StatusBadRequest,
			expectedAuthHeader: "",
		},
		{
			name:        "UserNotFound",
			requestBody: `{"login":"nonexistent","password":"testpass"}`,
			mockLoginFunc: func(ctx context.Context, login, password string) (string, error) {
				return "", storage.ErrUserNotFound
			},
			expectedStatus:     http.StatusUnauthorized,
			expectedAuthHeader: "",
		},
		{
			name:        "InvalidPassword",
			requestBody: `{"login":"testuser","password":"wrongpass"}`,
			mockLoginFunc: func(ctx context.Context, login, password string) (string, error) {
				return "", service.ErrInvalidPassword
			},
			expectedStatus:     http.StatusUnauthorized,
			expectedAuthHeader: "",
		},
		{
			name:        "ServiceError",
			requestBody: `{"login":"testuser","password":"testpass"}`,
			mockLoginFunc: func(ctx context.Context, login, password string) (string, error) {
				return "", errors.New("database error")
			},
			expectedStatus:     http.StatusInternalServerError,
			expectedAuthHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mocks.MockUserService{
				LoginFunc: tt.mockLoginFunc,
			}

			handler := NewUserHandler(mockService)

			body := bytes.NewBufferString(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/user/login", body)
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			authHeader := rec.Header().Get("Authorization")
			if authHeader != tt.expectedAuthHeader {
				t.Errorf("expected Authorization header %q, got %q", tt.expectedAuthHeader, authHeader)
			}
		})
	}
}
