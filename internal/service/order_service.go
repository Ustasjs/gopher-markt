package service

import (
	"context"
	"fmt"

	"github.com/ustasjs/gopher-markt/internal/storage"
)

type OrderServiceInterface interface {
	UploadOrder(ctx context.Context, userID, orderNumber string) error
}

type OrderService struct {
	repo storage.Repository
}

func NewOrderService(repo storage.Repository) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) UploadOrder(ctx context.Context, userID, orderNumber string) error {
	if !luhnValid(orderNumber) {
		return ErrInvalidOrderNumber
	}
	return s.repo.CreateOrder(ctx, userID, orderNumber)
}

var ErrInvalidOrderNumber = fmt.Errorf("invalid order number")

func luhnValid(number string) bool {
	if len(number) == 0 {
		return false
	}

	sum := 0
	double := false

	for i := len(number) - 1; i >= 0; i-- {
		ch := number[i]
		if ch < '0' || ch > '9' {
			return false
		}
		digit := int(ch - '0')

		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		double = !double
	}

	return sum%10 == 0
}
