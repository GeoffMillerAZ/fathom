package multi

// Run has a signature over several lines.
func Run(
	name string,
	count int,
) (
	string,
	error,
) {
	return name, nil
}

type Runner interface {
	Do(
		x int,
	) error
}
