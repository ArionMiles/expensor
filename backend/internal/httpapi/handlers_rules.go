package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// --- rules ---

type ruleHTTPJSON struct {
	ID                string     `json:"id,omitempty"`
	Name              string     `json:"name"`
	SenderEmail       string     `json:"sender_email,omitempty"`
	SenderEmails      []string   `json:"sender_emails"`
	SubjectContains   string     `json:"subject_contains"`
	AmountRegex       string     `json:"amount_regex"`
	MerchantRegex     string     `json:"merchant_regex"`
	CurrencyRegex     string     `json:"currency_regex"`
	TransactionSource string     `json:"transaction_source,omitempty"`
	SourceType        string     `json:"source_type,omitempty"`
	SourceLabel       string     `json:"source_label,omitempty"`
	Bank              string     `json:"bank,omitempty"`
	Source            api.Source `json:"source"`
	Predefined        bool       `json:"predefined"`
	CreatedAt         time.Time  `json:"created_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at,omitempty"`
}

type ruleDocumentJSON struct {
	Version int                 `json:"version"`
	Presets rules.Presets       `json:"presets"`
	Rules   []ruleDocumentEntry `json:"rules"`
}

type ruleDocumentEntry struct {
	Name            string     `json:"name"`
	SenderEmails    []string   `json:"sender_emails"`
	SubjectContains string     `json:"subject_contains"`
	AmountRegex     string     `json:"amount_regex"`
	MerchantRegex   string     `json:"merchant_regex"`
	CurrencyRegex   string     `json:"currency_regex"`
	Source          api.Source `json:"source"`
}

func ruleRowToHTTP(row store.RuleRow) ruleHTTPJSON {
	source := api.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank}
	return ruleHTTPJSON{
		ID:                row.ID,
		Name:              row.Name,
		SenderEmail:       row.SenderEmail,
		SenderEmails:      normalizedHTTPSenders(row.SenderEmails, row.SenderEmail),
		SubjectContains:   row.SubjectContains,
		AmountRegex:       row.AmountRegex,
		MerchantRegex:     row.MerchantRegex,
		CurrencyRegex:     row.CurrencyRegex,
		TransactionSource: row.TransactionSource,
		SourceType:        row.SourceType,
		SourceLabel:       row.SourceLabel,
		Bank:              row.Bank,
		Source:            source,
		Predefined:        row.Predefined,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func ruleRowsToHTTP(rows []store.RuleRow) []ruleHTTPJSON {
	out := make([]ruleHTTPJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, ruleRowToHTTP(row))
	}
	return out
}

func ruleHTTPToRow(body ruleHTTPJSON) store.RuleRow {
	source := body.Source
	if source.Type == "" {
		source.Type = body.SourceType
	}
	if source.Label == "" {
		source.Label = body.SourceLabel
	}
	if source.Bank == "" {
		source.Bank = body.Bank
	}
	senders := normalizedHTTPSenders(body.SenderEmails, body.SenderEmail)
	row := store.RuleRow{
		Name:              strings.TrimSpace(body.Name),
		SenderEmail:       "",
		SenderEmails:      senders,
		SubjectContains:   strings.TrimSpace(body.SubjectContains),
		AmountRegex:       strings.TrimSpace(body.AmountRegex),
		MerchantRegex:     strings.TrimSpace(body.MerchantRegex),
		CurrencyRegex:     strings.TrimSpace(body.CurrencyRegex),
		TransactionSource: strings.TrimSpace(body.TransactionSource),
		SourceType:        strings.TrimSpace(source.Type),
		SourceLabel:       strings.TrimSpace(source.Label),
		Bank:              strings.TrimSpace(source.Bank),
	}
	if len(senders) > 0 {
		row.SenderEmail = senders[0]
	}
	if row.TransactionSource == "" {
		row.TransactionSource = api.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank}.Display()
	}
	return row
}

func ruleMutationToRow(body RuleMutationRequest) store.RuleRow {
	return ruleHTTPToRow(ruleHTTPJSON{
		Name:            body.Name,
		SenderEmails:    body.SenderEmails,
		SubjectContains: body.SubjectContains,
		AmountRegex:     body.AmountRegex,
		MerchantRegex:   body.MerchantRegex,
		CurrencyRegex:   body.CurrencyRegex,
		Source: api.Source{
			Type:  body.Source.Type,
			Label: body.Source.Label,
			Bank:  body.Source.Bank,
		},
	})
}

