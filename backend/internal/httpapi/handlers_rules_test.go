package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/assistant"
	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func TestCreateRuleDraftReturnsDraftMatchesAndValidationIssues(t *testing.T) {
	service := &stubRuleDraftService{result: assistant.RuleDraftResult{
		Draft: assistant.RuleDraft{
			Name:            "ICICI Credit Card",
			SenderEmails:    []string{"alerts@example.com"},
			SubjectContains: "Card alert",
			AmountRegex:     `INR\s+([0-9.]+)`,
			MerchantRegex:   `at\s+(.+)`,
			CurrencyRegex:   `(INR)`,
			Source:          api.Source{Type: "Credit Card", Bank: "ICICI", Label: "ICICI Credit Card"},
			Notes:           "review amount",
		},
		Matches: []assistant.SampleMatch{{
			SampleIndex: 1,
			SampleName:  "Sample 2",
			Amount:      "1521.00",
			Merchant:    "Amazon",
			Currency:    "INR",
		}},
		ValidationIssues: []assistant.RuleDraftSampleIssue{{
			SampleIndex: 1,
			SampleName:  "Sample 2",
			Field:       "amount",
			Expected:    "1522.00",
			Actual:      "1521.00",
			Message:     `Amount matched "1521.00", expected "1522.00".`,
		}},
	}}
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.ruleDrafts = service
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	body := `{
		"name":"Current",
		"sender_emails":["alerts@example.com"],
		"subject_contains":"Card alert",
		"amount_regex":"",
		"merchant_regex":"",
		"currency_regex":"",
		"source":{"type":"Credit Card","bank":"ICICI","label":"ICICI Credit Card"},
		"samples":[
			{"name":"Sample 1","sender":"alerts@example.com","subject":"Card alert","body":"INR 10 at Cafe","expected":{"amount":"10","merchant":"Cafe","currency":"INR"}},
			{"name":"Sample 2","sender":"alerts@example.com","subject":"Card alert","body":"INR 1521.00 at Amazon","expected":{"amount":"1522.00","merchant":"Amazon","currency":"INR"}}
		]
	}`
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/rule-drafts", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.CreateRuleDraft(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if service.tenant.ID != "tenant-a" {
		t.Fatalf("tenant = %+v, want request tenant", service.tenant)
	}
	if len(service.input.Samples) != 2 || service.input.Samples[1].Expected.Amount != "1522.00" {
		t.Fatalf("service input = %+v, want decoded samples", service.input)
	}
	var resp ruleDraftResponseJSON
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.Draft.Name != "ICICI Credit Card" || len(resp.Matches) != 1 || len(resp.ValidationIssues) != 1 {
		t.Fatalf("response = %+v, want draft, match and issue", resp)
	}
	if resp.Matches[0].SampleIndex != 1 || resp.ValidationIssues[0].SampleIndex != 1 || resp.ValidationIssues[0].Field != "amount" {
		t.Fatalf("response = %+v, want sample-indexed validation issue", resp)
	}
}

func TestCreateRuleDraftValidatesSamplesBeforeCallingService(t *testing.T) {
	service := &stubRuleDraftService{}
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.ruleDrafts = service
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"/api/rule-drafts",
		strings.NewReader(`{"samples":[{"body":"email","expected":{"amount":"10","merchant":""}}]}`),
	)
	rr := httptest.NewRecorder()

	h.CreateRuleDraft(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if service.input.Samples != nil {
		t.Fatalf("service was called with %+v, want validation to stop request", service.input)
	}
	assertValidationError(t, rr, "samples.expected", "body", "must include expected amount and merchant for at least one email body")
}

