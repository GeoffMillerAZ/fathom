package main

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

var shellConfigLoad = config.Load

type shellConfigPaths struct{ user, project string }

// Declarations live only for this hook process, keyed by both config paths.
// This avoids repeating CUE validation on each lex; errors also memoize the
// zero-declaration fallback. Nothing persists or refreshes in the background.
var shellDeclarations = struct {
	sync.Mutex
	byPaths map[shellConfigPaths][]shellcmd.Wrapper
}{byPaths: make(map[shellConfigPaths][]shellcmd.Wrapper)}

// parseShellCommand keeps configuration at the application boundary. The
// lexer remains pure; callers supply their own cwd rather than the process's.
// No root means no declarations, including declarations from the user config.
func parseShellCommand(command string, roots ...string) shellcmd.Command {
	if len(roots) == 0 || roots[0] == "" {
		return shellcmd.ParseCommand(command)
	}
	root := roots[0]
	if top := gitToplevelByStat(root); top != "" {
		root = top
	}
	paths := shellConfigPaths{config.DefaultUserPath(), projectdir.Config.Path(root)}
	shellDeclarations.Lock()
	wrappers, found := shellDeclarations.byPaths[paths]
	if !found {
		if l, err := shellConfigLoad(paths.user, paths.project); err == nil {
			wrappers = l.Config.Shell.Wrappers
		}
		shellDeclarations.byPaths[paths] = wrappers
	}
	shellDeclarations.Unlock()
	return shellcmd.ParseCommand(command, wrappers...)
}
