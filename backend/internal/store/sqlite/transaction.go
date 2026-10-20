package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const writeRollbackTimeout = 5 * time.Second

type writeConnection interface {
	queryer
	Raw(func(any) error) error
	Close() error
}

var _ writeConnection = (*sql.Conn)(nil)

func (s *Store) withImmediateWrite(ctx context.Context, write func(queryer) error) (err error) {
	select {
	case <-ctx.Done():
		return errors.B.Op("sqlite.transaction.write").Kind(errors.WhatKind(ctx.Err())).Text("waiting for write access").Err(ctx.Err()).Build()
	case <-s.writeGate:
	}
	defer func() { s.writeGate <- struct{}{} }()

	conn, err := s.acquireWriteConnection(ctx)
	if err != nil {
		return errors.B.Op("sqlite.transaction.write").KindInternal().Text("acquiring write connection").Err(err).Build()
	}
	discarded := false
	defer func() {
		if closeErr := closeWriteConnection(conn, discarded); closeErr != nil {
			err = errors.Join(err, errors.B.Op("sqlite.transaction.close").KindInternal().Text("closing write connection").Err(closeErr).Build())
		}
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		mappedErr := mapSQLiteError("sqlite.transaction.begin", err)
		wasDiscarded, discardErr := discardWriteConnection(conn)
		discarded = discarded || wasDiscarded
		if discardErr != nil {
			mappedErr = errors.Join(mappedErr, errors.B.Op("sqlite.transaction.discard").KindInternal().Text("discarding write connection").Err(discardErr).Build())
		}
		return mappedErr
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeRollbackTimeout)
		defer cancel()
		if _, rollbackErr := conn.ExecContext(rollbackCtx, "ROLLBACK"); rollbackErr != nil {
			wasDiscarded, discardErr := discardWriteConnection(conn)
			discarded = discarded || wasDiscarded
			if discardErr != nil {
				rollbackErr = errors.Join(rollbackErr, discardErr)
			}
			err = errors.Join(err, errors.B.Op("sqlite.transaction.rollback").KindInternal().Text("rolling back write transaction").Err(rollbackErr).Build())
		}
	}()

	if err := write(conn); err != nil {
		return errors.B.Op("sqlite.transaction.write").Kind(errors.WhatKind(err)).Text("running write transaction").Err(err).Build()
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return mapSQLiteError("sqlite.transaction.commit", err)
	}
	committed = true
	return nil
}

func discardWriteConnection(conn writeConnection) (bool, error) {
	err := conn.Raw(func(any) error { return driver.ErrBadConn })
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) {
		return true, nil
	}
	return false, err
}

func closeWriteConnection(conn writeConnection, discarded bool) error {
	if discarded {
		return nil
	}
	err := conn.Close()
	if errors.Is(err, sql.ErrConnDone) {
		return nil
	}
	return err
}
