package ask

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/answers"
	"procoder/internal/copilot"
)

// Run is `procoder ask`. It collects what is undecided, asks a person when
// there is a person to ask, and records the answers. It exits 1 while any
// question is unanswered and 0 when none are, so a caller can tell the two
// apart without parsing prose.
//
// It never answers anything itself. That is the whole point: an invented
// answer is indistinguishable from a decision once it is written down.
func Run(root string, in *os.File, out func(string)) int {
	store, err := answers.Load(root)
	if err != nil {
		out("answers NOT read — " + err.Error())
		out("refusing to ask again over a file that may already hold decisions")
		return 2
	}
	qs, notes := Collect(root)
	for _, n := range notes {
		out(n)
	}
	pending := Unanswered(qs, store)
	if len(pending) == 0 {
		if len(qs) == 0 {
			out("nothing to decide — no domain has a question open")
		} else {
			out(fmt.Sprintf("all %d question(s) already answered — %s holds the decisions",
				len(qs), path.Join(Dir, answers.File)))
		}
		return 0
	}
	if !copilot.CanAsk(in) {
		return writeForLater(root, qs, pending, store, out)
	}
	answered := askEach(in, out, pending, store)
	if answered == 0 {
		// Every question skipped: nothing was decided, so nothing is
		// rewritten. A file whose timestamp moves without its content is a
		// dirty tree for no reason.
		out(fmt.Sprintf("nothing answered — %d question(s) still open", len(pending)))
		return 1
	}
	if err := WriteAnswers(root, qs, store, time.Now()); err != nil {
		out("answers NOT recorded — " + err.Error())
		return 2
	}
	out(fmt.Sprintf("%d answered, %d still open — recorded in %s",
		answered, len(pending)-answered, path.Join(Dir, answers.File)))
	if answered < len(pending) {
		return 1
	}
	return 0
}

// writeForLater is the no-terminal path: nobody is there to ask, so the
// questions go to a file with the route back in. Silence here would leave the
// coder to guess, which is the failure this package exists to prevent.
func writeForLater(root string, qs, pending []Question, store Answers, out func(string)) int {
	if err := WriteQuestions(root, pending, time.Now()); err != nil {
		out("questions NOT written — " + err.Error())
		return 2
	}
	if !Same(root, store) {
		if err := WriteAnswers(root, qs, store, time.Now()); err != nil {
			out("answers NOT written — " + err.Error())
			return 2
		}
	}
	qa := path.Join(Dir, QuestionsFile)
	out(fmt.Sprintf("%d question(s) need a human, and there is no terminal to ask on.", len(pending)))
	out("They are written to " + qa + ", one section each.")
	out("Put them to the user, write their answers into that file under the matching")
	out("`Answer:` line, and hand it back with `procoder ask --file " + qa + "`.")
	out("Do NOT answer them yourself: a guess recorded here reads as a decision.")
	return 1
}

// askEach puts one question at a time. An empty answer is a skip, not a
// decision — a blank line is somebody deferring, and recording it as an
// answer would silence the question forever.
func askEach(in *os.File, out func(string), pending []Question, store Answers) int {
	reader := bufio.NewReader(in)
	answered := 0
	for i, q := range pending {
		out("")
		out(fmt.Sprintf("(%d/%d) %s", i+1, len(pending), q.Label()))
		out(q.Text)
		out("answer, or empty to skip:")
		line, err := reader.ReadString('\n')
		answer := strings.TrimSpace(line)
		if answer != "" {
			store[q.Key()] = answers.Entry{Question: q.Text, Answer: answer}
			answered++
		}
		if err != nil && answer == "" {
			// Input ended. Counting the rest as skips would report that a
			// person deferred questions they were never shown.
			out(fmt.Sprintf("input ended — %d question(s) were never asked", len(pending)-i))
			break
		}
	}
	return answered
}

// FromFile records the answers a human wrote into a file — the route back in
// when there was no terminal to ask on. The file is parsed whole before
// anything is recorded: a partial reading of somebody's decisions is worse
// than refusing the file.
func FromFile(root, file string, out func(string)) int {
	raw, err := os.ReadFile(file)
	if err != nil {
		out("cannot read " + filepath.ToSlash(file) + " — " + err.Error())
		return 2
	}
	given := answers.Parse(string(raw))
	if len(given) == 0 {
		out("no answers found in " + filepath.ToSlash(file) + " — each one is a `Key:` line and an `Answer:` line beneath it")
		out("nothing was recorded")
		return 2
	}
	store, err := answers.Load(root)
	if err != nil {
		out("answers NOT read — " + err.Error())
		return 2
	}
	qs, notes := Collect(root)
	for _, n := range notes {
		out(n)
	}
	known := map[string]bool{}
	for _, q := range qs {
		known[q.Key()] = true
	}
	recorded, unknown := 0, 0
	for key, entry := range given {
		if !known[key] {
			// An answer to a question nobody is asking is kept rather than
			// dropped: the question may return, and a human's decision is
			// not ours to discard. It is counted out loud so a mistyped key
			// does not look like success.
			unknown++
		}
		store[key] = entry
		recorded++
	}
	if err := WriteAnswers(root, qs, store, time.Now()); err != nil {
		out("answers NOT recorded — " + err.Error())
		return 2
	}
	out(fmt.Sprintf("%d answer(s) recorded in %s", recorded, path.Join(Dir, answers.File)))
	if unknown > 0 {
		out(fmt.Sprintf("%d of them answer a question no domain is asking — kept, in case it comes back", unknown))
	}
	if left := Unanswered(qs, store); len(left) > 0 {
		out(fmt.Sprintf("%d question(s) still open", len(left)))
		return 1
	}
	return 0
}

// Pending is what the gate and the hooks need: the questions still waiting on
// a person. An unreadable answers file yields no questions and an error —
// unknown is never the same as nothing to ask.
func Pending(root string) ([]Question, error) {
	store, err := answers.Load(root)
	if err != nil {
		return nil, err
	}
	qs, _ := Collect(root)
	return Unanswered(qs, store), nil
}
