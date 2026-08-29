// Package envsync answers the question a pull leaves behind: what changed
// in this project's environment since I last said "I am set up". It compares
// the tree against a baseline the developer records explicitly — lockfile
// digests, migration directory contents, and the env var names an
// `.env.example` declares — and names the setup step each difference calls
// for. It never runs that step (P-CONTROL) and it never blocks: findings are
// judgment, so a report exits 0 and only a check that could not run exits 1.
//
// Security, hard: a value from `.env` or any of its example siblings never
// leaves this package. Key names are read, compared, and reported; values are
// dropped on the floor — not printed, not hashed into output, not stored.
package envsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"procoder/internal/store"
)

// Dir is the procoder-owned state directory; StateFile is the only file this
// package ever writes, and only under --sync.
const (
	Dir       = store.StateDir
	StateFile = store.EnvPath
)

// stateVersion is the shape version of the state file. A file carrying any
// other number is refused rather than guessed at.
const stateVersion = 1

// maxDepth caps the walk: lockfiles and migrations live near the top of a
// repository, and an unbounded walk of a monorepo is a different tool.
const maxDepth = 6

// lockfiles maps a lockfile name to the ecosystem's frozen-lockfile install —
// the exact command the reader should run when the digest moved.
var lockfiles = []struct{ name, install string }{
	{"package-lock.json", "npm ci"},
	{"pnpm-lock.yaml", "pnpm install --frozen-lockfile"},
	{"yarn.lock", "yarn install --immutable"},
	{"bun.lock", "bun install --frozen-lockfile"},
	{"bun.lockb", "bun install --frozen-lockfile"},
	{"go.sum", "go mod download"},
	{"Cargo.lock", "cargo fetch"},
	{"poetry.lock", "poetry install"},
	{"uv.lock", "uv sync"},
	{"requirements.txt", "pip install -r requirements.txt"},
	{"Gemfile.lock", "bundle install"},
	{"composer.lock", "composer install"},
	{"gradle.lockfile", "./gradlew dependencies"},
}

// migrationDirs are the slash-path suffixes that mark a migration directory,
// matched anywhere in the tree so a monorepo's app/db/migrate counts too.
var migrationDirs = []string{
	"migrations",
	"db/migrate",
	"alembic/versions",
	"prisma/migrations",
	"supabase/migrations",
}

// envExamples are the example files whose keys a local .env is expected to
// carry. The example's own directory supplies the .env it is compared to.
var envExamples = []string{".env.example", ".env.sample", ".env.template"}

// skipDirs are the caches and vendored trees no baseline should describe.
var skipDirs = map[string]bool{
	".git": true, ".procoder": true, "node_modules": true, "vendor": true,
	"target": true, "dist": true, "build": true, ".venv": true, "venv": true,
	"__pycache__": true, ".next": true, ".idea": true, ".gradle": true,
}

// migration is one migration directory as of a sync: how many entries it had
// and their sorted names, so a later run can name what arrived.
type migration struct {
	Count   int      `json:"count"`
	Entries []string `json:"entries"`
}

// state is the on-disk baseline. Keys are repo-root-relative forward-slash
// paths; lockfile values are hex SHA-256 digests, never contents; env_keys
// arrays hold key names and nothing else.
type state struct {
	Version    int                  `json:"version"`
	SyncedAt   string               `json:"synced_at"`
	Lockfiles  map[string]string    `json:"lockfiles"`
	Migrations map[string]migration `json:"migrations"`
	EnvKeys    map[string][]string  `json:"env_keys"`
}

// scan is the current tree as this package sees it, plus the honest list of
// checks that could not run.
type scan struct {
	root       string
	lockfiles  map[string]string
	installs   map[string]string // lockfile path → its install command
	migrations map[string]migration
	envKeys    map[string][]string // example path → declared key names
	envLocal   map[string][]string // example path → the sibling .env's key names
	envMissing map[string]bool     // example path → the sibling .env does not exist
	unchecked  []string
}

