package main

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
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

func TestParseRules_ICICICreditCardCoversBothExactSenders(t *testing.T) {
	rules, err := parseRules(rulesInput)
	if err != nil {
		t.Fatalf("parseRules() error = %v", err)
	}

	wantSenders := map[string]bool{
		"credit_cards@icicibank.com": false,
		"credit_cards@icici.bank.in": false,
	}

	for _, rule := range rules {
		if !strings.Contains(rule.Name, "ICICI Credit Card") {
			continue
		}
		for _, sender := range rule.SenderEmails {
			if _, ok := wantSenders[sender]; ok {
				wantSenders[sender] = true
			}
		}
	}

	for sender, found := range wantSenders {
		if !found {
			t.Errorf("expected ICICI credit card rule for sender %q", sender)
		}
	}
}

func TestBuildSystemRuleRowsPreservesV2SourceAndSenders(t *testing.T) {
	rows := buildSystemRuleRows([]api.Rule{
		{
			Name:            "HDFC Credit Card",
			SenderEmails:    []string{"alerts@hdfcbank.bank.in", "alerts@hdfcbank.net"},
			SubjectContains: "Alert",
			Amount:          regexp.MustCompile(`Rs\. ([\d.]+)`),
			MerchantInfo:    regexp.MustCompile(`at (.*?) on`),
			Source:          api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"},
		},
	})

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.SenderEmail != "alerts@hdfcbank.bank.in" {
		t.Fatalf("SenderEmail = %q", row.SenderEmail)
	}
	if strings.Join(row.SenderEmails, ",") != "alerts@hdfcbank.bank.in,alerts@hdfcbank.net" {
		t.Fatalf("SenderEmails = %#v", row.SenderEmails)
	}
	if row.SourceType != "Credit Card" || row.SourceLabel != "HDFC Credit Card" || row.Bank != "HDFC" {
		t.Fatalf("source = (%q, %q, %q)", row.SourceType, row.SourceLabel, row.Bank)
	}
	if row.TransactionSource != "HDFC Credit Card" {
		t.Fatalf("TransactionSource = %q", row.TransactionSource)
	}
}
