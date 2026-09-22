package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	stderrors "errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"golang.org/x/text/cases"
	moderncsqlite "modernc.org/sqlite"
)

const (
	testTenantID = "00000000-0000-0000-0000-000000000001"
	testTime     = "2026-09-17T10:11:12.123456Z"
)

type testConnector struct {
	driver *moderncsqlite.Driver
	dsn    string
}

func (c testConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.driver.Open(c.dsn)
}

func (c testConnector) Driver() driver.Driver { return c.driver }

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ownedDriver := &moderncsqlite.Driver{}
	if err := ownedDriver.RegisterDeterministicScalarFunction(
		"expensor_casefold",
		1,
		func(_ *moderncsqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			value, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("case-fold test value has type %T", args[0])
			}
			return cases.Fold().String(value), nil
		},
	); err != nil {
		t.Fatalf("register case-fold function: %v", err)
	}

	db := sql.OpenDB(testConnector{
		driver: ownedDriver,
		dsn:    "file:" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared",
	})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	return db
}

func migrateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t)
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return db
}

func TestRunFirstAndRepeatedRun(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := Run(ctx, db); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if got := userVersion(t, db); got != schemaVersion {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion)
	}
	if got := objectCount(t, db, "table", "users"); got != 1 {
		t.Fatalf("users table count = %d, want 1", got)
	}

	insertUser(t, db, testTenantID)
	if err := Run(ctx, db); err != nil {
		t.Fatalf("repeated Run() error = %v", err)
	}
	var users int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil {
		t.Fatalf("count users after repeated run: %v", err)
	}
	if users != 1 {
		t.Fatalf("users after repeated run = %d, want 1", users)
	}
}

func TestConcurrentRunSerializesVersionCheckWithMigration(t *testing.T) {
	state := newMigrationDriverState(true)
	db := sql.OpenDB(migrationTestConnector{state: state})
	db.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- runMigration(ctx, db, "MIGRATE")
		}()
	}
	close(start)

	for range 2 {
		if err := <-results; err != nil {
			t.Errorf("concurrent run error = %v", err)
		}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.schema || state.version != schemaVersion {
		t.Errorf("migration state = schema:%t version:%d, want schema:true version:%d", state.schema, state.version, schemaVersion)
	}
	if state.outsideVersionReads != 0 {
		t.Errorf("version reads outside immediate transaction = %d, want 0", state.outsideVersionReads)
	}
}

func TestRunRollsBackFailedMigration(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	err := runMigration(ctx, db, `
		CREATE TABLE rollback_probe (id TEXT PRIMARY KEY);
		INSERT INTO missing_table (id) VALUES ('failure');
	`)
	if err == nil {
		t.Fatal("run() error = nil, want failing statement error")
	}
	if got := objectCount(t, db, "table", "rollback_probe"); got != 0 {
		t.Fatalf("rollback_probe table count = %d, want 0", got)
	}
	if got := userVersion(t, db); got != 0 {
		t.Fatalf("user_version after rollback = %d, want 0", got)
	}
}

func TestRunDiscardsConnectionWhenRollbackFails(t *testing.T) {
	state := newMigrationDriverState(false)
	state.rollbackErr = errRollbackProbe
	db := sql.OpenDB(migrationTestConnector{state: state})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	state.cancelMigration = cancel
	defer cancel()
	err := runMigration(ctx, db, "FAIL_MIGRATION")
	if !stderrors.Is(err, errMigrationProbe) {
		t.Fatalf("run() error = %v, want migration cause", err)
	}
	if !stderrors.Is(err, errRollbackProbe) {
		t.Fatalf("run() error = %v, want rollback cause", err)
	}
	if stderrors.Is(err, sql.ErrConnDone) {
		t.Fatalf("run() error includes false close failure: %v", err)
	}
	state.mu.Lock()
	rollbackDeadline := state.rollbackDeadline
	rollbackContextErr := state.rollbackContextErr
	state.mu.Unlock()
	if rollbackDeadline.IsZero() {
		t.Error("rollback context has no deadline")
	} else if remaining := time.Until(rollbackDeadline); remaining <= 0 || remaining > migrationRollbackTimeout {
		t.Errorf("rollback deadline is %v away, want within (0, %v]", remaining, migrationRollbackTimeout)
	}
	if rollbackContextErr != nil {
		t.Errorf("rollback context error = %v, want nil after caller cancellation", rollbackContextErr)
	}

	select {
	case <-state.closed:
	case <-time.After(time.Second):
		t.Fatal("bad physical connection was not closed before deadline")
	}
}

func TestRunSuppressesAlreadyDiscardedConnectionErrors(t *testing.T) {
	tests := []struct {
		name        string
		beginErr    error
		rollbackErr error
		migration   string
	}{
		{name: "begin", beginErr: driver.ErrBadConn, migration: "MIGRATE"},
		{name: "rollback", rollbackErr: driver.ErrBadConn, migration: "FAIL_MIGRATION"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newMigrationDriverState(false)
			state.beginErr = test.beginErr
			state.rollbackErr = test.rollbackErr
			db := sql.OpenDB(migrationTestConnector{state: state})
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = db.Close() })

			err := runMigration(context.Background(), db, test.migration)
			if !stderrors.Is(err, driver.ErrBadConn) {
				t.Fatalf("runMigration error = %v, want driver.ErrBadConn", err)
			}
			if stderrors.Is(err, sql.ErrConnDone) {
				t.Fatalf("runMigration error includes false cleanup failure: %v", err)
			}
		})
	}
}

func TestRunRejectsNewerSchema(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := db.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
		t.Fatalf("set user_version: %v", err)
	}

	if err := Run(ctx, db); err == nil {
		t.Fatal("Run() error = nil, want newer-schema rejection")
	}
	if got := userVersion(t, db); got != 2 {
		t.Fatalf("user_version = %d, want 2", got)
	}
	if got := objectCount(t, db, "table", "users"); got != 0 {
		t.Fatalf("users table count = %d, want 0", got)
	}
}

func TestDownMigrationResetsVersionAndAllowsRebuild(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	downSQL, err := FS.ReadFile("001_init.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	if _, err := db.ExecContext(ctx, string(downSQL)); err != nil {
		t.Fatalf("apply down migration: %v", err)
	}
	if got := userVersion(t, db); got != 0 {
		t.Fatalf("user_version after down migration = %d, want 0", got)
	}
	if got := objectCount(t, db, "table", "users"); got != 0 {
		t.Fatalf("users table count after down migration = %d, want 0", got)
	}

	if err := Run(ctx, db); err != nil {
		t.Fatalf("Run() after down migration error = %v", err)
	}
	if got := userVersion(t, db); got != schemaVersion {
		t.Fatalf("user_version after rebuild = %d, want %d", got, schemaVersion)
	}
	if got := objectCount(t, db, "table", "users"); got != 1 {
		t.Fatalf("users table count after rebuild = %d, want 1", got)
	}
}

type expectedColumn struct {
	name     string
	typeName string
	notNull  bool
}

type actualColumn struct {
	ordinal      int
	name         string
	typeName     string
	notNull      bool
	defaultValue *string
	primaryKey   int
}

