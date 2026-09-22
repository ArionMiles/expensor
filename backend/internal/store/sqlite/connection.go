package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	moderncsqlite "modernc.org/sqlite"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	connectionPoolSize        = 8
	connectionSetupTimeout    = time.Second
	pathCreationRetryInterval = 5 * time.Millisecond
	pathCreationRetryTimeout  = 250 * time.Millisecond
	memoryDSN                 = ":memory:"
	memoryMode                = "memory"
	sharedCacheMode           = "shared"
	walJournalMode            = "wal"
)

type connector struct {
	driver driver.Driver
	dsn    string
}

type preparedDatabasePath struct {
	dsn      string
	path     string
	file     *os.File
	identity os.FileInfo
}

func (p *preparedDatabasePath) close() error {
	if p == nil || p.file == nil {
		return nil
	}
	err := p.file.Close()
	p.file = nil
	if err != nil {
		return filesystemError("sqlite.connection.close_path", "closing database validation handle", err)
	}
	return nil
}

func (p *preparedDatabasePath) verifyLiveIdentity(ctx context.Context, db *sql.DB) (resultErr error) {
	if p == nil || p.file == nil {
		return nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return errors.B.Op("sqlite.connection.verify_identity").KindInternal().Text("acquiring live database connection").Err(err).Build()
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, sql.ErrConnDone) {
			resultErr = errors.Join(resultErr, errors.B.Op("sqlite.connection.verify_identity").
				KindInternal().Text("closing live database connection").Err(closeErr).Build())
		}
	}()

	livePath, err := liveMainDatabasePath(ctx, conn)
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(livePath)
	if err != nil {
		return filesystemError("sqlite.connection.verify_identity", "checking live database file", err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 {
		return errors.B.Op("sqlite.connection.verify_identity").KindInvalidArgument().Text("live database path is a symbolic link").Build()
	}
	handleInfo, err := p.file.Stat()
	if err != nil {
		return filesystemError("sqlite.connection.verify_identity", "checking database validation handle", err)
	}
	if !os.SameFile(p.identity, handleInfo) || !os.SameFile(handleInfo, pathInfo) {
		return errors.B.Op("sqlite.connection.verify_identity").KindPermissionDenied().Text("live database identity does not match the validated file").Build()
	}
	return nil
}

func liveMainDatabasePath(ctx context.Context, conn *sql.Conn) (livePath string, resultErr error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return "", errors.B.Op("sqlite.connection.verify_identity").KindInternal().Text("reading live database identity").Err(err).Build()
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, errors.B.Op("sqlite.connection.verify_identity").
				KindInternal().Text("closing live database identity rows").Err(closeErr).Build())
		}
	}()

	for rows.Next() {
		var sequence int
		var name, path string
		if err := rows.Scan(&sequence, &name, &path); err != nil {
			return "", errors.B.Op("sqlite.connection.verify_identity").KindInternal().Text("scanning live database identity").Err(err).Build()
		}
		if name == "main" {
			livePath = path
			break
		}
	}
	if err := rows.Err(); err != nil {
		return "", errors.B.Op("sqlite.connection.verify_identity").KindInternal().Text("reading live database identity rows").Err(err).Build()
	}
	if livePath == "" {
		return "", errors.B.Op("sqlite.connection.verify_identity").KindInternal().Text("live database identity is unavailable").Build()
	}
	return livePath, nil
}

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := c.driver.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (c connector) Driver() driver.Driver {
	return c.driver
}

func openDatabase(ctx context.Context, dsn string, busyTimeout time.Duration) (*sql.DB, error) {
	if busyTimeout <= 0 || busyTimeout.Milliseconds() <= 0 {
		return nil, errors.B.Op("sqlite.connection.open").KindInvalidArgument().Text("busy timeout must be at least one millisecond").Build()
	}

	ownedDriver := &moderncsqlite.Driver{}
	if err := registerCasefold(ownedDriver); err != nil {
		return nil, errors.B.Op("sqlite.connection.open").KindInternal().Text("registering case-fold function").Err(err).Build()
	}
	registerConnectionSettings(ownedDriver, busyTimeout)

	db := sql.OpenDB(connector{driver: ownedDriver, dsn: dsn})
	poolSize := connectionPoolSize
	if isPrivateMemoryDSN(dsn) {
		poolSize = 1
	}
	db.SetMaxOpenConns(poolSize)
	db.SetMaxIdleConns(poolSize)

	if err := prepareDatabase(ctx, db, isMemoryDSN(dsn)); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, errors.B.Op("sqlite.connection.open").Text("preparing database connections").Err(err).Build()
	}
	return db, nil
}

