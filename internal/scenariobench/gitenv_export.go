package scenariobench

// GitEnv is gitEnv for callers outside this package: os.Environ() with the git hook variables scrubbed and LEFTHOOK=0 (I14, the 2026-09-14 incident).
func GitEnv() []string { return gitEnv() }
