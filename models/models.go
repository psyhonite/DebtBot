package models

import "time"

type User struct {
	ID        int64     `db:"id"` // Telegram User ID
	CreatedAt time.Time `db:"created_at"`
}

type Credit struct {
	ID         int       `db:"id"`
	UserID     int64     `db:"user_id"`
	BankName   string    `db:"bank_name"`
	LoanAmount float64   `db:"loan_amount"`
	DueDate    time.Time `db:"due_date"`
	CreatedAt  time.Time `db:"created_at"`
}
