package model

import "time"

type Order struct {
	ID         string
	Number     string
	UserID     string
	Status     string
	Accrual    int64
	UploadedAt time.Time
}
