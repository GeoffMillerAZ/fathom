package mcp

import "context"

// loadNavGraphs is the tests' shorthand for loadNavGraphsErr: nil on any
// load error. Product code never reads a failed load as nil (EW-78, O1013:
// the nil read answered "no symbols for service" for an indexed service);
// every product reader takes loadNavGraphsErr and names the error.
func (s *Server) loadNavGraphs(ctx context.Context, backend QueryInterface, service string) *navGraphs {
	g, err := loadNavGraphsErr(ctx, backend, service)
	if err != nil {
		return nil
	}
	return g
}
