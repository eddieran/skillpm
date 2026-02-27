package leaderboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Entry represents a single skill on the leaderboard.
type Entry struct {
	Rank        int      `json:"rank"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Downloads   int      `json:"downloads"`
	Rating      float64  `json:"rating"`
	Source      string   `json:"source"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// Options controls leaderboard filtering and pagination.
type Options struct {
	Category string
	Limit    int
	Live     bool   // attempt live fetch from API
	APIBase  string // base URL for trending endpoint
}

// ValidCategories returns the set of known skill categories.
func ValidCategories() []string {
	return []string{"agent", "tool", "workflow", "data", "security"}
}

// IsValidCategory returns true if cat is a recognized category or empty.
func IsValidCategory(cat string) bool {
	if cat == "" {
		return true
	}
	for _, c := range ValidCategories() {
		if strings.EqualFold(c, cat) {
			return true
		}
	}
	return false
}

// FetchLive queries a remote trending endpoint and returns leaderboard entries.
// On any failure (network, non-200, bad JSON) it returns an error.
func FetchLive(ctx context.Context, client *http.Client, apiBase string, opts Options) ([]Entry, error) {
	if apiBase == "" {
		return nil, fmt.Errorf("LB_LIVE: apiBase is required")
	}
	if client == nil {
		client = http.DefaultClient
	}

	u := strings.TrimRight(apiBase, "/") + "/api/v1/trending"
	q := url.Values{}
	if opts.Category != "" {
		q.Set("category", opts.Category)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("LB_LIVE: %w", err)
	}
	req.Header.Set("User-Agent", "skillpm/1.0 (+https://github.com/eddieran/skillpm)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LB_LIVE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LB_LIVE: endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("LB_LIVE: %w", err)
	}

	var entries []Entry
	if err := json.Unmarshal(body, &entries); err != nil {
		var wrapper map[string]json.RawMessage
		if wErr := json.Unmarshal(body, &wrapper); wErr == nil {
			for _, key := range []string{"items", "skills", "data", "results"} {
				if raw, ok := wrapper[key]; ok {
					if json.Unmarshal(raw, &entries) == nil && len(entries) > 0 {
						break
					}
				}
			}
		}
		if len(entries) == 0 {
			return nil, fmt.Errorf("LB_LIVE: failed to parse response: %w", err)
		}
	}

	return entries, nil
}

// Get returns leaderboard entries filtered by opts.
// When opts.Live is true, it attempts a live fetch and falls back to seed data on error.
// Entries are sorted by downloads descending and ranked starting at 1.
func Get(ctx context.Context, client *http.Client, opts Options) []Entry {
	var entries []Entry

	if opts.Live && opts.APIBase != "" {
		if live, err := FetchLive(ctx, client, opts.APIBase, opts); err == nil && len(live) > 0 {
			entries = live
		}
	}

	if len(entries) == 0 {
		entries = seedData()
	}

	// filter by category
	if opts.Category != "" {
		filtered := make([]Entry, 0)
		cat := strings.ToLower(opts.Category)
		for _, e := range entries {
			if strings.ToLower(e.Category) == cat {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// sort by downloads desc, then rating desc
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Downloads != entries[j].Downloads {
			return entries[i].Downloads > entries[j].Downloads
		}
		return entries[i].Rating > entries[j].Rating
	})

	// apply limit
	limit := opts.Limit
	if limit <= 0 {
		limit = 15
	}
	if limit > len(entries) {
		limit = len(entries)
	}
	entries = entries[:limit]

	// assign rank
	for i := range entries {
		entries[i].Rank = i + 1
	}

	return entries
}

func seedData() []Entry {
	return []Entry{
		{Slug: "steipete/code-review", Name: "Code Review Agent", Category: "tool", Downloads: 12480, Rating: 4.9, Source: "clawhub", Description: "AI-powered code review with inline suggestions and auto-fix", Tags: []string{"review", "quality"}},
		{Slug: "testingshop/auto-test-gen", Name: "Auto Test Generator", Category: "agent", Downloads: 11230, Rating: 4.8, Source: "clawhub", Description: "Generates unit and integration tests from source code", Tags: []string{"testing", "automation"}},
		{Slug: "secops/secret-scanner", Name: "Secret Scanner", Category: "security", Downloads: 9870, Rating: 4.8, Source: "community", Description: "Detects leaked credentials, API keys, and tokens in repos", Tags: []string{"secrets", "compliance"}},
		{Slug: "docsify/doc-writer", Name: "Documentation Writer", Category: "tool", Downloads: 9540, Rating: 4.7, Source: "clawhub", Description: "Generates and maintains API docs, READMEs, and changelogs", Tags: []string{"docs", "markdown"}},
		{Slug: "semverbot/dep-updater", Name: "Dependency Updater", Category: "workflow", Downloads: 8920, Rating: 4.7, Source: "clawhub", Description: "Automated dependency upgrades with compatibility checks", Tags: []string{"deps", "semver"}},
		{Slug: "perfops/perf-profiler", Name: "Performance Profiler", Category: "tool", Downloads: 8310, Rating: 4.6, Source: "community", Description: "Runtime profiling with bottleneck detection and fix suggestions", Tags: []string{"perf", "optimization"}},
		{Slug: "datamaster/schema-migrator", Name: "Schema Migrator", Category: "data", Downloads: 7890, Rating: 4.6, Source: "clawhub", Description: "Database schema diff, migration generation, and rollback plans", Tags: []string{"sql", "migration"}},
		{Slug: "ci-ninja/ci-optimizer", Name: "CI Optimizer", Category: "workflow", Downloads: 7650, Rating: 4.5, Source: "clawhub", Description: "Analyzes CI pipelines and suggests parallelization strategies", Tags: []string{"ci", "pipeline"}},
		{Slug: "secops/api-fuzzer", Name: "API Fuzzer", Category: "security", Downloads: 7420, Rating: 4.5, Source: "community", Description: "Automated API endpoint fuzzing with vulnerability reporting", Tags: []string{"fuzz", "api"}},
		{Slug: "cleancode/refactor-agent", Name: "Refactor Agent", Category: "agent", Downloads: 7100, Rating: 4.5, Source: "clawhub", Description: "Identifies code smells and applies safe automated refactorings", Tags: []string{"refactor", "clean-code"}},
		{Slug: "observa/log-analyzer", Name: "Log Analyzer", Category: "data", Downloads: 6780, Rating: 4.4, Source: "community", Description: "Structured log parsing with anomaly detection and alerting", Tags: []string{"logs", "observability"}},
		{Slug: "releasebot/release-drafter", Name: "Release Drafter", Category: "workflow", Downloads: 6540, Rating: 4.4, Source: "clawhub", Description: "Auto-generates release notes from commit history and PRs", Tags: []string{"release", "changelog"}},
		{Slug: "secops/vulnerability-patch", Name: "Vulnerability Patcher", Category: "security", Downloads: 6310, Rating: 4.4, Source: "clawhub", Description: "Scans CVE databases and generates targeted security patches", Tags: []string{"cve", "patch"}},
		{Slug: "global/i18n-translator", Name: "i18n Translator", Category: "tool", Downloads: 5980, Rating: 4.3, Source: "community", Description: "Extracts strings and generates locale files with AI translation", Tags: []string{"i18n", "l10n"}},
		{Slug: "hr/onboarding-agent", Name: "Onboarding Agent", Category: "agent", Downloads: 5720, Rating: 4.3, Source: "clawhub", Description: "Generates project walkthroughs and contributor guides for new devs", Tags: []string{"onboarding", "docs"}},
		{Slug: "privacy/data-anonymizer", Name: "Data Anonymizer", Category: "data", Downloads: 5480, Rating: 4.2, Source: "community", Description: "PII detection and anonymization for datasets and databases", Tags: []string{"pii", "privacy"}},
		{Slug: "apiops/contract-verifier", Name: "Contract Verifier", Category: "agent", Downloads: 5210, Rating: 4.2, Source: "clawhub", Description: "Validates API contracts and detects breaking changes", Tags: []string{"api", "contract"}},
		{Slug: "finops/cost-estimator", Name: "Cloud Cost Estimator", Category: "workflow", Downloads: 4950, Rating: 4.1, Source: "community", Description: "Estimates infrastructure costs from IaC definitions", Tags: []string{"cloud", "finops"}},
		{Slug: "wcag/accessibility-audit", Name: "Accessibility Auditor", Category: "tool", Downloads: 4730, Rating: 4.1, Source: "community", Description: "WCAG compliance checking with remediation suggestions", Tags: []string{"a11y", "wcag"}},
		{Slug: "secops/threat-modeler", Name: "Threat Modeler", Category: "security", Downloads: 4510, Rating: 4.0, Source: "clawhub", Description: "Automated threat modeling from architecture diagrams", Tags: []string{"threat", "stride"}},
	}
}
