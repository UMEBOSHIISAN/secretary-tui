package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
)

const maxGovernanceBytes = 1 << 20

var secretGovernanceKeys = map[string]struct{}{
	"password":      {},
	"secret":        {},
	"api_key":       {},
	"access_token":  {},
	"refresh_token": {},
	"credential":    {},
	"private_key":   {},
}

type governanceSnapshot struct {
	source      string
	taskID      string
	capability  string
	risk        string
	tokenBudget int
	status      string
	alias       string
	reasons     []string
	evidenceN   int
	available   bool
}

type wgmHandoff struct {
	SchemaVersion      string   `json:"schema_version"`
	TaskID             string   `json:"task_id"`
	Capability         string   `json:"capability"`
	Risk               string   `json:"risk"`
	TokenBudget        int      `json:"token_budget"`
	EvidenceReferences []string `json:"evidence_references"`
}

type routerManifest struct {
	Status           string   `json:"status"`
	RecommendedAlias string   `json:"recommended_alias"`
	Reasons          []string `json:"reasons"`
	AuthorityEffect  *bool    `json:"authority_effect"`
	ExecutionEffect  *bool    `json:"execution_effect"`
}

func readGovernance(path string) (governanceSnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return governanceSnapshot{}, errors.New("governance snapshot is not readable")
	}
	if info.IsDir() || info.Size() > maxGovernanceBytes {
		return governanceSnapshot{}, errors.New("governance snapshot must be a file no larger than 1 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return governanceSnapshot{}, errors.New("governance snapshot is not readable")
	}
	if len(data) > maxGovernanceBytes {
		return governanceSnapshot{}, errors.New("governance snapshot must be no larger than 1 MiB")
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return governanceSnapshot{}, errors.New("governance snapshot is not valid JSON")
	}
	if containsSecretGovernanceKey(document) {
		return governanceSnapshot{}, errors.New("governance snapshot contains a forbidden secret-bearing key")
	}
	if _, exists := document["schema_version"]; exists {
		return decodeWGMHandoff(data)
	}
	if _, exists := document["status"]; exists {
		return decodeRouterManifest(data)
	}
	return governanceSnapshot{}, errors.New("unsupported governance snapshot shape")
}

func decodeWGMHandoff(data []byte) (governanceSnapshot, error) {
	var handoff wgmHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		return governanceSnapshot{}, errors.New("invalid WGM handoff")
	}
	if handoff.SchemaVersion != "1.0" || handoff.TaskID == "" || handoff.Capability == "" || handoff.Risk == "" || handoff.TokenBudget < 1 || len(handoff.EvidenceReferences) == 0 {
		return governanceSnapshot{}, errors.New("incomplete or unsupported WGM handoff")
	}
	return governanceSnapshot{
		source: "WGM handoff", taskID: handoff.TaskID, capability: handoff.Capability,
		risk: handoff.Risk, tokenBudget: handoff.TokenBudget,
		evidenceN: len(handoff.EvidenceReferences), available: true,
	}, nil
}

func decodeRouterManifest(data []byte) (governanceSnapshot, error) {
	var manifest routerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return governanceSnapshot{}, errors.New("invalid Router manifest")
	}
	if manifest.Status == "" || manifest.AuthorityEffect == nil || manifest.ExecutionEffect == nil {
		return governanceSnapshot{}, errors.New("incomplete Router manifest")
	}
	if *manifest.AuthorityEffect || *manifest.ExecutionEffect {
		return governanceSnapshot{}, errors.New("Router manifest claims an authority or execution effect")
	}
	return governanceSnapshot{
		source: "Router manifest", status: manifest.Status, alias: manifest.RecommendedAlias,
		reasons: append([]string(nil), manifest.Reasons...), available: true,
	}, nil
}

func containsSecretGovernanceKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, forbidden := secretGovernanceKeys[strings.ToLower(key)]; forbidden {
				return true
			}
			if containsSecretGovernanceKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsSecretGovernanceKey(child) {
				return true
			}
		}
	}
	return false
}

func governanceLines(snapshot governanceSnapshot) []string {
	if !snapshot.available {
		return []string{"(governance snapshot unavailable)"}
	}
	lines := []string{"source: " + safeGovernanceText(snapshot.source)}
	if snapshot.source == "WGM handoff" {
		lines = append(lines,
			"task: "+safeGovernanceText(snapshot.taskID),
			"capability: "+safeGovernanceText(snapshot.capability),
			"risk: "+safeGovernanceText(snapshot.risk),
			fmt.Sprintf("budget: %d tokens", snapshot.tokenBudget),
			fmt.Sprintf("evidence: %d refs", snapshot.evidenceN),
		)
	} else {
		lines = append(lines, "status: "+safeGovernanceText(snapshot.status))
		if snapshot.alias != "" {
			lines = append(lines, "candidate: "+safeGovernanceText(snapshot.alias))
		}
		if len(snapshot.reasons) > 0 {
			lines = append(lines, "reasons: "+safeGovernanceText(strings.Join(snapshot.reasons, ", ")))
		}
	}
	return append(lines, "authority: none", "execution: none", "local snapshot")
}

func safeGovernanceText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, value)
	const limit = 120
	if len([]rune(value)) > limit {
		return string([]rune(value)[:limit]) + "…"
	}
	return value
}
