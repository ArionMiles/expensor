package store

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SearchPath is the backend runtime search path for application database pools.
const SearchPath = "expensor,public"

// ParsePoolConfig parses a pgxpool config and applies the runtime search path
// used by the backend application pools.
func ParsePoolConfig(connStr string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = SearchPath
	return cfg, nil
}
