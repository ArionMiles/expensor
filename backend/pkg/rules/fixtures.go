package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var fixtureNamePattern = regexp.MustCompile(`^[a-z0-9-]+_[a-z0-9-]+_[a-z0-9-]+$`)

// EmailFixture is one positive email example for one rule. The rule regexes are
// intentionally not part of the fixture; tests load those from rules.json.
type EmailFixture struct {
	TestName string
	Rule     string                  `yaml:"rule"`
	Sender   string                  `yaml:"sender"`
	Subject  string                  `yaml:"subject"`
	Body     string                  `yaml:"body"`
	Expected EmailFixtureExpectation `yaml:"expected"`
}

// EmailFixtureExpectation is the extraction result expected from a fixture.
type EmailFixtureExpectation struct {
	Amount   float64 `yaml:"amount"`
	Merchant string  `yaml:"merchant"`
	Currency string  `yaml:"currency"`
}

// LoadEmailFixtures discovers YAML fixtures from dir and returns them sorted by filename.
func LoadEmailFixtures(dir string) ([]EmailFixture, error) {
	var fixtures []EmailFixture
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !isYAML(path) {
			return nil
		}
		fixture, err := loadEmailFixture(path)
		if err != nil {
			return err
		}
		fixtures = append(fixtures, fixture)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking email fixtures: %w", err)
	}
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].TestName < fixtures[j].TestName
	})
	return fixtures, nil
}

func loadEmailFixture(path string) (EmailFixture, error) {
	base := filepath.Base(path)
	testName := strings.TrimSuffix(base, filepath.Ext(base))
	if !fixtureNamePattern.MatchString(testName) {
		return EmailFixture{}, fmt.Errorf("fixture %q must match <bank>_<source-type>_<case>.yaml", base)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return EmailFixture{}, fmt.Errorf("reading fixture %q: %w", path, err)
	}
	var fixture EmailFixture
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return EmailFixture{}, fmt.Errorf("parsing fixture %q: %w", path, err)
	}
	fixture.TestName = testName
	if err := validateEmailFixture(fixture); err != nil {
		return EmailFixture{}, fmt.Errorf("validating fixture %q: %w", path, err)
	}
	return fixture, nil
}

func validateEmailFixture(fixture EmailFixture) error {
	if strings.TrimSpace(fixture.Rule) == "" {
		return fmt.Errorf("rule is required")
	}
	if strings.TrimSpace(fixture.Sender) == "" {
		return fmt.Errorf("sender is required")
	}
	if strings.TrimSpace(fixture.Subject) == "" {
		return fmt.Errorf("subject is required")
	}
	if strings.TrimSpace(fixture.Body) == "" {
		return fmt.Errorf("body is required")
	}
	if strings.TrimSpace(fixture.Expected.Merchant) == "" {
		return fmt.Errorf("expected.merchant is required")
	}
	if strings.TrimSpace(fixture.Expected.Currency) == "" {
		return fmt.Errorf("expected.currency is required")
	}
	return nil
}

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
