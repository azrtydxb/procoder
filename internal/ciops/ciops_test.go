package ciops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sloppyWorkflow = `name: ci
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@93397bea11091df50f3d7e59dc26a7711a8bcfbe
      - run: make
`

func writeWorkflow(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ci.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSloppyWorkflowGetsEveryRule(t *testing.T) {
	root := writeWorkflow(t, sloppyWorkflow)
	got := Check(root, false)
	joined := ""
	for _, f := range got {
		joined += f.Message + "\n"
		if f.Blocking {
			t.Fatalf("report mode must not block: %+v", f)
		}
	}
	for _, want := range []string{
		"mutable ref (actions/checkout@v4)",
		"no timeout-minutes",
		"cancel-in-progress",
		"no workflow mentions tests",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	// the SHA-pinned setup-go must NOT be flagged
	if strings.Contains(joined, "setup-go") {
		t.Fatalf("SHA-pinned action wrongly flagged:\n%s", joined)
	}
}

func TestPinPolicyBlocks(t *testing.T) {
	root := writeWorkflow(t, sloppyWorkflow)
	blocking := 0
	for _, f := range Check(root, true) {
		if f.Blocking {
			blocking++
			if !strings.Contains(f.Message, "mutable ref") {
				t.Fatalf("only unpinned refs block under the policy: %+v", f)
			}
		}
	}
	if blocking != 1 {
		t.Fatalf("want exactly the checkout ref blocking, got %d", blocking)
	}
}

func TestDisciplinedWorkflowIsQuiet(t *testing.T) {
	root := writeWorkflow(t, `name: ci
on: [push]
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
jobs:
  test:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@93397bea11091df50f3d7e59dc26a7711a8bcfbe
      - run: go test ./...
`)
	if got := Check(root, false); len(got) != 0 {
		t.Fatalf("disciplined workflow must be quiet: %+v", got)
	}
}

func TestNoWorkflowsMeansSilence(t *testing.T) {
	if got := Check(t.TempDir(), false); got != nil {
		t.Fatalf("no workflows, no findings: %+v", got)
	}
}
