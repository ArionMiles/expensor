package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type insertParams struct {
	Tenant       store.Tenant
	MessageID    string
	Amount       float64
	Currency     string
	MerchantInfo string
	Category     string
	Bucket       string
	Source       string
	SourceType   string
	SourceLabel  string
	Bank         string
	Description  string
	Timestamp    time.Time
}

func insertForTest(ctx context.Context, st *Store, p insertParams) (string, error) {
	if p.Tenant.ID == "" {
		p.Tenant = testTenantForStore(st)
	}
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
		p.Tenant.ID, p.MessageID, p.Amount, p.Currency, p.Timestamp,
		p.MerchantInfo, p.Category, p.Source, p.SourceType, p.SourceLabel, p.Bank, p.Bucket, p.Description,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert transaction for test: %w", err)
	}
	return id, nil
}

func testTenantForStore(st *Store) store.Tenant {
	value, ok := testStores.Load(st)
	if !ok {
		panic("test store tenant is not registered")
	}
	ts := value.(*testStore)
	ts.tenantOnce.Do(func() {
		user, err := st.CreateUser(context.Background(), store.CreateUserInput{
			Email:        "test-tenant@example.com",
			DisplayName:  "Test Tenant",
			Role:         store.UserRoleUser,
			AvatarKey:    "default",
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuu6Z6RMcYbqVvB6KZlSmLfHLj6y8s3zme",
		})
		if err != nil {
			ts.tenantErr = err
			return
		}
		ts.tenant = store.Tenant{ID: user.TenantID}
	})
	if ts.tenantErr != nil {
		panic(ts.tenantErr)
	}
	return ts.tenant
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
