package mocks

import (
	"context"
)

type MockUserService struct {
	RegisterFunc func(ctx context.Context, login, password string) (token string, err error)
	LoginFunc    func(ctx context.Context, login, password string) (token string, err error)
}

func (m *MockUserService) Register(ctx context.Context, login, password string) (token string, err error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, login, password)
	}
	return "", nil
}

func (m *MockUserService) Login(ctx context.Context, login, password string) (token string, err error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, login, password)
	}
	return "", nil
}
