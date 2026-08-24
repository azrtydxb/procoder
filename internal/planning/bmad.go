// Package planning reads the planning artifacts a repository keeps, from
// wherever that repository keeps them.
//
// Procoder's governance never needed to know where a plan came from. It
// asks whether there is one, whether it is complete, and whether a story
// is done — and a repository planning in BMad Method can answer all three
// as readily as one planning in .procoder/. This package is the seam that
// lets it: `[planning] method = "bmad"` moves where those answers are
// read from, and moves nothing else. The gate, the suite, the release
// controller and the rest see the same tree and reach the same verdict
// either way.
//
// Nothing here invokes BMad, spawns its skills, or reaches the network.
// It reads files on disk, because the gate runs on every commit.
// Procoder also never writes into BMad's directories: BMad owns what it
// wrote, and a tool that edits the artifacts it judges has no standing to
// judge them.
package planning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"procoder/internal/gitx"
)

// InstallDir is BMad's own marker directory, created by its installer.
const InstallDir = "_bmad"

// defaultOutput is BMad's default output_folder. A repository that chose
// another is read from its config rather than assumed — see OutputFolder.
const defaultOutput = "_bmad-output"

// Status is one entry in BMad's development_status map.
type Status struct {
	Key   string
	Value string
}

// Known are the statuses BMad's own sprint-status template documents. A
// status outside this set is reported BY NAME rather than mapped onto the
// nearest procoder equivalent: deciding that "blocked" is close enough to
// "open" is how a status machine quietly loses a state, and the report
// then misrepresents work without anything saying so.
var Known = map[string]bool{
	"backlog": true, "ready-for-dev": true, "in-progress": true,
	"review": true, "done": true, "optional": true,
}

// Installed reports whether a BMad installation is present in root.
func Installed(root string) bool {
	info, err := os.Stat(filepath.Join(root, InstallDir))
	return err == nil && info.IsDir()
}

// OutputFolder is where BMad writes, read from its own config rather than
// assumed. A repository that answered its installer's prompt with
// something other than the default keeps that answer here, and procoder
// reporting on a directory the repository is not using would be worse than
// reporting nothing.
func OutputFolder(root string) (string, *gitx.Finding) {
	path := filepath.Join(root, InstallDir, "config.toml")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultOutput, nil
	}
	if err != nil {
		return "", &gitx.Finding{Blocking: true, File: path,
			Message: fmt.Sprintf("planning config NOT read — %v; procoder will not guess where the artifacts live (planning)", err)}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != "output_folder" {
			continue
		}
		if val := scalar(v); val != "" {
			return val, nil
		}
	}
	return defaultOutput, nil
}

// StatusFile is where the sprint state lives, given the output folder.
func StatusFile(root, output string) string {
	return filepath.Join(root, output, "implementation-artifacts", "sprint-status.yaml")
}

// SprintStatus reads BMad's development_status map.
//
// Three answers, and they are deliberately distinct. A file that is not
// there is a repository that has planned nothing yet, which is ordinary.
// A file that cannot be read or parsed is a check that did not run, which
// blocks. A file that parses is the sprint, whatever its statuses say.
func SprintStatus(path string) ([]Status, []gitx.Finding) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []gitx.Finding{{Blocking: true, File: path,
			Message: fmt.Sprintf("sprint status NOT read — %v (planning)", err)}}
	}

	entries, ok := parseDevelopmentStatus(string(raw))
	if !ok {
		return nil, []gitx.Finding{{Blocking: true, File: path,
			Message: "sprint status NOT parsed — no development_status block; procoder will not report a sprint it could not read (planning)"}}
	}

	var out []Status
	var findings []gitx.Finding
	for _, e := range entries {
		out = append(out, e)
		if !Known[e.Value] {
			findings = append(findings, gitx.Finding{File: path,
				Message: fmt.Sprintf("%s: status %q is not one procoder knows — reporting it by name rather than guessing which of %s it means (planning)",
					e.Key, e.Value, "backlog, ready-for-dev, in-progress, review, done")})
		}
	}
	return out, findings
}

// parseDevelopmentStatus pulls the development_status mapping out of the
// status file. Written by hand rather than with a YAML dependency: the
// block is a flat map of scalars, procoder adds no dependency for what a
// few lines cover, and a parser that accepts less than YAML does cannot
// silently accept something BMad would reject.
//
// ok is false when the block is absent — which is a file procoder cannot
// read as a sprint, distinct from a sprint with nothing in it.
func parseDevelopmentStatus(src string) (entries []Status, ok bool) {
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// A key at column zero ends the block and starts another.
			inBlock = strings.HasPrefix(trimmed, "development_status:")
			ok = ok || inBlock
			continue
		}
		if !inBlock {
			continue
		}
		k, v, cut := strings.Cut(trimmed, ":")
		if !cut {
			continue
		}
		value := scalar(v)
		if value == "" {
			continue
		}
		entries = append(entries, Status{Key: strings.TrimSpace(k), Value: value})
	}
	return entries, ok
}

