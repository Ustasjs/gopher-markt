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
)

type Repository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (userID string, err error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
	CreateOrder(ctx context.Context, userID, number string) error
	GetOrderByNumber(ctx context.Context, number string) (*model.Order, error)
}
