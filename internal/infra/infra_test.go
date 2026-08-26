package infra

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/tools"
)

func write(t *testing.T, root, name, body string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDetectFindsEachKindAndSkipsWorkflows(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Dockerfile", "FROM x\n")
	write(t, root, "infra/main.tf", "resource \"x\" \"y\" {}\n")
	write(t, root, "deploy/svc.yaml", "apiVersion: v1\nkind: Service\n")
	write(t, root, "chart/Chart.yaml", "name: c\n")
	write(t, root, ".github/workflows/ci.yml", "apiVersion: fake\nkind: NotK8s\n")

	inv := Detect(root)
	if len(inv.Dockerfiles) != 1 || len(inv.TfDirs) != 1 ||
		len(inv.K8sFiles) != 1 || len(inv.ChartDirs) != 1 {
		t.Fatalf("detection wrong: %+v", inv)
	}
	if strings.Contains(inv.K8sFiles[0], ".github") {
		t.Fatal("workflows are domain 7, not Kubernetes manifests")
	}
}

func TestNoInfraMeansSilence(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.go", "package main\n")
	if got := Check(root); got != nil {
		t.Fatalf("no infra, no findings: %+v", got)
	}
}

func TestSloppyDockerfileIsReported(t *testing.T) {
	if tools.Resolve(Hadolint, "") == "" {
		t.Skip("hadolint not installed")
	}
	root := t.TempDir()
	write(t, root, "Dockerfile", "FROM ubuntu:latest\nRUN apt-get install curl\n")
	got := Check(root)
	joined := ""
	for _, f := range got {
		joined += f.Message + "\n"
		if f.Blocking {
			t.Fatalf("dockerfile findings report, never block: %+v", f)
		}
	}
	if !strings.Contains(joined, "DL3007") {
		t.Fatalf("hadolint's latest-tag warning expected:\n%s", joined)
	}
}

// stubKubeconform puts a kubeconform on PATH that prints a fixed report.
//
// The real one fetches its schemas from raw.githubusercontent.com, so under
// a full parallel suite run that fetch can fail and Check returns nothing —
// making this test's verdict depend on whether GitHub answered rather than
// on whether procoder reported the manifest (#188). The gate tests stub
// gitleaks, semgrep and golangci-lint for exactly this reason; the infra
// tests did not.
//
// What is under test here is procoder's half: that it parses what the tool
// said. Whether the tool still says it that way is TestRealKubeconform.
func stubKubeconform(t *testing.T, output string) {
	t.Helper()
	bin := t.TempDir()
	name, script := "kubeconform", "#!/bin/sh\ncat <<'REPORT'\n"+output+"\nREPORT\nexit 1\n"
	if runtime.GOOS == "windows" {
		name = "kubeconform.cmd"
		script = "@echo off\r\n"
		for _, line := range strings.Split(output, "\n") {
			script += "echo " + line + "\r\n"
		}
		script += "exit /b 1\r\n"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// An invalid manifest is reported, with the same verdict whether or not
// this machine can reach GitHub.
//
// proved by: the ` is invalid:` filter in kubernetes() changed to
// ` is valid:` — nothing is reported and the test says so.
func TestInvalidK8sManifestIsReported(t *testing.T) {
	root := t.TempDir()
	// A message the real kubeconform would never produce. Asserting it
	// below is what proves the stub shadowed the installed binary rather
	// than the test happening to pass because the real one was reachable.
	stubKubeconform(t, "deploy/svc.yaml - Service x is invalid: STUBBED-VERDICT-NOT-FROM-THE-NETWORK")
	write(t, root, "deploy/svc.yaml",
		"apiVersion: v1\nkind: Service\nmetadata:\n  name: x\nspec:\n  ports:\n    - port: \"eighty\"\n")
	got := Check(root)
	found := false
	for _, f := range got {
		if strings.Contains(f.Message, "STUBBED-VERDICT-NOT-FROM-THE-NETWORK") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the stub's verdict was not what procoder reported — either the finding was lost or the real binary ran: %+v", got)
	}
}

// The stub must not be so loose that it passes whatever procoder does. A
// tool that reports nothing must produce no finding — otherwise the test
// above would pass against an implementation that invented findings.
//
// proved by: kubernetes() made to append a finding unconditionally — this
// test then fails while the one above still passes, which is the pair's
// whole purpose.
func TestAValidManifestProducesNoInvalidFinding(t *testing.T) {
	root := t.TempDir()
	stubKubeconform(t, "deploy/svc.yaml - Service x is valid")
	write(t, root, "deploy/svc.yaml", "apiVersion: v1\nkind: Service\nmetadata:\n  name: x\n")
	for _, f := range Check(root) {
		if strings.Contains(f.Message, "is invalid") {
			t.Fatalf("a finding was reported for a manifest the tool called valid: %+v", f)
		}
	}
}

// The real binary, kept so that "kubeconform's output shape changed" is
// still something the suite can find out — which a stub can never tell you.
//
// Opt-in rather than skipped-when-missing: a test that silently skips is
// one nobody notices has stopped running. PROCODER_TOOL_E2E=1 asks for it,
// and it is not part of the default suite precisely because its verdict
// depends on the network.
func TestRealKubeconformStillSpeaksTheShapeWeParse(t *testing.T) {
	if os.Getenv("PROCODER_TOOL_E2E") != "1" {
		t.Skip("set PROCODER_TOOL_E2E=1 to run the real-tool check (needs network)")
	}
	if tools.Resolve(Kubeconform, "") == "" {
		t.Fatal("PROCODER_TOOL_E2E=1 was set but kubeconform is not installed")
	}
	root := t.TempDir()
	write(t, root, "deploy/svc.yaml",
		"apiVersion: v1\nkind: Service\nmetadata:\n  name: x\nspec:\n  ports:\n    - port: \"eighty\"\n")
	for _, f := range Check(root) {
		if strings.Contains(f.Message, "is invalid") {
			return
		}
	}
	t.Fatal("the real kubeconform no longer produces the ' is invalid:' shape procoder parses")
}

func TestMissingToolsSayNotChecked(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Dockerfile", "FROM x\n")
	write(t, root, "infra/main.tf", "locals {}\n")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	joined := ""
	for _, f := range Check(root) {
		joined += f.Message + "\n"
	}
	for _, tool := range []string{"hadolint", "terraform"} {
		if !strings.Contains(joined, tool) || !strings.Contains(joined, "NOT checked") {
			t.Fatalf("missing %s must read NOT checked:\n%s", tool, joined)
		}
	}
}

func TestUninitialisedTerraformSaysNotValidatedNeverBlocks(t *testing.T) {
	if tools.Resolve(Terraform, "") == "" {
		t.Skip("terraform not installed")
	}
	root := t.TempDir()
	write(t, root, "infra/main.tf", "locals { a = 1 }\n")
	for _, f := range Check(root) {
		if strings.Contains(f.Message, "NOT validated") && f.Blocking {
			t.Fatalf("uninitialised is information, not a block: %+v", f)
		}
	}
}

// "No infrastructure here" and "I could not look" must never read the
// same. A survey that cannot walk the tree reports that it did not survey,
// rather than an empty inventory a caller would take for a clean answer.
func TestUnwalkableTreeIsNotSurveyedRatherThanEmpty(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not deny the walk here")
	}
	root := t.TempDir()
	inner := filepath.Join(root, "tree")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(inner, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(inner, 0o755); err != nil {
			t.Error(err)
		}
	})

	// the walk still completes (an unreadable subdirectory is skipped), so
	// the inventory is honestly empty here; the guarantee under test is that
	// a root that cannot be walked at all is reported, never silently empty
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(root, 0o755); err != nil {
			t.Error(err)
		}
	})
	got := Check(root)
	if len(got) != 1 || !strings.Contains(got[0].Message, "NOT surveyed") {
		t.Fatalf("an unwalkable root must say it was not surveyed, got %v", got)
	}
}