// Run compares root against the recorded baseline and prints what changed, or
// records a new baseline when sync is set. Exit code: 0 for a report (findings
// included), 1 when a check could not run or the baseline could not be read or
// written.
func Run(root string, sync bool, out func(string)) int {
	cur := scanTree(root)

	if sync {
		if err := writeState(root, cur); err != nil {
			out("baseline NOT recorded — " + err.Error())
			return 1
		}
		out("baseline recorded at " + StateFile)
		out(fmt.Sprintf("  %d lockfile(s), %d migration directory(ies), %d env example file(s)",
			len(cur.lockfiles), len(cur.migrations), len(cur.envKeys)))
		emitUnchecked(cur, out)
		if len(cur.unchecked) > 0 {
			return 1
		}
		return 0
	}

	base, err := readState(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if base == nil {
		out("no sync baseline recorded — run `procoder env --sync` once your setup is done")
		emitTracked(cur, out)
		emitUnchecked(cur, out)
		if len(cur.unchecked) > 0 {
			return 1
		}
		return 0
	}

	findings := 0
	findings += reportLockfiles(base, cur, out)
	findings += reportMigrations(base, cur, out)
	findings += reportEnvKeys(cur, out)

	if findings == 0 {
		out("no changes since your last sync (" + base.SyncedAt + ")")
	} else {
		out(fmt.Sprintf("%d change(s) since your last sync (%s)", findings, base.SyncedAt))
	}
	emitUnchecked(cur, out)
	if len(cur.unchecked) > 0 {
		return 1
	}
	return 0
}

// emitTracked lists what a baseline would cover — the answer to "what would
// --sync record?" on a repository that has none yet.
func emitTracked(cur scan, out func(string)) {
	var tracked []string
	for p := range cur.lockfiles {
		tracked = append(tracked, p)
	}
	for p := range cur.migrations {
		tracked = append(tracked, p+"/")
	}
	for p := range cur.envKeys {
		tracked = append(tracked, p)
	}
	if len(tracked) == 0 {
		out("  nothing to track — no lockfiles, migration directories, or env example files found")
		return
	}
	sort.Strings(tracked)
	out("  it would track:")
	for _, p := range tracked {
		out("    " + p)
	}
}

func emitUnchecked(cur scan, out func(string)) {
	for _, u := range cur.unchecked {
		out(u)
	}
}

// reportLockfiles names every lockfile that moved, vanished, or appeared, with
// the install command that answers it.
func reportLockfiles(base *state, cur scan, out func(string)) int {
	n := 0
	for _, path := range union(keysOf(base.Lockfiles), keysOf(cur.lockfiles)) {
		was, hadBase := base.Lockfiles[path]
		now, hasNow := cur.lockfiles[path]
		switch {
		case hadBase && !hasNow:
			out(path + " is gone since your last sync")
			n++
		case !hadBase && hasNow:
			out(path + " is new since your last sync — run " + cur.installs[path])
			n++
		case was != now:
			out(path + ": dependencies changed since your last sync — run " + cur.installs[path])
			n++
		}
	}
	return n
}

// reportMigrations names directories that gained entries — and says "changed"
// rather than a count when entries disappeared, because a squash or a rebase
// is not a negative migration.
func reportMigrations(base *state, cur scan, out func(string)) int {
	n := 0
	for _, dir := range union(keysOf(base.Migrations), keysOf(cur.migrations)) {
		was, hadBase := base.Migrations[dir]
		now, hasNow := cur.migrations[dir]
		if hadBase && !hasNow {
			out(dir + "/ is gone since your last sync")
			n++
			continue
		}
		if !hadBase {
			was = migration{}
		}
		added := missingFrom(now.Entries, was.Entries)
		removed := missingFrom(was.Entries, now.Entries)
		switch {
		case len(removed) > 0:
			out(fmt.Sprintf("%s/: migration set changed since your last sync — %d entry(ies) now, %d at your last sync",
				dir, len(now.Entries), len(was.Entries)))
			for _, a := range added {
				out("    + " + a)
			}
			n++
		case len(added) > 0:
			out(fmt.Sprintf("%s/: %d migration(s) added since your last sync", dir, len(added)))
			for _, a := range added {
				out("    " + a)
			}
			n++
		}
	}
	return n
}

// reportEnvKeys names the example-declared keys the local .env does not carry.
// Key names only — no value from either file is read into the report, ever.
func reportEnvKeys(cur scan, out func(string)) int {
	n := 0
	for _, path := range sortedKeys(cur.envKeys) {
		missing := missingFrom(cur.envKeys[path], cur.envLocal[path])
		if len(missing) == 0 {
			continue
		}
		local := ".env"
		if dir := pathDir(path); dir != "" {
			local = dir + "/.env"
		}
		if cur.envMissing[path] {
			out(path + ": the local " + local + " does not exist — every declared key is new")
		}
		out(path + ": new env var(s) declared: " + strings.Join(missing, ", "))
		n++
	}
	return n
}

// --- scanning ---------------------------------------------------------------

// scanTree walks root once and records everything a baseline describes. A file
// or directory it cannot read becomes a NOT-checked line naming the path and
// the reason — never a silently missing entry.
func scanTree(root string) scan {
	s := scan{
		root:       root,
		lockfiles:  map[string]string{},
		installs:   map[string]string{},
		migrations: map[string]migration{},
		envKeys:    map[string][]string{},
		envLocal:   map[string][]string{},
		envMissing: map[string]bool{},
	}
	// Files git ignores are not this project's environment: an agent
	// worktree's vendored lockfiles describe a copy, not the repo you are
	// syncing. git owns that question, so ask it rather than guessing with
	// a hand-kept skip list — the same call codeindex uses to scope itself.
	ignored := gitIgnoredDirs(root)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path != root {
				s.unchecked = append(s.unchecked, relSlash(root, path)+": NOT checked — "+reason(err))
			}
			return nil
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if path == root {
				return nil
			}
			if skipDirs[d.Name()] || ignored[rel] || strings.Count(rel, "/") >= maxDepth {
				return filepath.SkipDir
			}
			if isMigrationDir(rel) {
				s.readMigrations(path, rel)
				return filepath.SkipDir // the entries are the answer; their contents are not
			}
			return nil
		}
		if install, ok := installFor(d.Name()); ok {
			s.readLockfile(path, rel, install)
			return nil
		}
		if isEnvExample(d.Name()) {
			s.readEnvExample(path, rel)
		}
		return nil
	})
	sort.Strings(s.unchecked)
	return s
}