func prepareDatabasePath(ctx context.Context, dsn string, hooks pathCreationHooks) (*preparedDatabasePath, error) {
	if err := pathContextError(ctx); err != nil {
		return nil, err
	}
	resolved, err := expandHomePath(dsn)
	if err != nil {
		return nil, err
	}
	if err := validateDSNPolicy(resolved); err != nil {
		return nil, err
	}
	if isMemoryDSN(resolved) {
		return &preparedDatabasePath{dsn: resolved}, nil
	}

	path, ok, err := databaseFilePath(resolved)
	if err != nil {
		return nil, err
	}
	if !ok || path == "" {
		return nil, errors.B.Op("sqlite.connection.prepare_path").KindInvalidArgument().Text("database path is empty").Build()
	}
	if err := validateExistingPathComponents(ctx, path, hooks); err != nil {
		return nil, err
	}
	file, identity, err := createSecureDatabasePath(ctx, path, hooks)
	if err != nil {
		return nil, errors.B.Op("sqlite.connection.prepare_path").Text("preparing database path").Err(err).Build()
	}
	return &preparedDatabasePath{dsn: resolved, path: path, file: file, identity: identity}, nil
}

func validateDSNPolicy(dsn string) error {
	if dsn == memoryDSN {
		return nil
	}
	if !strings.HasPrefix(dsn, "file:") {
		return validatePlainPathDSNPolicy(dsn)
	}
	remainder := strings.TrimPrefix(dsn, "file:")
	if strings.Contains(remainder, "#") {
		return unsupportedDSNError()
	}
	name, rawQuery, hasQuery := strings.Cut(remainder, "?")
	if !hasQuery {
		return nil
	}
	if name == "" || rawQuery == "" {
		return unsupportedDSNError()
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return unsupportedDSNError()
	}
	for key, values := range query {
		if len(values) != 1 {
			return unsupportedDSNError()
		}
		switch key {
		case "mode":
			if values[0] != memoryMode {
				return unsupportedDSNError()
			}
		case "cache":
			if values[0] != sharedCacheMode {
				return unsupportedDSNError()
			}
		default:
			return unsupportedDSNError()
		}
	}
	if values := query["mode"]; len(values) != 1 || values[0] != memoryMode {
		return unsupportedDSNError()
	}
	return nil
}

func validatePlainPathDSNPolicy(dsn string) error {
	if strings.ContainsAny(dsn, "?#") {
		return unsupportedDSNError()
	}
	return nil
}

func unsupportedDSNError() error {
	return errors.B.Op("sqlite.connection.validate_dsn").KindInvalidArgument().Text("unsupported database connection options").Build()
}

func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.B.Op("sqlite.connection.expand_home").KindInvalidArgument().Text("home directory is unavailable").Err(err).Build()
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func databaseFilePath(dsn string) (string, bool, error) {
	if !strings.HasPrefix(dsn, "file:") {
		return dsn, true, nil
	}

	name, _, _ := strings.Cut(strings.TrimPrefix(dsn, "file:"), "?")
	path, err := url.PathUnescape(name)
	if err != nil {
		return "", false, errors.B.Op("sqlite.connection.file_path").KindInvalidArgument().Text("decoding database path").Build()
	}
	return filepath.FromSlash(path), true, nil
}

