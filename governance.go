package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	source              string
	sourceKind          string
	sourceSchemaVersion string
	taskID              string
	capability          string
	risk                string
	tokenBudget         int
	status              string
	alias               string
	reasons             []string
	evidenceN           int
	available           bool
	exportable          bool
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
	SchemaVersion    string   `json:"schema_version"`
	TaskID           *string  `json:"task_id"`
	Capability       *string  `json:"capability"`
	Status           string   `json:"status"`
	RecommendedAlias *string  `json:"recommended_alias"`
	RegistrySHA256   *string  `json:"registry_sha256"`
	Reasons          []string `json:"reasons"`
	AuthorityEffect  *bool    `json:"authority_effect"`
	ExecutionEffect  *bool    `json:"execution_effect"`
}

type observationDocument struct {
	SchemaVersion       string   `json:"schema_version"`
	TaskID              *string  `json:"task_id"`
	SourceKind          string   `json:"source_kind"`
	SourceSchemaVersion string   `json:"source_schema_version"`
	Status              string   `json:"status"`
	Summary             []string `json:"summary"`
	AuthorityEffect     bool     `json:"authority_effect"`
	ExecutionEffect     bool     `json:"execution_effect"`
}

func readGovernance(path string) (governanceSnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return governanceSnapshot{}, errors.New("governance snapshot is not readable")
	}
	if !info.Mode().IsRegular() || info.Size() > maxGovernanceBytes {
		return governanceSnapshot{}, errors.New("governance snapshot must be a file no larger than 1 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return governanceSnapshot{}, errors.New("governance snapshot is not readable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxGovernanceBytes+1))
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
	if _, exists := document["status"]; exists {
		return decodeRouterManifest(data)
	}
	if _, exists := document["schema_version"]; exists {
		return decodeWGMHandoff(data)
	}
	return governanceSnapshot{}, errors.New("unsupported governance snapshot shape")
}

func decodeWGMHandoff(data []byte) (governanceSnapshot, error) {
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil || !hasExactKeys(document,
		"schema_version", "task_id", "capability", "risk", "token_budget", "evidence_references",
	) {
		return governanceSnapshot{}, errors.New("invalid WGM handoff")
	}
	var handoff wgmHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		return governanceSnapshot{}, errors.New("invalid WGM handoff")
	}
	if handoff.SchemaVersion != "1.0" || !validNonPathIdentifier(handoff.TaskID) ||
		!validNonPathIdentifier(handoff.Capability) ||
		(handoff.Risk != "low" && handoff.Risk != "medium" && handoff.Risk != "high") ||
		handoff.TokenBudget < 1 || !validNonPathIdentifiers(handoff.EvidenceReferences) {
		return governanceSnapshot{}, errors.New("incomplete or unsupported WGM handoff")
	}
	return governanceSnapshot{
		source: "WGM handoff", sourceKind: "governance-handoff", sourceSchemaVersion: "1.0",
		taskID: handoff.TaskID, capability: handoff.Capability, risk: handoff.Risk,
		tokenBudget: handoff.TokenBudget, evidenceN: len(handoff.EvidenceReferences),
		available: true, exportable: true,
	}, nil
}

