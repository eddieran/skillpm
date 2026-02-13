package doctor

import (
	"context"
	"os"
	"strings"

	"skillpm/internal/adapter"
	"skillpm/internal/config"
	"skillpm/internal/store"
)

type Finding struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Report struct {
	Healthy          bool      `json:"healthy"`
	Findings         []Finding `json:"findings"`
	DetectedAdapters []string  `json:"detectedAdapters,omitempty"`
}

type Service struct {
	ConfigPath string
	StateRoot  string
	Runtime    *adapter.Runtime
}

func (s *Service) Run(ctx context.Context) Report {
	findings := []Finding{}
	detected := adapter.DetectAvailable()
	reportDetected := make([]string, 0, len(detected))
	for _, d := range detected {
		reportDetected = append(reportDetected, d.Name)
	}
	enabled := map[string]struct{}{}
	if _, err := os.Stat(s.ConfigPath); err != nil {
		findings = append(findings, Finding{Code: "DOC_CONFIG_MISSING", Level: "error", Message: err.Error()})
	} else if cfg, err := config.Load(s.ConfigPath); err != nil {
		findings = append(findings, Finding{Code: "DOC_CONFIG_INVALID", Level: "error", Message: err.Error()})
	} else {
		for _, a := range cfg.Adapters {
			if a.Enabled {
				enabled[strings.ToLower(a.Name)] = struct{}{}
			}
		}
	}

	if _, err := store.LoadState(s.StateRoot); err != nil {
		findings = append(findings, Finding{Code: "DOC_STATE_INVALID", Level: "error", Message: err.Error()})
	}

	if s.Runtime != nil {
		if probes, err := s.Runtime.ProbeAll(ctx); err != nil {
			findings = append(findings, Finding{Code: "ADP_PROBE_FAIL", Level: "error", Message: err.Error()})
		} else {
			for _, p := range probes {
				if !p.Available {
					findings = append(findings, Finding{Code: "ADP_UNAVAILABLE", Level: "warn", Message: p.Name + " unavailable"})
				}
			}
		}
	}
	for _, d := range detected {
		if _, ok := enabled[strings.ToLower(d.Name)]; ok {
			continue
		}
		findings = append(findings, Finding{
			Code:    "ADP_DETECTED_DISABLED",
			Level:   "warn",
			Message: d.Name + " detected at " + d.Path + " but not enabled in config",
		})
	}

	healthy := true
	for _, f := range findings {
		if f.Level == "error" {
			healthy = false
			break
		}
	}
	return Report{Healthy: healthy, Findings: findings, DetectedAdapters: reportDetected}
}
