package copilot

import (
	"strings"
	"testing"
	"time"
)

// An adversarial pass over Sanitise from outside the author's own imagination:
// every shape a real Copilot body carries that could smuggle the user's code
// or a credential out of the machine.
func TestAdversarialSanitise(t *testing.T) {
	root := "/Users/someone/Projects/secret-client"
	tok := "ghp" + "_" + strings.Repeat("A1b2", 9)
	aws := "AKIA" + strings.Repeat("Q", 16)
	cases := []struct{ name, body, mustNotContain string }{
		{"tilde fence", "Bug here\n~~~\nsecretFunc(apiKey)\n~~~\n", "secretFunc"},
		{"four backticks", "Bug\n````\nprivateBusinessLogic()\n````\n", "privateBusinessLogic"},
		{"indented block", "Bug\n\n    proprietaryAlgorithm()\n\n", "proprietaryAlgorithm"},
		{"unclosed fence", "Bug\n```go\nleakedSource()\n", "leakedSource"},
		{"token in prose", "Found " + tok + " committed", tok},
		{"aws key", "key " + aws + " in config", aws},
		{"abs path", "see " + root + "/internal/private/logic.go:12", root},
		{"nested fence in quote", "> ```\n> quotedSecretCode()\n> ```\n", "quotedSecretCode"},
		{"password kv", "config: password=hunter2seekrit", "hunter2seekrit"},
		{"crlf fence", "Bug\r\n```\r\ncrlfSource()\r\n```\r\n", "crlfSource"},
	}
	for _, c := range cases {
		got := Sanitise(Finding{Title: "t", Body: c.body, Created: time.Now()}, root)
		if strings.Contains(got.Body, c.mustNotContain) {
			t.Errorf("%s: %q survived sanitisation:\n%s", c.name, c.mustNotContain, got.Body)
		}
	}
}
