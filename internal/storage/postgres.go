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

func (r *PostgresRepository) GetOrdersByUserID(ctx context.Context, userID string) ([]model.Order, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get orders by user id: %w", err)
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, fmt.Errorf("get orders by user id: scan: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get orders by user id: rows: %w", err)
	}
	return orders, nil
}

func (r *PostgresRepository) GetBalance(ctx context.Context, userID string) (int64, int64, error) {
	var accrual int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE user_id = $1 AND status = 'PROCESSED'`,
		userID,
	).Scan(&accrual)
	if err != nil {
		return 0, 0, fmt.Errorf("get balance accrual: %w", err)
	}

	var withdrawn int64
	err = r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1`,
		userID,
	).Scan(&withdrawn)
	if err != nil {
		return 0, 0, fmt.Errorf("get balance withdrawn: %w", err)
	}

	return accrual - withdrawn, withdrawn, nil
}

func (r *PostgresRepository) CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create withdrawal: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		return fmt.Errorf("create withdrawal: lock user: %w", err)
	}

	var accrual int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE user_id = $1 AND status = 'PROCESSED'`,
		userID,
	).Scan(&accrual); err != nil {
		return fmt.Errorf("create withdrawal: get accrual: %w", err)
	}

	var withdrawn int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1`,
		userID,
	).Scan(&withdrawn); err != nil {
		return fmt.Errorf("create withdrawal: get withdrawn: %w", err)
	}

	if accrual-withdrawn < sum {
		return ErrInsufficientBalance
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)`,
		userID, orderNumber, sum,
	); err != nil {
		return fmt.Errorf("create withdrawal: insert: %w", err)
	}

	return tx.Commit()
}

func (r *PostgresRepository) GetWithdrawalsByUserID(ctx context.Context, userID string) ([]model.Withdrawal, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		var w model.Withdrawal
		if err := rows.Scan(&w.OrderNumber, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, fmt.Errorf("get withdrawals: scan: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get withdrawals: rows: %w", err)
	}
	return withdrawals, nil
}

func (r *PostgresRepository) GetPendingOrders(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT number FROM orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY uploaded_at ASC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending orders: %w", err)
	}
	defer rows.Close()

	var numbers []string
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			return nil, fmt.Errorf("get pending orders: scan: %w", err)
		}
		numbers = append(numbers, number)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get pending orders: rows: %w", err)
	}
	return numbers, nil
}

func (r *PostgresRepository) UpdateOrderStatus(ctx context.Context, number, status string, accrual *int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`,
		status, accrual, number,
	)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
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
