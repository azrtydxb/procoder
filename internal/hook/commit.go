package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/config"
	"procoder/internal/gate"
	"procoder/internal/host"
	"procoder/internal/tools"
)

// The gate at the commit boundary runs the same work `procoder check` does, so
// it gets the same wall the spec declares. A gate that ran out of time has not
// judged the tree, and "not judged" is never "clean".
const gateDeadline = 120 * time.Second

// preToolPayload is the slice of the host's PreToolUse event this hook needs:
// which tool is about to run, and — for Bash — the command line itself. `cwd`
// is Claude Code's; absent, the process working directory answers.
type preToolPayload struct {
	ToolName  string `json:"tool_name"`
	Cwd       string `json:"cwd"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type decisionOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// PreToolUse intercepts a Bash command on its way to the shell. When it is a
// `git commit` it runs the gate first and hands the verdict back as the host's
// decision envelope; everything else passes without a word.
//
// The exit code is always 0: the decision lives in the payload, and a hook that
// failed the host would break the session rather than gate it.
func PreToolUse(stdin io.Reader, stdout io.Writer) int {
	raw, ok := readAll(stdin)
	if !ok {
		// could not even read the payload — cannot judge, must not wedge
		return allow(stdout, "procoder gate: could not read the hook payload, so the gate did not judge this command — run `procoder check` before you commit.")
	}
	var p preToolPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return allow(stdout, "procoder gate: the hook payload could not be parsed, so the gate did not judge this command — run `procoder check` before you commit.")
	}
	if p.ToolName != "Bash" || p.ToolInput.Command == "" {
		return 0
	}
	c := parseCommand(p.ToolInput.Command)
	if !c.isCommit {
		return 0
	}

	root := commitRoot(p.Cwd, c.dir)
	cfg := config.Load(root)
	if cfg.CommitGate == "off" {
		return 0 // the repository turned the interception off; nothing to say
	}
	if c.noVerify {
		return allow(stdout, "procoder gate: --no-verify — the commit gate was bypassed deliberately. Nothing was checked; this commit is unverified.")
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return 0 // no repository to gate; git will have its own opinion
	}

	if c.message == "" && c.messageFile != "" {
		// The command's own heredoc may be what the file will hold when git
		// reads it, and the shell has not run yet, so the file read is the
		// honest fallback only when the command does not write to it.
		if body, ok := heredocInto(p.ToolInput.Command, c.messageFile); ok {
			c.message = body
		} else {
			c.message = readMessageFile(commitDir(p.Cwd, c.dir), c.messageFile)
		}
	}

	code, findings, timedOut := runGate(root, c.message)
	switch {
	case timedOut:
		// A gate that ran but never finished has NOT judged the tree. Under
		// report the commit proceeds anyway, so say it there too.
		reason := fmt.Sprintf("procoder gate: the gate did not finish within %s, so this commit was NOT checked — run `procoder check` directly.", gateDeadline)
		if cfg.CommitGate == "report" {
			return allow(stdout, reason)
		}
		return deny(stdout, reason)
	case code == 0:
		return 0 // clean — the commit goes through untouched
	}

	blocking := blockingLines(findings)
	reason := fmt.Sprintf("procoder gate: %d blocking finding(s) — fix these before committing:\n%s",
		len(blocking), strings.Join(blocking, "\n"))
	if len(blocking) == 0 {
		// exit non-zero with nothing quotable: the gate could not judge
		reason = "procoder gate: the gate could not judge this commit — run `procoder check` directly and read its output.\n" + strings.TrimSpace(findings)
	}
	if cfg.CommitGate == "report" {
		return allow(stdout, reason+"\n\n(commit_gate = \"report\": the commit proceeds.)")
	}
	return deny(stdout, reason)
}

// runGate runs the gate over the repository's changed files with its own
// buffer, because the caller needs both halves of the answer: the exit code
// and the text of what was found.
func runGate(root, commitMessage string) (code int, output string, timedOut bool) {
	var buf bytes.Buffer
	done := make(chan int, 1)
	go func() {
		defer func() {
			// a panic inside the gate is an inability to verify, not a clean tree
			if r := recover(); r != nil {
				done <- 2
			}
		}()
		done <- gate.RunWith(nil, root, commitMessage, &buf)
	}()
	select {
	case c := <-done:
		return c, buf.String(), false
	case <-time.After(gateDeadline):
		return 2, buf.String(), true
	}
}

// blockingLines keeps the gate lines an agent must act on: the unformatted and
// unchecked files, the BLOCKING hygiene findings. Info lines and the summary
// are dropped — the reason is a work list, not a transcript.
func blockingLines(output string) []string {
	var out []string
	for _, line := range strings.Split(output, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
		case strings.HasPrefix(t, "unformatted "),
			strings.HasPrefix(t, "UNCHECKED "),
			strings.HasPrefix(t, "BLOCKING "):
			out = append(out, "  "+strings.Join(strings.Fields(t), " "))
		}
	}
	return out
}

// commitRoot resolves the repository the commit would land in: the payload's
// working directory, moved by any `cd` or `git -C` the command itself carries,
// then walked up to the repository root git would use.
func commitRoot(payloadCwd, dirHint string) string {
	return tools.RepoRoot(commitDir(payloadCwd, dirHint))
}

// readMessageFile reads the message `git commit -F <file>` would read, from
// the directory the command runs in. A file that cannot be read leaves the
// message empty, which the documentation obligation reports as the
// acknowledgment path being unavailable rather than as no acknowledgment.
func readMessageFile(dir, file string) string {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// commitDir is the working directory the commit runs in: the payload's, moved
// by any `cd` or `git -C` the command carries.
func commitDir(payloadCwd, dirHint string) string {
	base := payloadCwd
	if base == "" {
		if wd, err := os.Getwd(); err == nil {
			base = wd
		}
	}
	if dirHint != "" {
		if filepath.IsAbs(dirHint) {
			base = dirHint
		} else {
			base = filepath.Join(base, dirHint)
		}
	}
	if base == "" {
		return "."
	}
	return base
}

func allow(stdout io.Writer, reason string) int { return decide(stdout, "allow", reason) }
func deny(stdout io.Writer, reason string) int  { return decide(stdout, "deny", reason) }

// decide writes the verdict in the shape the running host reads. Claude Code
// and the hookSpecificOutput hosts share the envelope; Copilot takes the flat
// object, exactly as principles' SessionStart hook already splits them.
func decide(stdout io.Writer, verdict, reason string) int {
	var enc []byte
	var err error
	if host.Detect() == host.Copilot {
		enc, err = json.Marshal(map[string]string{
			"permissionDecision":       verdict,
			"permissionDecisionReason": reason,
		})
	} else {
		var out decisionOutput
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.PermissionDecision = verdict
		out.HookSpecificOutput.PermissionDecisionReason = reason
		enc, err = json.Marshal(out)
	}
	if err != nil {
		return 0
	}
	fmt.Fprintln(stdout, string(enc))
	return 0
}

type commitCommand struct {
	isCommit    bool
	noVerify    bool
	dir         string // -C <dir> or a leading `cd <dir>`, whichever the command used
	message     string // -m/--message, so a documentation acknowledgment can clear its obligation here
	messageFile string // -F/--file, read at check time; "" when the commit carries none
}

// gitValueFlags are the git-level flags that swallow the next word, so the
// word after them is never mistaken for the subcommand.
var gitValueFlags = map[string]bool{
	"-C": true, "-c": true, "--git-dir": true, "--work-tree": true,
	"--namespace": true, "--exec-path": true, "--super-prefix": true,
}

// parseCommand walks the command segment by segment looking for a git
// invocation. It also picks up what the caller needs about that invocation:
// whether the commit opts out of hooks, and which directory it runs in.
func parseCommand(command string) commitCommand {
	var res commitCommand
	segments := splitSegments(tokenize(command))
	for _, seg := range segments {
		seg = stripPrefixWords(seg)
		if len(seg) == 0 {
			continue
		}
		// a `cd somewhere` earlier in the chain moves where the commit lands
		if seg[0].word == "cd" && len(seg) >= 2 && res.dir == "" {
			res.dir = seg[1].word
			continue
		}
		if !isGitWord(seg[0]) {
			continue
		}
		i := 1
		for i < len(seg) && strings.HasPrefix(seg[i].word, "-") && !seg[i].quoted {
			if gitValueFlags[seg[i].word] {
				if seg[i].word == "-C" && i+1 < len(seg) {
					res.dir = seg[i+1].word
				}
				i += 2
				continue
			}
			i++
		}
		if i >= len(seg) || seg[i].word != "commit" || seg[i].quoted {
			continue
		}
		res.isCommit = true
		rest := seg[i+1:]
		for j := 0; j < len(rest); j++ {
			t := rest[j]
			if t.quoted {
				continue
			}
			if t.word == "--no-verify" || t.word == "-n" {
				res.noVerify = true
				continue
			}
			// the message the commit will carry: the gate reads it so a
			// `docs: none — <reason>` line clears the documentation
			// obligation at the moment the decision is actually made
			if (t.word == "-m" || t.word == "--message") && j+1 < len(rest) {
				res.message += rest[j+1].word + "\n"
				j++
				continue
			}
			if v, ok := strings.CutPrefix(t.word, "--message="); ok {
				res.message += v + "\n"
				continue
			}
			// -F/--file is the other way a message arrives, and the one an
			// agent reaches for when the message has a preformatted body.
			// `-F -` reads stdin, which here is the heredoc the command
			// itself carries.
			if (t.word == "-F" || t.word == "--file") && j+1 < len(rest) {
				res.messageFile = rest[j+1].word
				j++
				continue
			}
			if v, ok := cutFlagValue(t.word, "--file=", "-F"); ok {
				res.messageFile = v
			}
		}
		if res.messageFile == "-" {
			// stdin: the heredoc body is the message, and it is right here
			// in the command line. A message piped from another process is
			// not knowable at this point and stays unavailable, said.
			res.messageFile = ""
			res.message = heredocBody(command)
		}
		return res
	}
	return res
}

// cutFlagValue reads a flag's attached value: `--file=msg.txt` for the long
// form, `-Fmsg.txt` for the short one git's parser also accepts. A bare `-F`
// carries no value and is left to the two-word branch above.
func cutFlagValue(word, long, short string) (string, bool) {
	if v, ok := strings.CutPrefix(word, long); ok && v != "" {
		return v, true
	}
	if v, ok := strings.CutPrefix(word, short); ok && v != "" {
		return v, true
	}
	return "", false
}

// heredocInto returns the body of the heredoc the command WRITES to file —
// what `git commit -F <file>` will read once the command's own shell has
// run — and says whether the command's heredocs write to it at all.
//
// The message file a commit names is often the command's own output: `cat >
// msg.txt <<'EOF' … EOF` followed by `git commit -F msg.txt`. The gate
// judges the command BEFORE the shell runs, so the file does not exist yet
// and a read of it hands back empty — the acknowledgment line was in the
// heredoc all along, right in the command line, and this reads it from
// there. The same shape that blocked a real merge: a resolved file whose
// message was staged through a heredoc-built file, the gate reporting "no
// commit message reached this check" while the message was sitting in the
// command it was judging.
//
// What it deliberately does not attempt: a `>>` append (the file's prior
// content is not knowable), a heredoc piped through another command, or a
// target spelled differently from the `-F` argument (an absolute path
// against a relative one). When none of the command's heredocs truncate
// exactly the file the commit names, the read of the file stands and the
// message stays empty, said as such.
func heredocInto(command, file string) (string, bool) {
	lines := strings.Split(command, "\n")
	var body []string
	inBody, stripTabs := false, false
	delim, target := "", ""
	for _, line := range lines {
		if inBody {
			c := strings.TrimRight(line, " \t\r")
			if stripTabs {
				c = strings.TrimLeft(c, "\t")
			}
			if c == delim {
				if target == file {
					return strings.Join(body, "\n"), true
				}
				inBody, body = false, nil
				continue
			}
			body = append(body, line)
			continue
		}
		i := strings.Index(line, "<<")
		if i < 0 {
			continue
		}
		rest := line[i+2:]
		// `<<-DELIM` lets the terminator be tab-indented; the minus is part
		// of the marker, not the delimiter, and it is quoted forms that
		// turn off expansion.
		stripTabs = strings.HasPrefix(rest, "-")
		rest = strings.TrimPrefix(rest, "-")
		delim = strings.Trim(strings.TrimSpace(rest), "\"'")
		if delim == "" {
			continue // a `<<` with no delimiter on the line is not one we can vouch for
		}
		inBody = true
		target = truncateTarget(line)
	}
	return "", false
}

// truncateTarget is the target of the last TRUNCATING redirect on the line,
// read the way a shell reads `cat > f`: the word after a `>` that is not
// itself `>>`. Append is deliberately not a target — the file's prior
// content is not in the command, and a message the gate cannot fully see is
// no message.
func truncateTarget(line string) string {
	var target string
	fields := strings.Fields(line)
	for i := 0; i < len(fields); i++ {
		if fields[i] == ">" && i+1 < len(fields) {
			target = fields[i+1]
		}
	}
	return target
}

// heredocBody returns the body of the first heredoc in the command — what
// `git commit -F -` would read on stdin. `<<EOF`, `<<-EOF`, and the quoted
// delimiters that turn off expansion all land here; the terminator is a line
// that is exactly the delimiter, as the shell reads it.
func heredocBody(command string) string {
	i := strings.Index(command, "<<")
	if i < 0 {
		return ""
	}
	rest := command[i+2:]
	// `<<-EOF` lets the terminator be indented with TABS, and only tabs;
	// plain `<<EOF` wants it at the start of the line. Accepting any
	// indentation would end the message at a body line that merely reads
	// like the delimiter, which loses everything after it — the
	// acknowledgment included.
	stripTabs := strings.HasPrefix(rest, "-")
	rest = strings.TrimPrefix(rest, "-")
	nl := strings.Index(rest, "\n")
	if nl < 0 {
		return ""
	}
	delim := strings.Trim(strings.TrimSpace(rest[:nl]), "\"'")
	if delim == "" {
		return ""
	}
	var body []string
	for _, line := range strings.Split(rest[nl+1:], "\n") {
		candidate := strings.TrimRight(line, " \t\r")
		if stripTabs {
			candidate = strings.TrimLeft(candidate, "\t")
		}
		if candidate == delim {
			return strings.Join(body, "\n")
		}
		body = append(body, line)
	}
	// no terminator in what we were handed: an unterminated heredoc is not a
	// message we can vouch for
	return ""
}

// isGitWord accepts `git` and an explicit path to it (`/usr/bin/git`), but
// never a quoted word: `echo "git commit"` is talk about git, not a call.
func isGitWord(t token) bool {
	if t.quoted {
		return false
	}
	base := t.word
	if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
		base = base[i+1:]
	}
	return base == "git" || base == "git.exe"
}

