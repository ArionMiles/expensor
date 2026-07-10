package postgres

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

type repositoryDependencies struct {
	pool      *pgxpool.Pool
	logger    *slog.Logger
	now       func() time.Time
	secretBox *auth.SecretBox
}
