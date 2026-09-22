package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const ruleColumns = `id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
	currency_regex, transaction_source, source_type, source_label, bank, predefined, created_at, updated_at`

type newRuleMetadata struct {
	id  string
	now string
}

func primarySender(rule store.RuleRow) string {
	if rule.SenderEmail != "" {
		return rule.SenderEmail
	}
	if len(rule.SenderEmails) > 0 {
		return rule.SenderEmails[0]
	}
	return ""
}

func normalizedRuleSenders(rule store.RuleRow) []string {
	if len(rule.SenderEmails) > 0 {
		return rule.SenderEmails
	}
	if rule.SenderEmail != "" {
		return []string{rule.SenderEmail}
	}
	return []string{}
}

func ruleSourceLabel(rule store.RuleRow) string {
	if rule.SourceLabel != "" {
		return rule.SourceLabel
	}
	return rule.TransactionSource
}

func (s *Store) ListRules(ctx context.Context, tenant store.Tenant) ([]store.RuleRow, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT `+ruleColumns+`
		FROM rules WHERE predefined = 1 OR tenant_id = ? ORDER BY predefined, name`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.rules.list", err)
	}
	defer rows.Close()
	return scanRuleRows(rows)
}

func (s *Store) GetRule(ctx context.Context, tenant store.Tenant, id string) (*store.RuleRow, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT `+ruleColumns+`
		FROM rules WHERE id = ? AND (predefined = 1 OR tenant_id = ?)`, id, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.rules.get", err)
	}
	defer rows.Close()
	result, err := scanRuleRows(rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.B.Op("store.rules.get").KindNotFound().UserMsg("rule not found").Build()
	}
	return &result[0], nil
}

func (s *Store) CreateRule(ctx context.Context, tenant store.Tenant, rule store.RuleRow) (*store.RuleRow, error) {
	var result *store.RuleRow
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		created, err := insertRule(ctx, q, tenant, rule, newRuleMetadata{
			id:  uuid.NewString(),
			now: timeToText(s.repositories.now()),
		})
		result = created
		return err
	})
	if err != nil {
		if errors.WhatKind(err) == errors.Conflict {
			return nil, errors.B.Op("store.rules.create").KindConflict().UserMsg("rule name already exists").Err(err).Build()
		}
		return nil, errors.B.Op("sqlite.rules.create").Text("creating rule").Err(err).Build()
	}
	return result, nil
}

