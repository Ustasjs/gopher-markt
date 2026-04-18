package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/ustasjs/gopher-markt/internal/storage"
)

type UserServiceInterface interface {
	Register(ctx context.Context, login, password string) (token string, err error)
	Login(ctx context.Context, login, password string) (token string, err error)
}

type UserService struct {
	repo       storage.Repository
	jwtService *JWTService
}

func NewUserService(repo storage.Repository, jwtService *JWTService) *UserService {
	return &UserService{
		repo:       repo,
		jwtService: jwtService,
	}
}

func (s *UserService) Register(ctx context.Context, login, password string) (token string, err error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	userID, err := s.repo.CreateUser(ctx, login, string(passwordHash))
	if err != nil {
		return "", err
	}

	token, err = s.jwtService.GenerateToken(userID)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

func (s *UserService) Login(ctx context.Context, login, password string) (token string, err error) {
	user, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", fmt.Errorf("invalid password")
	}

	token, err = s.jwtService.GenerateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}
