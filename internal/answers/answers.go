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

	"procoder/internal/store"
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
	HeadingPrefix  = "## "
	KeyPrefix      = "Key: "
	QuestionPrefix = "Question: "
	AnswerPrefix   = "Answer: "
)

// Entry is one recorded decision: what was asked, and what the human said.
// The question travels with the answer because this file is the durable
// record — rebuilding it from whatever questions happen to be live would
// destroy the text of any question since reworded, which is exactly when a
// reader most needs to see what was actually answered.
type Entry struct {
	Question string
	Answer   string
}

// Store maps a question's key to the decision recorded against it.
type Store map[string]Entry

// Key identifies a question across runs by hashing what was ASKED. An answer
// therefore survives a re-run and stops counting the moment the question is
// reworded — the old answer belonged to a different question. Source and
// origin are in the hash because the same sentence from two domains is two
// questions.
//
// Key is the legacy identifier: it hashes the text AS WRITTEN, so a
// reformatted or re-wrapped question changed the hash and orphaned every
// answer recorded under it — an answered decision came back on the next
// run. New code keys by KeyStable and reads the store through Settled, which
// also finds what Key recorded; a store full of legacy entries stays
// readable rather than re-asking what was already settled.
func Key(source, origin, text string) string {
	sum := sha1.Sum([]byte(source + "\x00" + origin + "\x00" + text)) // nosemgrep: use-of-sha1
	return hex.EncodeToString(sum[:])[:12]
}

// KeyStable is Key over the question's WORDS rather than its line breaks.
// A formatter reflowing a section, or a question re-wrapped across lines,
// keeps the key it was answered under; changing a word changes the key, and
// a reworded question is asked again, exactly as Key documented.
func KeyStable(source, origin, text string) string {
	return Key(source, origin, strings.Join(strings.Fields(text), " "))
}

// Settled reports whether the store holds an answer to the question, under
// either generation of key. Recording an answer used Key, so a store written
// before KeyStable was introduced must still read as settled, or every
// recorded decision would be re-asked the first time after the upgrade.
func (s Store) Settled(source, origin, text string) bool {
	k := KeyStable(source, origin, text)
	if _, ok := s[k]; ok {
		return true
	}
	if legacy := Key(source, origin, text); legacy != k {
		_, ok := s[legacy]
		return ok
	}
	return false
}

// Path is the answers file for a repository.
func Path(root string) string { return filepath.Join(root, filepath.FromSlash(Dir), File) }

// Load reads the decisions. A missing file is an empty store — nothing has
// been answered yet is the ordinary state. A file that cannot be READ is an
// error: treating it as empty would re-ask everything and then write the new
// answers over decisions already made.
func Load(root string) (Store, error) {
	raw, err := store.LoadIn(root, Dir, File)
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
	key, question := "", ""
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, HeadingPrefix):
			key, question = "", ""
		case strings.HasPrefix(t, KeyPrefix):
			key = strings.TrimSpace(strings.TrimPrefix(t, KeyPrefix))
		case strings.HasPrefix(t, QuestionPrefix):
			question = strings.TrimSpace(strings.TrimPrefix(t, QuestionPrefix))
		case strings.HasPrefix(t, AnswerPrefix) && key != "":
			if a := strings.TrimSpace(strings.TrimPrefix(t, AnswerPrefix)); a != "" {
				out[key] = Entry{Question: question, Answer: a}
			}
			key, question = "", ""
		}
	}
	return out
}
