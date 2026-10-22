package main

import (
	"os"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// parseShellCommand keeps configuration at the application boundary. The
// lexer remains pure; hooks supply their own cwd rather than the process's.
// No cache, daemon request or command execution is involved.
func parseShellCommand(command string, roots ...string) shellcmd.Command {
	root := ""
	if len(roots) > 0 {
		root = roots[0]
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	if top := gitToplevelByStat(root); top != "" {
		root = top
	}
	l, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	if err != nil {
		return shellcmd.Command{Unreadable: true, Reason: "shell wrapper configuration: " + err.Error()}
	}
	return shellcmd.ParseCommand(command, l.Config.Shell.Wrappers...)
}
