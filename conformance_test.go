package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestObservationSnapshotFromVersionedRouter(t *testing.T) {
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
	document, err := observationSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	want := observationDocument{
		SchemaVersion:       "1.0",
		TaskID:              stringPointer("demo-review-001"),
		SourceKind:          "router-manifest",
		SourceSchemaVersion: "1.0",
		Status:              "approval_required",
		Summary: []string{
			"source: Router manifest",
			"status: approval_required",
			"candidate: local-review",
			"reasons: manifest_only",
			"authority: none",
			"execution: none",
			"local snapshot",
		},
		AuthorityEffect: false,
		ExecutionEffect: false,
	}
	if !reflect.DeepEqual(document, want) {
		t.Fatalf("unexpected observation:\n got: %#v\nwant: %#v", document, want)
	}
}

func TestObservationSnapshotRejectsUnavailableAndUnversionedSources(t *testing.T) {
	for name, snapshot := range map[string]governanceSnapshot{
		"unavailable": {},
		"legacy router": {
			source: "Router manifest", sourceKind: "router-manifest",
			sourceSchemaVersion: "unversioned-0.2", status: "approval_required",
			available: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := observationSnapshot(snapshot); err == nil {
				t.Fatal("unsafe snapshot was exported")
			}
		})
	}
}

func TestObservationSnapshotSupportsClosedWGMHandoff(t *testing.T) {
	path := writeGovernanceFixture(t, `{
		"schema_version":"1.1",
		"task_id":"demo-review-001",
		"capability":"code-review",
		"risk":"low",
		"token_budget":4000,
		"evidence_references":["evidence:demo-change-v1"]
	}`)
	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := observationSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if document.SourceKind != "governance-handoff" || document.SourceSchemaVersion != "1.1" || document.Status != "reviewed_metadata" || document.TaskID == nil || *document.TaskID != "demo-review-001" || document.AuthorityEffect || document.ExecutionEffect {
		t.Fatalf("unexpected WGM observation: %#v", document)
	}
}

