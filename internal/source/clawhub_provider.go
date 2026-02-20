package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"skillpm/internal/config"
)

type clawHubProvider struct {
	client *http.Client
}

type wellKnownPayload struct {
	APIBase       string `json:"apiBase"`
	Registry      string `json:"registry"`
	AuthBase      string `json:"authBase"`
	MinCLIVersion string `json:"minCliVersion"`
}

func (p *clawHubProvider) Update(ctx context.Context, src config.SourceConfig) (UpdateResult, error) {
	site := src.Site
	if site == "" {
		site = src.Registry
	}
	if site == "" {
		site = "https://clawhub.ai/"
	}
	wellKnown := src.WellKnown
	if len(wellKnown) == 0 {
		wellKnown = []string{"/.well-known/clawhub.json", "/.well-known/clawdhub.json"}
	}
	payload, usedPath, err := p.discover(ctx, site, wellKnown)
	if err != nil {
		if src.Registry != "" {
			return UpdateResult{Source: src, Note: "discovery failed, kept configured registry"}, nil
		}
		return UpdateResult{}, err
	}
	resolved := payload.APIBase
	if resolved == "" {
		resolved = payload.Registry
	}
	if resolved == "" {
		resolved = src.Registry
	}
	if resolved == "" {
		resolved = "https://clawhub.ai/"
	}
	src.Registry = ensureTrailingSlash(resolved)
	if src.Site == "" {
		src.Site = ensureTrailingSlash(site)
	}
	src.AuthBase = payload.AuthBase
	src.MinCLIVersion = payload.MinCLIVersion
	src.CachedRegistry = src.Registry
	if src.APIVersion == "" {
		src.APIVersion = "v1"
	}
	if len(src.WellKnown) == 0 {
		src.WellKnown = wellKnown
	}
	return UpdateResult{Source: src, Note: "discovered via " + usedPath}, nil
}

func (p *clawHubProvider) discover(ctx context.Context, site string, paths []string) (wellKnownPayload, string, error) {
	base, err := url.Parse(ensureTrailingSlash(site))
	if err != nil {
		return wellKnownPayload{}, "", fmt.Errorf("SRC_CLAWHUB_DISCOVERY: invalid site %q", site)
	}
	for _, wkPath := range paths {
		u := *base
		u.Path = path.Join(base.Path, wkPath)
		status, body, err := p.getRaw(ctx, u.String())
		if err != nil {
			return wellKnownPayload{}, "", err
		}
		if status != http.StatusOK {
			continue
		}
		var payload wellKnownPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return wellKnownPayload{}, "", fmt.Errorf("SRC_CLAWHUB_DISCOVERY: invalid well-known payload: %w", err)
		}
		if payload.APIBase == "" && payload.Registry == "" {
			continue
		}
		return payload, wkPath, nil
	}
	return wellKnownPayload{}, "", fmt.Errorf("SRC_CLAWHUB_DISCOVERY: no valid well-known payload found")
}

func (p *clawHubProvider) Search(ctx context.Context, src config.SourceConfig, query string) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("SRC_SEARCH: query is required")
	}
	base := resolvedRegistry(src)
	q := url.Values{}
	q.Set("q", query)
	status, body, err := p.getJSONWithFallback(ctx, base, "/api/v1/search", q)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("SRC_SEARCH: provider returned status %d", status)
	}
	return parseSearchResponse(src.Name, body), nil
}

func (p *clawHubProvider) Resolve(ctx context.Context, src config.SourceConfig, req ResolveRequest) (ResolveResult, error) {
	if req.Skill == "" {
		return ResolveResult{}, fmt.Errorf("SRC_RESOLVE: empty skill")
	}
	base := resolvedRegistry(src)
	moderation, err := p.fetchModeration(ctx, base, req.Skill)
	if err != nil {
		return ResolveResult{}, err
	}

	constraint := strings.TrimSpace(req.Constraint)
	resolverHash := ""
	resolvedVersion := ""
	tag := ""
	if strings.HasPrefix(constraint, "sha256:") {
		resVersion, hash, err := p.resolveByHash(ctx, base, req.Skill, constraint)
		if err != nil {
			return ResolveResult{}, err
		}
		resolvedVersion = resVersion
		resolverHash = hash
	} else if constraint == "" || strings.EqualFold(constraint, "latest") {
		resVersion, err := p.resolveLatest(ctx, base, req.Skill)
		if err != nil {
			return ResolveResult{}, err
		}
		resolvedVersion = resVersion
	} else if strings.HasPrefix(constraint, "tag:") {
		tag = strings.TrimPrefix(constraint, "tag:")
	} else if looksLikeVersion(constraint) {
		resolvedVersion = constraint
	} else {
		tag = constraint
	}

	checksum, resolvedVersionFromDownload, content, err := p.downloadChecksum(ctx, base, req.Skill, resolvedVersion, tag)
	if err != nil {
		return ResolveResult{}, err
	}
	if resolvedVersion == "" {
		resolvedVersion = resolvedVersionFromDownload
	}
	if resolvedVersion == "" {
		resolvedVersion = "latest"
	}
	host := extractHost(base)
	sourceRef := fmt.Sprintf("clawhub://%s/skills/%s@%s", host, req.Skill, resolvedVersion)
	return ResolveResult{
		SkillRef:        fmt.Sprintf("%s/%s", src.Name, req.Skill),
		ResolvedVersion: resolvedVersion,
		Checksum:        checksum,
		SourceRef:       sourceRef,
		Source:          src.Name,
		Skill:           req.Skill,
		Content:         content,
		Moderation:      moderation,
		ResolverHash:    resolverHash,
	}, nil
}

