package toolregistry

import (
	"encoding/json"
	"errors"
	"fmt"

	toolexecutionpolicy "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/environment/tool-execution-policy"
)

// PolicyFromExecutionPolicy adapts a tool-execution-policy PolicySet — which
// decides on (tool name, environment name), deny-by-default — into this
// package's Policy by fixing the environment name at construction time:
//
//	reg := toolregistry.New(toolregistry.Options{
//		Policy: toolregistry.PolicyFromExecutionPolicy(policySet, "prod"),
//	})
//
// An allowed Decision returns nil. A denied Decision returns an error
// carrying the Decision.Reason, which Dispatch surfaces to the model as an
// is_error tool_result. A Decide error (empty tool or environment name) also
// denies — safe by default. Tool input is ignored: the execution policy is
// name- and environment-scoped, not argument-aware.
func PolicyFromExecutionPolicy(ps *toolexecutionpolicy.PolicySet, env string) Policy {
	return func(name string, _ json.RawMessage) error {
		decision, err := ps.Decide(name, env)
		if err != nil {
			return fmt.Errorf("execution policy: %w", err)
		}
		if !decision.Allowed {
			return errors.New(decision.Reason)
		}
		return nil
	}
}
