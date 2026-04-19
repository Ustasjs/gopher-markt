package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ustasjs/gopher-markt/internal/model"
)

const pgUniqueViolation = "23505"

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateUser(ctx context.Context, login, passwordHash string) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return "", ErrLoginConflict
		}
		return "", fmt.Errorf("create user: %w", err)
	}
	return userID, nil
}

func (r *PostgresRepository) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash FROM users WHERE login = $1`,
		login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by login: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, userID, number string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO orders (number, user_id) VALUES ($1, $2)`,
		number, userID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			existing, fetchErr := r.GetOrderByNumber(ctx, number)
			if fetchErr != nil {
				return fmt.Errorf("create order: check existing: %w", fetchErr)
			}
			if existing.UserID == userID {
				return ErrOrderConflictSameUser
			}
			return ErrOrderConflictOtherUser
		}
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetOrderByNumber(ctx context.Context, number string) (*model.Order, error) {
	o := &model.Order{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, user_id, status, accrual, uploaded_at FROM orders WHERE number = $1`,
		number,
	).Scan(&o.ID, &o.Number, &o.UserID, &o.Status, &o.Accrual, &o.UploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order by number: %w", err)
	}
	return o, nil
}
