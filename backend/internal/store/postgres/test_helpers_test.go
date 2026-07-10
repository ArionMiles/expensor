package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func insertForTest(ctx context.Context, st *Store, p InsertParams) (string, error) {
	if p.Currency == "" {
		p.Currency = defaultTransactionCurrency
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
	err := st.pool.QueryRow(ctx, `
		INSERT INTO transactions
			(tenant_id, message_id, amount, currency, timestamp, merchant_info, category, source, source_type, source_label, bank, bucket, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), $13)
		RETURNING id
	`,
		tenantIDParam(p.Tenant), p.MessageID, p.Amount, p.Currency, p.Timestamp,
		p.MerchantInfo, p.Category, p.Source, p.SourceType, p.SourceLabel, p.Bank, p.Bucket, p.Description,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert transaction for test: %w", err)
	}
	return id, nil
}

func poolForTest(st *Store) *pgxpool.Pool {
	return st.pool
}

func setNowForTestSequence(st *Store, seq ...time.Time) func() {
	prev := st.now
	var prevAnalyticsNow func() time.Time
	if st.analytics != nil {
		prevAnalyticsNow = st.analytics.now
	}
	if len(seq) == 0 {
		st.now = prev
		if st.analytics != nil {
			st.analytics.now = prevAnalyticsNow
		}
		return func() {}
	}

	var mu sync.Mutex
	idx := 0
	st.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		if idx >= len(seq) {
			return seq[len(seq)-1]
		}
		v := seq[idx]
		idx++
		return v
	}
	if st.analytics != nil {
		st.analytics.now = st.now
	}

	return func() {
		st.now = prev
		if st.analytics != nil {
			st.analytics.now = prevAnalyticsNow
		}
	}
}
