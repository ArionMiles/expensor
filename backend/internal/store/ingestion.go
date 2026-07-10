package store

import "time"

// IngestionConfig controls daemon transaction ingestion batching.
type IngestionConfig struct {
	// Tenant identifies the tenant that owns written transactions. Empty keeps the temporary legacy tenant.
	Tenant Tenant
	// BatchSize is the number of transactions to buffer before writing.
	BatchSize int
	// FlushInterval is the time between automatic flushes.
	FlushInterval time.Duration
}