func TestListRules_ReturnsSourceObjectAndSenderEmails(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{{
		ID:              "1",
		Name:            "HDFC Credit Card",
		SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
		SubjectContains: "HDFC Credit Card",
		AmountRegex:     `Rs\.([\d.]+)`,
		MerchantRegex:   `at (.*?) on`,
		SourceType:      "Credit Card",
		SourceLabel:     "HDFC Credit Card",
		Bank:            "HDFC",
	}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules", nil)
	rr := httptest.NewRecorder()

	h.ListRules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp []struct {
		Name         string     `json:"name"`
		SenderEmails []string   `json:"sender_emails"`
		Source       api.Source `json:"source"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(resp))
	}
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, resp[0].SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", resp[0].SenderEmails, want)
	}
	if want := (api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"}); resp[0].Source != want {
		t.Fatalf("source = %#v, want %#v", resp[0].Source, want)
	}
}

const validRuleBody = `{
	"name":"New Rule",
	"sender_emails":["alerts@example.com"],
	"amount_regex":"Rs\\.([\\d.]+)",
	"merchant_regex":"at (.*?) on",
	"currency_regex":"(INR)",
	"source":{"type":"Credit Card","label":"Example Card","bank":"Example Bank"}
}`

func TestCreateRule_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()
	h.CreateRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestCreateRule_AcceptsSourceObjectAndSenderEmails(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"HDFC Credit Card",
		"sender_emails":["alerts@hdfcbank.net","alerts@hdfcbank.bank.in"],
		"subject_contains":"HDFC Credit Card",
		"amount_regex":"Rs\\.([\\d.]+)",
		"merchant_regex":"at (.*?) on",
		"source":{"type":"Credit Card","label":"HDFC Credit Card","bank":"HDFC"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		SenderEmails []string   `json:"sender_emails"`
		Source       api.Source `json:"source"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, resp.SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", resp.SenderEmails, want)
	}
	if want := (api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"}); resp.Source != want {
		t.Fatalf("source = %#v, want %#v", resp.Source, want)
	}
}

func TestCreateRule_DuplicateNameReturns409(t *testing.T) {
	h := newTestHandlers(t, &mockStore{ruleErr: errStoreRuleNameConflict}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["message"] != "rule name already exists" {
		t.Fatalf("message = %q, want rule name already exists", resp["message"])
	}
}

func TestCreateRule_ClearsActiveReaderCheckpoint(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		appConfig:     map[string]string{"reader.gmail.last_scan_at": "2026-04-27T00:00:00Z"},
	}
	dm := &mockDaemon{}
	h := newTestHandlers(t, ms, dm)
	var restarted daemon.RunRequest
	h.daemon.(*mockDaemon).restartFn = func(req daemon.RunRequest) { restarted = req }

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := ms.appConfig["reader.gmail.last_scan_at"]; got != "" {
		t.Fatalf("reader checkpoint = %q, want empty", got)
	}
	if restarted.Reader != "" {
		t.Fatalf("restartFn called while daemon stopped: %q", restarted.Reader)
	}
}

func TestCreateRule_RestartsRunningDaemonAfterCheckpointClear(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		appConfig:     map[string]string{"reader.gmail.last_scan_at": "2026-04-27T00:00:00Z"},
	}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	var restarted daemon.RunRequest
	h.daemon.(*mockDaemon).restartFn = func(req daemon.RunRequest) { restarted = req }

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if restarted.Reader != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted.Reader)
	}
	if restarted.Tenant.ID != "tenant-a" {
		t.Fatalf("restartFn tenant = %q, want tenant-a", restarted.Tenant.ID)
	}
}

