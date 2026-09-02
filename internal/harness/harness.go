// Package harness is the seam between fathom's payloads and the coding
// harness that carries them (docs/vision/25-harness-port.md §2, ADR-029).
//
// A harness is DATA — a Descriptor: identity, detection markers, the
// instruction files it reads, its hook file(s), the event bindings with
// their stdin/stdout field maps, the tool names it reports, its fail mode,
// the scope-merge rule, skills directories, the MCP target, the env
// contract, transcript availability and what the descriptor was verified
// against. The only per-harness CODE is a Codec (codec.go): envelope in,
// envelope out. Claude Code's descriptor reproduces the hook table
// `fathom init` has always written, event for event, and the existing
// goldens hold it byte-identical (rule 1 of the vision: any changed Claude
// Code golden byte means this is not a seam).
//
// Determinism: every accessor returns descriptors in registry order (a
// fixed slice, never a map walk), Detect returns ids in that same order,
// and DemotionDue takes its clock as an argument.
package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Registry ids. The string form is the CLI vocabulary (`--harness <id>`).
const (
	ClaudeCode = "claude-code"
	Codex      = "codex"
	GeminiCLI  = "gemini-cli"
	Agents     = "agents" // the generic pull tier: AGENTS.md + MCP + skills, no hooks
)

// Capability names what fathom can deliver to a session, independent of
// any harness (vision 25 §2.1).
type Capability string

const (
	CapPrime               Capability = "prime"
	CapObligationsPostEdit Capability = "obligations-post-edit"
	CapGuardPreEdit        Capability = "guard-pre-edit"
	CapGuardPreShell       Capability = "guard-pre-shell"
	CapDiscoveryObserve    Capability = "discovery-observe"
	CapStopVerify          Capability = "stop-verify"
	CapDepositSessionEnd   Capability = "deposit-session-end"
	CapDepositPreCompact   Capability = "deposit-pre-compact"
	CapMCPTools            Capability = "mcp-tools"
	CapSkills              Capability = "skills"
	CapStanza              Capability = "stanza"
	CapVerifyGate          Capability = "verify-gate"
	CapClaimGate           Capability = "claim-gate"
	CapSessionLedger       Capability = "session-ledger"
	// CapStanzaLoaded is harness-side evidence that an instruction file
	// reached a session (vision 26 §4, PC-23): a silent beat on the
	// harness's "instructions loaded" event, read by doctor stanza_loaded.
	CapStanzaLoaded Capability = "stanza-loaded"
	// CapConfigAudit is the custodian audit run on a config change
	// (vision 26 §4, PC-24): a beat and an attention line, never a block.
	CapConfigAudit Capability = "config-audit"
)

// Capabilities is the fixed rendering order of every capability — the
// order RenderCapabilityTable and the JSON export use.
var Capabilities = []Capability{
	CapPrime, CapObligationsPostEdit, CapGuardPreEdit, CapGuardPreShell,
	CapDiscoveryObserve, CapStopVerify, CapDepositSessionEnd, CapDepositPreCompact,
	CapMCPTools, CapSkills, CapStanza, CapVerifyGate, CapClaimGate, CapSessionLedger,
	CapStanzaLoaded, CapConfigAudit,
}

// Vehicle is HOW a capability reaches a session on one harness.
type Vehicle string

const (
	VehicleHookInject Vehicle = "hook-inject" // a hook that injects context
	VehicleHookBlock  Vehicle = "hook-block"  // a hook that can block (fathom uses it only behind --block)
	VehicleHookSilent Vehicle = "hook-silent" // a hook that writes to disk and injects nothing
	VehiclePull       Vehicle = "pull"        // the agent runs a verb (`fathom prime`) or a skill
	VehicleStanza     Vehicle = "stanza"      // standing text in an instruction file
	VehicleMCP        Vehicle = "mcp"         // a tool call
	VehicleGate       Vehicle = "gate"        // git-native (lefthook / CI)
	VehicleAbsent     Vehicle = "absent"      // documented gap, never emulated
)

// Event is fathom's canonical hook event; a binding maps it to the
// harness's own name.
type Event string

