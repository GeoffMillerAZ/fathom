package run

// Runner runs.
type Runner struct{}

// Exec executes.
func (r Runner) Exec(cmd string) error {
	if cmd == "" {
		return nil
	}
	return exec(cmd)
}

func exec(cmd string) error { return nil }
