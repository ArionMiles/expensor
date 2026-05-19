package rules_test

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/rules"
)

func r(name string) api.Rule { return api.Rule{Name: name} }

func TestMergeRules_UserOverridesSystem(t *testing.T) {
	system := []api.Rule{r("A"), r("B")}
	user := []api.Rule{{Name: "B", SenderEmail: "override@example.com"}} // user edits B
	got := rules.MergeRules(system, user)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[1].Name != "B" || got[1].SenderEmail != "override@example.com" {
		t.Errorf("user override of B not applied, got %+v", got[1])
	}
}

func TestMergeRules_UserOnlyAppended(t *testing.T) {
	system := []api.Rule{r("A")}
	user := []api.Rule{r("C")} // C not in system
	got := rules.MergeRules(system, user)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[1].Name != "C" {
		t.Errorf("expected user-only rule C appended last, got %q", got[1].Name)
	}
}

func TestMergeRules_EmptyUser(t *testing.T) {
	system := []api.Rule{r("A"), r("B")}
	got := rules.MergeRules(system, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
}

func TestMergeRules_EmptySystem(t *testing.T) {
	user := []api.Rule{r("X")}
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
