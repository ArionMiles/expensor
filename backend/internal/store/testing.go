package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertParams holds the minimum fields needed to seed a transaction in tests.
type InsertParams struct {
	Tenant       Tenant
	MessageID    string
	Amount       float64
	Currency     string
	MerchantInfo string
	Category     string
	Bucket       string // empty → NULL in DB
	Source       string // empty → "test"
	SourceType   string
	SourceLabel  string
	Bank         string
	Description  string
	Timestamp    time.Time
}

// InsertForTest inserts a single transaction and returns its UUID.
// This is intended for use in tests only; production ingestion should use
// TransactionIngestor, which handles batching and acknowledgements.
func (s *Store) InsertForTest(ctx context.Context, p InsertParams) (string, error) {
	if p.Currency == "" {
		p.Currency = "INR"
	}
	if p.Timestamp.IsZero() {
		p.Timestamp = time.Now()
	}
	if p.Source == "" {
		p.Source = "test"
	}
	if p.SourceLabel == "" {
		p.SourceLabel = p.Source
	}

	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO transactions
			(tenant_id, message_id, amount, currency, timestamp, merchant_info, category, source, source_type, source_label, bank, bucket, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), $13)
		RETURNING id
	`,
		tenantIDParam(p.Tenant), p.MessageID, p.Amount, p.Currency, p.Timestamp,
		p.MerchantInfo, p.Category, p.Source, p.SourceType, p.SourceLabel, p.Bank, p.Bucket, p.Description,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("InsertForTest: %w", err)
	}
	return id, nil
}

// PoolForTest exposes the store pool for focused repository tests.
func (s *Store) PoolForTest() *pgxpool.Pool {
	return s.pool
}

// SetNowForTestSequence overrides the store clock for tests and returns a restore function.
// After the provided timestamps are exhausted, the last value is reused.
func (s *Store) SetNowForTestSequence(seq ...time.Time) func() {
	prev := s.now
	var prevReadModelNow func() time.Time
	if s.readModel != nil {
		prevReadModelNow = s.readModel.now
	}
	if len(seq) == 0 {
		s.now = prev
		if s.readModel != nil {
			s.readModel.now = prevReadModelNow
		}
		return func() {}
	}

	var mu sync.Mutex
	idx := 0
	s.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		if idx >= len(seq) {
			return seq[len(seq)-1]
		}
		v := seq[idx]
		idx++
		return v
	}
	if s.readModel != nil {
		s.readModel.now = s.now
	}

	return func() {
		s.now = prev
		if s.readModel != nil {
			s.readModel.now = prevReadModelNow
		}
	}
}