func createSecureDatabasePath(ctx context.Context, path string, hooks pathCreationHooks) (*os.File, os.FileInfo, error) {
	if err := pathContextError(ctx); err != nil {
		return nil, nil, err
	}
	parent := filepath.Dir(path)
	if err := createSecureDirectories(ctx, parent, hooks); err != nil {
		return nil, nil, err
	}
	if err := validateExistingPathComponents(ctx, path, hooks); err != nil {
		return nil, nil, err
	}
	if err := secureDatabaseParent(parent); err != nil {
		return nil, nil, err
	}
	if err := pathContextError(ctx); err != nil {
		return nil, nil, err
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return validateExistingDatabasePath(ctx, path, hooks)
		}
		return nil, nil, filesystemError("sqlite.connection.create_database", "creating database file", err)
	}
	if hooks.afterFileCreate != nil {
		hooks.afterFileCreate()
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, nil, filesystemError("sqlite.connection.secure_database", "securing database file", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, filesystemError("sqlite.connection.secure_database", "checking database file", err)
	}
	return file, info, nil
}

func validateExistingPathComponents(ctx context.Context, path string, hooks pathCreationHooks) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filesystemError("sqlite.connection.validate_components", "resolving database path", err)
	}

restart:
	for current := filepath.Clean(absolute); ; current = filepath.Dir(current) {
		if err := pathContextError(ctx); err != nil {
			return err
		}
		info, statErr := os.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 && !allowsSystemAncestorSymlink(info, current, absolute) {
			return errors.B.Op("sqlite.connection.validate_components").KindInvalidArgument().Text("database path contains a symbolic link").Build()
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			waited, waitErr := waitForTemporaryAncestor(ctx, current, hooks)
			if waitErr != nil {
				return waitErr
			}
			if waited {
				goto restart
			}
			return filesystemError("sqlite.connection.validate_components", "checking database path component", statErr)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func allowsSystemAncestorSymlink(info os.FileInfo, path, databasePath string) bool {
	// Trust root-owned platform aliases such as macOS /var only above the app-controlled parent.
	return path != databasePath && path != filepath.Dir(databasePath) && trustedSystemSymlink(info)
}

func secureDatabaseParent(path string) (resultErr error) {
	info, err := os.Lstat(path)
	if err != nil {
		return filesystemError("sqlite.connection.secure_parent", "checking database parent", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.B.Op("sqlite.connection.secure_parent").KindInvalidArgument().Text("database parent is not a directory").Build()
	}
	if !ownedByCurrentUser(info) {
		return errors.B.Op("sqlite.connection.secure_parent").KindPermissionDenied().Text("database parent has an unsafe owner").Build()
	}
	if mode := info.Mode().Perm(); mode&0o700 != 0o700 || mode&0o077 != 0 {
		return errors.B.Op("sqlite.connection.secure_parent").KindPermissionDenied().Text("database parent has unsafe permissions").Build()
	}
	directory, err := os.Open(path)
	if err != nil {
		return filesystemError("sqlite.connection.secure_parent", "opening database parent", err)
	}
	defer func() {
		if err := directory.Close(); err != nil {
			resultErr = errors.Join(resultErr, filesystemError("sqlite.connection.secure_parent", "closing database parent handle", err))
		}
	}()
	openedInfo, err := directory.Stat()
	if err != nil {
		return filesystemError("sqlite.connection.secure_parent", "checking opened database parent", err)
	}
	if !os.SameFile(info, openedInfo) {
		return errors.B.Op("sqlite.connection.secure_parent").KindPermissionDenied().Text("database parent changed during validation").Build()
	}
	return nil
}

func createSecureDirectories(ctx context.Context, path string, hooks pathCreationHooks) error {
	path = filepath.Clean(path)
	for {
		if err := pathContextError(ctx); err != nil {
			return err
		}
		missing, retry, err := missingDirectories(ctx, path, hooks)
		if err != nil {
			return err
		}
		if retry {
			continue
		}
		for index := len(missing) - 1; index >= 0; index-- {
			if err := ensureSecureDirectory(ctx, missing[index], hooks); err != nil {
				return err
			}
		}
		return nil
	}
}

func missingDirectories(ctx context.Context, path string, hooks pathCreationHooks) ([]string, bool, error) {
	missing := make([]string, 0, 2)
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			retry, err := inspectExistingDirectory(ctx, current, info, hooks)
			return missing, retry, err
		}
		if !os.IsNotExist(err) {
			waited, waitErr := waitForTemporaryAncestor(ctx, current, hooks)
			if waitErr != nil {
				return nil, false, waitErr
			}
			if waited {
				return nil, true, nil
			}
			return nil, false, filesystemError("sqlite.connection.create_directories", "checking database directory", err)
		}
		missing = append(missing, current)
	}
}

func inspectExistingDirectory(ctx context.Context, path string, info os.FileInfo, hooks pathCreationHooks) (bool, error) {
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, errors.B.Op("sqlite.connection.create_directories").KindInvalidArgument().Text("database parent is not a directory").Build()
	}
	if !isTemporaryOwnedEntry(info) {
		return false, nil
	}
	if err := waitForSecuredEntry(ctx, path, true, 0o700, hooks); err != nil {
		return false, err
	}
	return true, nil
}

