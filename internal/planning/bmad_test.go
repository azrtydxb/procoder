package planning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const statusYAML = `generated: 08-24-2026 09:00
project: Fixture

development_status:
  epic-1: in-progress
  1-1-auth: done
  1-2-profile: ready-for-dev
  epic-1-retrospective: optional
`

// A repository that named a methodology and does not have it installed
// must be told. Falling back to procoder's own chain would leave it
// believing BMad governs its planning while procoder quietly governed it
// instead, and the first they would learn of it is a report that does not
// match the artifacts on disk.
// proved by: returned nil from Check when the install is absent — the
// gate goes green and the repository is governed by a methodology it did
// not choose.
func TestAChosenMethodThatIsNotInstalledBlocks(t *testing.T) {
	root := fixture(t, map[string]string{"a.txt": "x\n"})

	got := Check(root, "bmad")
	if len(got) != 1 || !got[0].Blocking {
		t.Fatalf("a method that cannot be honoured blocks: %+v", got)
	}
	if !strings.Contains(got[0].Message, "bmad") {
		t.Errorf("the finding must name the setting: %q", got[0].Message)
	}

	// And the default method asks nothing at all — the setting governs
	// planning, so a repository that left it alone sees no planning
	// findings whatsoever.
	if got := Check(root, "procoder"); len(got) != 0 {
		t.Errorf("the default method has nothing to check: %+v", got)
	}
}

// The whole promise of the seam: a repository that plans elsewhere sees
// its own sprint reflected back, not an empty procoder backlog beside the
// one being worked.
// proved by: had Report ignore development_status and return the empty
// answer — the repository is told it has planned nothing while its status
// file lists four entries.
func TestSprintStateComesFromTheArtifactsOnDisk(t *testing.T) {
	root := fixture(t, map[string]string{
		"_bmad/manifest.yaml": "version: \"6.11.0\"\n",
		"_bmad-output/implementation-artifacts/sprint-status.yaml": statusYAML,
	})

	lines := Report(root, "bmad")
	joined := strings.Join(lines, "\n")
	// One done (1-1-auth). Two open: the epic and the ready-for-dev story.
	// The retrospective is "optional" and counts as neither — listing it
	// as work would make every epic look permanently unfinished.
	if !strings.Contains(joined, "1 done, 2 open") {
		t.Fatalf("the counts must come from the file:\n%s", joined)
	}
	for _, want := range []string{"1-2-profile", "epic-1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("outstanding work is named: %q missing from\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "retrospective") {
		t.Errorf("an optional retrospective is not outstanding work:\n%s", joined)
	}

	// The version doctor reports is read from the installation, not guessed.
	if v := Version(root); v != "6.11.0" {
		t.Errorf("the installed version is a fact to read: %q", v)
	}
}

// Unreadable and absent are different answers, and only one of them is
// the repository's choice. A parse failure reported as "nothing planned
// yet" sends somebody to plan work that is already planned.
// proved by: returned nil findings for a file with no development_status
// block — the sprint reads as empty, and a repository mid-sprint is told
// it has not started.
func TestAStatusFileThatWillNotParseIsNotAnEmptySprint(t *testing.T) {
	root := fixture(t, map[string]string{
		"_bmad/manifest.yaml": "version: \"6.11.0\"\n",
		"_bmad-output/implementation-artifacts/sprint-status.yaml": "generated: 08-24-2026\nproject: Fixture\n",
	})

	got := Check(root, "bmad")
	if len(got) != 1 || !got[0].Blocking {
		t.Fatalf("a file that cannot be read as a sprint blocks: %+v", got)
	}
	if !strings.Contains(got[0].File, "sprint-status.yaml") {
		t.Errorf("the finding must name the file: %q", got[0].File)
	}

	// Absent is the ordinary case: a fresh install has planned nothing,
	// and that is not a fault.
	bare := fixture(t, map[string]string{"_bmad/manifest.yaml": "version: \"6.11.0\"\n"})
	if got := Check(bare, "bmad"); len(got) != 0 {
		t.Errorf("no artifacts yet is not a finding: %+v", got)
	}
	if lines := Report(bare, "bmad"); !strings.Contains(strings.Join(lines, "\n"), "no planning artifacts yet") {
		t.Errorf("and the report says so plainly: %v", lines)
	}
}

// BMad owns its status vocabulary and may extend it. Deciding that
// "blocked" is close enough to one of procoder's own is how a status
// machine quietly loses a state, and the report then misrepresents work
// with nothing saying so.
// proved by: dropped the Known check and mapped anything unrecognised to
// "backlog" — a blocked story is reported as not yet started, and the
// finding that would have said so never appears.
func TestAnUnknownStatusIsReportedByName(t *testing.T) {
	root := fixture(t, map[string]string{
		"_bmad/manifest.yaml": "version: \"6.11.0\"\n",
		"_bmad-output/implementation-artifacts/sprint-status.yaml": "development_status:\n  1-1-auth: blocked\n  1-2-profile: done\n",
	})

	got := Check(root, "bmad")
	if len(got) != 1 {
		t.Fatalf("exactly one unknown status, exactly one finding: %+v", got)
	}
	if !strings.Contains(got[0].Message, `"blocked"`) {
		t.Errorf("the status must be quoted by name: %q", got[0].Message)
	}
	// Reported, not blocking: BMad extending its own vocabulary is not a
	// fault in the repository, and blocking every commit over it would
	// make procoder unusable to anyone tracking a state it has not heard of.
	if got[0].Blocking {
		t.Error("an unknown status is news, not a refusal")
	}
	// The known one alongside it produces nothing.
	if strings.Contains(got[0].Message, "1-2-profile") {
		t.Errorf("a status procoder knows is silent: %q", got[0].Message)
	}
}

// A repository that answered BMad's installer with a non-default output
// folder keeps that answer, and procoder reads it rather than assuming.
// Reporting on a directory the repository is not using would be worse
// than reporting nothing, because it looks like an answer.
// proved by: returned the hardcoded default from OutputFolder — the
// sprint reads as absent while the real one sits in the configured
// directory.
func TestTheOutputFolderIsReadFromTheInstallation(t *testing.T) {
	root := fixture(t, map[string]string{
		"_bmad/manifest.yaml": "version: \"6.11.0\"\n",
		"_bmad/config.toml":   "output_folder = \"planning\"\n",
		"planning/implementation-artifacts/sprint-status.yaml": statusYAML,
	})

	folder, problem := OutputFolder(root)
	if problem != nil {
		t.Fatalf("a readable config is not a problem: %+v", problem)
	}
	if folder != "planning" {
		t.Fatalf("the repository's own answer wins: %q", folder)
	}
	if lines := Report(root, "bmad"); !strings.Contains(strings.Join(lines, "\n"), "1 done, 2 open") {
		t.Errorf("and the sprint is found there: %v", lines)
	}
}
