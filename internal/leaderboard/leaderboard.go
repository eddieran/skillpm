package leaderboard

import (
	"sort"
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

// Get returns curated leaderboard entries filtered by opts.
// Entries are sorted by downloads descending and ranked starting at 1.
func Get(opts Options) []Entry {
	entries := seedData()

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
		{Slug: "code-review", Name: "Code Review Agent", Category: "tool", Downloads: 12480, Rating: 4.9, Source: "clawhub", Description: "AI-powered code review with inline suggestions and auto-fix", Tags: []string{"review", "quality"}},
		{Slug: "auto-test-gen", Name: "Auto Test Generator", Category: "agent", Downloads: 11230, Rating: 4.8, Source: "clawhub", Description: "Generates unit and integration tests from source code", Tags: []string{"testing", "automation"}},
		{Slug: "secret-scanner", Name: "Secret Scanner", Category: "security", Downloads: 9870, Rating: 4.8, Source: "community", Description: "Detects leaked credentials, API keys, and tokens in repos", Tags: []string{"secrets", "compliance"}},
		{Slug: "doc-writer", Name: "Documentation Writer", Category: "tool", Downloads: 9540, Rating: 4.7, Source: "clawhub", Description: "Generates and maintains API docs, READMEs, and changelogs", Tags: []string{"docs", "markdown"}},
		{Slug: "dep-updater", Name: "Dependency Updater", Category: "workflow", Downloads: 8920, Rating: 4.7, Source: "clawhub", Description: "Automated dependency upgrades with compatibility checks", Tags: []string{"deps", "semver"}},
		{Slug: "perf-profiler", Name: "Performance Profiler", Category: "tool", Downloads: 8310, Rating: 4.6, Source: "community", Description: "Runtime profiling with bottleneck detection and fix suggestions", Tags: []string{"perf", "optimization"}},
		{Slug: "schema-migrator", Name: "Schema Migrator", Category: "data", Downloads: 7890, Rating: 4.6, Source: "clawhub", Description: "Database schema diff, migration generation, and rollback plans", Tags: []string{"sql", "migration"}},
		{Slug: "ci-optimizer", Name: "CI Optimizer", Category: "workflow", Downloads: 7650, Rating: 4.5, Source: "clawhub", Description: "Analyzes CI pipelines and suggests parallelization strategies", Tags: []string{"ci", "pipeline"}},
		{Slug: "api-fuzzer", Name: "API Fuzzer", Category: "security", Downloads: 7420, Rating: 4.5, Source: "community", Description: "Automated API endpoint fuzzing with vulnerability reporting", Tags: []string{"fuzz", "api"}},
		{Slug: "refactor-agent", Name: "Refactor Agent", Category: "agent", Downloads: 7100, Rating: 4.5, Source: "clawhub", Description: "Identifies code smells and applies safe automated refactorings", Tags: []string{"refactor", "clean-code"}},
		{Slug: "log-analyzer", Name: "Log Analyzer", Category: "data", Downloads: 6780, Rating: 4.4, Source: "community", Description: "Structured log parsing with anomaly detection and alerting", Tags: []string{"logs", "observability"}},
		{Slug: "release-drafter", Name: "Release Drafter", Category: "workflow", Downloads: 6540, Rating: 4.4, Source: "clawhub", Description: "Auto-generates release notes from commit history and PRs", Tags: []string{"release", "changelog"}},
		{Slug: "vulnerability-patch", Name: "Vulnerability Patcher", Category: "security", Downloads: 6310, Rating: 4.4, Source: "clawhub", Description: "Scans CVE databases and generates targeted security patches", Tags: []string{"cve", "patch"}},
		{Slug: "i18n-translator", Name: "i18n Translator", Category: "tool", Downloads: 5980, Rating: 4.3, Source: "community", Description: "Extracts strings and generates locale files with AI translation", Tags: []string{"i18n", "l10n"}},
		{Slug: "onboarding-agent", Name: "Onboarding Agent", Category: "agent", Downloads: 5720, Rating: 4.3, Source: "clawhub", Description: "Generates project walkthroughs and contributor guides for new devs", Tags: []string{"onboarding", "docs"}},
		{Slug: "data-anonymizer", Name: "Data Anonymizer", Category: "data", Downloads: 5480, Rating: 4.2, Source: "community", Description: "PII detection and anonymization for datasets and databases", Tags: []string{"pii", "privacy"}},
		{Slug: "contract-verifier", Name: "Contract Verifier", Category: "agent", Downloads: 5210, Rating: 4.2, Source: "clawhub", Description: "Validates API contracts and detects breaking changes", Tags: []string{"api", "contract"}},
		{Slug: "cost-estimator", Name: "Cloud Cost Estimator", Category: "workflow", Downloads: 4950, Rating: 4.1, Source: "community", Description: "Estimates infrastructure costs from IaC definitions", Tags: []string{"cloud", "finops"}},
		{Slug: "accessibility-audit", Name: "Accessibility Auditor", Category: "tool", Downloads: 4730, Rating: 4.1, Source: "community", Description: "WCAG compliance checking with remediation suggestions", Tags: []string{"a11y", "wcag"}},
		{Slug: "threat-modeler", Name: "Threat Modeler", Category: "security", Downloads: 4510, Rating: 4.0, Source: "clawhub", Description: "Automated threat modeling from architecture diagrams", Tags: []string{"threat", "stride"}},
	}
}
