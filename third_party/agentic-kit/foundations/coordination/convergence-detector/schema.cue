// Package convergencedetector defines the CUE contract for convergence detection.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package convergencedetector

// #Verdict enumerates the possible convergence decisions.
#Verdict: "converged" | "exploring" | "abandoned"

// #Signal is a single result signal in the convergence stream.
#Signal: {
	// Value is the string-keyed result value from a worker.
	Value: string & !=""
}

// #Policy configures the convergence detector.
#Policy: {
	// Window is the number of most-recent signals the detector considers.
	// Must be at least 1.
	Window: >=1 & int

	// Threshold is the minimum fraction of signals in the window that must
	// share the same value to declare convergence. Range: (0, 1].
	Threshold: >0 & <=1 & number

	// AbandonAfter is the maximum number of signals allowed before the
	// detector declares abandoned if convergence has not been reached.
	// Must be at least Window.
	AbandonAfter: >=1 & int
}

// #DetectorState is the observable state of a running Detector.
#DetectorState: {
	// SignalCount is the total number of signals received so far.
	SignalCount: >=0 & int

	// Verdict is the most recent convergence decision.
	Verdict: #Verdict
}
