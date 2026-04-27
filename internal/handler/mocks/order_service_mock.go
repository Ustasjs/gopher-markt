package mocks

import (
	"context"

	"github.com/ustasjs/gopher-markt/internal/model"
)

type MockOrderService struct {
	UploadOrderFunc func(ctx context.Context, userID, orderNumber string) error
	GetOrdersFunc   func(ctx context.Context, userID string) ([]model.Order, error)
}

func (m *MockOrderService) UploadOrder(ctx context.Context, userID, orderNumber string) error {
	if m.UploadOrderFunc != nil {
		return m.UploadOrderFunc(ctx, userID, orderNumber)
	}
	return nil
}

func (m *MockOrderService) GetOrders(ctx context.Context, userID string) ([]model.Order, error) {
	if m.GetOrdersFunc != nil {
		return m.GetOrdersFunc(ctx, userID)
	}
	return nil, nil
}
