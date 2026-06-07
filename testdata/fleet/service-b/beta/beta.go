// Package beta is a tiny fixture service that calls an exported symbol from
// service-a/alpha.
package beta

import "github.com/geoffmilleraz/fleet/service-a/alpha"

// Hello calls alpha.Greet from service-a.
func Hello() string {
	return alpha.Greet("world")
}
