package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// buildRoutes builds the in-process service registry, the budgets, the
// console dependencies and the full HTTP handler, then starts the managed
// daemon's self-restart watch.
func (r *serveRun) buildRoutes() {
	flog, outLog := r.flog, r.outLog
	c, srv, backend, fleet := r.c, r.srv, r.backend, r.fleet
	// RT-61 (§3.3): the registry read goes through admission as
	// console-inprocess.
	listServicesClass, _ := srv.ToolCostClass("list_services", nil)
	// A refused read (any fleet call running) answers the last good
	// projection, disclosed as stale (§3.4).
	registry := newLastGoodRegistry(func(ctx context.Context) (map[string]svcInfo, error) {
		var out map[string]svcInfo
		err := inProcessToolCall(ctx, "list_services", listServicesClass, func(ctx context.Context) error {
			out = svcInfoFromSummaries(mcp.ServiceSummaries(ctx, backend, fleet))
			return nil
		})
		return out, err
	})
	r.registry = registry
	// BA-9: budget defaults are project-settable. The `budgets` block of
	// <cwd>/.fathom/capabilities.cue (the daemon's working repo) and
	// ~/.fathom/capabilities.cue sizes each tool class in tokens, resolved
	// project → user → built-in inside the server. A broken block is
	// reported loudly and the measured built-in stands for THAT scope —
	// never a silent partial read. Restart-to-apply, like config.toml.
	budgets, budgetErrs := capability.LoadBudgetResolution(capability.DefaultUserPath(), capability.DefaultProjectPath())
	for _, e := range budgetErrs {
		flog.Warn(fmt.Sprintf("%v — built-in budget defaults stand for that scope", e))
	}
	srv.SetBudgets(budgets)
	// The console's dependencies (vision 35): serve-http owns them so the
	// event feeders below publish to the same bus GET /events streams.
	consoleDeps := newConsoleDeps(c, "http://"+r.addr, time.Now())
	r.consoleDeps = consoleDeps
	consoleDeps.storePath = absStorePath(r.storePath)
	if n := r.loaded.Config.Console.ConserveMaxFiles; n > 0 {
		consoleConserveMaxFiles = n // [console] conserve_max_files (RT-118)
	}
	inProcessReg.Store(&inProcessRegistry{self: "http://" + r.addr, services: registry.services, stale: registry.staleNote,
		statusz: func(ctx context.Context) StatuszReport {
			return buildStatuszReport(ctx, c, consoleDeps.startTime, time.Now())
		},
		rpc: srv.HandleRequest})
	r.handler = newHTTPHandlerLoggedDeps(srv, c, r.addr, r.ports, flog, consoleDeps)

	// Adopt a new binary without a human (selfrestart.go). launchd
	// relaunches the job after the adoption exit (75, RT-11), which is what
	// lets an agent complete a deploy end to end, rather than building a
	// fix and then waiting for someone to run `launchctl`. Only the managed
	// daemon watches (RT-12): a scratch serve-http never adopts, never
	// hashes its binary and never writes the operator's lifecycle ledger.
	startSelfRestart(outLog, selfRestartStart{
		managedFlag:     r.managedFlag,
		xpcServiceName:  os.Getenv("XPC_SERVICE_NAME"),
		installedLabels: installedAgentLabels,
		disabled:        selfRestartDisabled(),
		executable:      os.Executable,
		watch: func(path string, cfg selfRestartConfig) {
			//fathom:tenancy-allow bare-go daemon lifetime goroutine started once at serve-http startup and stopped by the drain
			go watchOwnBinary(path, outLog, cfg)
		},
	})
}
