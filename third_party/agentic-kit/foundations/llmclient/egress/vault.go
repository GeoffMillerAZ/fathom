package llmegress

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/tokenization"
)

// VaultAction controls what GateFromVault does when a rule matches.
type VaultAction string

const (
	// VaultActionTokenize is the default: detected values are replaced with
	// opaque tokens minted via vault.Tokenize (random) or the deterministic
	// variant when WithDeterministicTokens is active.
	VaultActionTokenize VaultAction = "tokenize"

	// VaultActionBlock refuses egress entirely when the pattern matches any
	// portion of the outbound text. The Transformer returns a *BlockedError;
	// the inner client is never invoked (fail closed). Use this for content
	// classes that must never cross the trust boundary under any tokenization.
	VaultActionBlock VaultAction = "block"
)

// BlockedError is the typed error returned by a GateFromVault Transformer
// when a rule with Action==VaultActionBlock matches outbound text.
//
// It wraps no underlying cause: the block IS the cause. Use errors.As to
// detect it; IsBlocked is a convenience wrapper.
type BlockedError struct {
	// RuleIndex is the zero-based index of the blocking rule in the VaultRule
	// slice passed to GateFromVault.
	RuleIndex int
	// Class is the TokenClass of the blocking rule.
	Class tokenization.TokenClass
}

// Error implements the error interface, naming the blocking rule and token
// class.
func (e *BlockedError) Error() string {
	return fmt.Sprintf("llmegress: egress blocked by rule %d (class %s): content in this class may not leave the trust boundary", e.RuleIndex, e.Class)
}

// IsBlocked reports whether err (or any error it wraps) is a *BlockedError.
func IsBlocked(err error) bool {
	var be *BlockedError
	return errors.As(err, &be)
}

// VaultRule pairs a detection pattern with the tokenization class recorded on
// the values it detects. The catalog of rules is caller-supplied data — which
// shapes count as sensitive, and how they classify, is an org decision this
// package refuses to hard-code.
type VaultRule struct {
	// Pattern is a Go regexp matched against outbound text. The ENTIRE match
	// is acted on; capture groups get no special treatment.
	Pattern string

	// Class is the tokenization.TokenClass recorded on vaulted matches. It
	// drives the vault's detokenize access policy and is embedded in the
	// token string.
	Class tokenization.TokenClass

	// Action controls whether a match causes tokenization (default) or
	// outright egress refusal. A zero value is treated as VaultActionTokenize.
	Action VaultAction
}

// DeterministicVault is an optional extension of Vault for implementations
// that support value-stable per-namespace HMAC token minting. GateFromVault
// uses it when WithDeterministicTokens is active. If the supplied Vault does
// not implement DeterministicVault, WithDeterministicTokens silently falls
// back to random tokens (the caller supplied a vault that cannot do better).
//
// tokenization.MemoryVault satisfies this interface via its
// TokenizeDeterministic method.
type DeterministicVault interface {
	tokenization.Vault
	// TokenizeDeterministic stores value under a deterministic HMAC-derived
	// token (same value+class+namespace → same token across calls) and
	// returns the token. On success the vault can Detokenize it; the action
	// is audited identically to Tokenize. Every call for the same triple is
	// idempotent from the caller's perspective: the same token value is
	// returned and, if the value was already stored under that token, no new
	// entry is created (existing value is confirmed equal; a mismatch is an
	// error).
	TokenizeDeterministic(value string, class tokenization.TokenClass, namespace string, by aiprovenance.Author) (tokenization.Token, error)
}

// VaultGateOption configures the Transformer returned by GateFromVaultOpts.
type VaultGateOption func(*vaultGateOptions)

type vaultGateOptions struct {
	deterministic bool
}

// WithDeterministicTokens requests value-stable HMAC token minting: the same
// plaintext value in the same namespace always produces the same token value.
// This enables:
//
//   - Stable model references: the same entity gets the same token across
//     multiple calls, so the model can correlate records within a session.
//   - Reproducible PromptHash: gated text containing deterministic tokens
//     hashes identically across calls for the same input.
//
// Trade-off: deterministic tokens LEAK EQUALITY. Two requests that tokenize
// the same value produce identical token strings in the outbound text, so an
// observer of the outbound stream can infer that two occurrences refer to the
// same underlying value. This is acceptable (and useful) for join use-cases;
// do not use it where even equality must be hidden — use the default random
// tokens instead.
//
// The vault supplied to GateFromVaultOpts must implement DeterministicVault
// for this option to take effect. If the vault only implements the base Vault
// interface, random tokens are minted as usual (the option is a no-op for
// non-deterministic vaults).
func WithDeterministicTokens() VaultGateOption {
	return func(o *vaultGateOptions) { o.deterministic = true }
}