func (s *scan) readLockfile(path, rel, install string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		s.unchecked = append(s.unchecked, rel+": NOT checked — "+reason(err))
		return
	}
	sum := sha256.Sum256(raw)
	s.lockfiles[rel] = hex.EncodeToString(sum[:])
	s.installs[rel] = install
}

func (s *scan) readMigrations(path, rel string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		s.unchecked = append(s.unchecked, rel+"/: NOT checked — "+reason(err))
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	s.migrations[rel] = migration{Count: len(names), Entries: names}
}

// readEnvExample records the key names an example declares and the key names
// the sibling .env carries. Both files' values are discarded as they are
// parsed; nothing but a name survives this function.
func (s *scan) readEnvExample(path, rel string) {
	keys, err := envKeysOf(path)
	if err != nil {
		s.unchecked = append(s.unchecked, rel+": NOT checked — "+reason(err))
		return
	}
	s.envKeys[rel] = keys
	local := filepath.Join(filepath.Dir(path), ".env")
	localKeys, err := envKeysOf(local)
	if os.IsNotExist(err) {
		s.envMissing[rel] = true
		s.envLocal[rel] = nil
		return
	}
	if err != nil {
		s.unchecked = append(s.unchecked, relSlash(s.root, local)+": NOT checked — "+reason(err))
		delete(s.envKeys, rel)
		return
	}
	s.envLocal[rel] = localKeys
}

