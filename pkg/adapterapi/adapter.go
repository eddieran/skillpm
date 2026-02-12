package adapterapi

import "context"

type Adapter interface {
	Probe(ctx context.Context) (ProbeResult, error)
	Inject(ctx context.Context, req InjectRequest) (InjectResult, error)
	Remove(ctx context.Context, req RemoveRequest) (RemoveResult, error)
	ListInjected(ctx context.Context, req ListInjectedRequest) (ListInjectedResult, error)
	HarvestCandidates(ctx context.Context, req HarvestRequest) (HarvestResult, error)
	ValidateEnvironment(ctx context.Context) (ValidateResult, error)
}

type ProbeResult struct {
	Name         string   `json:"name"`
	Available    bool     `json:"available"`
	Capabilities []string `json:"capabilities"`
	Message      string   `json:"message,omitempty"`
}

type InjectRequest struct {
	SkillRefs []string `json:"skillRefs"`
	Scope     string   `json:"scope,omitempty"`
	Force     bool     `json:"force,omitempty"`
}

type InjectResult struct {
	Agent            string   `json:"agent"`
	Injected         []string `json:"injected"`
	SnapshotPath     string   `json:"snapshotPath,omitempty"`
	RollbackPossible bool     `json:"rollbackPossible"`
}

type RemoveRequest struct {
	SkillRefs []string `json:"skillRefs,omitempty"`
	Scope     string   `json:"scope,omitempty"`
}

type RemoveResult struct {
	Agent        string   `json:"agent"`
	Removed      []string `json:"removed"`
	SnapshotPath string   `json:"snapshotPath,omitempty"`
}

type ListInjectedRequest struct {
	Scope string `json:"scope,omitempty"`
}

type ListInjectedResult struct {
	Agent  string   `json:"agent"`
	Skills []string `json:"skills"`
}

type HarvestRequest struct {
	Scope string `json:"scope,omitempty"`
}

type HarvestCandidate struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Reason  string `json:"reason,omitempty"`
	Adapter string `json:"adapter"`
}

type HarvestResult struct {
	Agent      string             `json:"agent"`
	Candidates []HarvestCandidate `json:"candidates"`
	Supported  bool               `json:"supported"`
	Message    string             `json:"message,omitempty"`
}

type ValidateResult struct {
	Agent     string   `json:"agent"`
	Valid     bool     `json:"valid"`
	Warnings  []string `json:"warnings,omitempty"`
	Errors    []string `json:"errors,omitempty"`
	RootPaths []string `json:"rootPaths,omitempty"`
}