func normalizedHTTPSenders(senders []string, fallback string) []string {
	seen := make(map[string]struct{}, len(senders)+1)
	out := make([]string, 0, len(senders)+1)
	for _, value := range append(senders, fallback) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func ruleToRow(rule api.Rule) store.RuleRow {
	row := store.RuleRow{
		Name:            rule.Name,
		SenderEmail:     rule.SenderEmail,
		SenderEmails:    normalizedHTTPSenders(rule.SenderEmails, rule.SenderEmail),
		SubjectContains: rule.SubjectContains,
		SourceType:      rule.Source.Type,
		SourceLabel:     rule.Source.Label,
		Bank:            rule.Source.Bank,
	}
	if row.SenderEmail == "" && len(row.SenderEmails) > 0 {
		row.SenderEmail = row.SenderEmails[0]
	}
	if rule.Amount != nil {
		row.AmountRegex = rule.Amount.String()
	}
	if rule.MerchantInfo != nil {
		row.MerchantRegex = rule.MerchantInfo.String()
	}
	if rule.Currency != nil {
		row.CurrencyRegex = rule.Currency.String()
	}
	row.TransactionSource = rule.Source.Display()
	return row
}

func ruleDocumentEntryFromRow(row store.RuleRow) ruleDocumentEntry {
	return ruleDocumentEntry{
		Name:            row.Name,
		SenderEmails:    normalizedHTTPSenders(row.SenderEmails, row.SenderEmail),
		SubjectContains: row.SubjectContains,
		AmountRegex:     row.AmountRegex,
		MerchantRegex:   row.MerchantRegex,
		CurrencyRegex:   row.CurrencyRegex,
		Source:          api.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank},
	}
}

func ruleDocumentPresets(entries []ruleDocumentEntry) rules.Presets {
	return rules.Presets{
		SourceTypes: presetValuesFromRules(entries, func(source api.Source) string { return source.Type }),
		Banks:       presetValuesFromRules(entries, func(source api.Source) string { return source.Bank }),
	}
}

func presetValuesFromRules(entries []ruleDocumentEntry, value func(api.Source) string) []rules.PresetValue {
	seen := map[string]struct{}{}
	out := []rules.PresetValue{}
	for _, entry := range entries {
		v := strings.TrimSpace(value(entry.Source))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, rules.PresetValue{Value: v, Origin: "custom"})
	}
	return out
}

