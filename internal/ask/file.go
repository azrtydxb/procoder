package ask

import (
	"fmt"
	"os"
	"path/filepath"
	"procoder/internal/answers"
	"strings"
	"time"
)

// QuestionsFile is what a human reads when there was no terminal to ask on.
// The answers file is answers.File — the store is its own package, read by
// `spec` as well as by this one.
const QuestionsFile = "QA.md"

// AnswersFile is re-exported for the messages this package prints.
const AnswersFile = answers.File

// answerRe-free parsing on purpose: the format is headings and a prefix, and
// a regexp here would be a second definition of a shape the writer already
// owns.
const (
	headingPrefix = answers.HeadingPrefix
	keyPrefix     = answers.KeyPrefix
	answerPrefix  = answers.AnswerPrefix
)

// Answers is the recorded decisions. A key nobody asks about any more is
// kept, not pruned: the question may come back, and discarding it would ask a
// person something they already settled.
type Answers = answers.Store

// Path is where one of the two files lives.
func Path(root, name string) string { return filepath.Join(root, filepath.FromSlash(Dir), name) }

// LoadAnswers reads what has already been decided.
func LoadAnswers(root string) (Answers, error) { return answers.Load(root) }

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
	b.WriteString("hand the file back with `procoder ask --file " + filepath.ToSlash(filepath.Join(Dir, QuestionsFile)) + "`.\n")
	b.WriteString("Leave the `Key:` lines alone — they are what ties an answer to its question.\n")
	for i, q := range qs {
		fmt.Fprintf(&b, "\n%sQ%d: %s\n\n", headingPrefix, i+1, q.Label())
		b.WriteString(keyPrefix + q.Key() + "\n")
		b.WriteString("Question: " + q.Text + "\n")
		b.WriteString(answerPrefix + "\n")
	}
	return write(Path(root, QuestionsFile), b.String())
}

// WriteAnswers records the decisions. Questions are carried alongside their
// answers so the file reads as a record rather than a list of hashes.
func WriteAnswers(root string, qs []Question, answers Answers, now time.Time) error {
	known := map[string]Question{}
	for _, q := range qs {
		known[q.Key()] = q
	}
	var b strings.Builder
	b.WriteString("# What a human decided\n\n")
	b.WriteString("Written " + now.UTC().Format("2006-01-02 15:04") + " UTC. procoder reads this\n")
	b.WriteString("file to avoid asking a question twice; edit an answer here to change what\n")
	b.WriteString("it believes. Reword the question and it will be asked again.\n")
	for _, key := range sortedKeys(answers) {
		q, ok := known[key]
		heading := "(question no longer asked)"
		if ok {
			heading = q.Label()
		}
		fmt.Fprintf(&b, "\n%s%s\n\n", headingPrefix, heading)
		b.WriteString(keyPrefix + key + "\n")
		if ok {
			b.WriteString("Question: " + q.Text + "\n")
		}
		b.WriteString(answerPrefix + answers[key] + "\n")
	}
	return write(Path(root, AnswersFile), b.String())
}

func sortedKeys(a Answers) []string {
	out := make([]string, 0, len(a))
	for k := range a {
		out = append(out, k)
	}
	// Stable output: a file that reorders itself on every write is a diff
	// nobody can read.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func write(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}
