package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ustasjs/gopher-markt/internal/middleware"
	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/service"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

type OrderServiceInterface interface {
	UploadOrder(ctx context.Context, userID, orderNumber string) error
	GetOrders(ctx context.Context, userID string) ([]model.Order, error)
}

type orderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
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

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	orders, err := h.orderService.GetOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		item := orderResponse{
			Number:     o.Number,
			Status:     o.Status,
			UploadedAt: o.UploadedAt,
		}
		if o.Accrual > 0 {
			v := float64(o.Accrual)
			item.Accrual = &v
		}
		resp = append(resp, item)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
