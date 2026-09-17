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

func TestSetupRequiresReadableMasterBeforeOutputOrWrites(t *testing.T) {
	for _, unreadable := range []bool{false, true} {
		for _, command := range []string{"init", "agents"} {
			root := t.TempDir()
			if unreadable {
				if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			argv := []string{command, "--host=kilo"}
			var out, stderr, apiOut, apiErr bytes.Buffer
			code := run(argv, session{cwd: root, stdout: &out, stderr: &stderr, stdin: strings.NewReader("")})
			apiCode, _ := apiRunner(api.Request{Argv: argv, Cwd: root}, &apiOut, &apiErr)
			if code != 2 || apiCode != code || apiOut.String() != out.String() || apiErr.String() != stderr.String() {
				t.Fatalf("%s unreadable=%v: CLI=%d API=%d: %s", command, unreadable, code, apiCode, out.String())
			}
			if !strings.Contains(out.String(), "cannot read AGENTS.md") || strings.Contains(out.String(), "== write this") {
				t.Fatalf("generated output before validating master: %s", out.String())
			}
			for _, path := range []string{".gitignore", ".procoder", ".kilo"} {
				if _, err := os.Lstat(filepath.Join(root, path)); !os.IsNotExist(err) {
					t.Fatalf("%s mutated %s before validating master: %v", command, path, err)
				}
			}
		}
	}
}
