package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ustasjs/gopher-markt/internal/model"
)

type mockOrderRepo struct {
	createOrderFn       func(ctx context.Context, userID, number string) error
	getOrdersByUserIDFn func(ctx context.Context, userID string) ([]model.Order, error)
}

func (m *mockOrderRepo) CreateOrder(ctx context.Context, userID, number string) error {
	return m.createOrderFn(ctx, userID, number)
}
func (m *mockOrderRepo) GetOrdersByUserID(ctx context.Context, userID string) ([]model.Order, error) {
	return m.getOrdersByUserIDFn(ctx, userID)
}

func TestLuhnValid(t *testing.T) {
	cases := []struct {
		number string
		want   bool
	}{
		{"12345678903", true},
		{"4561261212345467", true},
		{"79927398713", true},
		{"0", true},
		{"", false},
		{"1", false},
		{"12345678901", false},
		{"abc", false},
		{"1234 5678 903", false},
		{"123a567890", false},
		{"-12345", false},
	}
	for _, c := range cases {
		t.Run(c.number, func(t *testing.T) {
			if got := luhnValid(c.number); got != c.want {
				t.Errorf("luhnValid(%q) = %v, want %v", c.number, got, c.want)
			}
		})
	}
}

func TestOrderService_UploadOrder_InvalidLuhn(t *testing.T) {
	repo := &mockOrderRepo{
		createOrderFn: func(ctx context.Context, userID, number string) error {
			t.Error("CreateOrder must not be called for invalid Luhn number")
			return nil
		},
	}
	svc := NewOrderService(repo)

	err := svc.UploadOrder(context.Background(), "user-1", "12345678901")
	if !errors.Is(err, ErrInvalidOrderNumber) {
		t.Errorf("err = %v, want ErrInvalidOrderNumber", err)
	}
}

func TestOrderService_UploadOrder_Success(t *testing.T) {
	var gotUserID, gotNumber string
	repo := &mockOrderRepo{
		createOrderFn: func(ctx context.Context, userID, number string) error {
			gotUserID = userID
			gotNumber = number
			return nil
		},
	}
	svc := NewOrderService(repo)

	if err := svc.UploadOrder(context.Background(), "user-1", "12345678903"); err != nil {
		t.Fatalf("UploadOrder: %v", err)
	}
	if gotUserID != "user-1" {
		t.Errorf("userID passed to repo = %q, want %q", gotUserID, "user-1")
	}
	if gotNumber != "12345678903" {
		t.Errorf("number passed to repo = %q, want %q", gotNumber, "12345678903")
	}
}

func TestOrderService_UploadOrder_RepoErrorPropagated(t *testing.T) {
	repoErr := errors.New("conflict")
	repo := &mockOrderRepo{
		createOrderFn: func(ctx context.Context, userID, number string) error {
			return repoErr
		},
	}
	svc := NewOrderService(repo)

	err := svc.UploadOrder(context.Background(), "user-1", "12345678903")
	if !errors.Is(err, repoErr) {
		t.Errorf("err = %v, want %v", err, repoErr)
	}
}

func TestOrderService_GetOrders(t *testing.T) {
	want := []model.Order{
		{Number: "111", Status: "PROCESSED"},
		{Number: "222", Status: "NEW"},
	}
	repo := &mockOrderRepo{
		getOrdersByUserIDFn: func(ctx context.Context, userID string) ([]model.Order, error) {
			if userID != "user-1" {
				t.Errorf("repo got userID = %q, want %q", userID, "user-1")
			}
			return want, nil
		},
	}
	svc := NewOrderService(repo)

	got, err := svc.GetOrders(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetOrders: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("orders[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOrderService_GetOrders_RepoErrorPropagated(t *testing.T) {
	repoErr := errors.New("db down")
	repo := &mockOrderRepo{
		getOrdersByUserIDFn: func(ctx context.Context, userID string) ([]model.Order, error) {
			return nil, repoErr
		},
	}
	svc := NewOrderService(repo)

	_, err := svc.GetOrders(context.Background(), "user-1")
	if !errors.Is(err, repoErr) {
		t.Errorf("err = %v, want %v", err, repoErr)
	}
}
