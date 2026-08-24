// Package infra is domain 8: DevOps/IaaS/CaaS hygiene — Dockerfiles,
// Terraform, Kubernetes manifests, and Helm charts, each held to its
// canonical tool, and each only when its files actually exist. Everything
// reports except a failing `terraform validate`, which is objectively
// broken infrastructure code and blocks.
package infra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"procoder/internal/gitx"
	"procoder/internal/textutil"
	"procoder/internal/tools"
)

// Hadolint lints Dockerfiles.
var Hadolint = &tools.Tool{
	Name:        "hadolint",
	Install:     "brew install hadolint",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "hadolint"}},
	},
}

// Terraform validates and formats infrastructure code.
var Terraform = &tools.Tool{
	Name:        "terraform",
	Install:     "brew install terraform",
	VersionArgs: []string{"version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "terraform"}},
	},
}

// Tflint lints Terraform beyond validation.
var Tflint = &tools.Tool{
	Name:        "tflint",
	Install:     "brew install tflint",
	VersionArgs: []string{"--version"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "tflint"}},
	},
}

// Kubeconform validates Kubernetes manifests against their schemas.
var Kubeconform = &tools.Tool{
	Name:        "kubeconform",
	Install:     "brew install kubeconform",
	VersionArgs: []string{"-v"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "kubeconform"}},
	},
}

// Helm lints charts.
var Helm = &tools.Tool{
	Name:        "helm",
	Install:     "brew install helm",
	VersionArgs: []string{"version", "--short"},
	InstallVia: []tools.InstallCandidate{
		{Manager: "brew", Args: []string{"install", "helm"}},
	},
}

const infraTimeout = 120 * time.Second

// Inventory is what the repository actually contains — each check runs only
// when its files exist.
type Inventory struct {
	Dockerfiles []string
	TfDirs      []string
	K8sFiles    []string
	ChartDirs   []string
	// WalkErr records a survey that could not complete. "No infrastructure
	// here" and "I could not look" must never read the same.
	WalkErr error
}

// Empty reports whether there is any infrastructure to check at all.
func (v Inventory) Empty() bool {
	return len(v.Dockerfiles) == 0 && len(v.TfDirs) == 0 &&
		len(v.K8sFiles) == 0 && len(v.ChartDirs) == 0
}

var k8sKindRe = regexp.MustCompile(`(?m)^kind:\s*\S+`)
var k8sAPIRe = regexp.MustCompile(`(?m)^apiVersion:\s*\S+`)

// Detect walks the repository for infrastructure files.
func Detect(root string) Inventory {
	var inv Inventory
	tfDirs := map[string]bool{}
	skip := map[string]bool{".git": true, "node_modules": true, "vendor": true,
		"dist": true, ".claude": true, ".terraform": true}
	inv.WalkErr = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		// one unreadable directory deeper in is skipped, not fatal — a
		// survey that stops at the first bad entry answers less than one
		// that continues. The ROOT is different: if that cannot be read
		// there is no survey at all, and "no infra files" must never read
		// the same as "could not look".
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		switch {
		case name == "Dockerfile" || strings.HasPrefix(name, "Dockerfile."):
			inv.Dockerfiles = append(inv.Dockerfiles, path)
		case strings.HasSuffix(name, ".tf"):
			tfDirs[filepath.Dir(path)] = true
		case name == "Chart.yaml":
			inv.ChartDirs = append(inv.ChartDirs, filepath.Dir(path))
		case strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"):
			if strings.Contains(filepath.ToSlash(path), ".github/") {
				return nil // workflows belong to domain 7
			}
			if raw, err := os.ReadFile(path); err == nil &&
				k8sKindRe.Match(raw) && k8sAPIRe.Match(raw) {
				inv.K8sFiles = append(inv.K8sFiles, path)
			}
		}
		return nil
	})
	for d := range tfDirs {
		inv.TfDirs = append(inv.TfDirs, d)
	}
	sort.Strings(inv.TfDirs)
	return inv
}