func ensureSecureDirectory(ctx context.Context, path string, hooks pathCreationHooks) error {
	for {
		created, err := createSecureDirectory(ctx, path, hooks)
		if err == nil {
			if created {
				return nil
			}
			return nil
		}
		waited, waitErr := waitForTemporaryAncestor(ctx, path, hooks)
		if waitErr != nil {
			return waitErr
		}
		if !waited {
			return err
		}
	}
}

func createSecureDirectory(ctx context.Context, path string, hooks pathCreationHooks) (bool, error) {
	err := os.Mkdir(path, 0o700)
	if err == nil {
		return true, secureCreatedDirectory(path, hooks)
	}
	if !os.IsExist(err) {
		return false, filesystemError("sqlite.connection.create_directories", "creating database directory", err)
	}
	info, statErr := os.Lstat(path)
	if statErr != nil {
		return false, filesystemError("sqlite.connection.create_directories", "checking database directory", statErr)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, errors.B.Op("sqlite.connection.create_directories").KindInvalidArgument().Text("database parent is not a directory").Build()
	}
	if isTemporaryOwnedEntry(info) {
		if waitErr := waitForSecuredEntry(ctx, path, true, 0o700, hooks); waitErr != nil {
			return false, waitErr
		}
	}
	return false, nil
}

func secureCreatedDirectory(path string, hooks pathCreationHooks) (resultErr error) {
	info, err := os.Lstat(path)
	if err != nil {
		return filesystemError("sqlite.connection.create_directories", "checking new database directory", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.B.Op("sqlite.connection.create_directories").KindInvalidArgument().Text("new database directory is not a directory").Build()
	}
	if !ownedByCurrentUser(info) {
		return errors.B.Op("sqlite.connection.create_directories").KindPermissionDenied().Text("new database directory has an unsafe owner").Build()
	}
	if info.Mode().Perm() == 0 {
		// Bootstrap owner traversal before opening the just-created directory; final permissions use the handle below.
		//nolint:gosec // The entry was created by this invocation and validated as a current-owner, non-symlink directory.
		if err := os.Chmod(path, 0o500); err != nil {
			return filesystemError("sqlite.connection.create_directories", "making new database directory traversable", err)
		}
	}
	directory, err := os.Open(path)
	if err != nil {
		return filesystemError("sqlite.connection.create_directories", "opening new database directory", err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, filesystemError("sqlite.connection.create_directories", "closing new database directory handle", closeErr))
		}
	}()
	openedInfo, err := directory.Stat()
	if err != nil {
		return filesystemError("sqlite.connection.create_directories", "checking new database directory handle", err)
	}
	if !os.SameFile(info, openedInfo) {
		return errors.B.Op("sqlite.connection.create_directories").KindPermissionDenied().Text("new database directory changed during validation").Build()
	}
	if hooks.afterDirectoryCreate != nil {
		hooks.afterDirectoryCreate()
	}
	currentInfo, err := os.Lstat(path)
	if err != nil {
		return filesystemError("sqlite.connection.create_directories", "rechecking new database directory", err)
	}
	if currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.IsDir() {
		return errors.B.Op("sqlite.connection.create_directories").KindInvalidArgument().Text("new database directory was replaced").Build()
	}
	if !ownedByCurrentUser(currentInfo) || !os.SameFile(openedInfo, currentInfo) {
		return errors.B.Op("sqlite.connection.create_directories").KindPermissionDenied().Text("new database directory changed during validation").Build()
	}
	if err := directory.Chmod(0o700); err != nil {
		return filesystemError("sqlite.connection.create_directories", "securing new database directory", err)
	}
	return nil
}

