// Package rules provides Expensor rule document, fixture, and merge utilities.
package rules

import "github.com/ArionMiles/expensor/backend/pkg/api"

// MergeRules combines system and user rules. A user rule with the same Name as
// a system rule completely replaces it. User-only rules are appended at the end.
// Order within each group is preserved.
func MergeRules(system, user []api.Rule) []api.Rule {
	userByName := make(map[string]api.Rule, len(user))
	for _, rule := range user {
		userByName[rule.Name] = rule
	}

	out := make([]api.Rule, 0, len(system)+len(user))
	seen := make(map[string]struct{}, len(system))

	for _, rule := range system {
		if override, ok := userByName[rule.Name]; ok {
			out = append(out, override)
		} else {
			out = append(out, rule)
		}
		seen[rule.Name] = struct{}{}
	}

	for _, rule := range user {
		if _, already := seen[rule.Name]; !already {
			out = append(out, rule)
		}
	}
	return out
}
