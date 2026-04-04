package rules_test

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/rules"
)

func r(name string, enabled bool) api.Rule {
	return api.Rule{Name: name, Enabled: enabled}
}

func TestMergeRules_UserOverridesSystem(t *testing.T) {
	system := []api.Rule{r("A", true), r("B", true)}
	user := []api.Rule{r("B", false)} // user disables B
	got := rules.MergeRules(system, user)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[1].Name != "B" || got[1].Enabled {
		t.Errorf("user override of B should be disabled, got Enabled=%v", got[1].Enabled)
	}
}

func TestMergeRules_UserOnlyAppended(t *testing.T) {
	system := []api.Rule{r("A", true)}
	user := []api.Rule{r("C", true)} // C not in system
	got := rules.MergeRules(system, user)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[1].Name != "C" {
		t.Errorf("expected user-only rule C appended last, got %q", got[1].Name)
	}
}

func TestMergeRules_EmptyUser(t *testing.T) {
	system := []api.Rule{r("A", true), r("B", true)}
	got := rules.MergeRules(system, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
}

func TestMergeRules_EmptySystem(t *testing.T) {
	user := []api.Rule{r("X", true)}
	got := rules.MergeRules(nil, user)
	if len(got) != 1 || got[0].Name != "X" {
		t.Error("expected user-only rule X returned")
	}
}

func TestMergeRules_BothEmpty(t *testing.T) {
	got := rules.MergeRules(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d rules", len(got))
	}
}

func TestFilterEnabled(t *testing.T) {
	rs := []api.Rule{r("A", true), r("B", false), r("C", true)}
	got := rules.FilterEnabled(rs)
	if len(got) != 2 {
		t.Fatalf("expected 2 enabled rules, got %d", len(got))
	}
	for _, rule := range got {
		if !rule.Enabled {
			t.Errorf("disabled rule %q in FilterEnabled output", rule.Name)
		}
	}
}

func TestFilterEnabled_AllDisabled(t *testing.T) {
	rs := []api.Rule{r("A", false), r("B", false)}
	got := rules.FilterEnabled(rs)
	if len(got) != 0 {
		t.Errorf("expected 0 rules, got %d", len(got))
	}
}
