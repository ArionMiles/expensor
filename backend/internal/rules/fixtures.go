package rules

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

var fixtureNamePattern = regexp.MustCompile(`^[a-z0-9-]+_[a-z0-9-]+_[a-z0-9-]+$`)

const ruleFixtureExtension = ".rule.fixture"

// EmailFixture is one positive email example for one rule. The rule regexes are
// intentionally not part of the fixture; tests load those from rules.json.
type EmailFixture struct {
	TestName string                  `yaml:"-"`
	Rule     string                  `yaml:"rule"`
	Sender   string                  `yaml:"sender"`
	Subject  string                  `yaml:"subject"`
	Body     string                  `yaml:"-"`
	Expected EmailFixtureExpectation `yaml:"expected"`
}

// EmailFixtureExpectation is the extraction result expected from a fixture.
type EmailFixtureExpectation struct {
	Amount   float64 `yaml:"amount"`
	Merchant string  `yaml:"merchant"`
	Currency string  `yaml:"currency"`
}

// LoadEmailFixtures discovers rule-email fixtures from dir and returns them sorted by filename.
func LoadEmailFixtures(dir string) ([]EmailFixture, error) {
	var fixtures []EmailFixture
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !isRuleFixture(path) {
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
		return nil, errors.E("rules.fixtures.load_email_fixtures", "walking email fixtures", err)
	}
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].TestName < fixtures[j].TestName
	})
	return fixtures, nil
}

func loadEmailFixture(path string) (EmailFixture, error) {
	base := filepath.Base(path)
	testName := strings.TrimSuffix(base, ruleFixtureExtension)
	if !fixtureNamePattern.MatchString(testName) {
		return EmailFixture{}, errors.E(
			errors.InvalidInput,
			fmt.Sprintf("fixture %q must match <bank>_<source-type>_<case>.rule.fixture", base),
		)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return EmailFixture{}, errors.E("rules.fixtures.load_email_fixture", fmt.Sprintf("reading fixture %q", path), err)
	}
	frontMatter, body, err := splitEmailFixture(data)
	if err != nil {
		return EmailFixture{}, errors.E("rules.fixtures.load_email_fixture", fmt.Sprintf("parsing fixture %q", path), err)
	}
	var fixture EmailFixture
	if err := yaml.Unmarshal(frontMatter, &fixture); err != nil {
		return EmailFixture{}, errors.E("rules.fixtures.load_email_fixture", fmt.Sprintf("parsing fixture %q", path), err)
	}
	fixture.Body = string(body)
	fixture.TestName = testName
	if err := validateEmailFixture(fixture); err != nil {
		return EmailFixture{}, errors.E("rules.fixtures.load_email_fixture", fmt.Sprintf("validating fixture %q", path), err)
	}
	return fixture, nil
}

func splitEmailFixture(data []byte) ([]byte, []byte, error) {
	if len(data) == 0 {
		return nil, nil, errors.E(errors.InvalidInput, "fixture is empty")
	}

	firstLineEnd := bytes.IndexByte(data, '\n')
	if firstLineEnd == -1 {
		return nil, nil, errors.E(errors.InvalidInput, "fixture must start with a front matter delimiter")
	}
	if !isDelimiterLine(data[:firstLineEnd]) {
		return nil, nil, errors.E(errors.InvalidInput, "fixture must start with a front matter delimiter")
	}

	frontMatterStart := firstLineEnd + 1
	for offset := frontMatterStart; offset <= len(data); {
		lineEnd := bytes.IndexByte(data[offset:], '\n')
		if lineEnd == -1 {
			break
		}
		lineEnd += offset
		if isDelimiterLine(data[offset:lineEnd]) {
			return data[frontMatterStart:offset], data[lineEnd+1:], nil
		}
		offset = lineEnd + 1
	}

	return nil, nil, errors.E(errors.InvalidInput, "fixture must include a closing front matter delimiter")
}

func isDelimiterLine(line []byte) bool {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	return bytes.Equal(line, []byte("---"))
}

func validateEmailFixture(fixture EmailFixture) error {
	if strings.TrimSpace(fixture.Rule) == "" {
		return errors.E(errors.InvalidInput, "rule is required")
	}
	if strings.TrimSpace(fixture.Sender) == "" {
		return errors.E(errors.InvalidInput, "sender is required")
	}
	if strings.TrimSpace(fixture.Subject) == "" {
		return errors.E(errors.InvalidInput, "subject is required")
	}
	if strings.TrimSpace(fixture.Body) == "" {
		return errors.E(errors.InvalidInput, "body is required")
	}
	if strings.TrimSpace(fixture.Expected.Merchant) == "" {
		return errors.E(errors.InvalidInput, "expected.merchant is required")
	}
	if strings.TrimSpace(fixture.Expected.Currency) == "" {
		return errors.E(errors.InvalidInput, "expected.currency is required")
	}
	return nil
}

func isRuleFixture(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ruleFixtureExtension)
}
