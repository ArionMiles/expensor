package store

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type repositoryDependencies struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	metrics *QueryInstrumentation
	now     func() time.Time
}
