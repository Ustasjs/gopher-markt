package storage

import (
	"context"
	"errors"

	"github.com/ustasjs/gopher-markt/internal/model"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrLoginConflict = errors.New("login already taken")
)

type Repository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (userID string, err error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
}
