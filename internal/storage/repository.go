package storage

import (
	"context"
	"errors"

	"github.com/ustasjs/gopher-markt/internal/model"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrLoginConflict          = errors.New("login already taken")
	ErrOrderNotFound          = errors.New("order not found")
	ErrOrderConflictSameUser  = errors.New("order already uploaded by this user")
	ErrOrderConflictOtherUser = errors.New("order already uploaded by other user")
	ErrInsufficientBalance    = errors.New("insufficient balance")
)

type Repository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (userID string, err error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
	CreateOrder(ctx context.Context, userID, number string) error
	GetOrderByNumber(ctx context.Context, number string) (*model.Order, error)
	GetOrdersByUserID(ctx context.Context, userID string) ([]model.Order, error)
	GetBalance(ctx context.Context, userID string) (current int64, withdrawn int64, err error)
	CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum int64) error
	GetWithdrawalsByUserID(ctx context.Context, userID string) ([]model.Withdrawal, error)
}
