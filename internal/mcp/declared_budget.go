package mcp

import "context"

// DeclaredBudgetHeader carries a machine caller's dispatch byte budget
// (O1014, O1169): a decimal byte count for the answer of a tool that
// honours it (toolDescriptor.declaredBudget; verify_plan today). The
// dispatch budget (budgetedHandler, defaultCharBudget) protects an agent's
// context; a caller that renders the answer itself, as `fathom verify` and
// `fathom plans status` do, declares what it can read instead of paying
// one call per cut. It is a header, not a tool argument, so no agent
// surface's schema or default answer changes; X-Fathom-Deadline is the
// same kind of per-request declaration.
const DeclaredBudgetHeader = "X-Fathom-Max-Bytes"

// MaxDeclaredBudget caps a declaration: the answer stays finite whatever a
// caller declares.
const MaxDeclaredBudget = 4 << 20

type declaredBudgetKey struct{}

// WithDeclaredBudget carries a caller's declared dispatch budget of n bytes,
// held to MaxDeclaredBudget. A non-positive n declares nothing.
func WithDeclaredBudget(ctx context.Context, n int) context.Context {
	if n <= 0 {
		return ctx
	}
	return context.WithValue(ctx, declaredBudgetKey{}, min(n, MaxDeclaredBudget))
}

// declaredBudgetFrom is ctx's declared dispatch budget; 0 for none.
func declaredBudgetFrom(ctx context.Context) int {
	n, _ := ctx.Value(declaredBudgetKey{}).(int)
	return n
}
