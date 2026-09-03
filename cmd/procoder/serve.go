package main

import (
	"fmt"
	"procoder/internal/api"
)

// serveCmd runs the daemon in the foreground until its listener closes.
//
// Foreground deliberately: a command that daemonised itself would own a
// process nobody can see, and the one thing worse than no daemon is one
// whose lifetime belongs to nothing. Whatever starts it — a shell, the
// SessionStart hook — owns it, exactly as `procoder run` refuses to own a
// server's lifetime for the same reason.
func (s session) serveCmd(args []string) int {
	exec := false
	socket := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exec":
			exec = true
		case "--socket":
			if i+1 >= len(args) {
				s.out("serve: --socket takes a path")
				return 2
			}
			i++
			socket = args[i]
		default:
			s.out("serve: unknown argument " + args[i])
			return 2
		}
	}

	if socket == "" {
		var err error
		if exec {
			socket, err = api.ExecSocket()
		} else {
			socket, err = api.WorkSocket()
		}
		if err != nil {
			s.out(err.Error())
			return 1
		}
	}

	srv := &api.Server{Run: apiRunner, Version: version, Exec: exec, Notice: s.stderr}
	l, err := srv.Listen(socket)
	if err != nil {
		s.out(err.Error())
		return 1
	}
	defer l.Close()

	door := "work"
	if exec {
		door = "exec"
	}
	s.out(fmt.Sprintf("procoder serve: %s socket at %s (version %s)", door, socket, version))
	srv.Accept(l)
	return 0
}
