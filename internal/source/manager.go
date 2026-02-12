package source

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"skillpm/internal/config"
)

type Provider interface {
	Update(ctx context.Context, src config.SourceConfig) (UpdateResult, error)
	Search(ctx context.Context, src config.SourceConfig, query string) ([]SearchResult, error)
	Resolve(ctx context.Context, src config.SourceConfig, req ResolveRequest) (ResolveResult, error)
}

type Manager struct {
	providers map[string]Provider
}

func NewManager(httpClient *http.Client) *Manager {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Manager{
		providers: map[string]Provider{
			"git":     &gitProvider{},
			"dir":     &gitProvider{},
			"clawhub": &clawHubProvider{client: httpClient},
		},
	}
}

func (m *Manager) provider(kind string) (Provider, error) {
	p, ok := m.providers[kind]
	if !ok {
		return nil, fmt.Errorf("SRC_PROVIDER: unsupported source kind %q", kind)
	}
	return p, nil
}

func (m *Manager) Update(ctx context.Context, cfg *config.Config, name string) ([]UpdateResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("SRC_UPDATE: nil config")
	}
	var targets []config.SourceConfig
	if name == "" {
		targets = append(targets, cfg.Sources...)
	} else {
		s, ok := config.FindSource(*cfg, name)
		if !ok {
			return nil, fmt.Errorf("SRC_UPDATE: source %q not found", name)
		}
		targets = append(targets, s)
	}

	results := make([]UpdateResult, 0, len(targets))
	for _, src := range targets {
		provider, err := m.provider(src.Kind)
		if err != nil {
			return nil, err
		}
		res, err := provider.Update(ctx, src)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
		_ = config.ReplaceSource(cfg, res.Source)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Source.Name < results[j].Source.Name })
	return results, nil
}

func (m *Manager) Search(ctx context.Context, cfg config.Config, sourceName string, query string) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("SRC_SEARCH: query is required")
	}
	var sources []config.SourceConfig
	if sourceName != "" {
		s, ok := config.FindSource(cfg, sourceName)
		if !ok {
			return nil, fmt.Errorf("SRC_SEARCH: source %q not found", sourceName)
		}
		sources = append(sources, s)
	} else {
		sources = cfg.Sources
	}

	var out []SearchResult
	for _, src := range sources {
		provider, err := m.provider(src.Kind)
		if err != nil {
			return nil, err
		}
		items, err := provider.Search(ctx, src, query)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source == out[j].Source {
			return out[i].Slug < out[j].Slug
		}
		return out[i].Source < out[j].Source
	})
	return out, nil
}

func (m *Manager) Resolve(ctx context.Context, src config.SourceConfig, req ResolveRequest) (ResolveResult, error) {
	provider, err := m.provider(src.Kind)
	if err != nil {
		return ResolveResult{}, err
	}
	return provider.Resolve(ctx, src, req)
}
