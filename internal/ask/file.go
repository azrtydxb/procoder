package ask

import (
	"fmt"
	"path"
	"path/filepath"
	"procoder/internal/answers"
	"procoder/internal/store"
	"sort"
	"strings"
	"time"
)

// QuestionsFile is what a human reads when there was no terminal to ask on.
// The answers file is answers.File — the store is its own package, read by
// `spec` as well as by this one.
const QuestionsFile = "QA.md"

// answerRe-free parsing on purpose: the format is headings and a prefix, and
// a regexp here would be a second definition of a shape the writer already
// owns.
// Answers is the recorded decisions. A key nobody asks about any more is
// kept, not pruned: the question may come back, and discarding it would ask a
// person something they already settled.
type Answers = answers.Store

// Path is where one of the two files lives.
func Path(root, name string) string { return filepath.Join(root, filepath.FromSlash(Dir), name) }

// Unanswered is the questions still waiting on a person.
func Unanswered(qs []Question, answers Answers) []Question {
	var out []Question
	for _, q := range qs {
		if _, done := answers[q.Key()]; !done {
			out = append(out, q)
		}
	}
	return out
}

// WriteQuestions records what is still open, for a human to answer and hand
// back through `ask --file`. The Key travels with each question because it is
// what ties the answer to the question: an answer file that lost it would be
// prose nobody could file.
func WriteQuestions(root string, qs []Question, now time.Time) error {
	var b strings.Builder
	b.WriteString("# Questions procoder cannot answer for you\n\n")
	b.WriteString("Written " + now.UTC().Format("2006-01-02 15:04") + " UTC.\n\n")
	b.WriteString("Answer each one by writing a line beginning `Answer: ` under it, then\n")
	b.WriteString("hand the file back with `procoder ask --file " + path.Join(Dir, QuestionsFile) + "`.\n")
	b.WriteString("Leave the `Key:` lines alone — they are what ties an answer to its question.\n")
	for i, q := range qs {
		fmt.Fprintf(&b, "\n%sQ%d: %s\n\n", answers.HeadingPrefix, i+1, q.Label())
		b.WriteString(answers.KeyPrefix + q.Key() + "\n")
		b.WriteString("Question: " + q.Text + "\n\n")
		// TrimRight, because "Answer: " with nothing after it is trailing
		// whitespace and a formatter strips it — which would make the file
		// procoder just wrote fail procoder's own formatting check. The
		// parser trims each line, so the space is not load-bearing.
		b.WriteString(strings.TrimRight(answers.AnswerPrefix, " ") + "\n")
	}
	return write(root, Path(root, QuestionsFile), b.String())
}

// WriteAnswers records the decisions. Questions are carried alongside their
// answers so the file reads as a record rather than a list of hashes.
func WriteAnswers(root string, qs []Question, decided Answers, now time.Time) error {
	known := map[string]Question{}
	for _, q := range qs {
		known[q.Key()] = q
	}
	var b strings.Builder
	b.WriteString("# What a human decided\n\n")
	b.WriteString("Written " + now.UTC().Format("2006-01-02 15:04") + " UTC. procoder reads this\n")
	b.WriteString("file to avoid asking a question twice; edit an answer here to change what\n")
	b.WriteString("it believes. Reword the question and it will be asked again.\n")
	keys := make([]string, 0, len(decided))
	for k := range decided {
		keys = append(keys, k)
	}
	// Stable output: a file that reorders itself on every write is a diff
	// nobody can read.
	sort.Strings(keys)
	for _, key := range keys {
		entry := decided[key]
		heading, question := "(no longer asked)", entry.Question
		if q, ok := known[key]; ok {
			heading, question = q.Label(), q.Text
		}
		fmt.Fprintf(&b, "\n%s%s\n\n", answers.HeadingPrefix, heading)
		b.WriteString(answers.KeyPrefix + key + "\n")
		if question != "" {
			// Carried from the record when the question is no longer live:
			// rebuilding this file from the questions of the moment used to
			// destroy the text of anything since reworded, leaving an answer
			// nobody could interpret.
			b.WriteString(answers.QuestionPrefix + question + "\n")
		}
		b.WriteString("\n" + answers.AnswerPrefix + entry.Answer + "\n")
	}
	return write(root, Path(root, answers.File), b.String())
}

// write replaces a file only once the new content is safely on disk. The
// answers file is the durable record of decisions nobody can reconstruct,
// and a plain write truncates first: a failure halfway leaves an empty
// record where the decisions were.
//
// The temp-and-rename this used to do by hand is now the store's, which
// does the same thing and also takes the file's lock — the guarantee this
// code never had.
func write(root, dest, body string) error {
	rel, err := store.Rel(root, dest)
	if err != nil {
		return err
	}
	return store.SaveDoc(root, rel, []byte(body))
}

// Same reports whether the store already on disk matches this one, so a run
// with nothing new to say leaves the file — and its timestamp — alone.
func Same(root string, a Answers) bool {
	existing, err := answers.Load(root)
	if err != nil || len(existing) != len(a) {
		return false
	}
	for k, v := range a {
		if existing[k] != v {
			return false
		}
	}
	return true
}