const (
	EventSessionStart Event = "session-start"
	EventPreTool      Event = "pre-tool"
	EventPostTool     Event = "post-tool"
	EventStop         Event = "stop"
	EventSessionEnd   Event = "session-end"
	EventPreCompact   Event = "pre-compact"
	// EventInstructionsLoaded fires when the harness loads an instruction
	// file (Claude Code: InstructionsLoaded); observability only.
	EventInstructionsLoaded Event = "instructions-loaded"
	// EventConfigChange fires when a harness configuration file changes
	// mid-session (Claude Code: ConfigChange).
	EventConfigChange Event = "config-change"
)

// ToolClass normalizes a harness's tool names (vision 25 §2.1): the guard
// and the discovery observer branch on the class, never on a literal.
type ToolClass string

const (
	ToolFileEdit      ToolClass = "file-edit"
	ToolShell         ToolClass = "shell"
	ToolDiscoveryRead ToolClass = "discovery-read"
	ToolDiscoveryGrep ToolClass = "discovery-grep"
	ToolDiscoveryGlob ToolClass = "discovery-glob"
	ToolOther         ToolClass = "other"
)

// InstructionFile is one standing-instruction file the harness reads.
type InstructionFile struct {
	Path string `json:"path"`
	// Mode: "native" (the harness reads it by default), "mirror" (fathom
	// writes the same stanza there for harnesses that read that file),
	// "settings-key" (read only when a settings key names it; not written).
	Mode string `json:"mode"`
	Note string `json:"note,omitempty"`
}

// HookFile is one place the harness discovers hooks.
type HookFile struct {
	Path  string `json:"path"`  // "~/" prefix means the user's home
	Scope string `json:"scope"` // project | project-local | user
	// Kind is the wiring editor that addresses entries in this file
	// (internal/wiring KeyKind); "" when no editor exists yet — the file
	// is documented as data and `init --harness` does not write it.
	Kind string `json:"kind,omitempty"`
	// Shape names the documented JSON shape: "claude-settings" is the
	// hooks.<Event>[{matcher, hooks:[{type,command,timeout}]}] block.
	Shape string `json:"shape"`
	Note  string `json:"note,omitempty"`
}

// EventBinding is one row of a harness's hook table: which fathom verb
// runs on which harness event, with the stdin/stdout field maps the
// harness documents for that event. Input/Output map canonical field
// names to the harness's own dotted JSON paths; a field the documentation
// does not show is absent.
type EventBinding struct {
	Capability Capability        `json:"capability"`
	Event      Event             `json:"event"`
	Harness    string            `json:"harness_event"`
	Matcher    string            `json:"matcher,omitempty"`
	Verb       string            `json:"verb"`   // fathom subcommand (+ args), e.g. "edit-hook --pre"
	Marker     string            `json:"marker"` // the substring that identifies fathom's own entry
	TimeoutSec int               `json:"timeout_sec"`
	Vehicle    Vehicle           `json:"vehicle"`
	Input      map[string]string `json:"input,omitempty"`
	Output     map[string]string `json:"output,omitempty"`
	Note       string            `json:"note,omitempty"`
}

// SkillsShape is where the harness discovers skills. Patterns carry
// {{name}}; "~/" means the user's home.
type SkillsShape struct {
	RepoDirs []string `json:"repo_dirs,omitempty"`
	UserDirs []string `json:"user_dirs,omitempty"`
	Variant  string   `json:"variant"` // "SKILL.md" (Agent Skills) — a legacy variant is named in Note
	Verified bool     `json:"verified"`
	Note     string   `json:"note,omitempty"`
}

// MCPTarget is how the harness registers an MCP server.
type MCPTarget struct {
	ConfigFiles     []string `json:"config_files,omitempty"`
	Format          string   `json:"format,omitempty"` // json | toml
	KeyPath         []string `json:"key_path,omitempty"`
	HTTPKey         string   `json:"http_key,omitempty"` // the key that selects streamable HTTP
	AddCommand      string   `json:"add_command"`        // template: {{url}} = daemon base URL
	StdioCommand    string   `json:"stdio_command,omitempty"`
	ToolNamePattern string   `json:"tool_name_pattern,omitempty"`
	Note            string   `json:"note,omitempty"`
}

