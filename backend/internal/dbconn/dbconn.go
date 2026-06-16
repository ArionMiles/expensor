// Package dbconn provides shared PostgreSQL connection configuration helpers.
package dbconn

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SearchPath is the backend runtime search path.
const SearchPath = "expensor,public"

// ParseConfig parses a pgxpool config and applies the runtime search path used
// by the backend application pools.
func ParseConfig(connStr string) (*pgxpool.Config, error) {
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
