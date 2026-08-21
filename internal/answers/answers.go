// Package answers is the record of what a human decided: a small store, read
// by whoever needs to know whether a question has been settled.
//
// It is its own package because two of its readers cannot import each other.
// `ask` collects questions from every domain, including `spec`; `spec` needs
// to know which of its questions have been answered. Putting the store in
// either one makes a cycle; putting it here makes it what it actually is —
// data both of them read.
package answers

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// Dir and File are where the record lives, beside the other state this
// repository keeps.
const (
	Dir  = ".procoder/ask"
	File = "answers.md"
)

// The shape of the file: a heading per decision, a key that ties it to the
// question, and the answer itself.
const (
	HeadingPrefix = "## "
	KeyPrefix     = "Key: "
	AnswerPrefix  = "Answer: "
)

// Store maps a question's key to what the human decided.
type Store map[string]string

// Key identifies a question across runs by hashing what was ASKED. An answer
// therefore survives a re-run and stops counting the moment the question is
// reworded — the old answer belonged to a different question. Source and
// origin are in the hash because the same sentence from two domains is two
// questions.
func Key(source, origin, text string) string {
	sum := sha1.Sum([]byte(source + "\x00" + origin + "\x00" + text)) // nosemgrep: use-of-sha1
	return hex.EncodeToString(sum[:])[:12]
}

// Path is the answers file for a repository.
func Path(root string) string { return filepath.Join(root, filepath.FromSlash(Dir), File) }

// Load reads the decisions. A missing file is an empty store — nothing has
// been answered yet is the ordinary state. A file that cannot be READ is an
// error: treating it as empty would re-ask everything and then write the new
// answers over decisions already made.
func Load(root string) (Store, error) {
	raw, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return Store{}, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(string(raw)), nil
}

// Parse reads answers out of the file's text. Anything unrecognised is
// skipped rather than guessed at, and an answer with no key above it is
// dropped: an answer that cannot be tied to a question is not evidence of
// anything.
func Parse(text string) Store {
	out := Store{}
	key := ""
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, HeadingPrefix):
			key = ""
		case strings.HasPrefix(t, KeyPrefix):
			key = strings.TrimSpace(strings.TrimPrefix(t, KeyPrefix))
		case strings.HasPrefix(t, AnswerPrefix) && key != "":
			if a := strings.TrimSpace(strings.TrimPrefix(t, AnswerPrefix)); a != "" {
				out[key] = a
			}
			key = ""
		}
	}
	return out
}