func TestBaselineTableColumns(t *testing.T) {
	db := migrateTestDB(t)

	want := map[string][]expectedColumn{
		"users": {
			{"id", "TEXT", true},
			{"email", "TEXT", true},
			{"password_hash", "TEXT", false},
			{"display_name", "TEXT", true},
			{"role", "TEXT", true},
			{"avatar_key", "TEXT", true},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
			{"disabled_at", "TEXT", false},
		},
		"sessions": {
			{"id", "TEXT", true},
			{"user_id", "TEXT", true},
			{"token_hash", "TEXT", true},
			{"created_at", "TEXT", true},
			{"expires_at", "TEXT", true},
			{"last_used_at", "TEXT", false},
			{"revoked_at", "TEXT", false},
		},
		"access_tokens": {
			{"id", "TEXT", true},
			{"user_id", "TEXT", true},
			{"name", "TEXT", true},
			{"token_hash", "TEXT", true},
			{"created_at", "TEXT", true},
			{"expires_at", "TEXT", false},
			{"last_used_at", "TEXT", false},
			{"revoked_at", "TEXT", false},
		},
		"account_setup_tokens": {
			{"id", "TEXT", true},
			{"user_id", "TEXT", true},
			{"token_hash", "TEXT", true},
			{"created_at", "TEXT", true},
			{"expires_at", "TEXT", true},
			{"used_at", "TEXT", false},
		},
		"transactions": {
			{"id", "TEXT", true},
			{"tenant_id", "TEXT", true},
			{"message_id", "TEXT", true},
			{"amount", "INTEGER", true},
			{"currency", "TEXT", true},
			{"original_amount", "INTEGER", false},
			{"original_currency", "TEXT", false},
			{"exchange_rate", "INTEGER", false},
			{"timestamp", "TEXT", true},
			{"merchant_info", "TEXT", true},
			{"category", "TEXT", false},
			{"bucket", "TEXT", false},
			{"source", "TEXT", true},
			{"source_type", "TEXT", true},
			{"source_label", "TEXT", true},
			{"bank", "TEXT", true},
			{"description", "TEXT", false},
			{"metadata", "TEXT", true},
			{"muted", "INTEGER", true},
			{"muted_by_merchant", "INTEGER", true},
			{"mute_reason", "TEXT", false},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
		"transaction_labels": {
			{"id", "TEXT", true}, {"transaction_id", "TEXT", true}, {"label", "TEXT", true}, {"created_at", "TEXT", true},
		},
		"transaction_label_sources": {
			{"id", "TEXT", true},
			{"transaction_id", "TEXT", true},
			{"label", "TEXT", true},
			{"source_type", "TEXT", true},
			{"merchant_pattern", "TEXT", true},
			{"created_at", "TEXT", true},
		},
		"app_config": {{"tenant_id", "TEXT", false}, {"key", "TEXT", true}, {"value", "TEXT", true}},
		"labels": {
			{"tenant_id", "TEXT", true}, {"name", "TEXT", true}, {"color", "TEXT", true}, {"created_at", "TEXT", true},
		},
		"categories": {
			{"tenant_id", "TEXT", false},
			{"name", "TEXT", true},
			{"description", "TEXT", false},
			{"is_default", "INTEGER", true},
			{"created_at", "TEXT", true},
		},
		"buckets": {
			{"tenant_id", "TEXT", false},
			{"name", "TEXT", true},
			{"description", "TEXT", false},
			{"is_default", "INTEGER", true},
			{"created_at", "TEXT", true},
		},
		"rules": {
			{"id", "TEXT", true},
			{"tenant_id", "TEXT", false},
			{"name", "TEXT", true},
			{"sender_email", "TEXT", true},
			{"sender_emails", "TEXT", true},
			{"subject_contains", "TEXT", true},
			{"amount_regex", "TEXT", true},
			{"merchant_regex", "TEXT", true},
			{"currency_regex", "TEXT", true},
			{"transaction_source", "TEXT", true},
			{"source_type", "TEXT", true},
			{"source_label", "TEXT", true},
			{"bank", "TEXT", true},
			{"predefined", "INTEGER", true},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
		"muted_merchants": {
			{"id", "TEXT", true},
			{"tenant_id", "TEXT", true},
			{"pattern", "TEXT", true},
			{"reason", "TEXT", false},
			{"created_at", "TEXT", true},
		},
		"mcc_codes": {
			{"code", "TEXT", true},
			{"description", "TEXT", true},
			{"category", "TEXT", true},
			{"bucket", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
		"merchant_categories": {
			{"id", "TEXT", true},
			{"tenant_id", "TEXT", false},
			{"fragment", "TEXT", true},
			{"mcc_code", "TEXT", false},
			{"category", "TEXT", false},
			{"bucket", "TEXT", false},
			{"source", "TEXT", true},
			{"user_locked", "INTEGER", true},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
		"label_merchants": {
			{"id", "TEXT", true},
			{"tenant_id", "TEXT", true},
			{"label", "TEXT", true},
			{"merchant_pattern", "TEXT", true},
			{"created_at", "TEXT", true},
		},
		"extraction_diagnostics": {
			{"id", "TEXT", true},
			{"tenant_id", "TEXT", true},
			{"status", "TEXT", true},
			{"reader", "TEXT", true},
			{"message_id", "TEXT", false},
			{"source", "TEXT", true},
			{"sender", "TEXT", true},
			{"sender_email", "TEXT", true},
			{"subject", "TEXT", true},
			{"email_body", "TEXT", true},
			{"received_at", "TEXT", false},
			{"snippet", "TEXT", true},
			{"rule_id", "TEXT", false},
			{"rule_name", "TEXT", true},
			{"amount_regex", "TEXT", true},
			{"merchant_regex", "TEXT", true},
			{"currency_regex", "TEXT", true},
			{"failure_reasons", "TEXT", true},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
			{"resolved_at", "TEXT", false},
		},
		"reader_runtime": {
			{"tenant_id", "TEXT", true},
			{"reader", "TEXT", true},
			{"client_secret_ciphertext", "BLOB", false},
			{"oauth_token_ciphertext", "BLOB", false},
			{"config", "TEXT", false},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
		"processed_messages": {
			{"tenant_id", "TEXT", true}, {"message_key", "TEXT", true}, {"processed_at", "TEXT", true},
		},
		"scheduler_config": {
			{"id", "INTEGER", true}, {"max_concurrent_scans", "INTEGER", true}, {"created_at", "TEXT", true}, {"updated_at", "TEXT", true},
		},
		"tenant_scanning_state": {
			{"tenant_id", "TEXT", true},
			{"active_reader", "TEXT", true},
			{"enabled", "INTEGER", true},
			{"state", "TEXT", true},
			{"reason_code", "TEXT", true},
			{"public_message", "TEXT", true},
			{"last_started_at", "TEXT", false},
			{"last_stopped_at", "TEXT", false},
			{"last_failed_at", "TEXT", false},
			{"next_retry_at", "TEXT", false},
			{"retry_count", "INTEGER", true},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
		"llm_provider_runtime": {
			{"tenant_id", "TEXT", true},
			{"provider", "TEXT", true},
			{"config", "TEXT", true},
			{"credentials_ciphertext", "BLOB", false},
			{"active", "INTEGER", true},
			{"created_at", "TEXT", true},
			{"updated_at", "TEXT", true},
		},
	}
	wantDefaults := expectedColumnDefaults()
	wantPrimaryKeys := map[string]int{
		"users.id": 1, "sessions.id": 1, "access_tokens.id": 1, "account_setup_tokens.id": 1,
		"transactions.id": 1, "transaction_labels.id": 1, "transaction_label_sources.id": 1,
		"rules.id": 1, "muted_merchants.id": 1, "mcc_codes.code": 1, "merchant_categories.id": 1,
		"label_merchants.id": 1, "extraction_diagnostics.id": 1, "scheduler_config.id": 1,
		"tenant_scanning_state.tenant_id": 1,
	}

	for table, columns := range want {
		t.Run(table, func(t *testing.T) {
			got := tableColumns(t, db, table)
			if len(got) != len(columns) {
				t.Fatalf("PRAGMA table_info(%s) column count = %d, want %d: %v", table, len(got), len(columns), got)
			}
			for ordinal, column := range columns {
				actual := got[ordinal]
				key := table + "." + column.name
				if actual.ordinal != ordinal || actual.name != column.name || actual.typeName != column.typeName || actual.notNull != column.notNull ||
					!equalOptionalString(actual.defaultValue, wantDefaults[key]) || actual.primaryKey != wantPrimaryKeys[key] {
					t.Errorf("PRAGMA table_info(%s) column %d = %+v, want name=%q type=%q notNull=%t default=%v primaryKey=%d",
						table, ordinal, actual, column.name, column.typeName, column.notNull, wantDefaults[key], wantPrimaryKeys[key])
				}
			}
		})
	}

	if got := objectCount(t, db, "table", "transactions_fts"); got != 1 {
		t.Fatalf("transactions_fts count = %d, want 1", got)
	}
}

type expectedForeignKey struct {
	id       int
	sequence int
	from     string
	table    string
	to       string
	onUpdate string
	onDelete string
	match    string
}

func foreignKey(id int, from, table, to, onDelete string) expectedForeignKey {
	return expectedForeignKey{
		id: id, from: from, table: table, to: to,
		onUpdate: "NO ACTION", onDelete: onDelete, match: "NONE",
	}
}

func TestBaselineOrdinaryTableInventory(t *testing.T) {
	db := migrateTestDB(t)
	rows, err := db.QueryContext(context.Background(), `
		SELECT name
		FROM sqlite_master
		WHERE type='table'
		  AND name NOT LIKE 'sqlite_%'
		  AND name <> 'transactions_fts'
		  AND name NOT GLOB 'transactions_fts_*'
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("list ordinary tables: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan ordinary table name: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate ordinary tables: %v", err)
	}
	want := []string{
		"access_tokens", "account_setup_tokens", "app_config", "buckets", "categories", "extraction_diagnostics",
		"label_merchants", "labels", "llm_provider_runtime", "mcc_codes", "merchant_categories", "muted_merchants",
		"processed_messages", "reader_runtime", "rules", "scheduler_config", "sessions", "tenant_scanning_state",
		"transaction_label_sources", "transaction_labels", "transactions", "users",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ordinary table inventory = %v, want %v", got, want)
	}
}

func TestBaselineForeignKeysAndCascades(t *testing.T) {
	db := migrateTestDB(t)

	want := map[string][]expectedForeignKey{
		"sessions":                  {foreignKey(0, "user_id", "users", "id", "CASCADE")},
		"access_tokens":             {foreignKey(0, "user_id", "users", "id", "CASCADE")},
		"account_setup_tokens":      {foreignKey(0, "user_id", "users", "id", "CASCADE")},
		"transactions":              {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"transaction_labels":        {foreignKey(0, "transaction_id", "transactions", "id", "CASCADE")},
		"transaction_label_sources": {foreignKey(0, "transaction_id", "transactions", "id", "CASCADE")},
		"app_config":                {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"labels":                    {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"categories":                {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"buckets":                   {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"rules":                     {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"muted_merchants":           {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"merchant_categories": {
			foreignKey(0, "mcc_code", "mcc_codes", "code", "SET NULL"),
			foreignKey(1, "tenant_id", "users", "id", "CASCADE"),
		},
		"label_merchants": {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"extraction_diagnostics": {
			foreignKey(0, "rule_id", "rules", "id", "SET NULL"),
			foreignKey(1, "tenant_id", "users", "id", "CASCADE"),
		},
		"reader_runtime":        {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"processed_messages":    {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"tenant_scanning_state": {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
		"llm_provider_runtime":  {foreignKey(0, "tenant_id", "users", "id", "CASCADE")},
	}

	allTables := []string{
		"users", "sessions", "access_tokens", "account_setup_tokens", "transactions", "transaction_labels",
		"transaction_label_sources", "app_config", "labels", "categories", "buckets", "rules", "muted_merchants",
		"mcc_codes", "merchant_categories", "label_merchants", "extraction_diagnostics", "reader_runtime",
		"processed_messages", "scheduler_config", "tenant_scanning_state", "llm_provider_runtime",
	}
	for _, table := range allTables {
		t.Run(table, func(t *testing.T) {
			got := foreignKeys(t, db, table)
			expected := want[table]
			if fmt.Sprint(got) != fmt.Sprint(expected) {
				t.Fatalf("PRAGMA foreign_key_list(%s) = %v, want %v", table, got, expected)
			}
		})
	}
}

type expectedIndex struct {
	name    string
	unique  bool
	origin  string
	partial bool
	targets []indexTarget
	sql     string
}

type indexTarget struct {
	name string
	desc bool
}

func TestBaselineIndexes(t *testing.T) {
	db := migrateTestDB(t)
	want := expectedIndexes()

	for table, expected := range want {
		t.Run(table, func(t *testing.T) {
			got := indexes(t, db, table)
			if fmt.Sprint(got) != fmt.Sprint(expected) {
				t.Fatalf("complete index metadata for %s =\n%#v\nwant\n%#v", table, got, expected)
			}
		})
	}
}

func TestBaselineCompleteChecksAndTableOptions(t *testing.T) {
	db := migrateTestDB(t)
	want := expectedChecks()

	for table, expectedChecks := range want {
		t.Run(table, func(t *testing.T) {
			schema := schemaSQL(t, db, "table", table)
			canonical := canonicalSQL(schema)
			if !strings.HasPrefix(canonical, "createtable"+table+"(") || !strings.HasSuffix(canonical, ")strict") {
				t.Errorf("%s table declaration or options changed: %s", table, schema)
			}
			gotChecks := extractCheckExpressions(t, schema)
			if fmt.Sprint(gotChecks) != fmt.Sprint(expectedChecks) {
				t.Errorf("complete CHECK constraints for %s =\n%v\nwant\n%v", table, gotChecks, expectedChecks)
			}
		})
	}

	ftsSchema := canonicalSQL(schemaSQL(t, db, "table", "transactions_fts"))
	wantFTS := "createvirtualtabletransactions_ftsusingfts5(transaction_idunindexed,merchant,description)"
	if ftsSchema != wantFTS {
		t.Errorf("transactions_fts DDL = %q, want %q", ftsSchema, wantFTS)
	}
}

func expectedColumnDefaults() map[string]*string {
	timestampDefault := "strftime('%Y-%m-%dT%H:%M:%f000Z','now')"
	values := map[string]string{
		"users.avatar_key": "'default'", "users.created_at": timestampDefault, "users.updated_at": timestampDefault,
		"sessions.created_at":             timestampDefault,
		"access_tokens.created_at":        timestampDefault,
		"account_setup_tokens.created_at": timestampDefault,
		"transactions.currency":           "'INR'", "transactions.source_type": "''", "transactions.source_label": "''",
		"transactions.bank": "''", "transactions.metadata": "'{}'", "transactions.muted": "0",
		"transactions.muted_by_merchant": "0", "transactions.created_at": timestampDefault, "transactions.updated_at": timestampDefault,
		"transaction_labels.created_at":              timestampDefault,
		"transaction_label_sources.merchant_pattern": "''", "transaction_label_sources.created_at": timestampDefault,
		"labels.color": "'#6366f1'", "labels.created_at": timestampDefault,
		"categories.is_default": "0", "categories.created_at": timestampDefault,
		"buckets.is_default": "0", "buckets.created_at": timestampDefault,
		"rules.sender_email": "''", "rules.sender_emails": "'[]'", "rules.subject_contains": "''",
		"rules.currency_regex": "''", "rules.transaction_source": "''", "rules.source_type": "''",
		"rules.source_label": "''", "rules.bank": "''", "rules.predefined": "0",
		"rules.created_at": timestampDefault, "rules.updated_at": timestampDefault,
		"muted_merchants.created_at": timestampDefault,
		"mcc_codes.bucket":           "'Wants'", "mcc_codes.updated_at": timestampDefault,
		"merchant_categories.source": "'community'", "merchant_categories.user_locked": "0",
		"merchant_categories.created_at": timestampDefault, "merchant_categories.updated_at": timestampDefault,
		"label_merchants.created_at":    timestampDefault,
		"extraction_diagnostics.status": "'open'", "extraction_diagnostics.source": "''",
		"extraction_diagnostics.sender": "''", "extraction_diagnostics.sender_email": "''",
		"extraction_diagnostics.subject": "''", "extraction_diagnostics.email_body": "''",
		"extraction_diagnostics.snippet": "''", "extraction_diagnostics.rule_name": "''",
		"extraction_diagnostics.amount_regex": "''", "extraction_diagnostics.merchant_regex": "''",
		"extraction_diagnostics.currency_regex": "''", "extraction_diagnostics.failure_reasons": "'[]'",
		"extraction_diagnostics.created_at": timestampDefault, "extraction_diagnostics.updated_at": timestampDefault,
		"reader_runtime.created_at": timestampDefault, "reader_runtime.updated_at": timestampDefault,
		"processed_messages.processed_at": timestampDefault,
		"scheduler_config.id":             "1", "scheduler_config.max_concurrent_scans": "4",
		"scheduler_config.created_at": timestampDefault, "scheduler_config.updated_at": timestampDefault,
		"tenant_scanning_state.active_reader": "''", "tenant_scanning_state.enabled": "1",
		"tenant_scanning_state.state": "'stopped'", "tenant_scanning_state.reason_code": "''",
		"tenant_scanning_state.public_message": "''", "tenant_scanning_state.retry_count": "0",
		"tenant_scanning_state.created_at": timestampDefault, "tenant_scanning_state.updated_at": timestampDefault,
		"llm_provider_runtime.config": "'{}'", "llm_provider_runtime.active": "0",
		"llm_provider_runtime.created_at": timestampDefault, "llm_provider_runtime.updated_at": timestampDefault,
	}
	result := make(map[string]*string, len(values))
	for key, value := range values {
		value := value
		result[key] = &value
	}
	return result
}

func expectedChecks() map[string][]string {
	timestamp := func(column string) string { return canonicalSQL(timestampValidation(column)) }
	nullableTimestamp := func(column string) string {
		return canonicalSQL(column + " IS NULL OR (" + timestampValidation(column) + ")")
	}
	checks := map[string][]string{
		"users":                {canonicalSQL("role IN ('admin','user')"), timestamp("created_at"), timestamp("updated_at"), nullableTimestamp("disabled_at")},
		"sessions":             {timestamp("created_at"), timestamp("expires_at"), nullableTimestamp("last_used_at"), nullableTimestamp("revoked_at")},
		"access_tokens":        {timestamp("created_at"), nullableTimestamp("expires_at"), nullableTimestamp("last_used_at"), nullableTimestamp("revoked_at")},
		"account_setup_tokens": {timestamp("created_at"), timestamp("expires_at"), nullableTimestamp("used_at")},
		"transactions": {
			timestamp("timestamp"), canonicalSQL("metadata IS NULL OR json_valid(metadata)"), canonicalSQL("muted IN (0,1)"),
			canonicalSQL("muted_by_merchant IN (0,1)"), timestamp("created_at"), timestamp("updated_at"),
		},
		"transaction_labels":        {timestamp("created_at")},
		"transaction_label_sources": {canonicalSQL("source_type IN ('manual','merchant')"), timestamp("created_at")},
		"app_config":                {},
		"labels":                    {timestamp("created_at")},
		"categories":                {canonicalSQL("is_default IN (0,1)"), timestamp("created_at")},
		"buckets":                   {canonicalSQL("is_default IN (0,1)"), timestamp("created_at")},
		"rules": {
			canonicalSQL("json_valid(sender_emails) AND json_type(sender_emails)='array'"), canonicalSQL("predefined IN (0,1)"),
			timestamp("created_at"), timestamp("updated_at"),
			canonicalSQL("(predefined=1 AND tenant_id IS NULL) OR (predefined=0 AND tenant_id IS NOT NULL)"),
		},
		"muted_merchants":     {timestamp("created_at")},
		"mcc_codes":           {timestamp("updated_at")},
		"merchant_categories": {canonicalSQL("user_locked IN (0,1)"), timestamp("created_at"), timestamp("updated_at")},
		"label_merchants":     {timestamp("created_at")},
		"extraction_diagnostics": {
			canonicalSQL("status IN ('open','resolved','ignored')"), nullableTimestamp("received_at"),
			canonicalSQL("json_valid(failure_reasons) AND json_type(failure_reasons)='array'"),
			timestamp("created_at"), timestamp("updated_at"), nullableTimestamp("resolved_at"),
		},
		"reader_runtime":     {canonicalSQL("config IS NULL OR json_valid(config)"), timestamp("created_at"), timestamp("updated_at")},
		"processed_messages": {timestamp("processed_at")},
		"scheduler_config":   {canonicalSQL("id=1"), canonicalSQL("max_concurrent_scans BETWEEN 1 AND 64"), timestamp("created_at"), timestamp("updated_at")},
		"tenant_scanning_state": {
			canonicalSQL("enabled IN (0,1)"),
			canonicalSQL("state IN ('queued','starting','running','backing_off','needs_auth','reader_not_configured','paused','stopped')"),
			nullableTimestamp("last_started_at"), nullableTimestamp("last_stopped_at"), nullableTimestamp("last_failed_at"),
			nullableTimestamp("next_retry_at"), canonicalSQL("retry_count>=0"), timestamp("created_at"), timestamp("updated_at"),
		},
		"llm_provider_runtime": {canonicalSQL("json_valid(config)"), canonicalSQL("active IN (0,1)"), timestamp("created_at"), timestamp("updated_at")},
	}
	return checks
}

const timestampPattern = "'[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'"

func timestampValidation(column string) string {
	return column + " GLOB " + timestampPattern +
		" AND strftime('%Y-%m-%dT%H:%M:%S'," + column + ",'+0 days') IS substr(" + column + ",1,19)"
}

func expectedIndexes() map[string][]expectedIndex {
	pk := func(table, column string) expectedIndex {
		return expectedIndex{name: "sqlite_autoindex_" + table + "_1", unique: true, origin: "pk", targets: []indexTarget{{name: column}}}
	}
	idx := func(name string, unique, partial bool, definition string, targets ...indexTarget) expectedIndex {
		return expectedIndex{name: name, unique: unique, origin: "c", partial: partial, targets: targets, sql: canonicalSQL(definition)}
	}
	t := func(name string) indexTarget { return indexTarget{name: name} }
	desc := func(name string) indexTarget { return indexTarget{name: name, desc: true} }
	expr := func() indexTarget { return indexTarget{} }

	result := map[string][]expectedIndex{
		"users": {
			pk("users", "id"),
			idx("users_email_casefold_unique", true, false, "CREATE UNIQUE INDEX users_email_casefold_unique ON users (expensor_casefold(email))", expr()),
		},
		"sessions": {
			pk("sessions", "id"),
			idx("sessions_active_lookup_idx", false, true, "CREATE INDEX sessions_active_lookup_idx ON sessions (token_hash) WHERE revoked_at IS NULL", t("token_hash")),
			idx("sessions_token_hash_unique", true, false, "CREATE UNIQUE INDEX sessions_token_hash_unique ON sessions (token_hash)", t("token_hash")),
			idx("sessions_user_id_idx", false, false, "CREATE INDEX sessions_user_id_idx ON sessions (user_id)", t("user_id")),
		},
		"access_tokens": {
			pk("access_tokens", "id"),
			idx("access_tokens_active_lookup_idx", false, true, "CREATE INDEX access_tokens_active_lookup_idx ON access_tokens (token_hash) WHERE revoked_at IS NULL", t("token_hash")),
			idx("access_tokens_active_name_unique", true, true, "CREATE UNIQUE INDEX access_tokens_active_name_unique ON access_tokens (user_id,name) WHERE revoked_at IS NULL", t("user_id"), t("name")),
			idx("access_tokens_token_hash_unique", true, false, "CREATE UNIQUE INDEX access_tokens_token_hash_unique ON access_tokens (token_hash)", t("token_hash")),
			idx("access_tokens_user_id_idx", false, false, "CREATE INDEX access_tokens_user_id_idx ON access_tokens (user_id)", t("user_id")),
		},
		"account_setup_tokens": {
			pk("account_setup_tokens", "id"),
			idx("account_setup_tokens_token_hash_unique", true, false, "CREATE UNIQUE INDEX account_setup_tokens_token_hash_unique ON account_setup_tokens (token_hash)", t("token_hash")),
			idx("account_setup_tokens_user_id_idx", false, false, "CREATE INDEX account_setup_tokens_user_id_idx ON account_setup_tokens (user_id)", t("user_id")),
		},
		"transactions": {
			pk("transactions", "id"),
			idx("transactions_currency_idx", false, false, "CREATE INDEX transactions_currency_idx ON transactions (currency)", t("currency")),
			idx("transactions_muted_by_merchant_idx", false, true, "CREATE INDEX transactions_muted_by_merchant_idx ON transactions (tenant_id,muted_by_merchant) WHERE muted_by_merchant=1", t("tenant_id"), t("muted_by_merchant")),
			idx("transactions_muted_idx", false, true, "CREATE INDEX transactions_muted_idx ON transactions (tenant_id,muted) WHERE muted=1", t("tenant_id"), t("muted")),
			idx("transactions_tenant_bucket_idx", false, false, "CREATE INDEX transactions_tenant_bucket_idx ON transactions (tenant_id,bucket)", t("tenant_id"), t("bucket")),
			idx("transactions_tenant_category_idx", false, false, "CREATE INDEX transactions_tenant_category_idx ON transactions (tenant_id,category)", t("tenant_id"), t("category")),
			idx("transactions_tenant_message_id_key", true, true, "CREATE UNIQUE INDEX transactions_tenant_message_id_key ON transactions (tenant_id,message_id) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("message_id")),
			idx("transactions_tenant_timestamp_idx", false, false, "CREATE INDEX transactions_tenant_timestamp_idx ON transactions (tenant_id,timestamp DESC)", t("tenant_id"), desc("timestamp")),
		},
		"transaction_labels": {
			pk("transaction_labels", "id"),
			idx("transaction_labels_label_idx", false, false, "CREATE INDEX transaction_labels_label_idx ON transaction_labels (label)", t("label")),
			idx("transaction_labels_transaction_id_idx", false, false, "CREATE INDEX transaction_labels_transaction_id_idx ON transaction_labels (transaction_id)", t("transaction_id")),
			idx("transaction_labels_unique", true, false, "CREATE UNIQUE INDEX transaction_labels_unique ON transaction_labels (transaction_id,label)", t("transaction_id"), t("label")),
		},
		"transaction_label_sources": {
			pk("transaction_label_sources", "id"),
			idx("transaction_label_sources_transaction_idx", false, false, "CREATE INDEX transaction_label_sources_transaction_idx ON transaction_label_sources (transaction_id,label)", t("transaction_id"), t("label")),
			idx("transaction_label_sources_unique", true, false, "CREATE UNIQUE INDEX transaction_label_sources_unique ON transaction_label_sources (transaction_id,label,source_type,merchant_pattern)", t("transaction_id"), t("label"), t("source_type"), t("merchant_pattern")),
		},
		"app_config": {
			idx("app_config_global_key", true, true, "CREATE UNIQUE INDEX app_config_global_key ON app_config (key) WHERE tenant_id IS NULL", t("key")),
			idx("app_config_tenant_key", true, true, "CREATE UNIQUE INDEX app_config_tenant_key ON app_config (tenant_id,key) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("key")),
		},
		"labels": {idx("labels_tenant_name_key", true, true, "CREATE UNIQUE INDEX labels_tenant_name_key ON labels (tenant_id,name) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("name"))},
		"categories": {
			idx("categories_global_name_key", true, true, "CREATE UNIQUE INDEX categories_global_name_key ON categories (name) WHERE tenant_id IS NULL", t("name")),
			idx("categories_tenant_name_key", true, true, "CREATE UNIQUE INDEX categories_tenant_name_key ON categories (tenant_id,name) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("name")),
		},
		"buckets": {
			idx("buckets_global_name_key", true, true, "CREATE UNIQUE INDEX buckets_global_name_key ON buckets (name) WHERE tenant_id IS NULL", t("name")),
			idx("buckets_tenant_name_key", true, true, "CREATE UNIQUE INDEX buckets_tenant_name_key ON buckets (tenant_id,name) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("name")),
		},
		"rules": {
			pk("rules", "id"),
			idx("rules_predefined_name_key", true, true, "CREATE UNIQUE INDEX rules_predefined_name_key ON rules (name) WHERE tenant_id IS NULL AND predefined=1", t("name")),
			idx("rules_tenant_user_name_key", true, true, "CREATE UNIQUE INDEX rules_tenant_user_name_key ON rules (tenant_id,name) WHERE tenant_id IS NOT NULL AND predefined=0", t("tenant_id"), t("name")),
		},
		"muted_merchants": {
			pk("muted_merchants", "id"),
			idx("muted_merchants_tenant_pattern_key", true, true, "CREATE UNIQUE INDEX muted_merchants_tenant_pattern_key ON muted_merchants (tenant_id,pattern) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("pattern")),
		},
		"mcc_codes": {pk("mcc_codes", "code")},
		"merchant_categories": {
			pk("merchant_categories", "id"),
			idx("merchant_categories_global_fragment_key", true, true, "CREATE UNIQUE INDEX merchant_categories_global_fragment_key ON merchant_categories (fragment) WHERE tenant_id IS NULL", t("fragment")),
			idx("merchant_categories_tenant_fragment_key", true, true, "CREATE UNIQUE INDEX merchant_categories_tenant_fragment_key ON merchant_categories (tenant_id,fragment) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("fragment")),
		},
		"label_merchants": {
			pk("label_merchants", "id"),
			idx("label_merchants_tenant_mapping_key", true, true, "CREATE UNIQUE INDEX label_merchants_tenant_mapping_key ON label_merchants (tenant_id,label,merchant_pattern) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("label"), t("merchant_pattern")),
		},
		"extraction_diagnostics": {
			pk("extraction_diagnostics", "id"),
			idx("extraction_diagnostics_open_tenant_unique", true, true, "CREATE UNIQUE INDEX extraction_diagnostics_open_tenant_unique ON extraction_diagnostics (tenant_id,reader,message_id,rule_name) WHERE tenant_id IS NOT NULL AND status='open' AND message_id IS NOT NULL", t("tenant_id"), t("reader"), t("message_id"), t("rule_name")),
			idx("extraction_diagnostics_tenant_status_created_idx", false, false, "CREATE INDEX extraction_diagnostics_tenant_status_created_idx ON extraction_diagnostics (tenant_id,status,created_at DESC)", t("tenant_id"), t("status"), desc("created_at")),
		},
		"reader_runtime":     {idx("reader_runtime_tenant_reader_key", true, true, "CREATE UNIQUE INDEX reader_runtime_tenant_reader_key ON reader_runtime (tenant_id,reader) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("reader"))},
		"processed_messages": {idx("processed_messages_tenant_key", true, true, "CREATE UNIQUE INDEX processed_messages_tenant_key ON processed_messages (tenant_id,message_key) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("message_key"))},
		"scheduler_config":   {},
		"tenant_scanning_state": {
			pk("tenant_scanning_state", "tenant_id"),
			idx("tenant_scanning_state_runnable_idx", false, false, "CREATE INDEX tenant_scanning_state_runnable_idx ON tenant_scanning_state (enabled,state,next_retry_at)", t("enabled"), t("state"), t("next_retry_at")),
		},
		"llm_provider_runtime": {
			idx("llm_provider_runtime_tenant_active_key", true, true, "CREATE UNIQUE INDEX llm_provider_runtime_tenant_active_key ON llm_provider_runtime (tenant_id) WHERE tenant_id IS NOT NULL AND active=1", t("tenant_id")),
			idx("llm_provider_runtime_tenant_provider_key", true, true, "CREATE UNIQUE INDEX llm_provider_runtime_tenant_provider_key ON llm_provider_runtime (tenant_id,provider) WHERE tenant_id IS NOT NULL", t("tenant_id"), t("provider")),
		},
	}
	for table := range result {
		sort.Slice(result[table], func(i, j int) bool { return result[table][i].name < result[table][j].name })
	}
	return result
}

func TestPartialUniqueIndexesMatchUpsertTargets(t *testing.T) {
	tests := []struct {
		name  string
		query string
		args  func(id string) []any
		count string
	}{
		{"transaction tenant message", `INSERT INTO transactions (id,tenant_id,message_id,amount,timestamp,merchant_info,source) VALUES (?,?,?,10000,?,'merchant','source') ON CONFLICT (tenant_id,message_id) WHERE tenant_id IS NOT NULL DO UPDATE SET amount=excluded.amount`, func(id string) []any { return []any{id, testTenantID, "message", testTime} }, "transactions"},
		{"tenant app config", `INSERT INTO app_config (tenant_id,key,value) VALUES (?,?,'value') ON CONFLICT (tenant_id,key) WHERE tenant_id IS NOT NULL DO UPDATE SET value=excluded.value`, func(string) []any { return []any{testTenantID, "key"} }, "app_config WHERE tenant_id IS NOT NULL"},
		{"global app config", `INSERT INTO app_config (tenant_id,key,value) VALUES (NULL,?,'value') ON CONFLICT (key) WHERE tenant_id IS NULL DO UPDATE SET value=excluded.value`, func(string) []any { return []any{"global-key"} }, "app_config WHERE tenant_id IS NULL"},
		{"tenant label", `INSERT INTO labels (tenant_id,name,color) VALUES (?,?,'#000000') ON CONFLICT (tenant_id,name) WHERE tenant_id IS NOT NULL DO NOTHING`, func(string) []any { return []any{testTenantID, "label"} }, "labels"},
		{"tenant category", `INSERT INTO categories (tenant_id,name) VALUES (?,?) ON CONFLICT (tenant_id,name) WHERE tenant_id IS NOT NULL DO NOTHING`, func(string) []any { return []any{testTenantID, "category"} }, "categories WHERE tenant_id IS NOT NULL"},
		{"global category", `INSERT INTO categories (tenant_id,name,is_default) VALUES (NULL,?,1) ON CONFLICT (name) WHERE tenant_id IS NULL DO NOTHING`, func(string) []any { return []any{"global-category"} }, "categories WHERE tenant_id IS NULL AND name='global-category'"},
		{"tenant bucket", `INSERT INTO buckets (tenant_id,name) VALUES (?,?) ON CONFLICT (tenant_id,name) WHERE tenant_id IS NOT NULL DO NOTHING`, func(string) []any { return []any{testTenantID, "bucket"} }, "buckets WHERE tenant_id IS NOT NULL"},
		{"global bucket", `INSERT INTO buckets (tenant_id,name,is_default) VALUES (NULL,?,1) ON CONFLICT (name) WHERE tenant_id IS NULL DO NOTHING`, func(string) []any { return []any{"global-bucket"} }, "buckets WHERE tenant_id IS NULL AND name='global-bucket'"},
		{"predefined rule", `INSERT INTO rules (id,name,amount_regex,merchant_regex,predefined) VALUES (?,?,'amount','merchant',1) ON CONFLICT (name) WHERE tenant_id IS NULL AND predefined=1 DO NOTHING`, func(id string) []any { return []any{id, "predefined"} }, "rules WHERE tenant_id IS NULL"},
		{"tenant user rule", `INSERT INTO rules (id,tenant_id,name,amount_regex,merchant_regex) VALUES (?,?,?,'amount','merchant') ON CONFLICT (tenant_id,name) WHERE tenant_id IS NOT NULL AND predefined=0 DO UPDATE SET amount_regex=excluded.amount_regex`, func(id string) []any { return []any{id, testTenantID, "user-rule"} }, "rules WHERE tenant_id IS NOT NULL"},
		{"muted merchant", `INSERT INTO muted_merchants (id,tenant_id,pattern) VALUES (?,?,?) ON CONFLICT (tenant_id,pattern) WHERE tenant_id IS NOT NULL DO UPDATE SET reason=excluded.reason`, func(id string) []any { return []any{id, testTenantID, "merchant"} }, "muted_merchants"},
		{"global merchant category", `INSERT INTO merchant_categories (id,fragment) VALUES (?,?) ON CONFLICT (fragment) WHERE tenant_id IS NULL DO UPDATE SET updated_at=excluded.updated_at`, func(id string) []any { return []any{id, "global-fragment"} }, "merchant_categories WHERE tenant_id IS NULL"},
		{"tenant merchant category", `INSERT INTO merchant_categories (id,tenant_id,fragment) VALUES (?,?,?) ON CONFLICT (tenant_id,fragment) WHERE tenant_id IS NOT NULL DO UPDATE SET updated_at=excluded.updated_at`, func(id string) []any { return []any{id, testTenantID, "tenant-fragment"} }, "merchant_categories WHERE tenant_id IS NOT NULL"},
		{"label merchant", `INSERT INTO label_merchants (id,tenant_id,label,merchant_pattern) VALUES (?,?,?,?) ON CONFLICT (tenant_id,label,merchant_pattern) WHERE tenant_id IS NOT NULL DO NOTHING`, func(id string) []any { return []any{id, testTenantID, "label", "pattern"} }, "label_merchants"},
		{"open diagnostic", `INSERT INTO extraction_diagnostics (id,tenant_id,reader,message_id,rule_name) VALUES (?,?,?,?,?) ON CONFLICT (tenant_id,reader,message_id,rule_name) WHERE tenant_id IS NOT NULL AND status='open' AND message_id IS NOT NULL DO UPDATE SET updated_at=excluded.updated_at`, func(id string) []any { return []any{id, testTenantID, "reader", "message", "rule"} }, "extraction_diagnostics"},
		{"reader runtime", `INSERT INTO reader_runtime (tenant_id,reader,config) VALUES (?,?,?) ON CONFLICT (tenant_id,reader) WHERE tenant_id IS NOT NULL DO UPDATE SET config=excluded.config`, func(string) []any { return []any{testTenantID, "reader", `{}`} }, "reader_runtime"},
		{"processed message", `INSERT INTO processed_messages (tenant_id,message_key,processed_at) VALUES (?,?,?) ON CONFLICT (tenant_id,message_key) WHERE tenant_id IS NOT NULL DO UPDATE SET processed_at=excluded.processed_at`, func(string) []any { return []any{testTenantID, "message", testTime} }, "processed_messages"},
		{"llm provider", `INSERT INTO llm_provider_runtime (tenant_id,provider,config) VALUES (?,?,?) ON CONFLICT (tenant_id,provider) WHERE tenant_id IS NOT NULL DO UPDATE SET config=excluded.config`, func(string) []any { return []any{testTenantID, "provider", `{}`} }, "llm_provider_runtime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := migrateTestDB(t)
			insertUser(t, db, testTenantID)
			for _, id := range []string{"00000000-0000-0000-0000-000000000101", "00000000-0000-0000-0000-000000000102"} {
				if _, err := db.ExecContext(context.Background(), tt.query, tt.args(id)...); err != nil {
					t.Fatalf("execute upsert: %v", err)
				}
			}
			var count int
			if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM "+tt.count).Scan(&count); err != nil {
				t.Fatalf("count conflict rows: %v", err)
			}
			if count != 1 {
				t.Fatalf("conflict row count = %d, want 1", count)
			}
		})
	}
}

func TestTransactionFTSTriggers(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	insertUser(t, db, testTenantID)

	const transactionID = "00000000-0000-0000-0000-000000000201"
	if _, err := db.ExecContext(ctx, `
		INSERT INTO transactions (id,tenant_id,message_id,amount,timestamp,merchant_info,source,description)
		VALUES (?,?,?,?,?,?,?,?)
	`, transactionID, testTenantID, "message", 123450, testTime, "First Merchant", "source", "First description"); err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	assertFTSRow(t, db, transactionID, "First Merchant", "First description")

	if _, err := db.ExecContext(ctx, "UPDATE transactions SET merchant_info=? WHERE id=?", "Second Merchant", transactionID); err != nil {
		t.Fatalf("update merchant: %v", err)
	}
	assertFTSRow(t, db, transactionID, "Second Merchant", "First description")

	if _, err := db.ExecContext(ctx, "UPDATE transactions SET description=? WHERE id=?", "Second description", transactionID); err != nil {
		t.Fatalf("update description: %v", err)
	}
	assertFTSRow(t, db, transactionID, "Second Merchant", "Second description")

	if _, err := db.ExecContext(ctx, "DELETE FROM transactions WHERE id=?", transactionID); err != nil {
		t.Fatalf("delete transaction: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM transactions_fts WHERE transaction_id=?", transactionID).Scan(&count); err != nil {
		t.Fatalf("count FTS rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("FTS rows after delete = %d, want 0", count)
	}

	wantTriggers := map[string]string{
		"transactions_fts_insert": canonicalSQL(`CREATE TRIGGER transactions_fts_insert AFTER INSERT ON transactions BEGIN
			INSERT INTO transactions_fts (transaction_id,merchant,description)
			VALUES (new.id,new.merchant_info,coalesce(new.description,''));
		END`),
		"transactions_fts_update": canonicalSQL(`CREATE TRIGGER transactions_fts_update AFTER UPDATE OF merchant_info,description ON transactions BEGIN
			DELETE FROM transactions_fts WHERE transaction_id=old.id;
			INSERT INTO transactions_fts (transaction_id,merchant,description)
			VALUES (new.id,new.merchant_info,coalesce(new.description,''));
		END`),
		"transactions_fts_delete": canonicalSQL(`CREATE TRIGGER transactions_fts_delete AFTER DELETE ON transactions BEGIN
			DELETE FROM transactions_fts WHERE transaction_id=old.id;
		END`),
	}
	for trigger, expectedSQL := range wantTriggers {
		if got := objectCount(t, db, "trigger", trigger); got != 1 {
			t.Errorf("trigger %s count = %d, want 1", trigger, got)
		}
		if got := canonicalSQL(schemaSQL(t, db, "trigger", trigger)); got != expectedSQL {
			t.Errorf("trigger %s DDL = %q, want %q", trigger, got, expectedSQL)
		}
	}
}

func TestTimestampConstraintsRequireValidUTCInstants(t *testing.T) {
	db := migrateTestDB(t)
	insertUser(t, db, testTenantID)

	accepted := []string{
		"2024-02-29T23:59:59.000000Z",
		"2026-09-17T00:00:00.999999Z",
		"2026-12-31T23:59:59.999999Z",
	}
	for index, timestamp := range accepted {
		key := fmt.Sprintf("accepted-%d", index)
		if _, err := db.ExecContext(context.Background(),
			"INSERT INTO processed_messages (tenant_id,message_key,processed_at) VALUES (?,?,?)",
			testTenantID, key, timestamp,
		); err != nil {
			t.Errorf("valid timestamp %q rejected: %v", timestamp, err)
			continue
		}
		var stored string
		if err := db.QueryRowContext(context.Background(),
			"SELECT processed_at FROM processed_messages WHERE tenant_id=? AND message_key=?", testTenantID, key,
		).Scan(&stored); err != nil {
			t.Fatalf("read accepted timestamp: %v", err)
		}
		if stored != timestamp {
			t.Errorf("stored timestamp = %q, want exact %q", stored, timestamp)
		}
	}

	rejected := []string{
		"2023-02-29T12:00:00.000000Z",
		"2024-02-30T12:00:00.000000Z",
		"2026-99-17T12:00:00.000000Z",
		"2026-00-17T12:00:00.000000Z",
		"2026-09-00T12:00:00.000000Z",
		"2026-09-17T24:00:00.000000Z",
		"2026-09-17T12:60:00.000000Z",
		"2026-09-17T12:00:60.000000Z",
		"2026-09-17T12:00:00.000Z",
	}
	for index, timestamp := range rejected {
		key := fmt.Sprintf("rejected-%d", index)
		if _, err := db.ExecContext(context.Background(),
			"INSERT INTO processed_messages (tenant_id,message_key,processed_at) VALUES (?,?,?)",
			testTenantID, key, timestamp,
		); err == nil {
			t.Errorf("invalid timestamp %q accepted", timestamp)
		}
		var count int
		if err := db.QueryRowContext(context.Background(),
			"SELECT count(*) FROM processed_messages WHERE tenant_id=? AND message_key=?", testTenantID, key,
		).Scan(&count); err != nil {
			t.Fatalf("count rejected timestamp row: %v", err)
		}
		if count != 0 {
			t.Errorf("invalid timestamp %q left %d rows, want 0", timestamp, count)
		}
	}
}

func TestStorageConstraintsRejectInvalidRepresentations(t *testing.T) {
	db := migrateTestDB(t)
	insertUser(t, db, testTenantID)

	tests := []struct {
		name  string
		query string
		args  []any
	}{
		{"private row without tenant", `INSERT INTO transactions (id,message_id,amount,timestamp,merchant_info,source) VALUES (?,?,?,?,?,?)`, []any{"no-tenant", "message", 1, testTime, "merchant", "source"}},
		{"real money", `INSERT INTO transactions (id,tenant_id,message_id,amount,timestamp,merchant_info,source) VALUES (?,?,?,?,?,?,?)`, []any{"real-money", testTenantID, "message", 1.25, testTime, "merchant", "source"}},
		{"invalid boolean", `INSERT INTO merchant_categories (id,tenant_id,fragment,user_locked) VALUES (?,?,?,?)`, []any{"invalid-boolean", testTenantID, "fragment", 2}},
		{"invalid JSON", `INSERT INTO reader_runtime (tenant_id,reader,config) VALUES (?,?,?)`, []any{testTenantID, "reader", `{invalid`}},
		{"invalid timestamp", `INSERT INTO processed_messages (tenant_id,message_key,processed_at) VALUES (?,?,?)`, []any{testTenantID, "message", "2026-09-17T10:11:12Z"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.ExecContext(context.Background(), tt.query, tt.args...); err == nil {
				t.Fatal("constraint error = nil")
			}
			if tt.name == "invalid boolean" {
				var count int
				if err := db.QueryRowContext(context.Background(),
					"SELECT count(*) FROM merchant_categories WHERE id=?", "invalid-boolean",
				).Scan(&count); err != nil {
					t.Fatalf("count invalid boolean row: %v", err)
				}
				if count != 0 {
					t.Fatalf("invalid boolean insert left %d rows, want 0", count)
				}
			}
		})
	}
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var version int
	if err := db.QueryRowContext(context.Background(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return version
}

func objectCount(t *testing.T, db *sql.DB, objectType, name string) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM sqlite_master WHERE type=? AND name=?", objectType, name,
	).Scan(&count); err != nil {
		t.Fatalf("count sqlite_master object %s: %v", name, err)
	}
	return count
}

func tableColumns(t *testing.T, db *sql.DB, table string) []actualColumn {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info("+table+")")
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()

	var result []actualColumn
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, typeName string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table_info(%s): %v", table, err)
		}
		var defaultText *string
		if defaultValue.Valid {
			value := defaultValue.String
			defaultText = &value
		}
		result = append(result, actualColumn{
			ordinal: cid, name: name, typeName: typeName, notNull: notNull == 1,
			defaultValue: defaultText, primaryKey: primaryKey,
		})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info(%s): %v", table, err)
	}
	return result
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func foreignKeys(t *testing.T, db *sql.DB, table string) []expectedForeignKey {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "PRAGMA foreign_key_list("+table+")")
	if err != nil {
		t.Fatalf("foreign_key_list(%s): %v", table, err)
	}
	defer rows.Close()

	var result []expectedForeignKey
	for rows.Next() {
		var id, seq int
		var parent, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan foreign_key_list(%s): %v", table, err)
		}
		result = append(result, expectedForeignKey{
			id: id, sequence: seq, from: from, table: parent, to: to,
			onUpdate: onUpdate, onDelete: onDelete, match: match,
		})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign_key_list(%s): %v", table, err)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].id == result[j].id {
			return result[i].sequence < result[j].sequence
		}
		return result[i].id < result[j].id
	})
	return result
}

func indexes(t *testing.T, db *sql.DB, table string) []expectedIndex {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "PRAGMA index_list("+table+")")
	if err != nil {
		t.Fatalf("index_list(%s): %v", table, err)
	}
	defer rows.Close()
	var result []expectedIndex
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index_list(%s): %v", table, err)
		}
		result = append(result, expectedIndex{
			name: name, unique: unique == 1, origin: origin, partial: partial == 1,
		})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index_list(%s): %v", table, err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close index_list(%s): %v", table, err)
	}
	for i := range result {
		result[i].targets = indexTargets(t, db, table, result[i].name)
		result[i].sql = indexSQL(t, db, result[i].name)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

type pragmaIndexColumn struct {
	sequence int
	cid      int
	name     string
}

func indexTargets(t *testing.T, db *sql.DB, table, index string) []indexTarget {
	t.Helper()
	info := indexInfo(t, db, index)
	columnOrdinals := make(map[string]int)
	for _, column := range tableColumns(t, db, table) {
		columnOrdinals[column.name] = column.ordinal
	}
	rows, err := db.QueryContext(context.Background(), "PRAGMA index_xinfo("+quoteIdentifier(index)+")")
	if err != nil {
		t.Fatalf("index_xinfo(%s): %v", index, err)
	}
	defer rows.Close()

	var result []indexTarget
	for rows.Next() {
		var sequence, cid, descending, key int
		var name, collation sql.NullString
		if err := rows.Scan(&sequence, &cid, &name, &descending, &collation, &key); err != nil {
			t.Fatalf("scan index_xinfo(%s): %v", index, err)
		}
		if key == 0 {
			continue
		}
		columnName := ""
		if name.Valid {
			columnName = name.String
		}
		if sequence >= len(info) || info[sequence] != (pragmaIndexColumn{sequence: sequence, cid: cid, name: columnName}) {
			t.Fatalf("index_info(%s) and index_xinfo(%s) disagree at sequence %d: info=%v", index, index, sequence, info)
		}
		wantCID := -2
		if columnName != "" {
			wantCID = columnOrdinals[columnName]
		}
		if cid != wantCID {
			t.Errorf("index_xinfo(%s) target %d cid = %d, want %d", index, sequence, cid, wantCID)
		}
		if !collation.Valid || collation.String != "BINARY" {
			t.Errorf("index_xinfo(%s) target %d collation = %q, want BINARY", index, sequence, collation.String)
		}
		result = append(result, indexTarget{name: columnName, desc: descending == 1})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index_xinfo(%s): %v", index, err)
	}
	if len(result) != len(info) {
		t.Fatalf("index_xinfo(%s) key target count = %d, index_info count = %d", index, len(result), len(info))
	}
	return result
}

func indexInfo(t *testing.T, db *sql.DB, index string) []pragmaIndexColumn {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "PRAGMA index_info("+quoteIdentifier(index)+")")
	if err != nil {
		t.Fatalf("index_info(%s): %v", index, err)
	}
	defer rows.Close()

	var result []pragmaIndexColumn
	for rows.Next() {
		var sequence, cid int
		var name sql.NullString
		if err := rows.Scan(&sequence, &cid, &name); err != nil {
			t.Fatalf("scan index_info(%s): %v", index, err)
		}
		columnName := ""
		if name.Valid {
			columnName = name.String
		}
		result = append(result, pragmaIndexColumn{sequence: sequence, cid: cid, name: columnName})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index_info(%s): %v", index, err)
	}
	return result
}

func indexSQL(t *testing.T, db *sql.DB, index string) string {
	t.Helper()
	var sqlText sql.NullString
	if err := db.QueryRowContext(context.Background(),
		"SELECT sql FROM sqlite_master WHERE type='index' AND name=?", index,
	).Scan(&sqlText); err != nil {
		t.Fatalf("read index schema %s: %v", index, err)
	}
	if !sqlText.Valid {
		return ""
	}
	return canonicalSQL(sqlText.String)
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func normalizedSchema(t *testing.T, db *sql.DB, objectType, name string) string {
	t.Helper()
	return strings.ToLower(strings.Join(strings.Fields(schemaSQL(t, db, objectType, name)), " "))
}

func schemaSQL(t *testing.T, db *sql.DB, objectType, name string) string {
	t.Helper()
	var schema string
	if err := db.QueryRowContext(context.Background(),
		"SELECT sql FROM sqlite_master WHERE type=? AND name=?", objectType, name,
	).Scan(&schema); err != nil {
		t.Fatalf("read %s schema %s: %v", objectType, name, err)
	}
	return schema
}

func canonicalSQL(value string) string {
	var result strings.Builder
	var quote rune
	for _, char := range value {
		if quote != 0 {
			result.WriteRune(char)
			if char == quote {
				quote = 0
			}
			continue
		}
		switch {
		case char == '\'' || char == '"' || char == '`':
			quote = char
			result.WriteRune(char)
		case unicode.IsSpace(char):
			continue
		default:
			result.WriteRune(unicode.ToLower(char))
		}
	}
	return result.String()
}

func extractCheckExpressions(t *testing.T, schema string) []string {
	t.Helper()
	var result []string
	for offset := 0; offset < len(schema); {
		start := nextCheckKeyword(schema, offset)
		if start < 0 {
			return result
		}
		open := start + len("CHECK")
		for open < len(schema) && unicode.IsSpace(rune(schema[open])) {
			open++
		}
		if open >= len(schema) || schema[open] != '(' {
			t.Fatalf("CHECK without opening parenthesis in schema: %s", schema[start:])
		}
		closeIndex := matchingParenthesis(t, schema, open)
		result = append(result, canonicalSQL(schema[open+1:closeIndex]))
		offset = closeIndex + 1
	}
	return result
}

func nextCheckKeyword(schema string, offset int) int {
	var quote byte
	for i := offset; i+len("CHECK") <= len(schema); i++ {
		char := schema[i]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		if strings.EqualFold(schema[i:i+len("CHECK")], "CHECK") {
			return i
		}
	}
	return -1
}

func matchingParenthesis(t *testing.T, value string, open int) int {
	t.Helper()
	depth := 0
	var quote byte
	for i := open; i < len(value); i++ {
		char := value[i]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		switch char {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	t.Fatalf("unclosed CHECK expression: %s", value[open:])
	return -1
}

func insertUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO users (id,email,display_name,role,created_at,updated_at)
		VALUES (?,?,?,?,?,?)
	`, id, id+"@example.com", "Test User", "user", testTime, testTime); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func assertFTSRow(t *testing.T, db *sql.DB, transactionID, merchant, description string) {
	t.Helper()
	var gotID, gotMerchant, gotDescription string
	if err := db.QueryRowContext(context.Background(), `
		SELECT transaction_id,merchant,description
		FROM transactions_fts
		WHERE transaction_id=?
	`, transactionID).Scan(&gotID, &gotMerchant, &gotDescription); err != nil {
		t.Fatalf("read FTS row: %v", err)
	}
	if gotID != transactionID || gotMerchant != merchant || gotDescription != description {
		t.Fatalf("FTS row = (%q,%q,%q), want (%q,%q,%q)", gotID, gotMerchant, gotDescription, transactionID, merchant, description)
	}
}

var (
	errDuplicateMigration = stderrors.New("migration already applied")
	errMigrationProbe     = stderrors.New("migration probe failed")
	errRollbackProbe      = stderrors.New("rollback probe failed")
)

type migrationDriverState struct {
	mu                       sync.Mutex
	writer                   chan struct{}
	closed                   chan struct{}
	closedOnce               sync.Once
	synchronizeOutsideReads  bool
	outsideVersionReads      int
	outsideVersionReadsReady chan struct{}
	schema                   bool
	version                  int
	beginErr                 error
	rollbackErr              error
	cancelMigration          context.CancelFunc
	rollbackDeadline         time.Time
	rollbackContextErr       error
}

func newMigrationDriverState(synchronizeOutsideReads bool) *migrationDriverState {
	state := &migrationDriverState{
		writer:                   make(chan struct{}, 1),
		closed:                   make(chan struct{}),
		synchronizeOutsideReads:  synchronizeOutsideReads,
		outsideVersionReadsReady: make(chan struct{}),
	}
	state.writer <- struct{}{}
	return state
}

type migrationTestConnector struct {
	state *migrationDriverState
}

func (c migrationTestConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &migrationTestConn{state: c.state}, nil
}

func (c migrationTestConnector) Driver() driver.Driver { return migrationTestDriver{} }

type migrationTestDriver struct{}

func (migrationTestDriver) Open(string) (driver.Conn, error) {
	return nil, stderrors.New("migration test driver requires connector")
}

type migrationTestConn struct {
	state         *migrationDriverState
	inTransaction bool
}

func (c *migrationTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }

func (c *migrationTestConn) Begin() (driver.Tx, error) { return nil, driver.ErrSkip }

func (c *migrationTestConn) Close() error {
	if c.inTransaction {
		c.inTransaction = false
		c.state.writer <- struct{}{}
	}
	c.state.closedOnce.Do(func() { close(c.state.closed) })
	return nil
}

func (c *migrationTestConn) ExecContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	switch strings.TrimSpace(query) {
	case "BEGIN IMMEDIATE":
		if c.state.beginErr != nil {
			return nil, c.state.beginErr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.state.writer:
			c.inTransaction = true
			return driver.RowsAffected(0), nil
		}
	case "MIGRATE":
		c.state.mu.Lock()
		defer c.state.mu.Unlock()
		if c.state.schema {
			return nil, errDuplicateMigration
		}
		c.state.schema = true
		return driver.RowsAffected(0), nil
	case "FAIL_MIGRATION":
		if c.state.cancelMigration != nil {
			c.state.cancelMigration()
		}
		return nil, errMigrationProbe
	case "PRAGMA user_version = 1":
		c.state.mu.Lock()
		c.state.version = schemaVersion
		c.state.mu.Unlock()
		return driver.RowsAffected(0), nil
	case "COMMIT":
		c.releaseWriter()
		return driver.RowsAffected(0), nil
	case "ROLLBACK":
		c.state.mu.Lock()
		c.state.rollbackDeadline, _ = ctx.Deadline()
		c.state.rollbackContextErr = ctx.Err()
		c.state.mu.Unlock()
		if c.state.rollbackErr != nil {
			return nil, c.state.rollbackErr
		}
		c.releaseWriter()
		return driver.RowsAffected(0), nil
	default:
		return nil, stderrors.New("unexpected migration test query: " + query)
	}
}

func (c *migrationTestConn) QueryContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.TrimSpace(query) != "PRAGMA user_version" {
		return nil, stderrors.New("unexpected migration test query: " + query)
	}

	c.state.mu.Lock()
	version := c.state.version
	if c.state.synchronizeOutsideReads && !c.inTransaction {
		c.state.outsideVersionReads++
		if c.state.outsideVersionReads == 2 {
			close(c.state.outsideVersionReadsReady)
		}
	}
	wait := c.state.synchronizeOutsideReads && !c.inTransaction
	c.state.mu.Unlock()
	if wait {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.state.outsideVersionReadsReady:
		}
	}
	return &singleValueRows{value: int64(version)}, nil
}

func (c *migrationTestConn) releaseWriter() {
	if !c.inTransaction {
		return
	}
	c.inTransaction = false
	c.state.writer <- struct{}{}
}

type singleValueRows struct {
	value int64
	done  bool
}

func (*singleValueRows) Columns() []string { return []string{"user_version"} }

func (*singleValueRows) Close() error { return nil }

func (r *singleValueRows) Next(values []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	values[0] = r.value
	return nil
}
