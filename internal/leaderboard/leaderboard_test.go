package leaderboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetReturnsDefaultEntries(t *testing.T) {
	entries := Get(context.Background(), nil, Options{})
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
	entries := Get(context.Background(), nil, Options{Limit: 20})
	for i := 1; i < len(entries); i++ {
		if entries[i].Downloads > entries[i-1].Downloads {
			t.Fatalf("entries not sorted by downloads desc at index %d: %d > %d",
				i, entries[i].Downloads, entries[i-1].Downloads)
		}
	}
}

func TestGetCategoryFilter(t *testing.T) {
	for _, cat := range ValidCategories() {
		entries := Get(context.Background(), nil, Options{Category: cat, Limit: 100})
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
	lower := Get(context.Background(), nil, Options{Category: "agent", Limit: 100})
	upper := Get(context.Background(), nil, Options{Category: "AGENT", Limit: 100})
	if len(lower) != len(upper) {
		t.Fatalf("case-insensitive filter broken: lower=%d upper=%d", len(lower), len(upper))
	}
}

func TestGetEmptyCategoryReturnsAll(t *testing.T) {
	all := Get(context.Background(), nil, Options{Category: "", Limit: 100})
	if len(all) < 15 {
		t.Fatalf("expected at least 15 entries for all categories, got %d", len(all))
	}
}

func TestGetInvalidCategoryReturnsEmpty(t *testing.T) {
	entries := Get(context.Background(), nil, Options{Category: "nonexistent", Limit: 100})
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for invalid category, got %d", len(entries))
	}
}

func TestGetLimit(t *testing.T) {
	entries := Get(context.Background(), nil, Options{Limit: 5})
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
}

func TestGetLimitExceedsTotalReturnsAll(t *testing.T) {
	entries := Get(context.Background(), nil, Options{Limit: 1000})
	total := len(seedData())
	if len(entries) != total {
		t.Fatalf("expected %d entries (all), got %d", total, len(entries))
	}
}

func TestGetLimitZeroUsesDefault(t *testing.T) {
	entries := Get(context.Background(), nil, Options{Limit: 0})
	if len(entries) != 15 {
		t.Fatalf("expected default 15, got %d", len(entries))
	}
}

func TestGetLimitNegativeUsesDefault(t *testing.T) {
	entries := Get(context.Background(), nil, Options{Limit: -1})
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
	entries := Get(context.Background(), nil, Options{Limit: 20})
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
	entries := Get(context.Background(), nil, Options{Category: "security", Limit: 2})
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
	entries := Get(context.Background(), nil, Options{Limit: 100})
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Slug] {
			t.Fatalf("duplicate slug: %s", e.Slug)
		}
		seen[e.Slug] = true
	}
}

// --- Live fetch tests ---

func TestFetchLiveSuccess(t *testing.T) {
	want := []Entry{
		{Slug: "live/skill-a", Name: "Live A", Category: "tool", Downloads: 999, Rating: 4.9, Source: "live"},
		{Slug: "live/skill-b", Name: "Live B", Category: "agent", Downloads: 888, Rating: 4.5, Source: "live"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/trending" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	got, err := FetchLive(context.Background(), srv.Client(), srv.URL, Options{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Slug != "live/skill-a" {
		t.Fatalf("expected live/skill-a, got %q", got[0].Slug)
	}
}

func TestFetchLive404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := FetchLive(context.Background(), srv.Client(), srv.URL, Options{})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchLiveInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := FetchLive(context.Background(), srv.Client(), srv.URL, Options{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchLiveWrappedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []Entry{
				{Slug: "wrapped/skill", Name: "Wrapped", Category: "tool", Downloads: 100, Rating: 4.0, Source: "live"},
			},
		})
	}))
	defer srv.Close()

	got, err := FetchLive(context.Background(), srv.Client(), srv.URL, Options{})
	if err != nil {
		t.Fatalf("expected no error for wrapped response, got %v", err)
	}
	if len(got) != 1 || got[0].Slug != "wrapped/skill" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestFetchLiveEmptyAPIBase(t *testing.T) {
	_, err := FetchLive(context.Background(), nil, "", Options{})
	if err == nil {
		t.Fatal("expected error for empty apiBase")
	}
}

func TestGetLiveFallbackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	entries := Get(context.Background(), srv.Client(), Options{
		Live: true, APIBase: srv.URL, Limit: 5,
	})
	if len(entries) != 5 {
		t.Fatalf("expected fallback to seed data with 5 entries, got %d", len(entries))
	}
	if entries[0].Downloads != 12480 {
		t.Fatalf("expected seed data first entry downloads=12480, got %d", entries[0].Downloads)
	}
}

func TestGetLiveSuccess(t *testing.T) {
	liveEntries := []Entry{
		{Slug: "top/skill", Name: "Top", Category: "tool", Downloads: 50000, Rating: 5.0, Source: "live"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(liveEntries)
	}))
	defer srv.Close()

	entries := Get(context.Background(), srv.Client(), Options{
		Live: true, APIBase: srv.URL,
	})
	if len(entries) == 0 {
		t.Fatal("expected entries from live fetch")
	}
	if entries[0].Slug != "top/skill" {
		t.Fatalf("expected live data, got seed data slug=%q", entries[0].Slug)
	}
}

func TestGetLiveDisabledUsesSeed(t *testing.T) {
	entries := Get(context.Background(), nil, Options{
		Live: false, APIBase: "http://should-not-be-called", Limit: 3,
	})
	if len(entries) != 3 {
		t.Fatalf("expected 3 seed entries, got %d", len(entries))
	}
}
