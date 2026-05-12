package storage

import "errors"

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrLoginConflict          = errors.New("login already taken")
	ErrOrderNotFound          = errors.New("order not found")
	ErrOrderConflictSameUser  = errors.New("order already uploaded by this user")
	ErrOrderConflictOtherUser = errors.New("order already uploaded by other user")
	ErrInsufficientBalance    = errors.New("insufficient balance")
)