// EnvContract is the environment the harness exports to processes it
// runs (hooks and the agent's shell).
type EnvContract struct {
	ProjectDir       string   `json:"project_dir,omitempty"`
	SessionID        string   `json:"session_id,omitempty"`
	SessionIDAliases []string `json:"session_id_aliases,omitempty"`
	Present          string   `json:"present,omitempty"` // set whenever the harness runs a process
	Home             string   `json:"home,omitempty"`    // relocates the harness's user dir
	Note             string   `json:"note,omitempty"`
}

// Descriptor is one harness as data.
type Descriptor struct {
	ID      string `json:"id"`
	Display string `json:"display"`
	// Label is the short ledger label (leases, ratings): "claude", "codex".
	Label string `json:"label"`
	// Tier is the degradation ladder rung (vision 25 §1): A hooks +
	// inject, B block-only, C in-process JS only, D none.
	Tier         string            `json:"tier"`
	Detect       []string          `json:"detect"`
	Instructions []InstructionFile `json:"instruction_files"`
	HookFiles    []HookFile        `json:"hook_files,omitempty"`
	Hooks        []EventBinding    `json:"hooks,omitempty"`
	// Tools maps the harness's tool names to classes; unknown names are
	// ToolOther.
	Tools    map[string]ToolClass   `json:"tools,omitempty"`
	Vehicles map[Capability]Vehicle `json:"vehicles"`
	// Notes are per-capability honesty notes rendered beside the vehicle.
	Notes       map[Capability]string `json:"notes,omitempty"`
	FailMode    string                `json:"fail_mode"`
	GatePosture string                `json:"gate_posture"`
	ScopeMerge  string                `json:"scope_merge,omitempty"`
	Skills      SkillsShape           `json:"skills"`
	MCP         MCPTarget             `json:"mcp"`
	Env         EnvContract           `json:"env"`
	// Transcript names the transcript format the session ledger can read;
	// "" makes CapSessionLedger absent by construction (ADR-029 §6).
	Transcript string `json:"transcript,omitempty"`
	// Codec names the wired codec ("claude"); "" means the descriptor is
	// data only and no hook verb speaks this harness yet.
	Codec string `json:"codec,omitempty"`
	// CodecLanded is the ISO date the codec landed — the demotion clock
	// (vision 25 §5) starts here.
	CodecLanded string `json:"codec_landed,omitempty"`
	// VerifiedAgainst is the doc URL + date + CLI version a real session
	// verified this descriptor against, or the literal "unverified".
	VerifiedAgainst string `json:"verified_against"`
}

// Unverified reports whether no real session has verified this descriptor.
func (d Descriptor) Unverified() bool { return d.VerifiedAgainst == "unverified" }

// HasHooks reports whether the descriptor carries any hook binding.
func (d Descriptor) HasHooks() bool { return len(d.Hooks) > 0 }

// HooksWired reports whether fathom actually speaks this harness's hooks
// (a codec exists); a descriptor may document hooks it cannot yet drive.
func (d Descriptor) HooksWired() bool { return d.Codec != "" && len(d.Hooks) > 0 }

// StanzaFiles returns the instruction files fathom writes its stanza
// into: every native or mirror file, in descriptor order.
func (d Descriptor) StanzaFiles() []string {
	var out []string
	for _, f := range d.Instructions {
		if f.Mode == "native" || f.Mode == "mirror" {
			out = append(out, f.Path)
		}
	}
	return out
}

// MCPAddLine renders the descriptor's MCP registration line for url.
func (d Descriptor) MCPAddLine(url string) string {
	return strings.ReplaceAll(d.MCP.AddCommand, "{{url}}", strings.TrimRight(url, "/"))
}

// Binding returns the first binding for a canonical event.
func (d Descriptor) Binding(ev Event) (EventBinding, bool) {
	for _, b := range d.Hooks {
		if b.Event == ev {
			return b, true
		}
	}
	return EventBinding{}, false
}

