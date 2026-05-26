package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	pkgapi "github.com/ArionMiles/expensor/backend/pkg/api"
	pkgrules "github.com/ArionMiles/expensor/backend/pkg/rules"
)

// --- rules ---

type ruleHTTPJSON struct {
	ID                string        `json:"id,omitempty"`
	Name              string        `json:"name"`
	SenderEmail       string        `json:"sender_email,omitempty"`
	SenderEmails      []string      `json:"sender_emails"`
	SubjectContains   string        `json:"subject_contains"`
	AmountRegex       string        `json:"amount_regex"`
	MerchantRegex     string        `json:"merchant_regex"`
	CurrencyRegex     string        `json:"currency_regex"`
	TransactionSource string        `json:"transaction_source,omitempty"`
	SourceType        string        `json:"source_type,omitempty"`
	SourceLabel       string        `json:"source_label,omitempty"`
	Bank              string        `json:"bank,omitempty"`
	Source            pkgapi.Source `json:"source"`
	Predefined        bool          `json:"predefined"`
	CreatedAt         time.Time     `json:"created_at,omitempty"`
	UpdatedAt         time.Time     `json:"updated_at,omitempty"`
}

type ruleDocumentJSON struct {
	Version int                 `json:"version"`
	Presets pkgrules.Presets    `json:"presets"`
	Rules   []ruleDocumentEntry `json:"rules"`
}

type ruleDocumentEntry struct {
	Name            string        `json:"name"`
	SenderEmails    []string      `json:"sender_emails"`
	SubjectContains string        `json:"subject_contains"`
	AmountRegex     string        `json:"amount_regex"`
	MerchantRegex   string        `json:"merchant_regex"`
	CurrencyRegex   string        `json:"currency_regex"`
	Source          pkgapi.Source `json:"source"`
}

func ruleRowToHTTP(row store.RuleRow) ruleHTTPJSON {
	source := pkgapi.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank}
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
		row.TransactionSource = pkgapi.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank}.Display()
	}
	return row
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

func ruleToRow(rule pkgapi.Rule) store.RuleRow {
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
		Source:          pkgapi.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank},
	}
}

func ruleDocumentPresets(entries []ruleDocumentEntry) pkgrules.Presets {
	return pkgrules.Presets{
		SourceTypes: presetValuesFromRules(entries, func(source pkgapi.Source) string { return source.Type }),
		Banks:       presetValuesFromRules(entries, func(source pkgapi.Source) string { return source.Bank }),
	}
}

func presetValuesFromRules(entries []ruleDocumentEntry, value func(pkgapi.Source) string) []pkgrules.PresetValue {
	seen := map[string]struct{}{}
	out := []pkgrules.PresetValue{}
	for _, entry := range entries {
		v := strings.TrimSpace(value(entry.Source))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, pkgrules.PresetValue{Value: v, Origin: "custom"})
	}
	return out
}

// validateRuleRegexes compiles the three regex fields on a RuleRow and returns the first error.
// An empty pattern is skipped (optional fields are allowed to be unset for updates).
func validateRuleRegexes(amountRegex, merchantRegex, currencyRegex string) error {
	if amountRegex != "" {
		if _, err := regexp.Compile(amountRegex); err != nil {
			return fmt.Errorf("invalid amount_regex: %w", err)
		}
	}
	if merchantRegex != "" {
		if _, err := regexp.Compile(merchantRegex); err != nil {
			return fmt.Errorf("invalid merchant_regex: %w", err)
		}
	}
	if currencyRegex != "" {
		if _, err := regexp.Compile(currencyRegex); err != nil {
			return fmt.Errorf("invalid currency_regex: %w", err)
		}
	}
	return nil
}

func validateRuleRow(row store.RuleRow) error {
	if row.Name == "" {
		return errors.New("name is required")
	}
	if len(row.SenderEmails) == 0 {
		return errors.New("sender_emails is required")
	}
	if row.AmountRegex == "" {
		return errors.New("amount_regex is required")
	}
	if row.MerchantRegex == "" {
		return errors.New("merchant_regex is required")
	}
	if row.SourceType == "" {
		return errors.New("source.type is required")
	}
	if row.Bank == "" {
		return errors.New("source.bank is required")
	}
	return validateRuleRegexes(row.AmountRegex, row.MerchantRegex, row.CurrencyRegex)
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
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	rules, err := h.store.ListRules(r.Context())
	if err != nil {
		h.logger.Error("list rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, ruleRowsToHTTP(rules))
}

// CreateRule handles POST /api/rules.
//
// @Summary Create a rule
// @Tags Rules
// @Accept json
// @Produce json
// @Param request body RuleMutationRequest true "Rule payload"
// @Success 201 {object} RuleResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules [post]
func (h *Handlers) CreateRule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body ruleHTTPJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	row := ruleHTTPToRow(body)
	if err := validateRuleRow(row); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	created, err := h.store.CreateRule(r.Context(), row)
	if err != nil {
		if errors.Is(err, store.ErrRuleNameConflict) {
			writeError(w, http.StatusConflict, "rule name already exists")
			return
		}
		h.logger.Error("create rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}
	h.clearActiveReaderCheckpointForNewRule(r.Context())
	writeJSON(w, http.StatusCreated, ruleRowToHTTP(*created))
}

func (h *Handlers) clearActiveReaderCheckpointForNewRule(ctx context.Context) {
	reader, err := h.readActiveReader(ctx)
	if err != nil || strings.TrimSpace(reader) == "" || h.store == nil {
		return
	}
	reader = strings.TrimSpace(reader)
	if err := h.store.SetAppConfig(ctx, "reader."+reader+".last_scan_at", ""); err != nil {
		h.logger.Warn("failed to clear checkpoint after rule creation", "reader", reader, "error", err)
		return
	}
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(reader)
	}
}

func (h *Handlers) readActiveReader(ctx context.Context) (string, error) {
	if h.store == nil {
		return "", nil
	}
	return h.store.GetActiveReader(ctx)
}

// UpdateRule handles PUT /api/rules/{id}.
// All rules (predefined and user-created) are fully editable.
//
// @Summary Update a rule
// @Tags Rules
// @Accept json
// @Produce json
// @Param id path string true "Rule ID"
// @Param request body RuleMutationRequest true "Rule payload"
// @Success 200 {object} RuleResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/{id} [put]
func (h *Handlers) UpdateRule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body ruleHTTPJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	row := ruleHTTPToRow(body)
	if err := validateRuleRow(row); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updated, err := h.store.UpdateRule(r.Context(), id, row)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		if errors.Is(err, store.ErrRuleNameConflict) {
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
// @Produce json
// @Param id path string true "Rule ID"
// @Success 204
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/{id} [delete]
func (h *Handlers) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	existing, err := h.store.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
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
	if err := h.store.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
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
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	all, err := h.store.ListRules(r.Context())
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
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /rules/import [post]
func (h *Handlers) ImportRules(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	doc, err := pkgrules.ParseDocument(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	rows := make([]store.RuleRow, 0, len(doc.Rules))
	for _, rule := range doc.Rules {
		rows = append(rows, ruleToRow(rule))
	}
	if err := h.store.ImportUserRules(r.Context(), rows); err != nil {
		h.logger.Error("import rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to import rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": len(rows)})
}
