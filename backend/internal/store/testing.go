package store

import "time"

// InsertParams holds the minimum fields needed to seed a transaction in tests.
type InsertParams struct {
	Tenant       Tenant
	MessageID    string
	Amount       float64
	Currency     string
	MerchantInfo string
	Category     string
	Bucket       string // empty means NULL in DB
	Source       string // empty means "test"
	SourceType   string
	SourceLabel  string
	Bank         string
	Description  string
	Timestamp    time.Time
}
