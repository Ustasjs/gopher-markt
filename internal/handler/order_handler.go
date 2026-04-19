package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ustasjs/gopher-markt/internal/middleware"
	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

type OrderServiceInterface interface {
	UploadOrder(ctx context.Context, userID, orderNumber string) error
}

type OrderHandler struct {
	orderService OrderServiceInterface
}

func NewOrderHandler(orderService OrderServiceInterface) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	orderNumber := strings.TrimSpace(string(body))
	if orderNumber == "" {
		http.Error(w, "order number is required", http.StatusBadRequest)
		return
	}

	err = h.orderService.UploadOrder(r.Context(), userID, orderNumber)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOrderNumber):
			http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		case errors.Is(err, storage.ErrOrderConflictSameUser):
			w.WriteHeader(http.StatusOK)
		case errors.Is(err, storage.ErrOrderConflictOtherUser):
			http.Error(w, "order already uploaded by another user", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
