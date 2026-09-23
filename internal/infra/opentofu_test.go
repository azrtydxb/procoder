package infra

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/tools"
)

func resolveInfraTool(t *testing.T, tool *tools.Tool, bin string) {
	t.Helper()
	old := tool.Resolved
	tool.Resolved = func(string) string { return bin }
	t.Cleanup(func() { tool.Resolved = old })
}

// proved by: TerraformTool always returns Terraform — the OpenTofu cases fail;
// dropping the registry.terraform.io checks hands a Terraform project to an
// installed tofu (#286 reversed); ignoring the config pin fails the pin cases.
func TestTerraformToolSelection(t *testing.T) {
	for _, tc := range []struct {
		name, lock, config                      string
		providers, tfProviders, tofu, terraform bool
		want                                    *tools.Tool
	}{
		{name: "terraform lock beats installed tofu", lock: `provider "registry.terraform.io/hashicorp/aws" {}`, tofu: true, terraform: true, want: Terraform},
		{name: "terraform providers beat installed tofu", tfProviders: true, tofu: true, terraform: true, want: Terraform},
		{name: "tofu init header", lock: "# This file is maintained automatically by \"tofu init\".\n", terraform: true, want: OpenTofu},
		{name: "config pins tofu over terraform evidence", config: "tofu", lock: `provider "registry.terraform.io/hashicorp/aws" {}`, terraform: true, want: OpenTofu},
		{name: "config pins terraform over opentofu evidence", config: "terraform", lock: `provider "registry.opentofu.org/hashicorp/aws" {}`, tofu: true, want: Terraform},
		{name: "config auto detects", config: "auto", lock: `provider "registry.opentofu.org/hashicorp/aws" {}`, terraform: true, want: OpenTofu},
		{name: "lock wins without binary", lock: `provider "registry.opentofu.org/hashicorp/aws" {}`, terraform: true, want: OpenTofu},
		{name: "mixed registries", lock: `provider "registry.terraform.io/hashicorp/aws" {}\nprovider "registry.opentofu.org/hashicorp/random" {}`, terraform: true, want: OpenTofu},
		{name: "installed providers", providers: true, terraform: true, want: OpenTofu},
		{name: "only tofu", tofu: true, want: OpenTofu},
		{name: "both tools", tofu: true, terraform: true, want: OpenTofu},
		{name: "only terraform", terraform: true, want: Terraform},
		{name: "neither tool", want: Terraform},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "infra")
			write(t, root, "infra/main.tf", "locals {}\n")
			if tc.lock != "" {
				write(t, dir, ".terraform.lock.hcl", tc.lock)
			}
			if tc.providers {
				if err := os.MkdirAll(filepath.Join(dir, ".terraform/providers/registry.opentofu.org"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.tfProviders {
				if err := os.MkdirAll(filepath.Join(dir, ".terraform/providers/registry.terraform.io"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.config != "" {
				write(t, root, ".procoder/config.toml", "[infra]\nterraform_binary = \""+tc.config+"\"\n")
			}
			tofuBin, terraformBin := "", ""
			if tc.tofu {
				tofuBin = "tofu"
			}
			if tc.terraform {
				terraformBin = "terraform"
			}
			resolveInfraTool(t, OpenTofu, tofuBin)
			resolveInfraTool(t, Terraform, terraformBin)
			got, err := TerraformTool(root, dir)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

// proved by: discard lock-file read errors — an unreadable selection becomes a normal check.
func TestUnreadableToolchainEvidenceBlocks(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.tf", "locals {}\n")
	if err := os.Mkdir(filepath.Join(root, ".terraform.lock.hcl"), 0755); err != nil {
		t.Fatal(err)
	}
	got := Check(root)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked — read infrastructure lock file:") {
		t.Fatalf("got %+v", got)
	}
}

// proved by: resolve Terraform unconditionally in terraformDir — recorded calls and missing-tool verdicts fail.
func TestOpenTofuCheckUsesSelectedBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixtures; selection is covered on every platform")
	}
	for _, mode := range []string{"success", "invalid", "unformatted", "uninitialised", "missing"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "infra")
			write(t, dir, "main.tf", "locals {}\n")
			write(t, dir, ".terraform.lock.hcl", `provider "registry.opentofu.org/hashicorp/aws" {}`)
			if mode != "uninitialised" {
				if err := os.Mkdir(filepath.Join(dir, ".terraform"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("PROCODER_INFRA_TEST_LOG", log)
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$PROCODER_INFRA_TEST_LOG\"\n"
			if mode == "invalid" {
				script += "if [ \"$1\" = validate ]; then echo invalid-configuration; exit 1; fi\n"
			}
			if mode == "unformatted" {
				script += "if [ \"$1\" = fmt ]; then echo main.tf; exit 3; fi\n"
			}
			script += "exit 0\n"
			bin := write(t, t.TempDir(), "tofu", script)
			if err := os.Chmod(bin, 0755); err != nil {
				t.Fatal(err)
			}
			linter := write(t, t.TempDir(), "tflint", "#!/bin/sh\nexit 0\n")
			if err := os.Chmod(linter, 0755); err != nil {
				t.Fatal(err)
			}
			if mode == "missing" {
				bin = ""
			}
			resolveInfraTool(t, OpenTofu, bin)
			resolveInfraTool(t, Terraform, "must-not-run-terraform")
			resolveInfraTool(t, Tflint, linter)
			got := Check(root)
			if mode == "missing" {
				if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "tofu is not installed") {
					t.Fatalf("got %+v", got)
				}
				if _, err := os.Stat(log); !os.IsNotExist(err) {
					t.Fatalf("unexpected invocation: %v", err)
				}
				return
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := "fmt -check\nvalidate -no-color\n"
			if mode == "uninitialised" {
				wantCalls = "fmt -check\n"
			}
			if string(calls) != wantCalls {
				t.Fatalf("calls %q, want %q", calls, wantCalls)
			}
			if mode == "success" {
				if len(got) != 0 {
					t.Fatalf("got %+v", got)
				}
				return
			}
			want := "tofu validate FAILED"
			blocking := true
			if mode == "uninitialised" {
				want = "tofu NOT validated"
				blocking = false
			}
			if mode == "unformatted" {
				want = "not tofu-formatted"
				blocking = false
			}
			if len(got) != 1 || got[0].Blocking != blocking || !strings.Contains(got[0].Message, want) {
				t.Fatalf("got %+v, want %s blocking=%v", got, want, blocking)
			}
		})
	}
}
