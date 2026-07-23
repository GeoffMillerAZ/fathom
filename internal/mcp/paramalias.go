// Package mcp — additive parameter aliasing (P1-1, closing O12(b): the
// four hot tools each name their primary argument differently, which cost
// the bench transcript ~5 error turns of guessed -32602 params).
//
// find_symbol wants "name", references wants "query", query_symbols wants
// "concern" — symbol_context (and find_callers) already want "symbol". Each
// of the other three additionally accepts "symbol" as an alias for its own
// primary argument: the native name stays primary (schemas still declare it
// "required"; toolDescriptor.aliases tells dispatch-level required-field
// validation the alias satisfies it too — see dispatch.go), and a call
// naming both is only an error when the two values actually disagree.
package mcp

import "encoding/json"

// resolveAliasedString extracts a string parameter that may be supplied
// under its native name or an additive alias. Both may be given only if
// they agree — both given with DIFFERENT values is a loud, named error
// (never "last one wins" or "alias wins"). raw is the tool's arguments
// object, already unmarshalled into a map of raw JSON values (so callers
// can reuse the same decode for their other fields). Returns ("", nil) when
// neither is present — each handler's own existing "X required" check
// catches that, unchanged.
func resolveAliasedString(raw map[string]json.RawMessage, toolName, primary, alias string) (string, *rpcError) {
	pRaw, pOK := raw[primary]
	aRaw, aOK := raw[alias]
	if !pOK && !aOK {
		return "", nil
	}
	var pVal, aVal string
	if pOK {
		if err := json.Unmarshal(pRaw, &pVal); err != nil {
			return "", &rpcError{Code: ErrCodeParams, Message: toolName + ": invalid args"}
		}
	}
	if aOK {
		if err := json.Unmarshal(aRaw, &aVal); err != nil {
			return "", &rpcError{Code: ErrCodeParams, Message: toolName + ": invalid args"}
		}
	}
	switch {
	case pOK && aOK:
		if pVal != aVal {
			return "", &rpcError{
				Code:    ErrCodeParams,
				Message: toolName + ": both \"" + primary + "\" and \"" + alias + "\" were provided with different values",
			}
		}
		return pVal, nil
	case pOK:
		return pVal, nil
	default: // aOK
		return aVal, nil
	}
}
