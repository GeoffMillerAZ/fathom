package typedfixture

// Pinger mirrors the ADR-021 §3 health-check-style Ping residue case.
type Pinger interface {
	Ping() error
}

// LocalPing is the first concrete implementer.
type LocalPing struct{}

// Ping implements Pinger.
func (LocalPing) Ping() error { return nil }

// RemotePing is the second concrete implementer.
type RemotePing struct{ addr string }

// Ping implements Pinger.
func (r RemotePing) Ping() error { return nil }

// CheckPing calls Ping through the interface — the Ping dispatch call site.
func CheckPing(p Pinger) error {
	return p.Ping()
}
