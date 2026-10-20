package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/storetest"
)

func TestStoreConformance(t *testing.T) {
	db := newTestPostgres(t, nil)
	t.Cleanup(db.cleanup)

	storetest.Run(t, func(t *testing.T) storetest.Subject[store.Backend] {
		t.Helper()
		db.resetSchema(t)
		current := db.newMigratedStore(t)
		setConformanceNow(current)
		return storetest.Subject[store.Backend]{
			Store: current,
			Close: func() { current.Close() },
			Restart: func(t *testing.T) store.Backend {
				t.Helper()
				current.Close()
				current = db.newMigratedStore(t)
				setConformanceNow(current)
				return current
			},
			RejectIngestionMessage: func(t *testing.T, messageID string) {
				t.Helper()
				if messageID != "rollback-reject" {
					t.Fatalf("unsupported rejected ingestion message %q", messageID)
				}
				installIngestionFailureTrigger(t, current)
			},
			Now: storetest.AnalyticsNow(),
		}
	})

	t.Run("RuntimeWithoutSecret", func(t *testing.T) {
		storetest.RunRuntimeWithoutSecret(t, func(t *testing.T) storetest.Subject[storetest.RuntimeSubject] {
			t.Helper()
			db.resetSchema(t)
			st := db.newMigratedStoreWithoutSecret(t)
			return storetest.Subject[storetest.RuntimeSubject]{Store: st, Close: st.Close}
		})
	})

	t.Run("RulesImportAtomicRollback", func(t *testing.T) {
		db.resetSchema(t)
		st := db.newMigratedStore(t)
		t.Cleanup(st.Close)
		testRulesImportAtomicRollback(t, st)
	})

	t.Run("RulesSeedPreservesStoredEdit", func(t *testing.T) {
		db.resetSchema(t)
		st := db.newMigratedStore(t)
		t.Cleanup(st.Close)
		testRulesSeedPreservesStoredEdit(t, st)
	})

	t.Run("CommunityLockedGlobalSeedPreservation", func(t *testing.T) {
		db.resetSchema(t)
		st := db.newMigratedStore(t)
		t.Cleanup(st.Close)
		testCommunityLockedGlobalSeedPreservation(t, st)
	})
}

func setConformanceNow(st *Store) {
	now := func() time.Time { return storetest.AnalyticsNow() }
	st.now = now
	st.analytics.now = now
}

func testRulesSeedPreservesStoredEdit(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	original := store.RuleRow{Name: "Edited Seed Rule", SubjectContains: "seed value", AmountRegex: "amount", MerchantRegex: "merchant"}
	if err := st.SeedPredefinedRules(ctx, []store.RuleRow{original}); err != nil {
		t.Fatalf("SeedPredefinedRules setup: %v", err)
	}
	if _, err := st.pool.Exec(ctx, `UPDATE rules SET subject_contains = 'stored edit' WHERE name = $1 AND predefined = true`, original.Name); err != nil {
		t.Fatalf("edit stored predefined rule: %v", err)
	}
	reseed := original
	reseed.SubjectContains = "new seed value"
	if err := st.SeedPredefinedRules(ctx, []store.RuleRow{reseed}); err != nil {
		t.Fatalf("SeedPredefinedRules after edit: %v", err)
	}
	rules, err := st.ListRules(ctx, store.Tenant{ID: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != 1 || rules[0].SubjectContains != "stored edit" {
		t.Fatalf("reseed overwrote stored edit: %#v", rules)
	}
}

func testRulesImportAtomicRollback(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	user, err := st.CreateUser(ctx, store.CreateUserInput{Email: "rules-rollback@example.test", DisplayName: "Rules Rollback"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tenant := store.Tenant{ID: user.TenantID}
	if err := st.ImportUserRules(ctx, tenant, []store.RuleRow{{Name: "Existing Rule", AmountRegex: "original", MerchantRegex: "merchant"}}); err != nil {
		t.Fatalf("ImportUserRules setup: %v", err)
	}
	if _, err := st.pool.Exec(ctx, `
		CREATE FUNCTION reject_conformance_rule() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.name = 'Reject Import' THEN
				RAISE EXCEPTION 'rejected by conformance trigger';
			END IF;
			RETURN NEW;
		END
		$$;
		CREATE TRIGGER reject_conformance_rule
		BEFORE INSERT OR UPDATE ON rules
		FOR EACH ROW EXECUTE FUNCTION reject_conformance_rule();
	`); err != nil {
		t.Fatalf("install import failure trigger: %v", err)
	}

	err = st.ImportUserRules(ctx, tenant, []store.RuleRow{
		{Name: "Accepted Rule", AmountRegex: "accepted", MerchantRegex: "merchant"},
		{Name: "Existing Rule", AmountRegex: "changed", MerchantRegex: "merchant"},
		{Name: "Reject Import", AmountRegex: "rejected", MerchantRegex: "merchant"},
	})
	if err == nil {
		t.Fatal("ImportUserRules with rejected final row returned nil error")
	}
	rules, err := st.ListRules(ctx, tenant)
	if err != nil {
		t.Fatalf("ListRules after failed import: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "Existing Rule" || rules[0].AmountRegex != "original" {
		t.Fatalf("failed import had side effects: %#v", rules)
	}
}

func testCommunityLockedGlobalSeedPreservation(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	lockedCategory, lockedBucket := "Locked Category", "Locked Bucket"
	if _, err := st.pool.Exec(ctx, `
		INSERT INTO merchant_categories (fragment, category, bucket, source, user_locked)
		VALUES ('Locked Merchant', $1, $2, 'user', true)
	`, lockedCategory, lockedBucket); err != nil {
		t.Fatalf("insert locked global merchant mapping: %v", err)
	}
	replacementCategory, replacementBucket := "Replacement Category", "Replacement Bucket"
	updated, err := st.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{
		Fragment: "Locked Merchant", Category: &replacementCategory, Bucket: &replacementBucket,
	}})
	if err != nil || updated != 0 {
		t.Fatalf("SeedMerchantCategories locked row updated=%d err=%v, want 0 nil", updated, err)
	}
	resolver, err := st.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	category, bucket := resolver("Locked Merchant")
	if category != lockedCategory || bucket != lockedBucket {
		t.Fatalf("locked mapping = (%q, %q), want (%q, %q)", category, bucket, lockedCategory, lockedBucket)
	}
}

func installIngestionFailureTrigger(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := st.pool.Exec(ctx, `
		CREATE FUNCTION reject_conformance_ingestion() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.message_id = 'rollback-reject' THEN
				RAISE EXCEPTION 'rejected by conformance trigger';
			END IF;
			RETURN NEW;
		END
		$$;
		CREATE TRIGGER reject_conformance_ingestion
		BEFORE INSERT OR UPDATE ON transactions
		FOR EACH ROW EXECUTE FUNCTION reject_conformance_ingestion();
	`); err != nil {
		t.Fatalf("install ingestion failure trigger: %v", err)
	}
}
