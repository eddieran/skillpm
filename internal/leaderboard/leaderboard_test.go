package leaderboard

import (
	"testing"
)

func TestGetReturnsDefaultEntries(t *testing.T) {
	entries := Get(Options{})
	if len(entries) == 0 {
		t.Fatal("expected non-empty leaderboard")
	}
	if len(entries) > 15 {
		t.Fatalf("expected default limit of 15, got %d", len(entries))
	}
	// verify rank assignment
	for i, e := range entries {
		if e.Rank != i+1 {
			t.Fatalf("entry %d: expected rank %d, got %d", i, i+1, e.Rank)
		}
	}
}

func TestGetSortedByDownloadsDesc(t *testing.T) {
	entries := Get(Options{Limit: 20})
	for i := 1; i < len(entries); i++ {
		if entries[i].Downloads > entries[i-1].Downloads {
			t.Fatalf("entries not sorted by downloads desc at index %d: %d > %d",
				i, entries[i].Downloads, entries[i-1].Downloads)
		}
	}
}

func TestGetCategoryFilter(t *testing.T) {
	for _, cat := range ValidCategories() {
		entries := Get(Options{Category: cat, Limit: 100})
		if len(entries) == 0 {
			t.Fatalf("expected entries for category %q", cat)
		}
		for _, e := range entries {
			if e.Category != cat {
				t.Fatalf("expected category %q, got %q for %s", cat, e.Category, e.Slug)
			}
		}
	}
}

func TestGetCategoryFilterCaseInsensitive(t *testing.T) {
	lower := Get(Options{Category: "agent", Limit: 100})
	upper := Get(Options{Category: "AGENT", Limit: 100})
	if len(lower) != len(upper) {
		t.Fatalf("case-insensitive filter broken: lower=%d upper=%d", len(lower), len(upper))
	}
}

func TestGetEmptyCategoryReturnsAll(t *testing.T) {
	all := Get(Options{Category: "", Limit: 100})
	if len(all) < 15 {
		t.Fatalf("expected at least 15 entries for all categories, got %d", len(all))
	}
}

func TestGetInvalidCategoryReturnsEmpty(t *testing.T) {
	entries := Get(Options{Category: "nonexistent", Limit: 100})
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for invalid category, got %d", len(entries))
	}
}

func TestGetLimit(t *testing.T) {
	entries := Get(Options{Limit: 5})
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
}

func TestGetLimitExceedsTotalReturnsAll(t *testing.T) {
	entries := Get(Options{Limit: 1000})
	total := len(seedData())
	if len(entries) != total {
		t.Fatalf("expected %d entries (all), got %d", total, len(entries))
	}
}

func TestGetLimitZeroUsesDefault(t *testing.T) {
	entries := Get(Options{Limit: 0})
	if len(entries) != 15 {
		t.Fatalf("expected default 15, got %d", len(entries))
	}
}

func TestGetLimitNegativeUsesDefault(t *testing.T) {
	entries := Get(Options{Limit: -1})
	if len(entries) != 15 {
		t.Fatalf("expected default 15, got %d", len(entries))
	}
}

func TestValidCategories(t *testing.T) {
	cats := ValidCategories()
	if len(cats) != 5 {
		t.Fatalf("expected 5 categories, got %d", len(cats))
	}
	expected := map[string]bool{"agent": true, "tool": true, "workflow": true, "data": true, "security": true}
	for _, c := range cats {
		if !expected[c] {
			t.Fatalf("unexpected category %q", c)
		}
	}
}

func TestIsValidCategory(t *testing.T) {
	if !IsValidCategory("") {
		t.Fatal("empty should be valid")
	}
	if !IsValidCategory("agent") {
		t.Fatal("agent should be valid")
	}
	if !IsValidCategory("TOOL") {
		t.Fatal("TOOL (uppercase) should be valid")
	}
	if IsValidCategory("bogus") {
		t.Fatal("bogus should not be valid")
	}
}

func TestEntryFieldsPopulated(t *testing.T) {
	entries := Get(Options{Limit: 20})
	for _, e := range entries {
		if e.Slug == "" {
			t.Fatal("slug must not be empty")
		}
		if e.Name == "" {
			t.Fatalf("name must not be empty for %s", e.Slug)
		}
		if e.Category == "" {
			t.Fatalf("category must not be empty for %s", e.Slug)
		}
		if e.Downloads <= 0 {
			t.Fatalf("downloads must be positive for %s", e.Slug)
		}
		if e.Rating < 1.0 || e.Rating > 5.0 {
			t.Fatalf("rating out of range for %s: %.1f", e.Slug, e.Rating)
		}
		if e.Source == "" {
			t.Fatalf("source must not be empty for %s", e.Slug)
		}
		if e.Description == "" {
			t.Fatalf("description must not be empty for %s", e.Slug)
		}
	}
}

func TestGetCategoryAndLimitCombined(t *testing.T) {
	entries := Get(Options{Category: "security", Limit: 2})
	if len(entries) != 2 {
		t.Fatalf("expected 2 security entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Category != "security" {
			t.Fatalf("expected security category, got %q", e.Category)
		}
	}
	if entries[0].Rank != 1 || entries[1].Rank != 2 {
		t.Fatal("ranks should restart at 1 after filtering")
	}
}

func TestSlugsUnique(t *testing.T) {
	entries := Get(Options{Limit: 100})
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Slug] {
			t.Fatalf("duplicate slug: %s", e.Slug)
		}
		seen[e.Slug] = true
	}
}
