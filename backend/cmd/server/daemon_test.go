package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/httpapi"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func TestDaemonManager_SetRunning(t *testing.T) {
	dm := &daemonManager{}
	now := time.Now()
	dm.setRunning(now)

	s := dm.Status()
	if !s.Running {
		t.Error("expected Running=true after setRunning")
	}
	if s.StartedAt == nil || !s.StartedAt.Equal(now) {
		t.Errorf("expected StartedAt=%v, got %v", now, s.StartedAt)
	}
	if s.LastError != "" {
		t.Errorf("expected empty LastError, got %q", s.LastError)
	}
}

func TestDaemonManager_SetStopped_WithError(t *testing.T) {
	dm := &daemonManager{}
	dm.setRunning(time.Now())
	dm.setStopped(errors.New("connection refused"))

	s := dm.Status()
	if s.Running {
		t.Error("expected Running=false after setStopped")
	}
	if s.LastError != "connection refused" {
		t.Errorf("expected LastError=%q, got %q", "connection refused", s.LastError)
	}
}

func TestDaemonManager_SetStopped_CanceledContextNotRecorded(t *testing.T) {
	dm := &daemonManager{}
	dm.setRunning(time.Now())
	dm.setStopped(context.Canceled)

	s := dm.Status()
	if s.Running {
		t.Error("expected Running=false")
	}
	if s.LastError != "" {
		t.Errorf("context.Canceled should not populate LastError, got %q", s.LastError)
	}
}

func TestDaemonManager_SetStopped_NilErrorClearsLastError(t *testing.T) {
	dm := &daemonManager{}
	dm.setRunning(time.Now())
	dm.setStopped(errors.New("first error"))
	dm.setRunning(time.Now())
	dm.setStopped(nil)

	s := dm.Status()
	if s.LastError != "" {
		t.Errorf("nil error should not populate LastError, got %q", s.LastError)
	}
}

func TestDaemonCoordinator_RescanPersistsActiveReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := &daemonCoordinatorTestStore{}
	tenant := store.Tenant{ID: "tenant-a"}
	dc := &daemonCoordinator{
		ctx:      ctx,
		registry: plugins.NewRegistry(),
		cfg: config.App{
			Persisted: config.Persisted{
				ReadTimeout: time.Second,
			},
		},
		st:     st,
		dm:     &daemonManager{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	dc.rescan(httpapi.DaemonRunRequest{Tenant: tenant, Reader: "thunderbird"})

	if st.activeScanningReaderTenant != tenant.ID || st.activeScanningReader != "thunderbird" {
		t.Fatalf(
			"active scanning reader write = tenant %q reader %q, want tenant %q reader thunderbird",
			st.activeScanningReaderTenant,
			st.activeScanningReader,
			tenant.ID,
		)
	}
}

type daemonCoordinatorTestStore struct {
	activeScanningReaderTenant string
	activeScanningReader       string
}

func (s *daemonCoordinatorTestStore) GetAppConfig(_ context.Context, _ store.Tenant, _ string) (string, error) {
	return "", store.ErrNotFound
}

func (s *daemonCoordinatorTestStore) SetAppConfig(_ context.Context, _ store.Tenant, _, _ string) error {
	return nil
}

func (s *daemonCoordinatorTestStore) SetActiveScanningReader(_ context.Context, tenant store.Tenant, reader string) error {
	s.activeScanningReaderTenant = tenant.ID
	s.activeScanningReader = reader
	return nil
}

func (s *daemonCoordinatorTestStore) ListRules(_ context.Context, _ store.Tenant) ([]store.RuleRow, error) {
	return nil, nil
}
