package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"skillpm/internal/app"

	"github.com/spf13/cobra"
)

func TestDocsConsistency_SelfShortcutAliases(t *testing.T) {
	// 1. Define the commands to check and their factory functions
	type cmdFactory func(func() (*app.Service, error), *bool) *cobra.Command

	dummySvc := func() (*app.Service, error) { return nil, nil }
	dummyJSON := false

	checks := []struct {
		name    string
		factory cmdFactory
		docKey  string // Key to find in README, e.g. "self-stable shortcut aliases"
	}{
		{
			name:    "self-stable",
			factory: newSelfStableShortcutCmd,
			docKey:  "self-stable shortcut aliases",
		},
		{
			name:    "self-edge",
			factory: newSelfEdgeShortcutCmd,
			docKey:  "self-edge shortcut aliases",
		},
		{
			name:    "self-beta",
			factory: newSelfBetaShortcutCmd,
			docKey:  "self-beta shortcut aliases",
		},
	}

	// 2. Read README.md
	readmePath := filepath.Join("..", "..", "README.md")
	file, err := os.Open(readmePath)
	if err != nil {
		t.Fatalf("failed to open README.md at %s: %v", readmePath, err)
	}
	defer file.Close()

	// 3. Parse README aliases
	// Format in README:
	// `self-stable shortcut aliases:`
	// - `self-stable` → `selfstable`, `stable-selfpm`, ...
	docAliases := make(map[string][]string)
	scanner := bufio.NewScanner(file)
	currentSection := ""
	
	// Regex to match section headers: "self-stable shortcut aliases:"
	sectionRegex := regexp.MustCompile("^`([^`]+)` shortcut aliases:$")
	// Regex to match alias line: "- `self-stable` → `alias1`, `alias2`"
	aliasLineRegex := regexp.MustCompile(`^-\s+` + "`[^`]+`" + `\s+→\s+(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 1 {
			currentSection = matches[1] // e.g., "self-stable"
			continue
		}

		if currentSection != "" {
			if matches := aliasLineRegex.FindStringSubmatch(line); len(matches) > 1 {
				// matches[1] is "`alias1`, `alias2`, `alias3`"
				rawList := matches[1]
				parts := strings.Split(rawList, ",")
				var extracted []string
				for _, p := range parts {
					p = strings.TrimSpace(p)
					p = strings.Trim(p, "`") // Remove backticks
					if p != "" {
						extracted = append(extracted, p)
					}
				}
				docAliases[currentSection] = extracted
				currentSection = "" // Reset after finding the line
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading README.md: %v", err)
	}

	// 4. Verify Consistency
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			// Get actual aliases from code
			cmd := check.factory(dummySvc, &dummyJSON)
			codeAliases := cmd.Aliases
			sort.Strings(codeAliases)

			// Get documented aliases
			docList, exists := docAliases[check.name]
			if !exists {
				t.Errorf("Documentation missing for '%s shortcut aliases' in README.md", check.name)
				return
			}
			sort.Strings(docList)

			// Compare
			if !equalSlices(codeAliases, docList) {
				t.Errorf("Alias mismatch for %s:\nCode: %v\nDocs: %v\n", check.name, codeAliases, docList)
				
				// Help debug: show diff
				missingInDocs := difference(codeAliases, docList)
				missingInCode := difference(docList, codeAliases)
				if len(missingInDocs) > 0 {
					t.Logf("Missing in Docs: %v", missingInDocs)
				}
				if len(missingInCode) > 0 {
					t.Logf("Missing in Code: %v", missingInCode)
				}
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