func validateExistingDatabasePath(ctx context.Context, path string, hooks pathCreationHooks) (*os.File, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, filesystemError("sqlite.connection.validate_path", "checking database file", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.B.Op("sqlite.connection.validate_path").KindInvalidArgument().Text("database path is a symbolic link").Build()
	}
	if isTemporaryOwnedEntry(info) {
		if err := waitForSecuredEntry(ctx, path, false, 0o600, hooks); err != nil {
			return nil, nil, err
		}
		info, err = os.Lstat(path)
		if err != nil {
			return nil, nil, filesystemError("sqlite.connection.validate_path", "checking secured database file", err)
		}
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errors.B.Op("sqlite.connection.validate_path").KindInvalidArgument().Text("database path is not a regular file").Build()
	}
	if !ownedByCurrentUser(info) {
		return nil, nil, errors.B.Op("sqlite.connection.validate_path").KindPermissionDenied().Text("database file has an unsafe owner").Build()
	}
	if mode := info.Mode().Perm(); mode&0o600 != 0o600 || mode&0o077 != 0 {
		return nil, nil, errors.B.Op("sqlite.connection.validate_path").KindPermissionDenied().Text("database file has unsafe permissions").Build()
	}
	validationFile, err := os.Open(path)
	if err != nil {
		return nil, nil, filesystemError("sqlite.connection.validate_path", "opening database file", err)
	}
	openedInfo, err := validationFile.Stat()
	if err != nil {
		_ = validationFile.Close()
		return nil, nil, filesystemError("sqlite.connection.validate_path", "checking opened database file", err)
	}
	if !os.SameFile(info, openedInfo) {
		_ = validationFile.Close()
		return nil, nil, errors.B.Op("sqlite.connection.validate_path").KindPermissionDenied().Text("database file changed during validation").Build()
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		_ = validationFile.Close()
		return nil, nil, filesystemError("sqlite.connection.validate_path", "opening secured database file", err)
	}
	securedInfo, err := file.Stat()
	if err != nil {
		_ = validationFile.Close()
		_ = file.Close()
		return nil, nil, filesystemError("sqlite.connection.validate_path", "checking secured database file", err)
	}
	if !os.SameFile(openedInfo, securedInfo) {
		_ = validationFile.Close()
		_ = file.Close()
		return nil, nil, errors.B.Op("sqlite.connection.validate_path").KindPermissionDenied().Text("database file changed while being secured").Build()
	}
	if err := validationFile.Close(); err != nil {
		_ = file.Close()
		return nil, nil, filesystemError("sqlite.connection.validate_path", "closing database security handle", err)
	}
	return file, securedInfo, nil
}

func waitForTemporaryAncestor(ctx context.Context, path string, hooks pathCreationHooks) (bool, error) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.IsDir() && isTemporaryOwnedEntry(info) {
			return true, waitForSecuredEntry(ctx, current, true, 0o700, hooks)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}

func waitForSecuredEntry(ctx context.Context, path string, directory bool, wantMode os.FileMode, hooks pathCreationHooks) error {
	if hooks.beforeRetry != nil {
		hooks.beforeRetry()
	}
	ticker := time.NewTicker(pathCreationRetryInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(pathCreationRetryTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.B.Op("sqlite.connection.wait_for_path").Kind(errors.WhatKind(ctx.Err())).Text("waiting for concurrent path creation").Err(ctx.Err()).Build()
		case <-ticker.C:
			info, err := os.Lstat(path)
			if err != nil {
				return filesystemError("sqlite.connection.wait_for_path", "checking concurrent path creation", err)
			}
			if isTemporaryOwnedEntry(info) {
				continue
			}
			if info.IsDir() != directory || info.Mode().Perm() != wantMode || !ownedByCurrentUser(info) {
				return errors.B.Op("sqlite.connection.wait_for_path").KindPermissionDenied().Text("concurrent path creation produced an insecure entry").Build()
			}
			return nil
		case <-timeout.C:
			return errors.B.Op("sqlite.connection.wait_for_path").KindPermissionDenied().Text("concurrent path creation remained inaccessible").Build()
		}
	}
}

