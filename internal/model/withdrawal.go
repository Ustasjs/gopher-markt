package model

import "time"

type Withdrawal struct {
	OrderNumber string
	Sum         int64
	ProcessedAt time.Time
}
