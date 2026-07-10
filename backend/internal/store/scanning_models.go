package store

import "time"

type ScanningState string

const (
	ScanningStateQueued              ScanningState = "queued"
	ScanningStateStarting            ScanningState = "starting"
	ScanningStateRunning             ScanningState = "running"
	ScanningStateBackingOff          ScanningState = "backing_off"
	ScanningStateNeedsAuth           ScanningState = "needs_auth"
	ScanningStateReaderNotConfigured ScanningState = "reader_not_configured"
	ScanningStatePaused              ScanningState = "paused"
	ScanningStateStopped             ScanningState = "stopped"
)

type ScanningReasonCode string

const (
	ScanningReasonNone                ScanningReasonCode = ""
	ScanningReasonMissingCredentials  ScanningReasonCode = "needs_auth_missing_credentials"
	ScanningReasonMissingToken        ScanningReasonCode = "needs_auth_missing_token"
	ScanningReasonInvalidGrant        ScanningReasonCode = "needs_auth_invalid_grant"
	ScanningReasonReaderNotConfigured ScanningReasonCode = "reader_not_configured"
	ScanningReasonTemporaryFailure    ScanningReasonCode = "temporary_failure"
)

type SchedulerConfig struct {
	MaxConcurrentScans int
	UpdatedAt          time.Time
}

type SchedulerConfigPatch struct {
	MaxConcurrentScans *int
}

type TenantScanningState struct {
	TenantID      string
	ActiveReader  string
	Enabled       bool
	State         ScanningState
	ReasonCode    ScanningReasonCode
	PublicMessage string
	LastStartedAt *time.Time
	LastStoppedAt *time.Time
	LastFailedAt  *time.Time
	NextRetryAt   *time.Time
	RetryCount    int
	UpdatedAt     time.Time
}

type ScanningStateUpdate struct {
	State         ScanningState
	ReasonCode    ScanningReasonCode
	PublicMessage string
	LastStartedAt *time.Time
	LastStoppedAt *time.Time
	LastFailedAt  *time.Time
	NextRetryAt   *time.Time
	RetryCount    *int
}