func TestObservationSnapshotBoundsLongWGMHandoffSummaryLines(t *testing.T) {
	taskID := strings.Repeat("a", 256)
	capability := strings.Repeat("b", 256)
	path := writeGovernanceFixture(t, fmt.Sprintf(`{
		"schema_version":"1.1",
		"task_id":%q,
		"capability":%q,
		"risk":"low",
		"token_budget":4000,
		"evidence_references":["evidence:demo-change-v1"]
	}`, taskID, capability))
	snapshot, err := readGovernance(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := observationSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if document.TaskID == nil || *document.TaskID != taskID {
		t.Fatal("task identity was not preserved")
	}
	for _, line := range document.Summary {
		if len([]rune(line)) > 120 {
			t.Fatalf("summary line exceeds 120 characters: %q", line)
		}
		for _, r := range line {
			if r < 0x20 || r > 0x7e {
				t.Fatalf("summary line is not printable ASCII: %q", line)
			}
		}
	}
}

func TestObservationSnapshotSanitizesControls(t *testing.T) {
	snapshot := governanceSnapshot{
		source: "Router manifest", sourceKind: "router-manifest", sourceSchemaVersion: "1.0",
		taskID: "demo-review-001", status: "approval_required\x1b[31m",
		reasons: []string{"line1\nline2"}, available: true, exportable: true,
	}
	document, err := observationSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range document.Summary {
		if strings.IndexFunc(line, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			t.Fatalf("control character survived sanitization: %q", line)
		}
	}
}

func TestSnapshotJSONModeIsIsolatedStableAndReadOnly(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "router.json")
	data, err := os.ReadFile("examples/router-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fakeHome := filepath.Join(root, "home")
	script := filepath.Join(fakeHome, "Workspace", "scripts", "llm-seat.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "worker-command-ran")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("LANG", "ja_JP.UTF-8")
	before := directoryEntries(t, root)
	firstDir := filepath.Join(root, "cwd-one")
	secondDir := filepath.Join(root, "cwd-two")
	if err := os.Mkdir(firstDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(secondDir, 0o700); err != nil {
		t.Fatal(err)
	}
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	var firstOut, firstErr bytes.Buffer
	if err := os.Chdir(firstDir); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"--snapshot-json", "--governance", input}, &firstOut, &firstErr); code != 0 {
		t.Fatalf("first run failed: code=%d stderr=%q", code, firstErr.String())
	}
	t.Setenv("LANG", "C")
	var secondOut, secondErr bytes.Buffer
	if err := os.Chdir(secondDir); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"--snapshot-json", "--governance", input}, &secondOut, &secondErr); code != 0 {
		t.Fatalf("second run failed: code=%d stderr=%q", code, secondErr.String())
	}
	if firstOut.String() != secondOut.String() || firstErr.Len() != 0 || secondErr.Len() != 0 {
		t.Fatalf("snapshot output changed with environment:\nfirst=%q\nsecond=%q", firstOut.String(), secondOut.String())
	}
	want, err := os.ReadFile(filepath.Join(originalDir, "examples", "observation-snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if firstOut.String() != string(want) {
		t.Fatalf("public example drifted:\n got: %s\nwant: %s", firstOut.String(), want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("snapshot mode executed llm-seat.sh")
	}
	after := directoryEntries(t, root)
	allowed := append(append([]string(nil), before...), "cwd-one", "cwd-two")
	sort.Strings(allowed)
	if !reflect.DeepEqual(after, allowed) {
		t.Fatalf("snapshot mode changed the filesystem:\n before+cwd=%v\n after=%v", allowed, after)
	}
}

func TestSnapshotJSONErrorsAreFixedPathFreeAndValidateBeforeRead(t *testing.T) {
	privatePath := filepath.Join(t.TempDir(), "private-customer-name.json")
	tests := map[string]struct {
		args    []string
		message string
	}{
		"missing input": {
			args:    []string{"--snapshot-json", "--governance", privatePath},
			message: "snapshot_error: unable to create safe observation\n",
		},
		"conflicting mode": {
			args:    []string{"--snapshot-json", "--dump", "--governance", privatePath},
			message: "argument error: invalid arguments\n",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(test.args, &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 {
				t.Fatalf("unexpected result: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.String() != test.message {
				t.Fatalf("unexpected fixed error: %q", stderr.String())
			}
			if strings.Contains(stderr.String(), privatePath) || strings.Contains(stderr.String(), "private-customer-name") {
				t.Fatalf("error leaked input path: %q", stderr.String())
			}
		})
	}
}

func TestClosedMothershipConformanceManifest(t *testing.T) {
	raw, err := os.ReadFile("suite/mothership-0.2-conformance.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"authority_effect", "example_path", "execution_effect", "protocol_kind", "protocol_version", "repository", "schema_path", "schema_sha256", "schema_version", "suite_release"}
	gotKeys := make([]string, 0, len(manifest))
	for key := range manifest {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("manifest is not closed: %v", gotKeys)
	}
	wantValues := map[string]any{
		"schema_version": "mothership.conformance.v1", "suite_release": "0.2.0",
		"repository": "secretary-tui", "protocol_kind": "observation-snapshot",
		"protocol_version": "1.0", "schema_path": "schemas/observation-snapshot.1.0.schema.json",
		"example_path": "examples/observation-snapshot.json", "authority_effect": false,
		"execution_effect": false,
	}
	for key, want := range wantValues {
		if !reflect.DeepEqual(manifest[key], want) {
			t.Fatalf("manifest[%q]=%#v want %#v", key, manifest[key], want)
		}
	}
	schema, err := os.ReadFile(manifest["schema_path"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var ownerSchema map[string]any
	if err := json.Unmarshal(schema, &ownerSchema); err != nil {
		t.Fatal(err)
	}
	properties := ownerSchema["properties"].(map[string]any)
	taskIDPattern := properties["task_id"].(map[string]any)["oneOf"].([]any)[1].(map[string]any)["pattern"]
	summaryPattern := properties["summary"].(map[string]any)["items"].(map[string]any)["pattern"]
	if taskIDPattern != `^(?![A-Za-z]:)[A-Za-z0-9][A-Za-z0-9._:-]*(?![\s\S])` ||
		summaryPattern != `^[\u0020-\u007e]*(?![\s\S])` {
		t.Fatalf("owner schema lost its portable true-end grammar")
	}
	digest := sha256.Sum256(schema)
	if hex.EncodeToString(digest[:]) != manifest["schema_sha256"] {
		t.Fatal("owner schema digest does not match conformance manifest")
	}
	var example observationDocument
	if data, err := os.ReadFile(manifest["example_path"].(string)); err != nil {
		t.Fatal(err)
	} else {
		var exampleMap map[string]any
		if err := json.Unmarshal(data, &exampleMap); err != nil {
			t.Fatal(err)
		}
		if !hasExactKeys(exampleMap, "schema_version", "task_id", "source_kind", "source_schema_version", "status", "summary", "authority_effect", "execution_effect") {
			t.Fatalf("example is not a closed observation object: %v", exampleMap)
		}
		if err := json.Unmarshal(data, &example); err != nil {
			t.Fatal(err)
		}
	}
	if example.SchemaVersion != "1.0" || example.SourceKind != "router-manifest" || example.SourceSchemaVersion != "1.0" || example.AuthorityEffect || example.ExecutionEffect || len(example.Summary) == 0 {
		t.Fatalf("unsafe or incomplete example: %#v", example)
	}
	for _, line := range example.Summary {
		if line == "" || strings.IndexFunc(line, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			t.Fatalf("unsafe example summary line: %q", line)
		}
	}
}

func directoryEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func stringPointer(value string) *string { return &value }