func insertRule(
	ctx context.Context,
	q queryer,
	tenant store.Tenant,
	rule store.RuleRow,
	metadata newRuleMetadata,
) (*store.RuleRow, error) {
	senders, err := jsonArrayToText(normalizedRuleSenders(rule))
	if err != nil {
		return nil, err
	}
	row := q.QueryRowContext(ctx, `INSERT INTO rules (
		id, tenant_id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
		currency_regex, transaction_source, source_type, source_label, bank, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+ruleColumns,
		metadata.id, tenant.ID, rule.Name, primarySender(rule), senders, rule.SubjectContains, rule.AmountRegex,
		rule.MerchantRegex, rule.CurrencyRegex, ruleSourceLabel(rule), rule.SourceType,
		ruleSourceLabel(rule), rule.Bank, metadata.now, metadata.now)
	result, err := scanRule(row)
	if err != nil {
		return nil, mapSQLiteError("sqlite.rules.insert", err)
	}
	return &result, nil
}

func (s *Store) UpdateRule(ctx context.Context, tenant store.Tenant, id string, rule store.RuleRow) (*store.RuleRow, error) {
	senders, err := jsonArrayToText(normalizedRuleSenders(rule))
	if err != nil {
		return nil, err
	}
	var result store.RuleRow
	err = s.repositories.writeTx(ctx, func(q queryer) error {
		row := q.QueryRowContext(ctx, `UPDATE rules SET name=?, sender_email=?, sender_emails=?, subject_contains=?,
			amount_regex=?, merchant_regex=?, currency_regex=?, transaction_source=?, source_type=?, source_label=?, bank=?, updated_at=?
			WHERE id=? AND predefined=0 AND tenant_id=? RETURNING `+ruleColumns,
			rule.Name, primarySender(rule), senders, rule.SubjectContains, rule.AmountRegex, rule.MerchantRegex,
			rule.CurrencyRegex, ruleSourceLabel(rule), rule.SourceType, ruleSourceLabel(rule), rule.Bank,
			timeToText(s.repositories.now()), id, tenant.ID)
		var scanErr error
		result, scanErr = scanRule(row)
		return mapSQLiteError("sqlite.rules.update", scanErr)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.WhatKind(err) == errors.NotFound {
			return nil, errors.B.Op("store.rules.update").KindNotFound().UserMsg("rule not found").Build()
		}
		if errors.WhatKind(err) == errors.Conflict {
			return nil, errors.B.Op("store.rules.update").KindConflict().UserMsg("rule name already exists").Err(err).Build()
		}
		return nil, errors.B.Op("sqlite.rules.update").Text("updating rule").Err(err).Build()
	}
	return &result, nil
}

func (s *Store) DeleteRule(ctx context.Context, tenant store.Tenant, id string) error {
	var affected int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		result, err := q.ExecContext(ctx, `DELETE FROM rules WHERE id=? AND predefined=0 AND tenant_id=?`, id, tenant.ID)
		if err == nil {
			affected, err = result.RowsAffected()
		}
		return mapSQLiteError("sqlite.rules.delete", err)
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.B.Op("store.rules.delete").KindNotFound().UserMsg("rule not found").Build()
	}
	return nil
}

func (s *Store) SeedPredefinedRules(ctx context.Context, rules []store.RuleRow) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		for _, rule := range rules {
			senders, err := jsonArrayToText(normalizedRuleSenders(rule))
			if err != nil {
				return err
			}
			now := timeToText(s.repositories.now())
			_, err = q.ExecContext(ctx, `INSERT INTO rules (
				id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				currency_regex, transaction_source, source_type, source_label, bank, predefined, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
			ON CONFLICT(name) WHERE tenant_id IS NULL AND predefined=1 DO NOTHING`,
				uuid.NewString(), rule.Name, primarySender(rule), senders, rule.SubjectContains, rule.AmountRegex,
				rule.MerchantRegex, rule.CurrencyRegex, ruleSourceLabel(rule), rule.SourceType,
				ruleSourceLabel(rule), rule.Bank, now, now)
			if err != nil {
				return mapSQLiteError("sqlite.rules.seed", err)
			}
		}
		return nil
	})
}

func (s *Store) ImportUserRules(ctx context.Context, tenant store.Tenant, rules []store.RuleRow) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		for _, rule := range rules {
			senders, err := jsonArrayToText(normalizedRuleSenders(rule))
			if err != nil {
				return err
			}
			now := timeToText(s.repositories.now())
			_, err = q.ExecContext(ctx, `INSERT INTO rules (
				id, tenant_id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
				currency_regex, transaction_source, source_type, source_label, bank, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(tenant_id, name) WHERE tenant_id IS NOT NULL AND predefined=0 DO UPDATE SET
				sender_email=excluded.sender_email, sender_emails=excluded.sender_emails,
				subject_contains=excluded.subject_contains, amount_regex=excluded.amount_regex,
				merchant_regex=excluded.merchant_regex, currency_regex=excluded.currency_regex,
				transaction_source=excluded.transaction_source, source_type=excluded.source_type,
				source_label=excluded.source_label, bank=excluded.bank, updated_at=excluded.updated_at`,
				uuid.NewString(), tenant.ID, rule.Name, primarySender(rule), senders, rule.SubjectContains,
				rule.AmountRegex, rule.MerchantRegex, rule.CurrencyRegex, ruleSourceLabel(rule), rule.SourceType,
				ruleSourceLabel(rule), rule.Bank, now, now)
			if err != nil {
				return mapSQLiteError("sqlite.rules.import", err)
			}
		}
		return nil
	})
}

type ruleScanner interface {
	Scan(...any) error
}

func scanRule(scanner ruleScanner) (store.RuleRow, error) {
	var result store.RuleRow
	var senders, createdAt, updatedAt string
	if err := scanner.Scan(&result.ID, &result.Name, &result.SenderEmail, &senders, &result.SubjectContains,
		&result.AmountRegex, &result.MerchantRegex, &result.CurrencyRegex, &result.TransactionSource,
		&result.SourceType, &result.SourceLabel, &result.Bank, &result.Predefined, &createdAt, &updatedAt); err != nil {
		return result, err
	}
	if err := jsonArrayFromText(senders, &result.SenderEmails); err != nil {
		return result, err
	}
	var err error
	result.CreatedAt, err = timeFromText(createdAt)
	if err != nil {
		return result, err
	}
	result.UpdatedAt, err = timeFromText(updatedAt)
	return result, err
}

func scanRuleRows(rows *sql.Rows) ([]store.RuleRow, error) {
	result := []store.RuleRow{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, errors.B.Op("sqlite.rules.scan").Text("scanning rule").Err(err).Build()
		}
		result = append(result, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.rules.scan", err)
	}
	return result, nil
}
