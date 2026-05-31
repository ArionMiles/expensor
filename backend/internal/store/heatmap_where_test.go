package store

import (
	"testing"
	"time"
)

func TestBuildHeatmapWhereSupportsSingleSidedBounds(t *testing.T) {
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name      string
		from      *time.Time
		to        *time.Time
		wantWhere string
		wantArgs  int
	}{
		{
			name:      "no bounds",
			wantWhere: " WHERE muted = false",
		},
		{
			name:      "from only",
			from:      &from,
			wantWhere: " WHERE muted = false AND timestamp >= $1",
			wantArgs:  1,
		},
		{
			name:      "to only",
			to:        &to,
			wantWhere: " WHERE muted = false AND timestamp <= $1",
			wantArgs:  1,
		},
		{
			name:      "from and to",
			from:      &from,
			to:        &to,
			wantWhere: " WHERE muted = false AND timestamp >= $1 AND timestamp <= $2",
			wantArgs:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			where, args := buildHeatmapWhere(tc.from, tc.to)
			if where != tc.wantWhere {
				t.Fatalf("where = %q, want %q", where, tc.wantWhere)
			}
			if len(args) != tc.wantArgs {
				t.Fatalf("args = %d, want %d", len(args), tc.wantArgs)
			}
		})
	}
}