func decodeRouterManifest(data []byte) (governanceSnapshot, error) {
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return governanceSnapshot{}, errors.New("invalid Router manifest")
	}
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
	if manifest.SchemaVersion == "" {
		if !hasExactKeys(document,
			"status", "recommended_alias", "registry_sha256", "reasons",
			"authority_effect", "execution_effect",
		) || !validRouterStatus(manifest.Status) ||
			!validOptionalNonPathIdentifier(manifest.RecommendedAlias) ||
			!validOptionalDigest(manifest.RegistrySHA256) || !validNonPathIdentifiers(manifest.Reasons) {
			return governanceSnapshot{}, errors.New("invalid legacy Router manifest")
		}
		return governanceSnapshot{
			source: "Router manifest", sourceKind: "router-manifest",
			sourceSchemaVersion: "unversioned-0.2", status: manifest.Status,
			alias:   optionalString(manifest.RecommendedAlias),
			reasons: append([]string(nil), manifest.Reasons...), available: true,
		}, nil
	}
	if manifest.SchemaVersion != "1.0" || !hasExactKeys(document,
		"schema_version", "task_id", "capability", "status", "recommended_alias",
		"registry_sha256", "reasons", "authority_effect", "execution_effect",
	) {
		return governanceSnapshot{}, errors.New("unsupported Router manifest version")
	}
	if !validRouterStatus(manifest.Status) || !validOptionalNonPathIdentifier(manifest.TaskID) ||
		!validOptionalNonPathIdentifier(manifest.Capability) ||
		!validOptionalNonPathIdentifier(manifest.RecommendedAlias) ||
		!validOptionalDigest(manifest.RegistrySHA256) || !validNonPathIdentifiers(manifest.Reasons) {
		return governanceSnapshot{}, errors.New("incomplete Router manifest 1.0")
	}
	return governanceSnapshot{
		source: "Router manifest", sourceKind: "router-manifest", sourceSchemaVersion: "1.0",
		taskID: optionalString(manifest.TaskID), capability: optionalString(manifest.Capability),
		status: manifest.Status, alias: optionalString(manifest.RecommendedAlias),
		reasons: append([]string(nil), manifest.Reasons...), available: true, exportable: true,
	}, nil
}

func observationSnapshot(snapshot governanceSnapshot) (observationDocument, error) {
	if !snapshot.available || !snapshot.exportable || snapshot.sourceSchemaVersion != "1.0" {
		return observationDocument{}, errors.New("governance snapshot is not eligible for export")
	}
	if snapshot.sourceKind != "governance-handoff" && snapshot.sourceKind != "router-manifest" {
		return observationDocument{}, errors.New("unsupported observation source")
	}
	var taskID *string
	if snapshot.taskID != "" {
		if safeGovernanceText(snapshot.taskID) != snapshot.taskID {
			return observationDocument{}, errors.New("unsafe task identity")
		}
		value := snapshot.taskID
		taskID = &value
	}
	status := snapshot.status
	if snapshot.sourceKind == "governance-handoff" {
		status = "reviewed_metadata"
	}
	status = safeGovernanceText(status)
	if status == "" {
		return observationDocument{}, errors.New("unsafe observation status")
	}
	summary := governanceLines(snapshot)
	if len(summary) == 0 {
		return observationDocument{}, errors.New("observation summary is empty")
	}
	for _, line := range summary {
		if line == "" || safeGovernanceText(line) != line {
			return observationDocument{}, errors.New("unsafe observation summary")
		}
	}
	return observationDocument{
		SchemaVersion: "1.0", TaskID: taskID, SourceKind: snapshot.sourceKind,
		SourceSchemaVersion: snapshot.sourceSchemaVersion, Status: status, Summary: summary,
		AuthorityEffect: false, ExecutionEffect: false,
	}, nil
}

func hasExactKeys(document map[string]any, keys ...string) bool {
	if len(document) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := document[key]; !ok {
			return false
		}
	}
	return true
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validOptionalNonPathIdentifier(value *string) bool {
	return value == nil || validNonPathIdentifier(*value)
}

func validOptionalDigest(value *string) bool {
	if value == nil {
		return true
	}
	if len(*value) != 64 {
		return false
	}
	for _, char := range *value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func validNonPathIdentifiers(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !validNonPathIdentifier(value) {
			return false
		}
	}
	return true
}

func validNonPathIdentifier(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~/") {
		return false
	}
	return len(value) < 3 || value[1] != ':' ||
		(value[2] != '\\' && value[2] != '/')
}

func validRouterStatus(status string) bool {
	switch status {
	case "invalid_input", "human_review_required", "no_ready_executor", "approval_required", "approved_dry_run":
		return true
	default:
		return false
	}
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
