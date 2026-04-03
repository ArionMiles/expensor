package store

import (
	"context"
	"fmt"
	"time"
)

// InsertParams holds the minimum fields needed to seed a transaction in tests.
type InsertParams struct {
	MessageID    string
	Amount       float64
	Currency     string
	MerchantInfo string
	Category     string
	Description  string
	Timestamp    time.Time
}

// InsertForTest inserts a single transaction and returns its UUID.
// This is intended for use in tests only; production code should use the
// postgres writer plugin which handles batching and acknowledgements.
func (s *Store) InsertForTest(ctx context.Context, p InsertParams) (string, error) {
	if p.Currency == "" {
		p.Currency = "INR"
	}
	if p.Timestamp.IsZero() {
		p.Timestamp = time.Now()
	}

	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO transactions
			(message_id, amount, currency, timestamp, merchant_info, category, source, description)
		VALUES ($1, $2, $3, $4, $5, $6, 'test', $7)
		RETURNING id
	`,
		p.MessageID, p.Amount, p.Currency, p.Timestamp,
		p.MerchantInfo, p.Category, p.Description,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("InsertForTest: %w", err)
	}
	return id, nil
}