// IDs returns every registry id in registry order (a fixed priority:
// the verified primary first, then the documented tier-A descriptors,
// then the generic pull tier). Never sorted alphabetically: Detect and
// doctor's auto resolution rely on this order.
func IDs() []string {
	out := make([]string, 0, len(registry))
	for _, d := range registry {
		out = append(out, d.ID)
	}
	return out
}

// All returns a copy of every descriptor in registry order.
func All() []Descriptor {
	out := make([]Descriptor, len(registry))
	copy(out, registry)
	return out
}

// Lookup returns the descriptor for id.
func Lookup(id string) (Descriptor, bool) {
	for _, d := range registry {
		if d.ID == id {
			return d, true
		}
	}
	return Descriptor{}, false
}

// Detect returns the ids of every descriptor whose presence markers exist
// under root (repo-relative markers) or home ("~/"-prefixed markers), in
// registry order. A descriptor with no markers (the generic pull tier) is
// never detected. Missing or unreadable markers are absence here — this
// is a routing hint for init and doctor, never a health verdict.
func Detect(root, home string) []string {
	var out []string
	for _, d := range registry {
		for _, m := range d.Detect {
			if markerPresent(m, root, home) {
				out = append(out, d.ID)
				break
			}
		}
	}
	return out
}

func markerPresent(marker, root, home string) bool {
	var p string
	if strings.HasPrefix(marker, "~/") {
		if home == "" {
			return false
		}
		p = filepath.Join(home, marker[2:])
	} else {
		p = filepath.Join(root, marker)
	}
	_, err := os.Stat(p)
	return err == nil
}

// ClassifyTool maps a harness tool name to its class. An unknown
// descriptor or an unlisted tool is ToolOther — the callers' default
// branch, which is what the pre-refactor literal switch fell through to.
func ClassifyTool(descriptorID, toolName string) ToolClass {
	d, ok := Lookup(descriptorID)
	if !ok {
		return ToolOther
	}
	if c, ok := d.Tools[toolName]; ok {
		return c
	}
	return ToolOther
}

// DemotionWindow is how long a descriptor may stay unverified after its
// codec lands before the generated table renders it as tier D and
// `init --harness` refuses it (vision 25 §5).
const DemotionWindow = 60 * 24 * time.Hour

// DemotionDue reports whether desc has been unverified for longer than
// DemotionWindow since its codec landed, with the reason. A descriptor
// with no codec has no clock (data rows are cheap; promises are not); a
// verified descriptor never demotes; an unparsable CodecLanded is treated
// as due, because a clock nobody can read must not run forever.
func DemotionDue(desc Descriptor, now time.Time) (bool, string) {
	if !desc.Unverified() || desc.Codec == "" {
		return false, ""
	}
	landed, err := time.Parse("2006-01-02", desc.CodecLanded)
	if err != nil {
		return true, fmt.Sprintf("%s: codec %q landed on an unparsable date %q — demoted to tier D until a real session verifies it", desc.ID, desc.Codec, desc.CodecLanded)
	}
	age := now.Sub(landed)
	if age <= DemotionWindow {
		return false, ""
	}
	return true, fmt.Sprintf("%s: codec %q landed %s and no real session has verified it in %d days (limit %d) — demoted to tier D: stanza + MCP only until doctor hooks_firing sees beats from a real install",
		desc.ID, desc.Codec, desc.CodecLanded, int(age.Hours()/24), int(DemotionWindow.Hours()/24))
}

// EffectiveTier is the descriptor's tier after the demotion clock.
func EffectiveTier(desc Descriptor, now time.Time) string {
	if due, _ := DemotionDue(desc, now); due {
		return "D"
	}
	return desc.Tier
}

// ExportJSON renders the whole registry as indented JSON — the shape
// schema/harness/schema.cue validates and `fathom harness list --json`
// prints. Deterministic: registry order, encoding/json's sorted map keys.
func ExportJSON() ([]byte, error) {
	return json.MarshalIndent(All(), "", "  ")
}