// stripPrefixWords drops what stands between the start of a segment and the
// program being run: `env`, and the VAR=value assignments shells allow there.
func stripPrefixWords(seg []token) []token {
	for len(seg) > 0 {
		w := seg[0].word
		if seg[0].quoted {
			return seg
		}
		if w == "env" {
			seg = seg[1:]
			continue
		}
		if i := strings.Index(w, "="); i > 0 && !strings.ContainsAny(w[:i], "-/. ") {
			seg = seg[1:]
			continue
		}
		return seg
	}
	return seg
}

// token is one shell word plus whether it arrived quoted — quoting is what
// separates a call to git from a sentence mentioning it.
type token struct {
	word   string
	quoted bool
	sep    bool
}

// splitSegments cuts the token stream at the operators that start a new
// command: only a segment's first word can be the program being run.
func splitSegments(tokens []token) [][]token {
	var segs [][]token
	cur := []token{}
	for _, t := range tokens {
		if t.sep {
			segs = append(segs, cur)
			cur = []token{}
			continue
		}
		cur = append(cur, t)
	}
	return append(segs, cur)
}

// tokenize splits a command into words the way a shell would, well enough for
// this decision: quotes hold a word together (and are remembered), and the
// chain operators come back as their own separator tokens.
func tokenize(command string) []token {
	var out []token
	var cur strings.Builder
	quoted, has := false, false
	flush := func() {
		if has {
			out = append(out, token{word: cur.String(), quoted: quoted})
		}
		cur.Reset()
		quoted, has = false, false
	}
	sep := func() {
		flush()
		out = append(out, token{sep: true})
	}
	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch ch {
		case '$':
			// $'…' and $"…" quote exactly like '…' and "…" for this purpose
			if i+1 < len(command) && (command[i+1] == '\'' || command[i+1] == '"') {
				continue
			}
			cur.WriteByte(ch)
			has = true
		case '\'', '"':
			q := ch
			has, quoted = true, true
			for i++; i < len(command) && command[i] != q; i++ {
				// a backslash escape inside double quotes keeps the next
				// byte in the word: `-m "he said \"no\""` is one argument,
				// and reading it as two loses everything after it
				if command[i] == '\\' && q == '"' && i+1 < len(command) {
					i++
				}
				cur.WriteByte(command[i])
			}
		case '\\':
			if i+1 < len(command) {
				i++
				if command[i] == '\n' {
					continue
				}
				cur.WriteByte(command[i])
				has = true
			}
		case ' ', '\t', '\r':
			flush()
		case '\n', ';', '&', '|', '(', ')', '{', '}':
			if (ch == '&' || ch == '|') && i+1 < len(command) && command[i+1] == ch {
				i++
			}
			sep()
		default:
			cur.WriteByte(ch)
			has = true
		}
	}
	flush()
	return out
}

