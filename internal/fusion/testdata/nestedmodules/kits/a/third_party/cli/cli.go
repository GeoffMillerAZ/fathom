// Package cli is a cobra-shaped command framework with no dependency: a
// command's behaviour is a function value held in its RunE field, called
// only through that field.
package cli

import "os"

// Command is one verb of a command tree.
type Command struct {
	Use      string
	RunE     func(cmd *Command, args []string) error
	children []*Command
}

// AddCommand attaches sub-commands.
func (c *Command) AddCommand(subs ...*Command) { c.children = append(c.children, subs...) }

// Execute runs the sub-command os.Args names and returns the exit code.
func (c *Command) Execute() int {
	args := os.Args[1:]
	if len(args) == 0 {
		return 2
	}
	for _, sub := range c.children {
		if sub.Use == args[0] && sub.RunE != nil {
			if err := sub.RunE(sub, args[1:]); err != nil {
				return 1
			}
			return 0
		}
	}
	return 2
}