func (p *clawHubProvider) fetchModeration(ctx context.Context, base, slug string) (Moderation, error) {
	status, body, err := p.getJSONWithFallback(ctx, base, "/api/v1/skills/"+url.PathEscape(slug), nil)
	if err != nil {
		return Moderation{}, err
	}
	if status != http.StatusOK {
		return Moderation{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return Moderation{}, nil
	}
	modAny, ok := payload["moderation"]
	if !ok {
		return Moderation{}, nil
	}
	modMap, ok := modAny.(map[string]any)
	if !ok {
		return Moderation{}, nil
	}
	mod := Moderation{}
	if v, ok := modMap["isMalwareBlocked"].(bool); ok {
		mod.IsMalwareBlocked = v
	}
	if v, ok := modMap["isSuspicious"].(bool); ok {
		mod.IsSuspicious = v
	}
	return mod, nil
}

func (p *clawHubProvider) resolveByHash(ctx context.Context, base, slug, hash string) (string, string, error) {
	q := url.Values{}
	q.Set("slug", slug)
	q.Set("hash", hash)
	status, body, err := p.getJSONWithFallback(ctx, base, "/api/v1/resolve", q)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("SRC_RESOLVE: hash resolve returned status %d", status)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("SRC_RESOLVE: bad resolve payload: %w", err)
	}
	version, _ := payload["version"].(string)
	resolverHash := hash
	if r, ok := payload["hash"].(string); ok && r != "" {
		resolverHash = r
	}
	if version == "" {
		return "", "", fmt.Errorf("SRC_RESOLVE: resolve payload missing version")
	}
	return version, resolverHash, nil
}

func (p *clawHubProvider) resolveLatest(ctx context.Context, base, slug string) (string, error) {
	status, body, err := p.getJSONWithFallback(ctx, base, "/api/v1/skills/"+url.PathEscape(slug)+"/versions", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("SRC_RESOLVE: versions returned status %d", status)
	}
	versions := parseVersions(body)
	if len(versions) == 0 {
		return "", fmt.Errorf("SRC_RESOLVE: no versions found for %s", slug)
	}
	return chooseLatest(versions), nil
}

func (p *clawHubProvider) downloadChecksum(ctx context.Context, base, slug, version, tag string) (checksum string, resolvedVersion string, content string, err error) {
	q := url.Values{}
	q.Set("slug", slug)
	if version != "" {
		q.Set("version", version)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	status, body, err := p.getRawWithFallback(ctx, base, "/api/v1/download", q)
	if err != nil {
		return "", "", "", err
	}
	if status != http.StatusOK {
		return "", "", "", fmt.Errorf("SRC_DOWNLOAD: status %d", status)
	}
	h := sha256.Sum256(body)
	checksum = "sha256:" + hex.EncodeToString(h[:])

	// Try to parse metadata-style response that includes version.
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		if v, ok := payload["version"].(string); ok {
			resolvedVersion = v
		}
		if c, ok := payload["content"].(string); ok {
			content = c
			h2 := sha256.Sum256([]byte(c))
			checksum = "sha256:" + hex.EncodeToString(h2[:])
		}
	}
	return checksum, resolvedVersion, content, nil
}

func (p *clawHubProvider) getJSONWithFallback(ctx context.Context, base, endpoint string, query url.Values) (int, []byte, error) {
	status, body, err := p.getRawWithFallback(ctx, base, endpoint, query)
	if err != nil {
		return 0, nil, err
	}
	return status, body, nil
}

func (p *clawHubProvider) getRawWithFallback(ctx context.Context, base, endpoint string, query url.Values) (int, []byte, error) {
	url1 := buildURL(base, endpoint, query)
	status, body, err := p.getRaw(ctx, url1)
	if err != nil {
		return 0, nil, err
	}
	if status != http.StatusNotFound || !strings.Contains(endpoint, "/api/v1/") {
		return status, body, nil
	}
	legacy := strings.Replace(endpoint, "/api/v1/", "/api/", 1)
	url2 := buildURL(base, legacy, query)
	return p.getRaw(ctx, url2)
}

