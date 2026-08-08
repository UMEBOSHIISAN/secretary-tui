package main

import (
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

func TestReadGovernanceWGMHandoff(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"schema_version":"1.0",
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
	if !snapshot.available || snapshot.source != "WGM handoff" || snapshot.taskID != "review-1" || snapshot.risk != "low" || snapshot.evidenceN != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestReadGovernanceRouterManifest(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"status":"approval_required",
		"recommended_alias":"local-review",
		"registry_sha256":"digest-not-rendered",
		"reasons":["manifest_only"],
		"authority_effect":false,
		"execution_effect":false
	}`)

	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.available || snapshot.source != "Router manifest" || snapshot.status != "approval_required" || snapshot.alias != "local-review" || len(snapshot.reasons) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if strings.Contains(strings.Join(governanceLines(snapshot), "\n"), "digest-not-rendered") {
		t.Fatal("raw registry digest must not be rendered")
	}
}

func TestReadGovernanceRejectsUnsafeDocuments(t *testing.T) {
	tests := map[string]string{
		"malformed":        `{"status":`,
		"nested secret":    `{"schema_version":"1.0","task_id":"x","capability":"review","risk":"low","token_budget":1,"evidence_references":["e"],"meta":{"api_key":"never-render"}}`,
		"authority effect": `{"status":"approved_dry_run","authority_effect":true,"execution_effect":false}`,
		"execution effect": `{"status":"approved_dry_run","authority_effect":false,"execution_effect":true}`,
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
	dump, path, err := parseArgs([]string{"--dump", "--governance", "result.json"})
	if err != nil || !dump || path != "result.json" {
		t.Fatalf("unexpected parse result: dump=%v path=%q err=%v", dump, path, err)
	}
	dump, path, err = parseArgs(nil)
	if err != nil || dump || path != "" {
		t.Fatalf("empty args changed defaults: dump=%v path=%q err=%v", dump, path, err)
	}
	if _, _, err := parseArgs([]string{"--governance"}); err == nil {
		t.Fatal("missing governance flag value was accepted")
	}
}

func TestGovernancePanelIsOptionalAndReadOnly(t *testing.T) {
	without := initialModel("").View()
	if strings.Contains(without, "AI governance") {
		t.Fatal("governance panel appeared without an explicit path")
	}

	path := writeGovernanceFixture(t, `{
		"status":"approval_required",
		"recommended_alias":"local-review",
		"registry_sha256":"digest-not-rendered",
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
	if strings.Contains(with, "digest-not-rendered") {
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
