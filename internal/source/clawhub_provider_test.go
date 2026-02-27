package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"skillpm/internal/config"
)

func TestClawHubUpdateDiscoveryFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch r.URL.Path {
		case "/.well-known/clawhub.json":
			http.NotFound(w, r)
		case "/.well-known/clawdhub.json":
			_ = json.NewEncoder(w).Encode(map[string]string{"registry": base + "/api-root/", "authBase": base})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Sources = []config.SourceConfig{{
		Name:      "clawhub",
		Kind:      "clawhub",
		Site:      server.URL + "/",
		TrustTier: "review",
	}}
	mgr := NewManager(server.Client(), t.TempDir(), false)
	updated, err := mgr.Update(context.Background(), &cfg, "clawhub")
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 updated source")
	}
	if updated[0].Source.Registry != server.URL+"/api-root/" {
		t.Fatalf("expected registry from legacy well-known, got %q", updated[0].Source.Registry)
	}
}

func TestClawHubUpdatePrefersAPIBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		if r.URL.Path == "/.well-known/clawhub.json" {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"apiBase":  base + "/preferred/",
				"registry": base + "/legacy/",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Sources = []config.SourceConfig{{Name: "clawhub", Kind: "clawhub", Site: server.URL + "/", TrustTier: "review"}}
	mgr := NewManager(server.Client(), t.TempDir(), false)
	updated, err := mgr.Update(context.Background(), &cfg, "clawhub")
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if got := updated[0].Source.Registry; got != server.URL+"/preferred/" {
		t.Fatalf("expected apiBase precedence, got %q", got)
	}
}

func TestClawHubSearchHandlesRateLimitRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]string{{"slug": "forms-extractor", "description": "forms"}}})
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Sources = []config.SourceConfig{{Name: "clawhub", Kind: "clawhub", Registry: server.URL + "/", TrustTier: "review"}}
	mgr := NewManager(server.Client(), t.TempDir(), false)
	results, err := mgr.Search(context.Background(), cfg, "clawhub", "forms")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 || results[0].Slug != "forms-extractor" {
		t.Fatalf("unexpected search results: %+v", results)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected retry path to execute")
	}
}

func TestClawHubResolveLatestAndModeration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/skills/forms-extractor":
			_ = json.NewEncoder(w).Encode(map[string]any{"moderation": map[string]bool{"isSuspicious": true, "isMalwareBlocked": false}})
		case r.URL.Path == "/api/v1/skills/forms-extractor/versions":
			_ = json.NewEncoder(w).Encode(map[string]any{"versions": []string{"1.0.0", "1.2.0"}})
		case r.URL.Path == "/api/v1/download":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.2.0", "content": "artifact-blob"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.SourceConfig{Name: "clawhub", Kind: "clawhub", Registry: server.URL + "/", TrustTier: "review"}
	mgr := NewManager(server.Client(), t.TempDir(), false)
	res, err := mgr.Resolve(context.Background(), cfg, ResolveRequest{Skill: "forms-extractor", Constraint: ""})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if res.ResolvedVersion != "1.2.0" {
		t.Fatalf("expected latest version 1.2.0, got %q", res.ResolvedVersion)
	}
	if !res.Moderation.IsSuspicious {
		t.Fatalf("expected moderation signal to propagate")
	}
}

func TestEscapeSlugPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"forms-extractor", "forms-extractor"},
		{"steipete/code-review", "steipete/code-review"},
		{"secops/secret-scanner", "secops/secret-scanner"},
		{"org/sub/deep", "org/sub/deep"},
		{"has space/slug", "has%20space/slug"},
	}
	for _, tc := range tests {
		got := escapeSlugPath(tc.input)
		if got != tc.want {
			t.Errorf("escapeSlugPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClawHubResolveOrgScopedSlug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/skills/steipete/code-review":
			_ = json.NewEncoder(w).Encode(map[string]any{"moderation": map[string]bool{}})
		case r.URL.Path == "/api/v1/skills/steipete/code-review/versions":
			_ = json.NewEncoder(w).Encode(map[string]any{"versions": []string{"2.1.0"}})
		case r.URL.Path == "/api/v1/download":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "2.1.0", "content": "# Code Review"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.SourceConfig{Name: "clawhub", Kind: "clawhub", Registry: server.URL + "/", TrustTier: "review"}
	mgr := NewManager(server.Client(), t.TempDir(), false)
	res, err := mgr.Resolve(context.Background(), cfg, ResolveRequest{Skill: "steipete/code-review", Constraint: ""})
	if err != nil {
		t.Fatalf("resolve org-scoped slug failed: %v", err)
	}
	if res.ResolvedVersion != "2.1.0" {
		t.Fatalf("expected version 2.1.0, got %q", res.ResolvedVersion)
	}
	if res.SkillRef != "clawhub/steipete/code-review" {
		t.Fatalf("expected skillRef clawhub/steipete/code-review, got %q", res.SkillRef)
	}
}
