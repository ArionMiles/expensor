package main

import (
	"context"
	"errors"
	"testing"
	"time"
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
