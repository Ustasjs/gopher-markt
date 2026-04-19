package mocks

import (
	"context"
)

type MockOrderService struct {
	UploadOrderFunc func(ctx context.Context, userID, orderNumber string) error
}

func (m *MockOrderService) UploadOrder(ctx context.Context, userID, orderNumber string) error {
	if m.UploadOrderFunc != nil {
		return m.UploadOrderFunc(ctx, userID, orderNumber)
	}
	return nil
}