// Check runs every applicable instrument over the inventory.
func Check(root string) []gitx.Finding {
	inv := Detect(root)
	if inv.WalkErr != nil {
		return []gitx.Finding{{Blocking: true,
			Message: "infrastructure NOT surveyed — the tree could not be walked: " + inv.WalkErr.Error()}}
	}
	if inv.Empty() {
		return nil
	}
	var out []gitx.Finding
	if len(inv.Dockerfiles) > 0 {
		out = append(out, dockerfiles(root, inv.Dockerfiles)...)
	}
	for _, dir := range inv.TfDirs {
		out = append(out, terraformDir(root, dir)...)
	}
	if len(inv.K8sFiles) > 0 {
		out = append(out, kubernetes(root, inv.K8sFiles)...)
	}
	for _, dir := range inv.ChartDirs {
		out = append(out, helmChart(root, dir)...)
	}
	return out
}

// hadolintLine: file:line CODE level: message
var hadolintLine = regexp.MustCompile(`^(.+?):(\d+)\s+(\S+\s+(?:error|warning|info|style):.*)$`)

func dockerfiles(root string, files []string) []gitx.Finding {
	bin := tools.Resolve(Hadolint, root)
	if bin == "" {
		return notChecked(files[0], "hadolint")
	}
	raw, code, timedOut := run(root, bin, append([]string{"--no-color"}, files...))
	if timedOut {
		return timeout(files[0], "hadolint")
	}
	var out []gitx.Finding
	for _, line := range strings.Split(raw, "\n") {
		if m := hadolintLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			n, _ := strconv.Atoi(m[2])
			out = append(out, gitx.Finding{File: m[1], Line: n,
				Message: m[3] + " (infra)"})
		}
	}
	out = append(out, failedClean(len(out), code, []int{1}, raw, files[0], "hadolint")...)
	return out
}

func terraformDir(root, dir string) []gitx.Finding {
	bin := tools.Resolve(Terraform, root)
	if bin == "" {
		return notChecked(dir, "terraform")
	}
	var out []gitx.Finding

	// fmt -check lists unformatted files; exit 3 is its findings-exist code
	raw, code, timedOut := run(dir, bin, []string{"fmt", "-check"})
	if timedOut {
		return timeout(dir, "terraform")
	}
	fmtCount := 0
	for _, f := range strings.Split(strings.TrimSpace(raw), "\n") {
		if f != "" && code != 0 {
			out = append(out, gitx.Finding{File: filepath.Join(dir, f),
				Message: "not terraform-formatted — run `terraform fmt` and review (infra)"})
			fmtCount++
		}
	}
	out = append(out, failedClean(fmtCount, code, []int{3}, raw, dir, "terraform fmt")...)

	// validate needs an initialised working dir; validating uninitialised
	// code would fail on providers, not on the code — say so instead
	if _, err := os.Stat(filepath.Join(dir, ".terraform")); err != nil {
		out = append(out, gitx.Finding{File: dir,
			Message: "terraform NOT validated — the directory is not initialised (`terraform init`) (infra)"})
	} else {
		raw, code, timedOut := runExit(dir, bin, []string{"validate", "-no-color"})
		if timedOut {
			return append(out, timeout(dir, "terraform validate")...)
		}
		if code != 0 {
			// objectively broken infrastructure code — the one blocking line
			out = append(out, gitx.Finding{File: dir, Blocking: true,
				Message: "terraform validate FAILED: " + textutil.FirstLine(raw) + " (infra)"})
		}
	}

	if tfl := tools.Resolve(Tflint, root); tfl != "" {
		raw, code, timedOut := run(dir, tfl, []string{"--format", "compact", "--no-color"})
		if timedOut {
			return append(out, timeout(dir, "tflint")...)
		}
		tflCount := 0
		for _, line := range strings.Split(raw, "\n") {
			t := strings.TrimSpace(line)
			if t == "" || !strings.Contains(t, ":") || strings.HasPrefix(t, "Warning") {
				continue
			}
			out = append(out, gitx.Finding{File: dir, Message: t + " (infra)"})
			tflCount++
		}
		out = append(out, failedClean(tflCount, code, []int{2}, raw, dir, "tflint")...)
	} else {
		// Blocking, like every other check that did not run: Terraform
		// unlinted is not Terraform approved, and this domain reaches
		// infrastructure where the cost of a missed finding is highest.
		out = append(out, gitx.Finding{File: dir, Blocking: true,
			Message: "tflint not installed — Terraform lint NOT run; `procoder init` (infra)"})
	}
	return out
}

