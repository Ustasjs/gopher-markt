package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ustasjs/gopher-markt/internal/middleware"
	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/money"
	"github.com/ustasjs/gopher-markt/internal/service"
)

type BalanceServiceInterface interface {
	GetBalance(ctx context.Context, userID string) (current int64, withdrawn int64, err error)
	Withdraw(ctx context.Context, userID, orderNumber string, sumKopecks int64) error
	GetWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error)
}

type BalanceHandler struct {
	balanceService BalanceServiceInterface
}

func NewBalanceHandler(balanceService BalanceServiceInterface) *BalanceHandler {
	return &BalanceHandler{balanceService: balanceService}
}

type withdrawalResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type balanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type withdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func (h *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	current, withdrawn, err := h.balanceService.GetBalance(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := balanceResponse{
		Current:   money.FromKopecks(current),
		Withdrawn: money.FromKopecks(withdrawn),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	var req withdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Order == "" || req.Sum <= 0 {
		http.Error(w, "order and sum are required", http.StatusBadRequest)
		return
	}

	sumKopecks := money.ToKopecks(req.Sum)

	err := h.balanceService.Withdraw(r.Context(), userID, req.Order, sumKopecks)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOrderNumber):
			http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		case errors.Is(err, service.ErrInsufficientBalance):
			http.Error(w, "insufficient balance", http.StatusPaymentRequired)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *BalanceHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())

	withdrawals, err := h.balanceService.GetWithdrawals(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]withdrawalResponse, 0, len(withdrawals))
	for _, ww := range withdrawals {
		resp = append(resp, withdrawalResponse{
			Order:       ww.OrderNumber,
			Sum:         money.FromKopecks(ww.Sum),
			ProcessedAt: ww.ProcessedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