// Check is the planning domain's gate leg: it answers whether the method
// the repository chose can actually be honoured.
//
// A repository that set "bmad" and has no BMad installed gets a blocking
// finding naming both. Falling back to procoder's own chain would leave
// it believing BMad governs its planning while procoder quietly governed
// it instead, and the first they would learn of it is a report that does
// not match the artifacts on disk.
func Check(root, method string) []gitx.Finding {
	if method != "bmad" {
		return nil
	}
	if !Installed(root) {
		return []gitx.Finding{{Blocking: true, File: filepath.Join(root, InstallDir),
			Message: "[planning] method = \"bmad\" but no BMad installation is here — procoder did NOT fall back to its own chain, because a repository that chose one methodology must not be governed by the other without being told. Install it, or set the method back to \"procoder\" (planning)"}}
	}
	output, problem := OutputFolder(root)
	if problem != nil {
		return []gitx.Finding{*problem}
	}
	_, findings := SprintStatus(StatusFile(root, output))
	return findings
}

// Report renders sprint state for `procoder status`, in the same shape
// the procoder-native report uses: one line for the sprint, then the work
// that is not finished. A repository reads its own state, not a
// translation of it.
//
// The lines carry the `sprint:` prefix the native report uses rather than
// a prefix of their own. That prefix is an invariant — the report is
// searched by it, in this repository's own tests among other places — and
// a repository that switched methodology would otherwise have no sprint
// line at all as far as any reader looking for one is concerned. Which
// methodology produced it goes in the content, where it informs without
// hiding the line.
func Report(root, method string) []string {
	if method != "bmad" {
		return nil
	}
	if !Installed(root) {
		return []string{"sprint: unknown — no BMad installation found, and this repository plans in bmad"}
	}
	output, problem := OutputFolder(root)
	if problem != nil {
		return []string{"sprint: unknown — " + problem.Message}
	}
	path := StatusFile(root, output)
	entries, findings := SprintStatus(path)
	for _, f := range findings {
		if f.Blocking {
			return []string{"sprint: unknown — " + f.Message}
		}
	}
	if len(entries) == 0 {
		return []string{"sprint: none — no planning artifacts yet (bmad)"}
	}

	done, open := 0, []Status{}
	for _, e := range entries {
		if e.Value == "done" {
			done++
			continue
		}
		// "optional" is a retrospective that may be skipped: counted as
		// neither done nor outstanding, because listing it as work would
		// make every epic look permanently unfinished.
		if e.Value == "optional" {
			continue
		}
		open = append(open, e)
	}
	out := []string{fmt.Sprintf("sprint: %d done, %d open (bmad)", done, len(open))}
	for i, e := range open {
		if i == openCap {
			out = append(out, fmt.Sprintf("  … %d more open — read %s", len(open)-openCap,
				filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))))
			break
		}
		out = append(out, "  open: "+e.Key+" — "+e.Value)
	}
	return out
}

// openCap is how many outstanding entries the report names before it
// starts counting: enough to see what the sprint is about, short enough
// that a large backlog does not bury the rest of the report.
const openCap = 5

// Version is the installed BMad version, read from the manifest its own
// installer writes. "present (version unknown)" where the installation is
// there but does not say — the same wording doctor uses for a tool whose
// --version it could not read, because it is the same situation.
func Version(root string) string {
	for _, name := range []string{"manifest.yaml", "config.toml", "manifest.json"} {
		raw, err := os.ReadFile(filepath.Join(root, InstallDir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				k, v, ok = strings.Cut(line, "=")
			}
			if !ok {
				continue
			}
			key := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(k), "-"))
			if key != "version" && key != "bmad_version" {
				continue
			}
			if val := scalar(v); val != "" {
				return val
			}
		}
	}
	return "present (version unknown)"
}

// scalar reads the value half of a `key: value` or `key = value` line as
// the format's own reader would: an unquoted `#` starts a comment, one
// inside quotes does not, and the surrounding quotes come off last.
//
// Written because getting this wrong is silent in both directions. A
// TOML `output_folder = "planning" # where we put it` parsed naively
// yields a directory that does not exist, so a repository mid-sprint
// reads as one that has planned nothing. A YAML `1-1: done # tuesday`
// yields a status nothing recognises, so a finished story is counted
// open AND reported as an unknown status. Both look like answers.
func scalar(v string) string {
	v = strings.TrimSpace(v)
	quote := byte(0)
	for i := 0; i < len(v); i++ {
		switch {
		case quote != 0:
			if v[i] == quote {
				quote = 0
			}
		case v[i] == '"' || v[i] == '\'':
			quote = v[i]
		case v[i] == '#':
			v = strings.TrimSpace(v[:i])
			i = len(v) // stop: everything after is a comment
		}
	}
	return strings.Trim(v, `"'`)
}
