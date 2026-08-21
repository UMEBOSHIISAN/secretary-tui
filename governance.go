package main

import (
	"bytes"
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

type decisionCardSnapshot struct {
	schemaVersion         string
	decisionID            string
	taskID                string
	question              string
	recommendation        *string
	reasons               []string
	evidenceRefs          []string
	unknowns              []string
	risk                  string
	authorityRequired     string
	consequenceIfApproved string
	available             bool
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

type decisionCardDocument struct {
	SchemaVersion         string   `json:"schema_version"`
	DecisionID            string   `json:"decision_id"`
	TaskID                string   `json:"task_id"`
	Question              string   `json:"question"`
	Recommendation        *string  `json:"recommendation"`
	Reasons               []string `json:"reasons"`
	EvidenceRefs          []string `json:"evidence_refs"`
	Unknowns              []string `json:"unknowns"`
	Risk                  string   `json:"risk"`
	AuthorityRequired     string   `json:"authority_required"`
	ConsequenceIfApproved string   `json:"consequence_if_approved"`
	AuthorityEffect       bool     `json:"authority_effect"`
	ExecutionEffect       bool     `json:"execution_effect"`
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
	if err := validateUniqueJSONKeys(data); err != nil {
		return governanceSnapshot{}, errors.New("governance snapshot is not valid JSON")
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

func readDecisionCard(path string) (decisionCardSnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return decisionCardSnapshot{}, errors.New("decision card is not readable")
	}
	if !info.Mode().IsRegular() || info.Size() > maxGovernanceBytes {
		return decisionCardSnapshot{}, errors.New("decision card must be a file no larger than 1 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return decisionCardSnapshot{}, errors.New("decision card is not readable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxGovernanceBytes+1))
	if err != nil {
		return decisionCardSnapshot{}, errors.New("decision card is not readable")
	}
	if len(data) > maxGovernanceBytes {
		return decisionCardSnapshot{}, errors.New("decision card must be no larger than 1 MiB")
	}
	if err := validateUniqueJSONKeys(data); err != nil {
		return decisionCardSnapshot{}, errors.New("decision card is not valid JSON")
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return decisionCardSnapshot{}, errors.New("decision card is not valid JSON")
	}
	if containsSecretGovernanceKey(document) {
		return decisionCardSnapshot{}, errors.New("decision card contains a forbidden secret-bearing key")
	}
	if !hasExactKeys(document,
		"schema_version", "decision_id", "task_id", "question", "recommendation",
		"reasons", "evidence_refs", "unknowns", "risk", "authority_required",
		"consequence_if_approved", "authority_effect", "execution_effect",
	) || !decisionCardStringArray(document, "reasons") ||
		!decisionCardStringArray(document, "evidence_refs") ||
		!decisionCardStringArray(document, "unknowns") {
		return decisionCardSnapshot{}, errors.New("invalid decision card")
	}
	if authorityEffect, ok := document["authority_effect"].(bool); !ok || authorityEffect {
		return decisionCardSnapshot{}, errors.New("invalid decision card effects")
	}
	if executionEffect, ok := document["execution_effect"].(bool); !ok || executionEffect {
		return decisionCardSnapshot{}, errors.New("invalid decision card effects")
	}
	var card decisionCardDocument
	if err := json.Unmarshal(data, &card); err != nil {
		return decisionCardSnapshot{}, errors.New("invalid decision card")
	}
	if card.SchemaVersion != "decision-card.v0" || !validNonPathIdentifier(card.DecisionID) ||
		!validNonPathIdentifier(card.TaskID) || card.Question == "" ||
		!validDecisionCardStrings(card.Reasons) || !validDecisionCardEvidence(card.EvidenceRefs) ||
		!validDecisionCardStrings(card.Unknowns) || !validDecisionCardRisk(card.Risk) ||
		card.AuthorityRequired != "human" || card.ConsequenceIfApproved == "" ||
		len([]rune(card.ConsequenceIfApproved)) > 1024 {
		return decisionCardSnapshot{}, errors.New("invalid decision card")
	}
	return decisionCardSnapshot{
		schemaVersion: card.SchemaVersion, decisionID: card.DecisionID, taskID: card.TaskID,
		question: card.Question, recommendation: card.Recommendation,
		reasons:      append([]string(nil), card.Reasons...),
		evidenceRefs: append([]string(nil), card.EvidenceRefs...),
		unknowns:     append([]string(nil), card.Unknowns...), risk: card.Risk,
		authorityRequired: card.AuthorityRequired, consequenceIfApproved: card.ConsequenceIfApproved,
		available: true,
	}, nil
}

func decisionCardStringArray(document map[string]any, key string) bool {
	values, ok := document[key].([]any)
	if !ok {
		return false
	}
	for _, value := range values {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}

func validDecisionCardStrings(values []string) bool {
	for _, value := range values {
		if value == "" {
			return false
		}
	}
	return true
}

func validDecisionCardEvidence(values []string) bool {
	for _, value := range values {
		if !validNonPathIdentifier(value) {
			return false
		}
	}
	return true
}

func validDecisionCardRisk(risk string) bool {
	switch risk {
	case "low", "medium", "high", "unknown":
		return true
	default:
		return false
	}
}

func validateUniqueJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("multiple JSON values")
	}
	return nil
}

func consumeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return errors.New("duplicate object key")
			}
			seen[key] = struct{}{}
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("invalid object closing delimiter")
		}
	case '[':
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("invalid array closing delimiter")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
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
	if (handoff.SchemaVersion != "1.0" && handoff.SchemaVersion != "1.1") || !validNonPathIdentifier(handoff.TaskID) ||
		!validNonPathIdentifier(handoff.Capability) ||
		(handoff.Risk != "low" && handoff.Risk != "medium" && handoff.Risk != "high") ||
		handoff.TokenBudget < 1 || !validNonPathIdentifiers(handoff.EvidenceReferences) {
		return governanceSnapshot{}, errors.New("incomplete or unsupported WGM handoff")
	}
	return governanceSnapshot{
		source: "WGM handoff", sourceKind: "governance-handoff", sourceSchemaVersion: handoff.SchemaVersion,
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
	if !snapshot.available || !snapshot.exportable || !validObservationSourceVersion(snapshot) {
		return observationDocument{}, errors.New("governance snapshot is not eligible for export")
	}
	if snapshot.sourceKind != "governance-handoff" && snapshot.sourceKind != "router-manifest" {
		return observationDocument{}, errors.New("unsupported observation source")
	}
	var taskID *string
	if snapshot.taskID != "" {
		if !validNonPathIdentifier(snapshot.taskID) {
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

func validObservationSourceVersion(snapshot governanceSnapshot) bool {
	if snapshot.sourceKind == "governance-handoff" {
		return snapshot.sourceSchemaVersion == "1.0" || snapshot.sourceSchemaVersion == "1.1"
	}
	return snapshot.sourceKind == "router-manifest" && snapshot.sourceSchemaVersion == "1.0"
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
	if value == "" || !isASCIIAlphanumeric(value[0]) {
		return false
	}
	if len(value) >= 2 && isASCIIAlpha(value[0]) && value[1] == ':' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !isASCIIAlphanumeric(character) && !strings.ContainsRune("._:-", rune(character)) {
			return false
		}
	}
	return true
}

func isASCIIAlpha(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func isASCIIAlphanumeric(character byte) bool {
	return isASCIIAlpha(character) || (character >= '0' && character <= '9')
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
	lines := []string{safeGovernanceText("source: " + snapshot.source)}
	if snapshot.source == "WGM handoff" {
		lines = append(lines,
			safeGovernanceText("task: "+snapshot.taskID),
			safeGovernanceText("capability: "+snapshot.capability),
			safeGovernanceText("risk: "+snapshot.risk),
			fmt.Sprintf("budget: %d tokens", snapshot.tokenBudget),
			fmt.Sprintf("evidence: %d refs", snapshot.evidenceN),
		)
	} else {
		lines = append(lines, safeGovernanceText("status: "+snapshot.status))
		if snapshot.alias != "" {
			lines = append(lines, safeGovernanceText("candidate: "+snapshot.alias))
		}
		if len(snapshot.reasons) > 0 {
			lines = append(lines, safeGovernanceText("reasons: "+strings.Join(snapshot.reasons, ", ")))
		}
	}
	return append(lines, "authority: none", "execution: none", "local snapshot")
}

func decisionCardLines(card decisionCardSnapshot) []string {
	if !card.available {
		return []string{"(decision card unavailable)"}
	}
	recommendation := "(none)"
	if card.recommendation != nil {
		recommendation = safeGovernanceText(*card.recommendation)
	}
	return []string{
		safeGovernanceText("decision_id: " + card.decisionID),
		safeGovernanceText("task: " + card.taskID),
		safeGovernanceText("question: " + card.question),
		safeGovernanceText("recommendation: " + recommendation),
		safeDecisionCardList("reasons", card.reasons),
		safeDecisionCardList("evidence", card.evidenceRefs),
		safeDecisionCardList("unknowns", card.unknowns),
		safeGovernanceText("risk: " + card.risk),
		safeGovernanceText("authority required: " + card.authorityRequired),
		safeGovernanceText("if approved: " + card.consequenceIfApproved),
		"authority: none", "execution: none", "read-only card",
	}
}

func safeDecisionCardList(label string, values []string) string {
	if len(values) == 0 {
		return label + ": (none)"
	}
	safeValues := make([]string, 0, len(values))
	for _, value := range values {
		safeValues = append(safeValues, safeGovernanceText(value))
	}
	return safeGovernanceText(label + ": " + strings.Join(safeValues, ", "))
}

func safeGovernanceText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, value)
	const limit = 120
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit-3]) + "..."
	}
	return value
}