// InstallGit prints the pre-commit hook that holds the gate for commits made
// outside any agent — a terminal, an IDE button, another tool — and the one
// command that installs it. Printed, never written: `.git/hooks` is the user's
// and procoder does not reach into it.
func InstallGit(root string, out func(string)) int {
	out("== procoder commit gate as a git hook")
	out("")
	out("Repository: " + filepath.ToSlash(root))
	out("Target:     " + filepath.ToSlash(filepath.Join(root, ".git/hooks/pre-commit")))
	out("")
	out("procoder prints this hook and installs nothing — .git/hooks is yours.")
	out("Read it, then run the command below from the repository root.")
	out("")
	out("-- .git/hooks/pre-commit --")
	for _, line := range strings.Split(preCommitScript, "\n") {
		out(line)
	}
	out("-- end --")
	out("")
	out("Install it with:")
	out("")
	out("  cat > .git/hooks/pre-commit <<'PROCODER_EOF'")
	for _, line := range strings.Split(preCommitScript, "\n") {
		out(line)
	}
	out("PROCODER_EOF")
	out("  chmod +x .git/hooks/pre-commit")
	out("")
	out("`git commit --no-verify` skips it, deliberately and visibly, as always.")
	return 0
}

// The script is POSIX sh so it runs wherever git runs. A procoder that is not
// on PATH fails the commit rather than passing it: a gate that could not run
// is not a clean gate.
const preCommitScript = `#!/bin/sh
# procoder commit gate — runs the same check the agent hook runs.
# Blocking findings stop the commit; --no-verify skips this hook entirely.
if ! command -v procoder >/dev/null 2>&1; then
	echo "procoder: not on PATH — the commit gate could NOT run, so this commit is unverified" >&2
	exit 1
fi
if ! procoder check; then
	echo "" >&2
	echo "procoder: blocking findings above — fix them, then commit again." >&2
	exit 1
fi
exit 0`