// ListRules handles GET /api/rules.
//
// @Summary List rules
// @Tags Rules
// @Accept json
// @Produce json
// @Success 200 {array} RuleResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules [get]
func (h *Handlers) ListRules(w http.ResponseWriter, r *http.Request) {
	ruleRows, err := h.ruleStore.ListRules(r.Context(), requestTenant(r))
	if err != nil {
		h.logger.Error("list rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, ruleRowsToHTTP(ruleRows))
}

// CreateRule handles POST /api/rules.
//
// @Summary Create a rule
// @Tags Rules
// @Accept json
// @Produce json
// @Param request body RuleMutationRequest true "Rule payload"
// @Success 201 {object} RuleResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules [post]
func (h *Handlers) CreateRule(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[RuleMutationRequest](h, w, r)
	if !ok {
		return
	}
	row := ruleMutationToRow(body)
	created, err := h.ruleStore.CreateRule(r.Context(), requestTenant(r), row)
	if err != nil {
		if errors.WhatKind(err) == errors.Conflict {
			writeError(w, http.StatusConflict, "rule name already exists")
			return
		}
		h.logger.Error("create rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}
	h.clearActiveReaderCheckpointForNewRule(r.Context(), requestTenant(r))
	writeJSON(w, http.StatusCreated, ruleRowToHTTP(*created))
}

func (h *Handlers) clearActiveReaderCheckpointForNewRule(ctx context.Context, tenant store.Tenant) {
	reader, err := h.readActiveReader(ctx, tenant)
	if err != nil || strings.TrimSpace(reader) == "" {
		return
	}
	reader = strings.TrimSpace(reader)
	if err := h.settingsStore.SetAppConfig(ctx, tenant, "reader."+reader+".last_scan_at", ""); err != nil {
		h.logger.Warn("failed to clear checkpoint after rule creation", "reader", reader, "error", err)
		return
	}
	if h.daemon != nil && h.daemon.Status().Running {
		h.daemon.Restart(daemon.RunRequest{Tenant: tenant, Reader: reader})
	}
}

func (h *Handlers) readActiveReader(ctx context.Context, tenant store.Tenant) (string, error) {
	state, err := h.scanningStore.GetScanningState(ctx, tenant)
	if err != nil {
		return "", err
	}
	return state.ActiveReader, nil
}

// UpdateRule handles PUT /api/rules/{id}.
// All rules (predefined and user-created) are fully editable.
//
// @Summary Update a rule
// @Tags Rules
// @Accept json
// @Produce json
// @Param id path string true "Rule ID" format(uuid) example(00000000-0000-0000-0000-00000000c001)
// @Param request body RuleMutationRequest true "Rule payload"
// @Success 200 {object} RuleResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/{id} [put]
func (h *Handlers) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "rule")
	if !ok {
		return
	}
	body, ok := decodeAndValidateJSON[RuleMutationRequest](h, w, r)
	if !ok {
		return
	}
	row := ruleMutationToRow(body)
	updated, err := h.ruleStore.UpdateRule(r.Context(), requestTenant(r), id, row)
	if err != nil {
		if errors.WhatKind(err) == errors.NotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		if errors.WhatKind(err) == errors.Conflict {
			writeError(w, http.StatusConflict, "rule name already exists")
			return
		}
		h.logger.Error("update rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}
	writeJSON(w, http.StatusOK, ruleRowToHTTP(*updated))
}

// DeleteRule handles DELETE /api/rules/{id}.
// Returns 403 for system rules.
//
// @Summary Delete a rule
// @Tags Rules
// @Accept json
// @Param id path string true "Rule ID" format(uuid) example(00000000-0000-0000-0000-00000000c001)
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/{id} [delete]
func (h *Handlers) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidPathValue(w, r, "id", "rule")
	if !ok {
		return
	}
	existing, err := h.ruleStore.GetRule(r.Context(), requestTenant(r), id)
	if err != nil {
		if errors.WhatKind(err) == errors.NotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch rule")
		return
	}
	if existing.Predefined {
		writeError(w, http.StatusForbidden, "predefined rules cannot be deleted")
		return
	}
	if err := h.ruleStore.DeleteRule(r.Context(), requestTenant(r), id); err != nil {
		if errors.WhatKind(err) == errors.NotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		h.logger.Error("delete rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ExportRules handles GET /api/rules/export.
// Downloads all user rules as a JSON file in rules.json format.
//
// @Summary Export rules
// @Tags Rules
// @Accept json
// @Produce json
// @Success 200 {object} RuleDocumentResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/export [get]
func (h *Handlers) ExportRules(w http.ResponseWriter, r *http.Request) {
	all, err := h.ruleStore.ListRules(r.Context(), requestTenant(r))
	if err != nil {
		h.logger.Error("export rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch rules")
		return
	}
	export := make([]ruleDocumentEntry, 0)
	for _, row := range all {
		if row.Predefined {
			continue // export only user-created rules
		}
		export = append(export, ruleDocumentEntryFromRow(row))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="expensor-rules.json"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ruleDocumentJSON{Version: 2, Presets: ruleDocumentPresets(export), Rules: export})
}

// ImportRules handles POST /api/rules/import.
// Validates all rules first; rejects the entire import if any rule fails.
//
// @Summary Import rules
// @Tags Rules
// @Accept json
// @Produce json
// @Param request body RuleDocumentResponse true "Rules document"
// @Success 200 {object} RuleImportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/import [post]
func (h *Handlers) ImportRules(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	doc, err := rules.ParseDocument(body)
	if err != nil {
		writeValidationErrors(w, []ValidationErrorDetail{ruleImportValidationError(err)})
		return
	}
	rows := make([]store.RuleRow, 0, len(doc.Rules))
	for _, rule := range doc.Rules {
		rows = append(rows, ruleToRow(rule))
	}
	if err := h.ruleStore.ImportUserRules(r.Context(), requestTenant(r), rows); err != nil {
		h.logger.Error("import rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to import rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": len(rows)})
}

func ruleImportValidationError(err error) ValidationErrorDetail {
	message := err.Error()
	field := "rules"
	for _, candidate := range []string{"version", "sender_emails", "amount_regex", "merchant_regex", "currency_regex", "source", "name"} {
		if strings.Contains(message, candidate) {
			field = candidate
			break
		}
	}
	return ValidationErrorDetail{Field: field, Location: "body", Message: message}
}
