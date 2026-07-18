package toolregistry

import (
	"errors"
	"fmt"
	"strings"
)

// Capability names a security-relevant class of access a tool exercises. A
// toolset that holds all three of CapReadsUntrusted, CapReadsPrivate, and
// CapWritesExternal completes the lethal trifecta: an agent that reads
// untrusted content (prompt-injection surface), has access to private data
// (exfiltration target), and can write externally (exfiltration channel) can
// be steered into exfiltration by prompt injection alone.
//
// Taxonomy is SELF-DECLARED. A lying tool defeats detection; this is
// defense-in-depth, not a security boundary. The value is making the risk
// visible at registration time — before any token is spent — so operators
// can audit toolsets and require explicit overrides.
type Capability string

const (
	// CapReadsUntrusted declares that the tool fetches or reads content from
	// untrusted sources (web pages, user-supplied files, email bodies, tool
	// outputs from external APIs). Such content is a prompt-injection surface.
	CapReadsUntrusted Capability = "reads_untrusted"

	// CapReadsPrivate declares that the tool can access private or sensitive
	// data (credentials, personal data, internal documents, secrets). This
	// makes the tool an exfiltration target.
	CapReadsPrivate Capability = "reads_private"

	// CapWritesExternal declares that the tool can write data outside the
	// agent's own context (send email/Slack/HTTP, write files, call external
	// APIs with side effects). This is the exfiltration channel.
	CapWritesExternal Capability = "writes_external"
)

// CapabilitySet is the set of capabilities declared for a single tool.
// A nil CapabilitySet is UNKNOWN — meaning "not declared at all" — and is
// treated differently from an empty CapabilitySet (which means "declared
// as having no relevant capabilities"). Unknown is treated as potentially
// all three capabilities under strict trifecta mode (fail-closed means
// suspicion, not trust).
//
// Construct with NewCapabilitySet or the zero-value nil pointer (unknown).
// Use DeclareNone to create an explicitly empty (declared-clean) set.
type CapabilitySet map[Capability]struct{}

// NewCapabilitySet returns a CapabilitySet containing exactly the given
// capabilities. Duplicate values are silently de-duplicated.
func NewCapabilitySet(caps ...Capability) CapabilitySet {
	s := make(CapabilitySet, len(caps))
	for _, c := range caps {
		s[c] = struct{}{}
	}
	return s
}

// DeclareNone returns an explicitly empty CapabilitySet, recording that the
// tool has been consciously declared to have none of the tracked capabilities.
// This is distinct from nil (unknown): DeclareNone is treated as safe under
// strict mode; nil is treated as potentially all capabilities.
func DeclareNone() CapabilitySet {
	return make(CapabilitySet)
}

// Has reports whether c is present in the set.
func (s CapabilitySet) Has(c Capability) bool {
	if s == nil {
		return false
	}
	_, ok := s[c]
	return ok
}

// union returns a new CapabilitySet that is the union of s and other. A nil
// receiver is treated as empty for union purposes (not as unknown).
func (s CapabilitySet) union(other CapabilitySet) CapabilitySet {
	out := make(CapabilitySet, len(s)+len(other))
	for c := range s {
		out[c] = struct{}{}
	}
	for c := range other {
		out[c] = struct{}{}
	}
	return out
}

// completesTrifecta reports whether the capability set contains all three
// trifecta capabilities.
func (s CapabilitySet) completesTrifecta() bool {
	return s.Has(CapReadsUntrusted) && s.Has(CapReadsPrivate) && s.Has(CapWritesExternal)
}

// TrifectaMode controls how the registry responds when the registered toolset
// completes the lethal trifecta (CapReadsUntrusted + CapReadsPrivate +
// CapWritesExternal in the same registry).
type TrifectaMode int

const (
	// TrifectaPermissive (zero value) allows trifecta completion with no
	// intervention. Suitable for development and for registries where operators
	// have independently verified the toolset. Trifecta detection is still
	// performed and reported via TrifectaOptions.OnComplete, but registration
	// and dispatch proceed normally.
	TrifectaPermissive TrifectaMode = 0

	// TrifectaStrict refuses registration of any tool whose declared capabilities
	// (or unknown capabilities treated as all-three) would complete the trifecta,
	// unless a TrifectaOverride is supplied in TrifectaOptions. This is the
	// fail-closed posture: the trifecta is blocked before any token is spent.
	TrifectaStrict TrifectaMode = 1
)

// TrifectaOverride is an explicit, named, human-readable record authorizing
// trifecta completion. It provides provenance — who authorized it and why —
// rather than being a bare boolean. A non-nil Override in TrifectaOptions
// permits trifecta completion under TrifectaStrict mode; the record is
// returned by Registry.TrifectaOverrideRecord so callers can log or audit it.
//
// Residual risk: taxonomy is self-declared; a lying tool defeats trifecta
// detection. The override record documents that the operator reviewed the
// risk and accepted it; it does not change the underlying risk.
type TrifectaOverride struct {
	// AuthorizedBy identifies the human or system that reviewed and accepted
	// the trifecta risk. Must be non-empty.
	AuthorizedBy string

	// Reason is a human-readable explanation of why the trifecta is acceptable
	// for this toolset. Must be non-empty.
	Reason string
}

// validate returns an error if the override is missing required fields.
func (o TrifectaOverride) validate() error {
	if strings.TrimSpace(o.AuthorizedBy) == "" {
		return errors.New("TrifectaOverride.AuthorizedBy must be non-empty")
	}
	if strings.TrimSpace(o.Reason) == "" {
		return errors.New("TrifectaOverride.Reason must be non-empty")
	}
	return nil
}

// TrifectaOptions configures trifecta detection and enforcement for a Registry.
type TrifectaOptions struct {
	// Mode controls enforcement behavior. Zero value is TrifectaPermissive.
	Mode TrifectaMode

	// Override, when non-nil, authorizes trifecta completion under
	// TrifectaStrict. Required fields: AuthorizedBy and Reason. Ignored under
	// TrifectaPermissive. An invalid Override (missing fields) is treated as
	// absent — strict mode still blocks.
	Override *TrifectaOverride

	// OnComplete, when non-nil, is called with the completing tool's name when
	// a Register call would complete the trifecta. It is called under the
	// registry write-lock — it must not call back into the registry. Useful for
	// telemetry and audit logging in both strict and permissive modes. nil is
	// safe.
	OnComplete func(completingToolName string)
}

// ErrTrifectaBlocked is returned by Register when TrifectaStrict mode would be
// violated. The error carries the name of the tool whose registration was
// refused and describes the missing override mechanism.
type ErrTrifectaBlocked struct {
	// ToolName is the tool whose registration was blocked.
	ToolName string
	// CapabilitiesUnion is the union of capabilities that completed the trifecta,
	// formatted for human reading.
	CapabilitiesUnion string
}

// Error implements the error interface, naming the blocked tool and the
// capability union that would have completed the lethal trifecta.
func (e *ErrTrifectaBlocked) Error() string {
	return fmt.Sprintf(
		"toolregistry: Register: tool %q blocked — registering it would complete the lethal trifecta "+
			"(reads_untrusted + reads_private + writes_external = exfiltration risk); "+
			"supply a TrifectaOverride in TrifectaOptions to authorize this toolset explicitly. "+
			"Capabilities union: %s",
		e.ToolName, e.CapabilitiesUnion,
	)
}
