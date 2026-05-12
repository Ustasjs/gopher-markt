package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

type mockBalanceRepo struct {
	getBalanceFn             func(ctx context.Context, userID string) (int64, int64, error)
	createWithdrawalFn       func(ctx context.Context, userID, orderNumber string, sum int64) error
	getWithdrawalsByUserIDFn func(ctx context.Context, userID string) ([]model.Withdrawal, error)
}

func (m *mockBalanceRepo) GetBalance(ctx context.Context, userID string) (int64, int64, error) {
	return m.getBalanceFn(ctx, userID)
}
func (m *mockBalanceRepo) CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum int64) error {
	return m.createWithdrawalFn(ctx, userID, orderNumber, sum)
}
func (m *mockBalanceRepo) GetWithdrawalsByUserID(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	return m.getWithdrawalsByUserIDFn(ctx, userID)
}

func TestBalanceService_GetBalance(t *testing.T) {
	repo := &mockBalanceRepo{
		getBalanceFn: func(ctx context.Context, userID string) (int64, int64, error) {
			if userID != "user-1" {
				t.Errorf("repo got userID = %q, want %q", userID, "user-1")
			}
			return 10050, 2500, nil
		},
	}
	svc := NewBalanceService(repo)

	current, withdrawn, err := svc.GetBalance(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if current != 10050 {
		t.Errorf("current = %d, want 10050", current)
	}
	if withdrawn != 2500 {
		t.Errorf("withdrawn = %d, want 2500", withdrawn)
	}
}

func TestBalanceService_GetBalance_RepoErrorPropagated(t *testing.T) {
	repoErr := errors.New("db down")
	repo := &mockBalanceRepo{
		getBalanceFn: func(ctx context.Context, userID string) (int64, int64, error) {
			return 0, 0, repoErr
		},
	}
	svc := NewBalanceService(repo)

	_, _, err := svc.GetBalance(context.Background(), "user-1")
	if !errors.Is(err, repoErr) {
		t.Errorf("err = %v, want %v", err, repoErr)
	}
}

func TestBalanceService_GetWithdrawals(t *testing.T) {
	want := []model.Withdrawal{
		{OrderNumber: "111", Sum: 5000},
		{OrderNumber: "222", Sum: 2500},
	}
	repo := &mockBalanceRepo{
		getWithdrawalsByUserIDFn: func(ctx context.Context, userID string) ([]model.Withdrawal, error) {
			return want, nil
		},
	}
	svc := NewBalanceService(repo)

	got, err := svc.GetWithdrawals(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetWithdrawals: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("withdrawals[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBalanceService_GetWithdrawals_RepoErrorPropagated(t *testing.T) {
	repoErr := errors.New("db down")
	repo := &mockBalanceRepo{
		getWithdrawalsByUserIDFn: func(ctx context.Context, userID string) ([]model.Withdrawal, error) {
			return nil, repoErr
		},
	}
	svc := NewBalanceService(repo)

	_, err := svc.GetWithdrawals(context.Background(), "user-1")
	if !errors.Is(err, repoErr) {
		t.Errorf("err = %v, want %v", err, repoErr)
	}
}

func TestBalanceService_Withdraw_InvalidLuhn(t *testing.T) {
	repo := &mockBalanceRepo{
		createWithdrawalFn: func(ctx context.Context, userID, orderNumber string, sum int64) error {
			t.Error("CreateWithdrawal must not be called for invalid Luhn number")
			return nil
		},
	}
	svc := NewBalanceService(repo)

	err := svc.Withdraw(context.Background(), "user-1", "12345678901", 1000)
	if !errors.Is(err, ErrInvalidOrderNumber) {
		t.Errorf("err = %v, want ErrInvalidOrderNumber", err)
	}
}

func TestBalanceService_Withdraw_Success(t *testing.T) {
	var gotUserID, gotOrder string
	var gotSum int64
	repo := &mockBalanceRepo{
		createWithdrawalFn: func(ctx context.Context, userID, orderNumber string, sum int64) error {
			gotUserID = userID
			gotOrder = orderNumber
			gotSum = sum
			return nil
		},
	}
	svc := NewBalanceService(repo)

	if err := svc.Withdraw(context.Background(), "user-1", "12345678903", 7500); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if gotUserID != "user-1" || gotOrder != "12345678903" || gotSum != 7500 {
		t.Errorf("repo got (%q, %q, %d), want (user-1, 12345678903, 7500)", gotUserID, gotOrder, gotSum)
	}
}

func TestBalanceService_Withdraw_InsufficientBalanceMapped(t *testing.T) {
	repo := &mockBalanceRepo{
		createWithdrawalFn: func(ctx context.Context, userID, orderNumber string, sum int64) error {
			return storage.ErrInsufficientBalance
		},
	}
	svc := NewBalanceService(repo)

	err := svc.Withdraw(context.Background(), "user-1", "12345678903", 7500)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Errorf("err = %v, want service.ErrInsufficientBalance", err)
	}
	if errors.Is(err, storage.ErrInsufficientBalance) {
		t.Error("storage.ErrInsufficientBalance must not leak through the service boundary")
	}
}

func TestBalanceService_Withdraw_OtherRepoErrorWrapped(t *testing.T) {
	repoErr := errors.New("db down")
	repo := &mockBalanceRepo{
		createWithdrawalFn: func(ctx context.Context, userID, orderNumber string, sum int64) error {
			return repoErr
		},
	}
	svc := NewBalanceService(repo)

	err := svc.Withdraw(context.Background(), "user-1", "12345678903", 7500)
	if !errors.Is(err, repoErr) {
		t.Errorf("err = %v, want wrapped %v", err, repoErr)
	}
	if errors.Is(err, ErrInsufficientBalance) {
		t.Error("non-insufficient error must not be remapped to ErrInsufficientBalance")
	}
}
