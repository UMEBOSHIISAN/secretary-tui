package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGovernanceFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "governance.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var unsafeGovernanceIdentifiers = []string{
	"/Users/example/private.json",
	"~/private.json",
	`C:\Users\example\private.json`,
	`\\server\share\private.json`,
	`\\?\C:\private.json`,
	`~\private.json`,
	"../private.json",
	"private/path.json",
	"private\nvalue",
	"private\n",
	"private\x7fvalue",
	"private\u0085value",
	"private\u009bvalue",
	"private\u2028value",
	"C:private.json",
	"日本語",
}

func TestReadGovernanceWGMHandoff(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"schema_version":"1.1",
		"task_id":"review-1",
		"capability":"code-review",
		"risk":"low",
		"token_budget":4000,
		"evidence_references":["evidence:design-v1"]
	}`)

	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.available || snapshot.source != "WGM handoff" || snapshot.sourceSchemaVersion != "1.1" || snapshot.taskID != "review-1" || snapshot.risk != "low" || snapshot.evidenceN != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestReadGovernanceRouterManifest(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"schema_version":"1.0",
		"task_id":"demo-review-001",
		"capability":"code-review",
		"status":"approval_required",
		"recommended_alias":"local-review",
		"registry_sha256":"0000000000000000000000000000000000000000000000000000000000000000",
		"reasons":["manifest_only"],
		"authority_effect":false,
		"execution_effect":false
	}`)

	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.available || !snapshot.exportable || snapshot.sourceKind != "router-manifest" || snapshot.sourceSchemaVersion != "1.0" || snapshot.taskID != "demo-review-001" || snapshot.status != "approval_required" || snapshot.alias != "local-review" || len(snapshot.reasons) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if strings.Contains(strings.Join(governanceLines(snapshot), "\n"), "000000000000") {
		t.Fatal("raw registry digest must not be rendered")
	}
}

func TestLegacyRouterManifestIsDisplayOnly(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"status":"approval_required",
		"recommended_alias":"local-review",
		"registry_sha256":"0000000000000000000000000000000000000000000000000000000000000000",
		"reasons":["manifest_only"],
		"authority_effect":false,
		"execution_effect":false
	}`)
	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.exportable || snapshot.sourceSchemaVersion != "unversioned-0.2" {
		t.Fatalf("legacy manifest became exportable: %#v", snapshot)
	}
	if _, err := observationSnapshot(snapshot); err == nil {
		t.Fatal("legacy manifest was exported as observation-snapshot 1.0")
	}
}

func TestReadGovernanceRecognizesLegacyWGM10WithConsumerSafety(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"schema_version":"1.0",
		"task_id":"review-1",
		"capability":"code-review",
		"risk":"low",
		"token_budget":4000,
		"evidence_references":["evidence:design-v1"]
	}`)
	snapshot, err := readGovernance(path)
	if err != nil || !snapshot.available || snapshot.sourceSchemaVersion != "1.0" {
		t.Fatalf("legacy WGM 1.0 was not safely recognized: snapshot=%#v err=%v", snapshot, err)
	}
}

