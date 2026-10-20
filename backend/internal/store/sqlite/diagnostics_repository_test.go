package sqlite

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

var _ store.DiagnosticStore = (*Store)(nil)

func TestDiagnosticsRepositoryDeduplicatesOpenMessage(t *testing.T) {
	st := newRepositoryTestStore(t)
	tenant := newRepositoryTestTenant(t, st)
	received := time.Date(2026, time.January, 2, 3, 4, 5, 6000, time.UTC)
	diagnostic := api.ExtractionDiagnostic{
		Reader: "gmail", MessageID: "message", RuleName: "Rule", SenderEmail: "sender@example.test",
		ReceivedAt: &received, FailureReasons: []string{"amount_missing"}, Subject: "first",
	}
	if err := st.RecordExtractionDiagnostic(context.Background(), tenant, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic first: %v", err)
	}
	diagnostic.Subject = "second"
	diagnostic.FailureReasons = []string{"merchant_missing"}
	if err := st.RecordExtractionDiagnostic(context.Background(), tenant, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic second: %v", err)
	}
	rows, err := st.ListExtractionDiagnostics(context.Background(), tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 || rows[0].Subject != "second" || !reflect.DeepEqual(rows[0].FailureReasons, diagnostic.FailureReasons) {
		t.Fatalf("diagnostics = %#v", rows)
	}
}