// envKeysOf reads a dotenv-shaped file and returns its declared key names,
// sorted. Comments and blank lines declare nothing; the text right of the
// first `=` is never retained.
func envKeysOf(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(strings.TrimPrefix(line[:eq], "export "))
		if isEnvKey(key) {
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// isEnvKey holds a key to the shell's own shape: a letter or underscore, then
// letters, digits, or underscores. Anything else is not a declaration.
func isEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, c := range k {
		switch {
		case c == '_':
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// --- state file -------------------------------------------------------------

// readState loads the baseline. A missing file answers (nil, nil) — no
// baseline is not an error. A corrupt file or an unknown version is an error
// naming the file, never an empty baseline read as "nothing changed".
func readState(root string) (*state, error) {
	raw, err := store.LoadEnvState(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: NOT read — %s; re-run `procoder env --sync`", StateFile, reason(err))
	}
	var s state
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s is corrupt (%s) — re-run `procoder env --sync` to record a fresh baseline", StateFile, err.Error())
	}
	if s.Version != stateVersion {
		return nil, fmt.Errorf("%s has version %d, this procoder writes version %d — re-run `procoder env --sync` to record a fresh baseline",
			StateFile, s.Version, stateVersion)
	}
	if s.Lockfiles == nil {
		s.Lockfiles = map[string]string{}
	}
	if s.Migrations == nil {
		s.Migrations = map[string]migration{}
	}
	if s.EnvKeys == nil {
		s.EnvKeys = map[string][]string{}
	}
	return &s, nil
}

// writeState records the scan as the new baseline: a temp file in the target
// directory, then a rename, so a failure leaves no partial state behind. This
// is the only write this package ever performs.
func writeState(root string, cur scan) error {
	s := state{
		Version:    stateVersion,
		SyncedAt:   time.Now().UTC().Format(time.RFC3339),
		Lockfiles:  cur.lockfiles,
		Migrations: cur.migrations,
		EnvKeys:    cur.envKeys,
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	// The temp-and-rename this used to do by hand now lives in the store,
	// which also takes the file's lock — the same guarantee, plus the one
	// this never had.
	if err := store.SaveEnvState(root, body); err != nil {
		return reasonedPath(StateFile, err)
	}
	return nil
}

// --- small helpers ----------------------------------------------------------

func installFor(name string) (string, bool) {
	for _, l := range lockfiles {
		if l.name == name {
			return l.install, true
		}
	}
	return "", false
}

// pathDir is the directory part of a forward-slash relative path, empty when
// the path sits at the repository root.
func pathDir(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

func isMigrationDir(rel string) bool {
	for _, suffix := range migrationDirs {
		if rel == suffix || strings.HasSuffix(rel, "/"+suffix) {
			return true
		}
	}
	return false
}

func isEnvExample(name string) bool {
	for _, e := range envExamples {
		if name == e {
			return true
		}
	}
	return false
}

// relSlash is the repo-root-relative path in forward slashes — the only path
// form this package ever prints, on every platform.
func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	return filepath.ToSlash(rel)
}

// reason strips the OS path out of a filesystem error: the message keeps the
// cause, and the path we print is our own forward-slash one.
func reason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Op + ": " + pe.Err.Error()
	}
	return err.Error()
}

func reasonedPath(path string, err error) error {
	return fmt.Errorf("%s: %s", path, reason(err))
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := keysOf(m)
	sort.Strings(out)
	return out
}

// union is the sorted set of both lists.
func union(a, b []string) []string {
	seen := map[string]bool{}
	for _, s := range append(append([]string{}, a...), b...) {
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// missingFrom is the members of want that are absent from have, order
// preserved.
func missingFrom(want, have []string) []string {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	var out []string
	for _, w := range want {
		if !set[w] {
			out = append(out, w)
		}
	}
	return out
}

// gitIgnoredDirs asks git which directories under root it ignores, so the
// survey never describes a tree the repository itself disowns. A repo
// without git (or without a working `git`) simply gets an empty set and
// the static skip list still applies.
func gitIgnoredDirs(root string) map[string]bool {
	out := map[string]bool{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root,
		"ls-files", "--directory", "--others", "--ignored", "--exclude-standard")
	raw, err := cmd.Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSuffix(strings.TrimSpace(line), "/"); line != "" {
			out[line] = true
		}
	}
	return out
}