func TestReadGovernanceRejectsUnsafeDocuments(t *testing.T) {
	tests := map[string]string{
		"malformed":                  `{"status":`,
		"nested secret":              `{"schema_version":"1.0","task_id":"x","capability":"review","risk":"low","token_budget":1,"evidence_references":["e"],"meta":{"api_key":"never-render"}}`,
		"authority effect":           `{"status":"approved_dry_run","authority_effect":true,"execution_effect":false}`,
		"duplicate authority effect": `{"schema_version":"1.0","task_id":"x","capability":"review","status":"approval_required","recommended_alias":null,"registry_sha256":null,"reasons":["manifest_only"],"authority_effect":true,"authority_effect":false,"execution_effect":false}`,
		"execution effect":           `{"status":"approved_dry_run","authority_effect":false,"execution_effect":true}`,
		"wrong router version":       `{"schema_version":"2.0","task_id":"x","capability":"review","status":"approval_required","recommended_alias":null,"registry_sha256":null,"reasons":["manifest_only"],"authority_effect":false,"execution_effect":false}`,
		"router extra field":         `{"schema_version":"1.0","task_id":"x","capability":"review","status":"approval_required","recommended_alias":null,"registry_sha256":null,"reasons":["manifest_only"],"authority_effect":false,"execution_effect":false,"approved":true}`,
		"bad registry digest":        `{"schema_version":"1.0","task_id":"x","capability":"review","status":"approval_required","recommended_alias":null,"registry_sha256":"not-a-digest","reasons":["manifest_only"],"authority_effect":false,"execution_effect":false}`,
		"bad WGM risk":               `{"schema_version":"1.0","task_id":"x","capability":"review","risk":"critical","token_budget":1,"evidence_references":["e"]}`,
		"empty WGM evidence":         `{"schema_version":"1.0","task_id":"x","capability":"review","risk":"low","token_budget":1,"evidence_references":[""]}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeGovernanceFixture(t, content)
			snapshot, err := readGovernance(path)
			if err == nil || snapshot.available {
				t.Fatalf("unsafe document accepted: snapshot=%#v err=%v", snapshot, err)
			}
			if strings.Contains(err.Error(), "never-render") {
				t.Fatal("error leaked a secret value")
			}
		})
	}
}

func TestReadGovernanceRejectsPathBearingWGMIdentifiers(t *testing.T) {
	base := `{
		"schema_version":"1.1",
		"task_id":"review-1",
		"capability":"code-review",
		"risk":"low",
		"token_budget":4000,
		"evidence_references":["evidence:design-v1"]
	}`
	var handoff map[string]any
	if err := json.Unmarshal([]byte(base), &handoff); err != nil {
		t.Fatal(err)
	}
	for _, privatePath := range unsafeGovernanceIdentifiers {
		for _, field := range []string{"task_id", "capability"} {
			t.Run(field+privatePath, func(t *testing.T) {
				changed := maps.Clone(handoff)
				changed[field] = privatePath
				data, err := json.Marshal(changed)
				if err != nil {
					t.Fatal(err)
				}
				path := writeGovernanceFixture(t, string(data))
				if snapshot, err := readGovernance(path); err == nil || snapshot.available {
					t.Fatalf("path-bearing WGM field accepted: snapshot=%#v err=%v", snapshot, err)
				}
			})
		}
		t.Run("evidence"+privatePath, func(t *testing.T) {
			changed := maps.Clone(handoff)
			changed["evidence_references"] = []string{privatePath}
			data, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			path := writeGovernanceFixture(t, string(data))
			if snapshot, err := readGovernance(path); err == nil || snapshot.available {
				t.Fatalf("path-bearing WGM evidence accepted: snapshot=%#v err=%v", snapshot, err)
			}
		})
	}
}

func TestReadGovernanceRejectsPathBearingRouterMetadata(t *testing.T) {
	base := map[string]any{
		"schema_version": "1.0", "task_id": "review-1", "capability": "code-review",
		"status": "approval_required", "recommended_alias": "local-review",
		"registry_sha256": strings.Repeat("0", 64), "reasons": []string{"manifest_only"},
		"authority_effect": false, "execution_effect": false,
	}
	for _, privatePath := range unsafeGovernanceIdentifiers {
		for _, field := range []string{"task_id", "capability", "recommended_alias"} {
			t.Run(field+privatePath, func(t *testing.T) {
				changed := maps.Clone(base)
				changed[field] = privatePath
				data, err := json.Marshal(changed)
				if err != nil {
					t.Fatal(err)
				}
				path := writeGovernanceFixture(t, string(data))
				if snapshot, err := readGovernance(path); err == nil || snapshot.available {
					t.Fatalf("path-bearing Router field accepted: snapshot=%#v err=%v", snapshot, err)
				}
			})
		}
		t.Run("reasons"+privatePath, func(t *testing.T) {
			changed := maps.Clone(base)
			changed["reasons"] = []string{privatePath}
			data, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			path := writeGovernanceFixture(t, string(data))
			if snapshot, err := readGovernance(path); err == nil || snapshot.available {
				t.Fatalf("path-bearing Router reason accepted: snapshot=%#v err=%v", snapshot, err)
			}
		})
	}
}

func TestReadGovernanceRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(path, make([]byte, maxGovernanceBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readGovernance(path); err == nil {
		t.Fatal("oversized governance file was accepted")
	}
}

func TestReadGovernanceDoesNotLeakSourcePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-project-name.json")
	_, err := readGovernance(path)
	if err == nil {
		t.Fatal("missing file was accepted")
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "private-project-name") {
		t.Fatalf("error leaked source path: %v", err)
	}
}

func TestParseArgs(t *testing.T) {
	options, err := parseOptions([]string{"--dump", "--governance", "result.json"})
	if err != nil || !options.dump || options.snapshotJSON || options.governancePath != "result.json" {
		t.Fatalf("unexpected parse result: options=%#v err=%v", options, err)
	}
	options, err = parseOptions(nil)
	if err != nil || options.dump || options.snapshotJSON || options.governancePath != "" {
		t.Fatalf("empty args changed defaults: options=%#v err=%v", options, err)
	}
	if _, err := parseOptions([]string{"--governance"}); err == nil {
		t.Fatal("missing governance flag value was accepted")
	}
	if _, err := parseOptions([]string{"--snapshot-json"}); err == nil {
		t.Fatal("snapshot mode without governance input was accepted")
	}
	if _, err := parseOptions([]string{"--snapshot-json", "--dump", "--governance", "result.json"}); err == nil {
		t.Fatal("snapshot mode was combined with dashboard dump")
	}
}

func TestGovernancePanelIsOptionalAndReadOnly(t *testing.T) {
	without := initialModel("").View()
	if strings.Contains(without, "AI governance") {
		t.Fatal("governance panel appeared without an explicit path")
	}

	path := writeGovernanceFixture(t, `{
		"schema_version":"1.0",
		"task_id":"demo-review-001",
		"capability":"code-review",
		"status":"approval_required",
		"recommended_alias":"local-review",
		"registry_sha256":"0000000000000000000000000000000000000000000000000000000000000000",
		"reasons":["manifest_only"],
		"authority_effect":false,
		"execution_effect":false
	}`)
	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	with := model{governancePath: path, governance: snapshot}.View()
	for _, expected := range []string{"AI governance", "approval_required", "authority: none", "execution: none"} {
		if !strings.Contains(with, expected) {
			t.Fatalf("panel missing %q:\n%s", expected, with)
		}
	}
	if strings.Contains(with, "000000000000") {
		t.Fatal("panel rendered an unapproved raw field")
	}
}

func TestGovernanceLinesSanitizeTerminalControlCharacters(t *testing.T) {
	snapshot := governanceSnapshot{
		source: "Router manifest", status: "approval_required\x1b[31m",
		reasons: []string{"line1\nline2"}, available: true,
	}
	output := strings.Join(governanceLines(snapshot), "\n")
	if strings.Contains(output, "\x1b") || strings.Contains(output, "line1\nline2") {
		t.Fatalf("terminal control character was not sanitized: %q", output)
	}
}
