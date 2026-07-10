package postgres

import shared "github.com/ArionMiles/expensor/backend/internal/store"

type UserRole = shared.UserRole

const (
	UserRoleAdmin = shared.UserRoleAdmin
	UserRoleUser  = shared.UserRoleUser
)

type (
	Tenant                       = shared.Tenant
	User                         = shared.User
	CreateBootstrapAdminInput    = shared.CreateBootstrapAdminInput
	CreateUserInput              = shared.CreateUserInput
	UpdateUserInput              = shared.UpdateUserInput
	UpdateUserPasswordInput      = shared.UpdateUserPasswordInput
	CompleteAccountSetupInput    = shared.CompleteAccountSetupInput
	Session                      = shared.Session
	CreateSessionInput           = shared.CreateSessionInput
	AccessToken                  = shared.AccessToken
	CreateAccessTokenInput       = shared.CreateAccessTokenInput
	AccountSetupToken            = shared.AccountSetupToken
	CreateAccountSetupTokenInput = shared.CreateAccountSetupTokenInput
	Transaction                  = shared.Transaction
	MutedMerchant                = shared.MutedMerchant
	MutedMerchantWithCount       = shared.MutedMerchantWithCount
	Stats                        = shared.Stats
	CategoryMonthlyEntry         = shared.CategoryMonthlyEntry
	TimeBucket                   = shared.TimeBucket
	ChartData                    = shared.ChartData
	DashboardSection             = shared.DashboardSection
	DashboardData                = shared.DashboardData
	MonthlyBreakdownSeries       = shared.MonthlyBreakdownSeries
	MonthlyBreakdownData         = shared.MonthlyBreakdownData
	Facets                       = shared.Facets
	WeekdayHourBucket            = shared.WeekdayHourBucket
	DayOfMonthBucket             = shared.DayOfMonthBucket
	HeatmapData                  = shared.HeatmapData
	DailyBucket                  = shared.DailyBucket
	Label                        = shared.Label
	Category                     = shared.Category
	Bucket                       = shared.Bucket
	MCCEntry                     = shared.MCCEntry
	MerchantCategoryEntry        = shared.MerchantCategoryEntry
	SyncStatus                   = shared.SyncStatus
	CommunitySyncSettings        = shared.CommunitySyncSettings
	CommunitySyncSettingsPatch   = shared.CommunitySyncSettingsPatch
	LLMProviderRuntime           = shared.LLMProviderRuntime
	RuleRow                      = shared.RuleRow
	ExtractionDiagnosticRow      = shared.ExtractionDiagnosticRow
	DiagnosticFilter             = shared.DiagnosticFilter
	TransactionUpdate            = shared.TransactionUpdate
	TransactionListResult        = shared.TransactionListResult
	ListFilter                   = shared.ListFilter
	IngestionConfig              = shared.IngestionConfig
	InsertParams                 = shared.InsertParams
	ScanningState                = shared.ScanningState
	ScanningReasonCode           = shared.ScanningReasonCode
	SchedulerConfig              = shared.SchedulerConfig
	SchedulerConfigPatch         = shared.SchedulerConfigPatch
	TenantScanningState          = shared.TenantScanningState
	ScanningStateUpdate          = shared.ScanningStateUpdate
)

const (
	DiagnosticStatusOpen     = shared.DiagnosticStatusOpen
	DiagnosticStatusResolved = shared.DiagnosticStatusResolved
	DiagnosticStatusIgnored  = shared.DiagnosticStatusIgnored
	DiagnosticStatusAll      = shared.DiagnosticStatusAll

	ScanningStateQueued              = shared.ScanningStateQueued
	ScanningStateStarting            = shared.ScanningStateStarting
	ScanningStateRunning             = shared.ScanningStateRunning
	ScanningStateBackingOff          = shared.ScanningStateBackingOff
	ScanningStateNeedsAuth           = shared.ScanningStateNeedsAuth
	ScanningStateReaderNotConfigured = shared.ScanningStateReaderNotConfigured
	ScanningStatePaused              = shared.ScanningStatePaused
	ScanningStateStopped             = shared.ScanningStateStopped

	ScanningReasonNone                = shared.ScanningReasonNone
	ScanningReasonMissingCredentials  = shared.ScanningReasonMissingCredentials
	ScanningReasonMissingToken        = shared.ScanningReasonMissingToken
	ScanningReasonInvalidGrant        = shared.ScanningReasonInvalidGrant
	ScanningReasonReaderNotConfigured = shared.ScanningReasonReaderNotConfigured
	ScanningReasonTemporaryFailure    = shared.ScanningReasonTemporaryFailure
)

var (
	ValidateDiagnosticFilterStatus = shared.ValidateDiagnosticFilterStatus
	validateDiagnosticRowStatus    = shared.ValidateDiagnosticUpdateStatus
)