func TestCreateRule_MissingAmountRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"test",
		"sender_emails":["alerts@example.com"],
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.CreateRule(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestCreateRule_InvalidAmountRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"test",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"[invalid",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.CreateRule(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestUpdateRule_AnyRule_FullUpdate(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: true}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{
		"name":"updated",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"\\d+",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/22222222-2222-2222-2222-222222222222", strings.NewReader(body))
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()
	h.UpdateRule(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateRule_DuplicateNameReturns409(t *testing.T) {
	ms := &mockStore{ruleErr: errStoreRuleNameConflict}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{
		"name":"duplicate",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"\\d+",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/22222222-2222-2222-2222-222222222222", strings.NewReader(body))
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()

	h.UpdateRule(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["message"] != "rule name already exists" {
		t.Fatalf("message = %q, want rule name already exists", resp["message"])
	}
}

func TestUpdateRule_InvalidIDReturns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"updated",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"\\d+",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/not-a-uuid", strings.NewReader(body))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()

	h.UpdateRule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDeleteRule_PredefinedRule_Returns403(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: true}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()
	h.DeleteRule(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestDeleteRule_UserRule_Returns204(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: false}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()
	h.DeleteRule(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestDeleteRule_InvalidIDReturns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()

	h.DeleteRule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestExportRules_OnlyNonPredefinedRules(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{
		{ID: "1", Name: "predefined", Predefined: true, AmountRegex: `\d+`, MerchantRegex: `.+`},
		{ID: "2", Name: "usr", Predefined: false, AmountRegex: `\d+`, MerchantRegex: `.+`},
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules/export", nil)
	rr := httptest.NewRecorder()
	h.ExportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var exported struct {
		Rules []struct {
			Name string `json:"name"`
		} `json:"rules"`
	}
	decodeJSON(t, rr.Body.String(), &exported)
	if len(exported.Rules) != 1 {
		t.Errorf("expected 1 exported rule (user only), got %d", len(exported.Rules))
	}
	if exported.Rules[0].Name != "usr" {
		t.Errorf("expected exported name=usr, got %v", exported.Rules[0].Name)
	}
}

func TestExportRules_UsesVersionedDocument(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{
		{ID: "1", Name: "predefined", Predefined: true, AmountRegex: `\d+`, MerchantRegex: `.+`},
		{
			ID:              "2",
			Name:            "HDFC Credit Card",
			SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
			SubjectContains: "HDFC Credit Card",
			AmountRegex:     `Rs\.([\d.]+)`,
			MerchantRegex:   `at (.*?) on`,
			SourceType:      "Credit Card",
			SourceLabel:     "HDFC Credit Card",
			Bank:            "HDFC",
		},
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules/export", nil)
	rr := httptest.NewRecorder()

	h.ExportRules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var exported struct {
		Version int `json:"version"`
		Rules   []struct {
			Name         string     `json:"name"`
			SenderEmails []string   `json:"sender_emails"`
			Source       api.Source `json:"source"`
		} `json:"rules"`
	}
	decodeJSON(t, rr.Body.String(), &exported)
	if exported.Version != 2 {
		t.Fatalf("version = %d, want 2", exported.Version)
	}
	if len(exported.Rules) != 1 {
		t.Fatalf("expected 1 exported user rule, got %d", len(exported.Rules))
	}
	if exported.Rules[0].Name != "HDFC Credit Card" {
		t.Fatalf("exported rule name = %q", exported.Rules[0].Name)
	}
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, exported.Rules[0].SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", exported.Rules[0].SenderEmails, want)
	}
}

func TestImportRules_AcceptsVersionedDocument(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{
		"version":2,
		"presets":{"source_types":[{"value":"Credit Card","origin":"predefined"}],"banks":[{"value":"HDFC","origin":"custom"}]},
		"rules":[{
			"name":"HDFC Credit Card",
			"sender_emails":["alerts@hdfcbank.net","alerts@hdfcbank.bank.in"],
			"subject_contains":"HDFC Credit Card",
			"amount_regex":"Rs\\.([\\d.]+)",
			"merchant_regex":"at (.*?) on",
			"source":{"type":"Credit Card","label":"HDFC Credit Card","bank":"HDFC"}
		}]
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules/import", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.ImportRules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if len(ms.importedRules) != 1 {
		t.Fatalf("imported rules = %d, want 1", len(ms.importedRules))
	}
	got := ms.importedRules[0]
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, got.SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", got.SenderEmails, want)
	}
	if got.SourceType != "Credit Card" || got.SourceLabel != "HDFC Credit Card" || got.Bank != "HDFC" {
		t.Fatalf("source fields = (%q, %q, %q)", got.SourceType, got.SourceLabel, got.Bank)
	}
}

func TestImportRules_InvalidRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `[{"name":"bad","amountRegex":"[invalid","merchantInfoRegex":".+"}]`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules/import", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.ImportRules(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