// GateFromVault returns a Transformer that detects sensitive values in
// outbound text via the rules' regexps and acts on each match as configured
// by the rule's Action:
//
//   - VaultActionTokenize (default): replace with an opaque token minted via
//     vault.Tokenize. The raw value never leaves; an authorized principal can
//     later recover it. Every tokenization is audited.
//   - VaultActionBlock: any match causes an immediate egress refusal with
//     *BlockedError; the inner client is never invoked (fail closed).
//
// Rules are applied in order over the running result. namespace scopes the
// minted tokens (tenant/case/domain isolation). by is recorded on tokenize
// audit events.
//
// Fail closed: any vault error aborts the transformation, which aborts the
// gated call. Detection is only as good as the rules — encoded, split, or
// novel-format values that match no pattern pass through untouched.
//
// The returned Transformer is safe for concurrent use when vault is (the
// Vault contract requires it). It ignores ctx: vault work is synchronous and
// in-process.
func GateFromVault(vault tokenization.Vault, namespace string, by aiprovenance.Author, rules ...VaultRule) (Transformer, error) {
	return GateFromVaultOpts(vault, namespace, by, nil, rules...)
}

// GateFromVaultOpts is the full-option variant of GateFromVault. Pass nil
// opts to get identical behaviour to GateFromVault.
func GateFromVaultOpts(vault tokenization.Vault, namespace string, by aiprovenance.Author, opts []VaultGateOption, rules ...VaultRule) (Transformer, error) {
	if vault == nil {
		return nil, fmt.Errorf("llmegress: GateFromVault: vault must not be nil")
	}
	if namespace == "" {
		return nil, fmt.Errorf("llmegress: GateFromVault: namespace must not be empty")
	}

	var o vaultGateOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Resolve optional deterministic-vault extension once at construction.
	var detVault DeterministicVault
	if o.deterministic {
		if dv, ok := vault.(DeterministicVault); ok {
			detVault = dv
		}
		// If the vault does not satisfy DeterministicVault, fall through to
		// random tokens silently — the caller supplied a non-deterministic vault.
	}

	rs := slicesCloneRules(rules)
	compiled := make([]*regexp.Regexp, len(rs))
	for i, r := range rs {
		if err := r.Class.Validate(); err != nil {
			return nil, fmt.Errorf("llmegress: GateFromVault: rule %d: %w", i, err)
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("llmegress: GateFromVault: rule %d: invalid pattern %q: %w", i, r.Pattern, err)
		}
		compiled[i] = re
	}

	return TransformerFunc(func(_ context.Context, text string) (string, error) {
		for i, re := range compiled {
			rule := rs[i]
			class := rule.Class

			// Block action: any match in this field is an egress refusal.
			if rule.Action == VaultActionBlock {
				if re.MatchString(text) {
					return "", &BlockedError{RuleIndex: i, Class: class}
				}
				continue
			}

			// Tokenize action (default or explicit).
			out, err := replaceAllMatches(re, text, func(match string) (string, error) {
				if detVault != nil {
					tok, err := detVault.TokenizeDeterministic(match, class, namespace, by)
					if err != nil {
						return "", err
					}
					return tok.Value, nil
				}
				tok, err := vault.Tokenize(match, class, namespace, by)
				if err != nil {
					return "", err
				}
				return tok.Value, nil
			})
			if err != nil {
				return "", fmt.Errorf("llmegress: GateFromVault: rule %d (class %s): %w", i, class, err)
			}
			text = out
		}
		return text, nil
	}), nil
}

// replaceAllMatches rebuilds text with every match of re replaced by
// repl(match). It works by index, not Regexp.ReplaceAllString, so replacement
// values are inserted verbatim (no $-reference expansion) and repl can fail.
func replaceAllMatches(re *regexp.Regexp, text string, repl func(match string) (string, error)) (string, error) {
	locs := re.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return text, nil
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(text[last:loc[0]])
		r, err := repl(text[loc[0]:loc[1]])
		if err != nil {
			return "", err
		}
		b.WriteString(r)
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String(), nil
}

// slicesCloneRules defensively copies the variadic rule slice so later caller
// mutation cannot change the gate's behavior.
func slicesCloneRules(rules []VaultRule) []VaultRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]VaultRule, len(rules))
	copy(out, rules)
	return out
}
