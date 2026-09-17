package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/api"
	"procoder/internal/host"
)

func TestSetupCLIAPIAndCallerIsolation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROCODER_HOST", "cursor") // daemon environment must not leak
	for _, name := range []string{"kilo", "cursor"} {
		env := host.Env{"PROCODER_HOST": name}
		var out, stderr bytes.Buffer
		code := run([]string{"agents"}, session{cwd: root, env: env, stdout: &out, stderr: &stderr, stdin: strings.NewReader("")})
		var apiOut, apiErr bytes.Buffer
		apiCode, _ := apiRunner(api.Request{Argv: []string{"agents"}, Cwd: root, Env: env}, &apiOut, &apiErr)
		if code != apiCode || out.String() != apiOut.String() || stderr.String() != apiErr.String() {
			t.Fatal("setup differs across CLI/API")
		}
		if !strings.Contains(out.String(), "."+name+"/rules/") {
			t.Fatal(out.String())
		}
	}
	for _, argv := range [][]string{{"init"}, {"init", "--host", "kilo", "--all"}, {"agents", "--host=unknown"}} {
		var out, stderr bytes.Buffer
		if code := run(argv, session{cwd: root, stdout: &out, stderr: &stderr, stdin: strings.NewReader("")}); code != 2 {
			t.Fatalf("%v exit %d: %s", argv, code, out.String())
		}
		if _, err := os.Stat(filepath.Join(root, ".gitignore")); !os.IsNotExist(err) {
			t.Fatal("invalid setup mutated ignore file")
		}
	}
}

func TestInitUsesHostSelectionWithoutWritingRules(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	run([]string{"init", "--host=kilo"}, session{cwd: root, stdout: &out, stderr: &stderr, stdin: strings.NewReader("")})
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil || !strings.Contains(string(ignore), "/.kilo/\n") || strings.Contains(string(ignore), "/.cursor/") {
		t.Fatalf("selected ignore not written: %s %v", ignore, err)
	}
	if !strings.Contains(out.String(), ".kilo/rules/procoder.md") || strings.Contains(out.String(), ".kilocode/") {
		t.Fatalf("wrong integration output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".kilo")); !os.IsNotExist(err) {
		t.Fatal("init wrote generated files")
	}
	if _, err := os.Stat(filepath.Join(root, ".procoder/hosts.json")); !os.IsNotExist(err) {
		t.Fatal("init wrote host declaration")
	}
}
