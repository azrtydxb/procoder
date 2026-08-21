package ask

import (
	"bufio"
	"fmt"
	"os"
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
	answers, err := LoadAnswers(root)
	if err != nil {
		out("answers NOT read — " + err.Error())
		out("refusing to ask again over a file that may already hold decisions")
		return 2
	}
	qs := Collect(root)
	pending := Unanswered(qs, answers)
	if len(pending) == 0 {
		if len(qs) == 0 {
			out("nothing to decide — no domain has a question open")
		} else {
			out(fmt.Sprintf("all %d question(s) already answered — %s holds the decisions",
				len(qs), filepath.ToSlash(filepath.Join(Dir, AnswersFile))))
		}
		return 0
	}
	if !copilot.CanAsk(in) {
		return writeForLater(root, qs, pending, answers, out)
	}
	answered := askEach(in, out, pending, answers)
	if err := WriteAnswers(root, qs, answers, time.Now()); err != nil {
		out("answers NOT recorded — " + err.Error())
		return 2
	}
	out(fmt.Sprintf("%d answered, %d still open — recorded in %s",
		answered, len(pending)-answered, filepath.ToSlash(filepath.Join(Dir, AnswersFile))))
	if answered < len(pending) {
		return 1
	}
	return 0
}

// writeForLater is the no-terminal path: nobody is there to ask, so the
// questions go to a file with the route back in. Silence here would leave the
// coder to guess, which is the failure this package exists to prevent.
func writeForLater(root string, qs, pending []Question, answers Answers, out func(string)) int {
	if err := WriteQuestions(root, pending, time.Now()); err != nil {
		out("questions NOT written — " + err.Error())
		return 2
	}
	if err := WriteAnswers(root, qs, answers, time.Now()); err != nil {
		out("answers NOT written — " + err.Error())
		return 2
	}
	qa := filepath.ToSlash(filepath.Join(Dir, QuestionsFile))
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
func askEach(in *os.File, out func(string), pending []Question, answers Answers) int {
	reader := bufio.NewReader(in)
	answered := 0
	for i, q := range pending {
		out("")
		out(fmt.Sprintf("(%d/%d) %s", i+1, len(pending), q.Label()))
		out(q.Text)
		out("answer, or empty to skip:")
		line, _ := reader.ReadString('\n')
		if answer := strings.TrimSpace(line); answer != "" {
			answers[q.Key()] = answer
			answered++
		}
	}
	return answered
}

// FromFile records the answers a human wrote into a file — the route back in
// when there was no terminal to ask on. The file is parsed whole before
// anything is recorded: a partial reading of somebody's decisions is worse
// than refusing the file.
func FromFile(root, path string, out func(string)) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		out("cannot read " + filepath.ToSlash(path) + " — " + err.Error())
		return 2
	}
	given := answers.Parse(string(raw))
	if len(given) == 0 {
		out("no answers found in " + filepath.ToSlash(path) + " — each one is a `Key:` line and an `Answer:` line beneath it")
		out("nothing was recorded")
		return 2
	}
	answers, err := LoadAnswers(root)
	if err != nil {
		out("answers NOT read — " + err.Error())
		return 2
	}
	qs := Collect(root)
	known := map[string]bool{}
	for _, q := range qs {
		known[q.Key()] = true
	}
	recorded, unknown := 0, 0
	for key, answer := range given {
		if !known[key] {
			// An answer to a question nobody is asking is kept rather than
			// dropped: the question may return, and a human's decision is
			// not ours to discard. It is counted out loud so a mistyped key
			// does not look like success.
			unknown++
		}
		answers[key] = answer
		recorded++
	}
	if err := WriteAnswers(root, qs, answers, time.Now()); err != nil {
		out("answers NOT recorded — " + err.Error())
		return 2
	}
	out(fmt.Sprintf("%d answer(s) recorded in %s", recorded, filepath.ToSlash(filepath.Join(Dir, AnswersFile))))
	if unknown > 0 {
		out(fmt.Sprintf("%d of them answer a question no domain is asking — kept, in case it comes back", unknown))
	}
	if left := Unanswered(qs, answers); len(left) > 0 {
		out(fmt.Sprintf("%d question(s) still open", len(left)))
		return 1
	}
	return 0
}

// Pending is what the gate and the hooks need: the questions still waiting on
// a person. An unreadable answers file yields no questions and an error —
// unknown is never the same as nothing to ask.
func Pending(root string) ([]Question, error) {
	answers, err := LoadAnswers(root)
	if err != nil {
		return nil, err
	}
	return Unanswered(Collect(root), answers), nil
}