func kubernetes(root string, files []string) []gitx.Finding {
	bin := tools.Resolve(Kubeconform, root)
	if bin == "" {
		return notChecked(files[0], "kubeconform")
	}
	raw, code, timedOut := run(root, bin, append([]string{"-output", "text"}, files...))
	if timedOut {
		return timeout(files[0], "kubeconform")
	}
	var out []gitx.Finding
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || !strings.Contains(t, " is invalid:") {
			continue
		}
		file := t
		if i := strings.Index(t, " - "); i > 0 {
			file = t[:i]
		}
		out = append(out, gitx.Finding{File: file,
			Message: strings.TrimPrefix(t, file+" - ") + " (infra)"})
	}
	out = append(out, failedClean(len(out), code, []int{1}, raw, files[0], "kubeconform")...)
	return out
}

func helmChart(root, dir string) []gitx.Finding {
	bin := tools.Resolve(Helm, root)
	if bin == "" {
		return notChecked(dir, "helm")
	}
	raw, code, timedOut := runExit(root, bin, []string{"lint", dir})
	if timedOut {
		return timeout(dir, "helm lint")
	}
	var out []gitx.Finding
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[ERROR]") || strings.HasPrefix(t, "[WARNING]") {
			out = append(out, gitx.Finding{File: dir, Message: t + " (infra)"})
		}
	}
	if code != 0 && len(out) == 0 {
		out = append(out, gitx.Finding{File: dir, Blocking: true,
			Message: "helm lint failed without findings — NOT checked: " + textutil.FirstLine(raw) + " (infra)"})
	}
	return out
}

func run(dir, bin string, args []string) (string, int, bool) {
	return runExit(dir, bin, args)
}

func runExit(dir, bin string, args []string) (string, int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), infraTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed tool table, never user input
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", -1, true
	}
	code := 0
	if err != nil {
		code = 1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
	}
	return buf.String(), code, false
}

// failedClean applies the honesty rule: exit codes outside the tool's
// documented findings-exist set, with nothing parsed, mean the check did
// not happen — never clean. okCodes are the exits that legitimately carry
// zero-or-more findings (hadolint 1 = findings, terraform fmt 3 = diffs,
// tflint 2 = issues, kubeconform 1 = invalid found).
func failedClean(parsed int, code int, okCodes []int, raw, file, tool string) []gitx.Finding {
	if parsed > 0 || code == 0 {
		return nil
	}
	for _, ok := range okCodes {
		if code == ok {
			return nil
		}
	}
	return []gitx.Finding{{File: file, Blocking: true,
		Message: fmt.Sprintf("NOT checked — %s failed (exit %d): %s (infra)", tool, code, textutil.FirstLine(raw))}}
}

func notChecked(file, tool string) []gitx.Finding {
	return []gitx.Finding{{File: file, Blocking: true,
		Message: fmt.Sprintf("NOT checked — %s is not installed; run `procoder init` (infra)", tool)}}
}

func timeout(file, tool string) []gitx.Finding {
	return []gitx.Finding{{File: file, Blocking: true,
		Message: fmt.Sprintf("%s gave no answer in %s — NOT checked (infra)", tool, infraTimeout)}}
}
