package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

var ErrInsufficientBalance = fmt.Errorf("insufficient balance")

type BalanceServiceInterface interface {
	GetBalance(ctx context.Context, userID string) (current int64, withdrawn int64, err error)
	Withdraw(ctx context.Context, userID, orderNumber string, sumKopecks int64) error
	GetWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error)
}

type balanceRepository interface {
	GetBalance(ctx context.Context, userID string) (current int64, withdrawn int64, err error)
	CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum int64) error
	GetWithdrawalsByUserID(ctx context.Context, userID string) ([]model.Withdrawal, error)
}

type BalanceService struct {
	repo balanceRepository
}

func NewBalanceService(repo balanceRepository) *BalanceService {
	return &BalanceService{repo: repo}
}

func (s *BalanceService) GetBalance(ctx context.Context, userID string) (int64, int64, error) {
	return s.repo.GetBalance(ctx, userID)
}

func (s *BalanceService) GetWithdrawals(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	return s.repo.GetWithdrawalsByUserID(ctx, userID)
}

func (s *BalanceService) Withdraw(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
	if !luhnValid(orderNumber) {
		return ErrInvalidOrderNumber
	}
	if err := s.repo.CreateWithdrawal(ctx, userID, orderNumber, sumKopecks); err != nil {
		if errors.Is(err, storage.ErrInsufficientBalance) {
			return ErrInsufficientBalance
		}
		return fmt.Errorf("withdraw: %w", err)
	}
	return nil
}
