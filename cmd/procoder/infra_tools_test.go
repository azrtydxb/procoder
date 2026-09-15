package main

import (
	"os"
	"path/filepath"
	"testing"

	"procoder/internal/doctor"
	"procoder/internal/infra"
)

// proved by: restore the unconditional infra.Terraform inventory entry — tofu is absent from doctor/init.
func TestDoctorUsesInfrastructureToolchainPerDirectory(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"tofu-project", "terraform-project"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "main.tf"), []byte("locals {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "tofu-project", ".terraform.lock.hcl"), []byte(`provider "registry.opentofu.org/hashicorp/aws" {}`), 0644); err != nil {
		t.Fatal(err)
	}
	old := infra.OpenTofu.Resolved
	infra.OpenTofu.Resolved = func(string) string { return "" }
	t.Cleanup(func() { infra.OpenTofu.Resolved = old })
	found := map[string]bool{}
	for _, tool := range doctor.ExtraTools(root) {
		found[tool.Name] = true
	}
	for _, name := range []string{"tofu", "terraform", "tflint"} {
		if !found[name] {
			t.Fatalf("missing %s from tool inventory: %v", name, found)
		}
	}
}

// proved by: drop the selection-error inventory entry — doctor no longer reports the unresolved requirement.
func TestDoctorReportsUnreadableToolchain(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.tf"), []byte("locals {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".terraform.lock.hcl"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range doctor.ExtraTools(root) {
		if tool.Name == "infra toolchain: "+root {
			if tool.Resolved == nil || tool.Resolved(root) != "" || len(tool.InstallVia) != 0 {
				t.Fatalf("unresolved evidence must not install a guessed tool: %+v", tool)
			}
			return
		}
	}
	t.Fatal("unreadable toolchain omitted from doctor/init")
}
