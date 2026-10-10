// Package shared is an in-repo module outside kits/a that kits/a reaches
// through a replace directive (=> ../../shared).
package shared

// Banner is the text the doctor leaf prints.
func Banner() string { return "doctor" }
