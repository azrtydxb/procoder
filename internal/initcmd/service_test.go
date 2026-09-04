package initcmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func configBody(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".procoder", "config.toml"))
	if err != nil {
		return ""
	}
	return string(b)
}

// Saying server writes the key; saying anything else writes nothing.
//
// proved by: writing the key whatever the answer — a person who said cli
// gets a daemon, which is the one outcome asking was supposed to prevent.
func TestInitAsksAboutTheServer(t *testing.T) {
	root := fixture(t)
	yes := "server"
	if err := AskAboutTheServer(root, &yes, io.Discard); err != nil {
		t.Fatalf("AskAboutTheServer: %v", err)
	}
	body := configBody(t, root)
	if !strings.Contains(body, "[service]") || !strings.Contains(body, `mode = "local"`) {
		t.Fatalf("server did not write the key:\n%s", body)
	}

	other := fixture(t)
	no := "cli"
	if err := AskAboutTheServer(other, &no, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b := configBody(t, other); strings.Contains(b, "mode") {
		t.Fatalf("cli wrote a mode key:\n%s", b)
	}
}

// Nobody to ask means nothing is written. A repository must never acquire
// a daemon because a script ran init.
func TestNoAnswerWritesNothing(t *testing.T) {
	root := fixture(t)
	if err := AskAboutTheServer(root, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b := configBody(t, root); b != "" {
		t.Fatalf("a run with nobody to ask wrote config:\n%s", b)
	}
}

// A typo is the default. A machine that acquires a daemon from a
// mistyped answer is worse than one that has to answer twice.
func TestATypoIsNotConsent(t *testing.T) {
	root := fixture(t)
	typo := "sever"
	if err := AskAboutTheServer(root, &typo, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b := configBody(t, root); strings.Contains(b, "mode") {
		t.Fatalf("a typo turned the daemon on:\n%s", b)
	}
}

// An existing [service] section keeps its other keys.
func TestExistingServiceSectionIsNotClobbered(t *testing.T) {
	root := fixture(t)
	existing := "[service]\nrepo = \"acme/widgets\"\n"
	if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	yes := "server"
	if err := AskAboutTheServer(root, &yes, io.Discard); err != nil {
		t.Fatal(err)
	}
	body := configBody(t, root)
	if !strings.Contains(body, `repo = "acme/widgets"`) {
		t.Fatalf("the existing key was lost:\n%s", body)
	}
	if !strings.Contains(body, `mode = "local"`) {
		t.Fatalf("the mode was not added:\n%s", body)
	}
}

// A repository that already chose is not asked again. That decision lives
// in a tracked file, which is where it can be changed properly.
func TestAlreadyChosenIsNotAskedAgain(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"),
		[]byte("[service]\nmode = \"local\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := configBody(t, root)
	no := "cli"
	if err := AskAboutTheServer(root, &no, io.Discard); err != nil {
		t.Fatal(err)
	}
	if configBody(t, root) != before {
		t.Fatal("a repository that had already chosen was changed")
	}
}
