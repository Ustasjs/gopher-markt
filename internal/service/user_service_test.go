package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ustasjs/gopher-markt/internal/model"
	"github.com/ustasjs/gopher-markt/internal/storage"
)

type mockUserRepo struct {
	createUserFn     func(ctx context.Context, login, passwordHash string) (string, error)
	getUserByLoginFn func(ctx context.Context, login string) (*model.User, error)
}

func (m *mockUserRepo) CreateUser(ctx context.Context, login, passwordHash string) (string, error) {
	return m.createUserFn(ctx, login, passwordHash)
}
func (m *mockUserRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	return m.getUserByLoginFn(ctx, login)
}

func TestUserService_Register_Success(t *testing.T) {
	var passedLogin, passedHash string
	repo := &mockUserRepo{
		createUserFn: func(ctx context.Context, login, passwordHash string) (string, error) {
			passedLogin = login
			passedHash = passwordHash
			return "user-123", nil
		},
	}
	jwtSvc := NewJWTService([]byte("test-secret"))
	svc := NewUserService(repo, jwtSvc)

	token, err := svc.Register(context.Background(), "alice", "p@ssw0rd")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if passedLogin != "alice" {
		t.Errorf("login passed to repo = %q, want %q", passedLogin, "alice")
	}
	if passedHash == "p@ssw0rd" {
		t.Error("raw password passed to repo instead of hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passedHash), []byte("p@ssw0rd")); err != nil {
		t.Errorf("passed hash doesn't match raw password: %v", err)
	}

	claims, err := jwtSvc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken on returned token: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("token UserID = %q, want %q", claims.UserID, "user-123")
	}
}

func TestUserService_Register_RepoConflictPropagated(t *testing.T) {
	repo := &mockUserRepo{
		createUserFn: func(ctx context.Context, login, passwordHash string) (string, error) {
			return "", storage.ErrLoginConflict
		},
	}
	svc := NewUserService(repo, NewJWTService([]byte("test-secret")))

	_, err := svc.Register(context.Background(), "alice", "p@ssw0rd")
	if !errors.Is(err, storage.ErrLoginConflict) {
		t.Errorf("err = %v, want storage.ErrLoginConflict", err)
	}
}

func TestUserService_Login_Success(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("p@ssw0rd"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt setup: %v", err)
	}

	repo := &mockUserRepo{
		getUserByLoginFn: func(ctx context.Context, login string) (*model.User, error) {
			if login != "alice" {
				t.Errorf("repo got login = %q, want %q", login, "alice")
			}
			return &model.User{ID: "user-123", Login: "alice", PasswordHash: string(hash)}, nil
		},
	}
	jwtSvc := NewJWTService([]byte("test-secret"))
	svc := NewUserService(repo, jwtSvc)

	token, err := svc.Login(context.Background(), "alice", "p@ssw0rd")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := jwtSvc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("token UserID = %q, want %q", claims.UserID, "user-123")
	}
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt setup: %v", err)
	}

	repo := &mockUserRepo{
		getUserByLoginFn: func(ctx context.Context, login string) (*model.User, error) {
			return &model.User{ID: "user-123", Login: "alice", PasswordHash: string(hash)}, nil
		},
	}
	svc := NewUserService(repo, NewJWTService([]byte("test-secret")))

	_, err = svc.Login(context.Background(), "alice", "wrong")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("err = %v, want ErrInvalidPassword", err)
	}
}

func TestUserService_Login_UserNotFoundPropagated(t *testing.T) {
	repo := &mockUserRepo{
		getUserByLoginFn: func(ctx context.Context, login string) (*model.User, error) {
			return nil, storage.ErrUserNotFound
		},
	}
	svc := NewUserService(repo, NewJWTService([]byte("test-secret")))

	_, err := svc.Login(context.Background(), "ghost", "p@ssw0rd")
	if !errors.Is(err, storage.ErrUserNotFound) {
		t.Errorf("err = %v, want storage.ErrUserNotFound", err)
	}
}
