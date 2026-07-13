package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// SearchPath is the backend runtime search path for application database pools.
const SearchPath = "expensor,public"

// ParsePoolConfig parses a pgxpool config and applies the runtime search path
// used by the backend application pools.
func ParsePoolConfig(connStr string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, errors.E("postgres.connection.parse", errors.InvalidArgument, "parsing connection string", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = SearchPath
	return cfg, nil
}