func (p *clawHubProvider) getRaw(ctx context.Context, fullURL string) (int, []byte, error) {
	attempts := 5
	var lastErr error
	for i := 0; i < attempts; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("User-Agent", "skillpm/1.0 (+https://github.com/eddieran/skillpm)")
		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = err
			// Check if we should retry network errors
			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			case <-time.After(time.Duration(1<<i) * 500 * time.Millisecond):
			}
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return 0, nil, readErr
		}
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && i < attempts-1 {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"), i)
			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		return resp.StatusCode, body, nil
	}
	if lastErr != nil {
		return 0, nil, fmt.Errorf("SRC_HTTP: %w", lastErr)
	}
	return 0, nil, errors.New("SRC_HTTP: request failed")
}

func parseRetryAfter(value string, attempt int) time.Duration {
	defaultBackoff := time.Duration(1<<attempt) * 500 * time.Millisecond
	if value == "" {
		return defaultBackoff
	}
	secs, err := strconv.Atoi(value)
	if err != nil || secs < 0 {
		return defaultBackoff
	}
	if secs > 10 {
		secs = 10
	}
	return time.Duration(secs) * time.Second
}

func parseSearchResponse(sourceName string, body []byte) []SearchResult {
	var out []SearchResult

	var arr []map[string]any
	if json.Unmarshal(body, &arr) == nil {
		for _, row := range arr {
			out = append(out, mapSearch(sourceName, row))
		}
		return compactSearch(out)
	}
	var obj map[string]any
	if json.Unmarshal(body, &obj) != nil {
		return nil
	}
	for _, key := range []string{"items", "skills", "data", "results"} {
		list, ok := obj[key].([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, mapSearch(sourceName, row))
		}
	}
	return compactSearch(out)
}

func mapSearch(sourceName string, row map[string]any) SearchResult {
	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := row[k].(string); ok {
				return v
			}
		}
		return ""
	}
	return SearchResult{
		Source:      sourceName,
		Slug:        get("slug", "name", "id"),
		Name:        get("title", "displayName", "name", "slug"),
		Description: get("description", "summary"),
	}
}

func compactSearch(in []SearchResult) []SearchResult {
	seen := map[string]struct{}{}
	out := make([]SearchResult, 0, len(in))
	for _, item := range in {
		if item.Slug == "" {
			continue
		}
		key := item.Source + "/" + item.Slug
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if item.Name == "" {
			item.Name = item.Slug
		}
		out = append(out, item)
	}
	return out
}

func parseVersions(body []byte) []string {
	var versions []string
	var rawAny any
	if json.Unmarshal(body, &rawAny) != nil {
		return nil
	}
	switch v := rawAny.(type) {
	case []any:
		for _, item := range v {
			versions = append(versions, parseVersionItem(item)...)
		}
	case map[string]any:
		for _, key := range []string{"items", "versions", "data", "results"} {
			if list, ok := v[key].([]any); ok {
				for _, item := range list {
					versions = append(versions, parseVersionItem(item)...)
				}
			}
		}
	}
	uniq := map[string]struct{}{}
	out := make([]string, 0, len(versions))
	for _, ver := range versions {
		if ver == "" {
			continue
		}
		if _, ok := uniq[ver]; ok {
			continue
		}
		uniq[ver] = struct{}{}
		out = append(out, ver)
	}
	return out
}

func parseVersionItem(item any) []string {
	switch iv := item.(type) {
	case string:
		return []string{iv}
	case map[string]any:
		for _, key := range []string{"version", "name", "id"} {
			if v, ok := iv[key].(string); ok {
				return []string{v}
			}
		}
	}
	return nil
}

func chooseLatest(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	sort.SliceStable(versions, func(i, j int) bool {
		vi := normalizeSemver(versions[i])
		vj := normalizeSemver(versions[j])
		if vi == "" || vj == "" {
			return versions[i] > versions[j]
		}
		return semver.Compare(vi, vj) > 0
	})
	return versions[0]
}

func normalizeSemver(v string) string {
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

func looksLikeVersion(v string) bool {
	if v == "" {
		return false
	}
	if normalizeSemver(v) != "" {
		return true
	}
	if strings.Count(v, ".") >= 1 && !strings.ContainsAny(v, "/ @") {
		return true
	}
	return false
}

func buildURL(base, endpoint string, query url.Values) string {
	u, _ := url.Parse(base)
	u.Path = path.Join(u.Path, endpoint)
	u.RawQuery = query.Encode()
	return u.String()
}

func resolvedRegistry(src config.SourceConfig) string {
	if src.Registry != "" {
		return ensureTrailingSlash(src.Registry)
	}
	if src.CachedRegistry != "" {
		return ensureTrailingSlash(src.CachedRegistry)
	}
	if src.Site != "" {
		return ensureTrailingSlash(src.Site)
	}
	return "https://clawhub.ai/"
}

func ensureTrailingSlash(v string) string {
	if strings.HasSuffix(v, "/") {
		return v
	}
	return v + "/"
}

func extractHost(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "clawhub.ai"
	}
	return u.Host
}
