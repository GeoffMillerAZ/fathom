package typedfixture

// Direct calls another plain function directly — the simplest EdgeDirect case.
func Direct() int {
	return helper()
}

func helper() int {
	return 1
}
