package infra

import (
	"os"
	"path/filepath"
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

func TestInvalidK8sManifestIsReported(t *testing.T) {
	if tools.Resolve(Kubeconform, "") == "" {
		t.Skip("kubeconform not installed")
	}
	root := t.TempDir()
	write(t, root, "deploy/svc.yaml",
		"apiVersion: v1\nkind: Service\nmetadata:\n  name: x\nspec:\n  ports:\n    - port: \"eighty\"\n")
	got := Check(root)
	found := false
	for _, f := range got {
		if strings.Contains(f.Message, "is invalid") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the invalid manifest must be reported: %+v", got)
	}
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
