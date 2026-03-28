package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

type registrySkill struct {
	Slug        string
	Version     string
	Name        string
	Category    string
	Description string
	Content     string
	Files       map[string]string
	Downloads   int
}

type mockRegistry struct {
	server *httptest.Server
	mu     sync.Mutex
	skills map[string]*registrySkill
}

func TestOfficialSkillsPublishInstallInject(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}

	reg := startMockRegistry(t)
	defer reg.server.Close()
	cfgPath := writeRegistryConfig(t, home, reg.server.URL)

	slugs := []string{
		"code-reviewer",
		"test-writer",
		"git-conventional",
		"dependency-auditor",
		"doc-sync",
	}

	for _, slug := range slugs {
		skillDir := filepath.Join(repoRoot(t), "skills", slug)
		publishOut := runCLIWithEnv(t, bin, env, map[string]string{"CLAWHUB_TOKEN": "dummy"}, "--config", cfgPath, "publish", skillDir, "--source", "clawhub")
		assertContains(t, publishOut, "Published "+slug+"@1.0.0")

		installOut := runCLI(t, bin, env, "--config", cfgPath, "install", "clawhub/"+slug)
		assertContains(t, installOut, "installed clawhub/"+slug+"@1.0.0")
	}

	claudeOut := runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "claude")
	assertContains(t, claudeOut, "injected into claude:")
	codexOut := runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "codex")
	assertContains(t, codexOut, "injected into codex:")

	for _, slug := range slugs {
		assertInjectedFile(t, home, ".claude", slug, "SKILL.md")
		assertInjectedFile(t, home, ".claude", slug, "README.md")
		assertInjectedFile(t, home, ".claude", slug, filepath.Join("tests", "cases.yaml"))
		assertInjectedFile(t, home, ".agents", slug, "SKILL.md")
		assertInjectedFile(t, home, ".agents", slug, "README.md")
		assertInjectedFile(t, home, ".agents", slug, filepath.Join("tests", "cases.yaml"))
	}

}

func startMockRegistry(t *testing.T) *mockRegistry {
	t.Helper()
	reg := &mockRegistry{skills: map[string]*registrySkill{}}
	reg.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reg.handle(t, w, r)
	}))
	return reg
}

func (r *mockRegistry) handle(t *testing.T, w http.ResponseWriter, req *http.Request) {
	t.Helper()
	switch {
	case req.Method == http.MethodPost && strings.HasPrefix(req.URL.Path, "/api/v1/skills/") && strings.HasSuffix(req.URL.Path, "/versions"):
		slug := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/api/v1/skills/"), "/versions")
		var payload struct {
			Slug        string            `json:"slug"`
			Version     string            `json:"version"`
			Content     string            `json:"content"`
			Description string            `json:"description"`
			Files       map[string]string `json:"files"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.mu.Lock()
		r.skills[slug] = &registrySkill{
			Slug:        slug,
			Version:     payload.Version,
			Name:        frontmatterField(payload.Content, "name"),
			Category:    frontmatterField(payload.Content, "category"),
			Description: payload.Description,
			Content:     payload.Content,
			Files:       payload.Files,
		}
		r.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/api/v1/skills/") && !strings.HasSuffix(req.URL.Path, "/versions"):
		_ = json.NewEncoder(w).Encode(map[string]any{"moderation": map[string]bool{"isSuspicious": false, "isMalwareBlocked": false}})
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/api/v1/skills/") && strings.HasSuffix(req.URL.Path, "/versions"):
		slug := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/api/v1/skills/"), "/versions")
		r.mu.Lock()
		skill := r.skills[slug]
		r.mu.Unlock()
		if skill == nil {
			http.NotFound(w, req)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"versions": []string{skill.Version}})
	case req.Method == http.MethodGet && req.URL.Path == "/api/v1/download":
		slug := req.URL.Query().Get("slug")
		r.mu.Lock()
		skill := r.skills[slug]
		if skill != nil {
			skill.Downloads++
		}
		r.mu.Unlock()
		if skill == nil {
			http.NotFound(w, req)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": skill.Version,
			"content": skill.Content,
			"files":   skill.Files,
		})
	case req.Method == http.MethodGet && req.URL.Path == "/api/v1/trending":
		r.mu.Lock()
		var skills []*registrySkill
		for _, skill := range r.skills {
			copySkill := *skill
			skills = append(skills, &copySkill)
		}
		r.mu.Unlock()
		sort.Slice(skills, func(i, j int) bool {
			if skills[i].Downloads != skills[j].Downloads {
				return skills[i].Downloads > skills[j].Downloads
			}
			return skills[i].Slug < skills[j].Slug
		})
		limit := len(skills)
		if raw := req.URL.Query().Get("limit"); raw != "" {
			var parsed int
			fmt.Sscanf(raw, "%d", &parsed)
			if parsed > 0 && parsed < limit {
				limit = parsed
			}
		}
		category := req.URL.Query().Get("category")
		var entries []map[string]any
		for _, skill := range skills {
			if category != "" && skill.Category != category {
				continue
			}
			entries = append(entries, map[string]any{
				"slug":        skill.Slug,
				"name":        skill.Name,
				"category":    skill.Category,
				"downloads":   skill.Downloads,
				"rating":      5.0,
				"source":      "clawhub",
				"description": skill.Description,
			})
			if len(entries) == limit {
				break
			}
		}
		_ = json.NewEncoder(w).Encode(entries)
	default:
		http.NotFound(w, req)
	}
}

func frontmatterField(content, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		return strings.TrimSpace(strings.Trim(strings.TrimPrefix(trimmed, prefix), `"'`))
	}
	return ""
}

func writeRegistryConfig(t *testing.T, home, registryURL string) string {
	t.Helper()
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	body := fmt.Sprintf(`version = 1

[[sources]]
name = "clawhub"
kind = "clawhub"
site = %q
registry = %q
trust_tier = "review"

[[adapters]]
name = "claude"
enabled = true
scope = "global"

[[adapters]]
name = "codex"
enabled = true
scope = "global"
`, registryURL+"/", registryURL+"/")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}

func assertInjectedFile(t *testing.T, home, agentRoot, slug, rel string) {
	t.Helper()
	path := filepath.Join(home, agentRoot, "skills", slug, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", path)
	}
}
