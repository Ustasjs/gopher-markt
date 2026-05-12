package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/ustasjs/gopher-markt/internal/model"
)

var ErrInvalidPassword = errors.New("invalid password")

type UserServiceInterface interface {
	Register(ctx context.Context, login, password string) (token string, err error)
	Login(ctx context.Context, login, password string) (token string, err error)
}

type userRepository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (string, error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
}

type UserService struct {
	repo       userRepository
	jwtService *JWTService
}

func NewUserService(repo userRepository, jwtService *JWTService) *UserService {
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidPassword
	}

	token, err = s.jwtService.GenerateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}
