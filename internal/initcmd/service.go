package initcmd

import (
	"fmt"
	"io"
	"strings"

	"procoder/internal/config"
	"procoder/internal/store"
)

// AskAboutTheServer offers the local daemon to a machine that has not
// chosen, and writes the answer to `[service] mode`.
//
// A question rather than a default, because the daemon is a change to how
// this repository's commands run and nobody should discover it by
// noticing a socket. Asked here because `init` is the one command a person
// runs deliberately, once, while setting the repository up.
//
// answer is what the person said, or nil when there was nobody to ask —
// over the API, in CI, or with a pipe on stdin. Nobody to ask means
// nothing is written: a repository must never acquire a daemon because a
// script ran init.
func AskAboutTheServer(root string, answer *string, stdout io.Writer) error {
	if config.Load(root).ServiceMode != "off" {
		// Already chosen. Asking again would be a second chance to change
		// a decision that lives in a tracked file, which is where it can
		// be changed properly.
		return nil
	}
	if answer == nil {
		return nil
	}
	if !wantsServer(*answer) {
		fmt.Fprintln(stdout, "procoder init: commands run in-process (the default) — `[service] mode` is unchanged")
		return nil
	}

	raw, err := store.LoadDoc(root, ".procoder/config.toml")
	if err != nil {
		raw = nil
	}
	body := string(raw)
	if strings.Contains(body, "[service]") {
		body = strings.Replace(body, "[service]", "[service]\nmode = \"local\"", 1)
	} else {
		if body != "" && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		if body != "" {
			body += "\n"
		}
		body += "[service]\n# Commands answer over a local socket; the CLI is unchanged.\nmode = \"local\"\n"
	}
	if err := store.SaveDoc(root, ".procoder/config.toml", []byte(body)); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "procoder init: [service] mode = \"local\" — commands answer over the local socket where one is running")
	return nil
}

// wantsServer is the whole definition of the answer: "server" or "local",
// and nothing else. Anything unrecognised is the default, because a
// machine that acquires a daemon from a typo is worse than one that has
// to answer the question twice.
func wantsServer(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "server", "local":
		return true
	}
	return false
}

// ServerQuestion is the text `init` puts to a person.
const ServerQuestion = "procoder init: run commands through a local server, or as the CLI?\n" +
	"  cli     (default) every command runs in-process, as it does today\n" +
	"  server  a local daemon answers on a socket only you can open; the CLI still works\n" +
	"[cli/server]"