func isTemporaryOwnedEntry(info os.FileInfo) bool {
	mode := info.Mode().Perm()
	return (mode == 0 || info.IsDir() && mode == 0o500) && ownedByCurrentUser(info)
}

func filesystemError(operation, text string, err error) error {
	cause := err
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		cause = pathErr.Err
	}
	return errors.B.Op(operation).Kind(pathFailureKind(cause)).Text(text).Err(cause).Build()
}

func pathContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.B.Op("sqlite.connection.prepare_path").Kind(errors.WhatKind(err)).Text("preparing database path").Err(err).Build()
	}
	return nil
}

func registerCasefold(ownedDriver *moderncsqlite.Driver) error {
	return ownedDriver.RegisterDeterministicScalarFunction(
		"expensor_casefold",
		1,
		func(_ *moderncsqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			switch value := args[0].(type) {
			case nil:
				return nil, nil
			case string:
				return cases.Fold().String(value), nil
			case []byte:
				return cases.Fold().String(string(value)), nil
			default:
				return nil, errors.B.Op("sqlite.connection.casefold").KindInvalidArgument().Text("case-fold value must be text").Build()
			}
		},
	)
}

func registerConnectionSettings(ownedDriver *moderncsqlite.Driver, busyTimeout time.Duration) {
	busyTimeoutSQL := "PRAGMA busy_timeout = " + strconv.FormatInt(busyTimeout.Milliseconds(), 10)
	ownedDriver.RegisterConnectionHook(func(conn moderncsqlite.ExecQuerierContext, dsn string) error {
		// modernc's hook has no caller context; local-only PRAGMAs use a short internal bound.
		ctx, cancel := context.WithTimeout(context.Background(), connectionSetupTimeout)
		defer cancel()
		return configureConnection(ctx, conn, dsn, busyTimeoutSQL)
	})
}

func configureConnection(ctx context.Context, conn moderncsqlite.ExecQuerierContext, dsn, busyTimeoutSQL string) error {
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON", nil); err != nil {
		return errors.B.Op("sqlite.connection.configure").KindInternal().Text("enabling foreign keys").Err(err).Build()
	}
	if _, err := conn.ExecContext(ctx, busyTimeoutSQL, nil); err != nil {
		return errors.B.Op("sqlite.connection.configure").KindInternal().Text("setting busy timeout").Err(err).Build()
	}
	if !isMemoryDSN(dsn) {
		if _, err := conn.ExecContext(ctx, "PRAGMA synchronous = NORMAL", nil); err != nil {
			return errors.B.Op("sqlite.connection.configure").KindInternal().Text("setting synchronous mode").Err(err).Build()
		}
	}
	return nil
}

func prepareDatabase(ctx context.Context, db *sql.DB, memory bool) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, conn.Close())
	}()

	if memory {
		return conn.PingContext(ctx)
	}

	var journalMode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return err
	}
	if journalMode != walJournalMode {
		return errors.B.Op("sqlite.connection.prepare").KindInternal().Text("database did not enter WAL mode").Build()
	}
	return nil
}

func isMemoryDSN(dsn string) bool {
	if dsn == memoryDSN {
		return true
	}
	if !strings.HasPrefix(dsn, "file:") {
		return false
	}

	name, rawQuery, _ := strings.Cut(strings.TrimPrefix(dsn, "file:"), "?")
	if name == memoryDSN {
		return true
	}
	query, err := url.ParseQuery(rawQuery)
	return err == nil && query.Get("mode") == memoryMode
}

func isPrivateMemoryDSN(dsn string) bool {
	if dsn == memoryDSN {
		return true
	}
	if !strings.HasPrefix(dsn, "file:") {
		return false
	}

	name, rawQuery, _ := strings.Cut(strings.TrimPrefix(dsn, "file:"), "?")
	query, err := url.ParseQuery(rawQuery)
	if err != nil || name != memoryDSN && query.Get("mode") != memoryMode {
		return false
	}
	return query.Get("cache") != sharedCacheMode
}
