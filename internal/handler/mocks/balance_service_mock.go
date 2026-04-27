package mocks

import "context"

type MockBalanceService struct {
	GetBalanceFunc func(ctx context.Context, userID string) (int64, int64, error)
	WithdrawFunc   func(ctx context.Context, userID, orderNumber string, sumKopecks int64) error
}

func (m *MockBalanceService) GetBalance(ctx context.Context, userID string) (int64, int64, error) {
	if m.GetBalanceFunc != nil {
		return m.GetBalanceFunc(ctx, userID)
	}
	return 0, 0, nil
}

func (m *MockBalanceService) Withdraw(ctx context.Context, userID, orderNumber string, sumKopecks int64) error {
	if m.WithdrawFunc != nil {
		return m.WithdrawFunc(ctx, userID, orderNumber, sumKopecks)
	}
	return nil
}
