package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// policyNodeTypes are the node types a Policy may set, in the order
// ComposeFromConfig names them.
var policyNodeTypes = []string{"provider", "cascade", "fallback", "cache", "record", "replay"}

// policyBodies maps each decorator node type to its body's struct, whose
// json tags are the only keys the body accepts.
var policyBodies = map[string]reflect.Type{
	"cascade":  reflect.TypeFor[CascadePolicy](),
	"fallback": reflect.TypeFor[FallbackPolicy](),
	"cache":    reflect.TypeFor[CachePolicy](),
	"record":   reflect.TypeFor[RecordPolicy](),
	"replay":   reflect.TypeFor[ReplayPolicy](),
}

// Policy is one node of a composition policy: the configuration data
// ComposeFromConfig assembles into a Decider, so a consumer reorders
// cascade stages or fallback Deciders, moves a threshold or wraps a node
// in a cache or a recorder by editing data, not code. Exactly one field is
// set. Provider names a leaf Decider by its Registry.Deciders key; each
// other field is a decorator node whose children are Policy nodes again.
// It mirrors #Policy in schema.cue and decodes from JSON, for example
// cache(cascade(cheap, fallback(a, b))):
//
//	{"cache": {"store": "memory", "template_version": "v1", "inner":
//	  {"cascade": {"escalate_below": 0.8, "stages": [
//	    {"provider": "cheap"},
//	    {"fallback": {"observe": "fallback-log",
//	      "deciders": [{"provider": "a"}, {"provider": "b"}]}}]}}}}
//
// Decoding never drops a key silently: a key that is not a node type, or a
// key a decorator body does not have, is kept with the node and
// ComposeFromConfig refuses it with the node's path, so a typo never
// becomes a default. Keys match exactly, as in CUE (encoding/json alone
// would match them case-insensitively).
type Policy struct {
	// Provider is a Registry.Deciders key: the node is that Decider.
	Provider string `json:"provider,omitempty"`
	// Cascade makes the node a Cascade (NewCascade).
	Cascade *CascadePolicy `json:"cascade,omitempty"`
	// Fallback makes the node a Fallback (NewFallback).
	Fallback *FallbackPolicy `json:"fallback,omitempty"`
	// Cache makes the node a Cached (NewCached).
	Cache *CachePolicy `json:"cache,omitempty"`
	// Record makes the node a Recorder (NewRecorder).
	Record *RecordPolicy `json:"record,omitempty"`
	// Replay makes the node a Replay (NewReplay).
	Replay *ReplayPolicy `json:"replay,omitempty"`

	// unknown lists, sorted, the keys UnmarshalJSON found that this node
	// does not have; ComposeFromConfig refuses the first.
	unknown []unknownPolicyKey
}

// unknownPolicyKey is one key a decoded Policy node does not have: a node
// type when body is empty, else a field of the body named body.
type unknownPolicyKey struct {
	body, key string
}

// CascadePolicy is a cascade node: CascadeOptions as data.
type CascadePolicy struct {
	// Name is CascadeOptions.Name, and the key Composed.Node finds the
	// Cascade under. Optional; names are unique within a policy.
	Name string `json:"name,omitempty"`
	// Stages are the stage nodes, cheapest first. At least one.
	Stages []Policy `json:"stages"`
	// OrderByCost is CascadeOptions.OrderByCost.
	OrderByCost bool `json:"order_by_cost,omitempty"`
	// EscalateBelow is the confidence below which an answer escalates, in
	// [0,1]. Absent means DefaultEscalateBelow, an uncalibrated starting
	// value; 0 means never escalate on confidence (only on abstain), which
	// CascadeOptions spells as a negative value.
	EscalateBelow *float64 `json:"escalate_below,omitempty"`
	// Observe is a Registry.CascadeObservers key: the hook receives this
	// node's CascadeReport on every call. Optional.
	Observe string `json:"observe,omitempty"`
}

// FallbackPolicy is a fallback node: FallbackOptions as data.
type FallbackPolicy struct {
	// Name is FallbackOptions.Name, and the key Composed.Node finds the
	// Fallback under. Optional; names are unique within a policy.
	Name string `json:"name,omitempty"`
	// Deciders are the nodes tried in order. At least one.
	Deciders []Policy `json:"deciders"`
	// Observe is a Registry.FallbackObservers key. Required: a Fallback is
	// never silent.
	Observe string `json:"observe"`
}

// CachePolicy is a cache node: CachedOptions as data, over a store the
// caller names. Composition creates no store.
type CachePolicy struct {
	// Name is CachedOptions.Name, and the key Composed.Node finds the
	// Cached under. Optional; names are unique within a policy.
	Name string `json:"name,omitempty"`
	// Store is a Registry.Stores key. Required.
	Store string `json:"store"`
	// TemplateVersion is CachedOptions.TemplateVersion. Required here
	// (CachedOptions allows it empty): it is part of every key, so bumping
	// it is how a policy re-grades.
	TemplateVersion string `json:"template_version"`
	// Observe is a Registry.CachedObservers key: the hook receives this
	// node's CachedReport on every call. Optional.
	Observe string `json:"observe,omitempty"`
	// Inner is the node the cache wraps. Required.
	Inner *Policy `json:"inner"`
}

// RecordPolicy is a record node: RecorderOptions as data.
type RecordPolicy struct {
	// Name is RecorderOptions.Name. Required: Composed.Node(Name) is the
	// only way to read the Recorder's Transcript.
	Name string `json:"name"`
	// Inner is the node recorded. Required.
	Inner *Policy `json:"inner"`
}

// ReplayPolicy is a replay node, a leaf: ReplayOptions as data, over a
// transcript the caller names.
type ReplayPolicy struct {
	// Name is ReplayOptions.Name, and the key Composed.Node finds the
	// Replay under. Optional; names are unique within a policy.
	Name string `json:"name,omitempty"`
	// Transcript is a Registry.Transcripts key. Required.
	Transcript string `json:"transcript"`
}

// UnmarshalJSON decodes a Policy node, keeping every key it does not
// recognise (an unknown node type, or an unknown field of the decorator
// body it sets) for ComposeFromConfig to refuse with the node's path. It
// fails only on JSON that does not decode at all.
func (p *Policy) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	type plain Policy // no UnmarshalJSON method, so no recursion here
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	v.unknown = nil
	for _, key := range slices.Sorted(maps.Keys(raw)) {
		if key == "provider" {
			continue
		}
		body, ok := policyBodies[key]
		if !ok {
			v.unknown = append(v.unknown, unknownPolicyKey{key: key})
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw[key], &fields); err != nil {
			return err
		}
		known := jsonKeys(body)
		for _, f := range slices.Sorted(maps.Keys(fields)) {
			if !known[f] {
				v.unknown = append(v.unknown, unknownPolicyKey{body: key, key: f})
			}
		}
	}
	*p = Policy(v)
	return nil
}

// jsonKeys returns the JSON names of t's fields, read from their json tags.
func jsonKeys(t reflect.Type) map[string]bool {
	keys := map[string]bool{}
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			keys[name] = true
		}
	}
	return keys
}

// nodeTypes returns the node types p sets, in policyNodeTypes order.
func (p Policy) nodeTypes() []string {
	set := []bool{p.Provider != "", p.Cascade != nil, p.Fallback != nil, p.Cache != nil, p.Record != nil, p.Replay != nil}
	var out []string
	for i, ok := range set {
		if ok {
			out = append(out, policyNodeTypes[i])
		}
	}
	return out
}

// Registry supplies what a Policy names by key. ComposeFromConfig reads it
// once: the composed tree holds the Deciders, stores and hooks it found,
// so changing a map afterwards changes no composed tree. Composition never
// constructs a provider by name: every leaf is a Decider the caller built.
type Registry struct {
	// Deciders are the leaf Deciders, by the key a provider node names.
	Deciders map[string]Decider
	// Stores are the CacheStores, by the key a cache node names.
	Stores map[string]CacheStore
	// Transcripts are the recorded Transcripts, by the key a replay node
	// names.
	Transcripts map[string]Transcript
	// CascadeObservers are the hooks a cascade node's observe key names.
	CascadeObservers map[string]func(CascadeReport)
	// FallbackObservers are the hooks a fallback node's observe key names.
	FallbackObservers map[string]func(FallbackReport)
	// CachedObservers are the hooks a cache node's observe key names.
	CachedObservers map[string]func(CachedReport)
}

// ComposeError is ComposeFromConfig's refusal: where in the policy, and
// why.
type ComposeError struct {
	// Path names the refused node from the root, "policy", through each
	// node type and field, for example "policy.cascade.stages[1]".
	Path string
	// Err says why. It wraps the decorator constructor's error when that
	// constructor refused.
	Err error
}

// Error returns "decision: ComposeFromConfig: <Path>: <Err>".
func (e *ComposeError) Error() string {
	return "decision: ComposeFromConfig: " + e.Path + ": " + e.Err.Error()
}

// Unwrap returns Err.
func (e *ComposeError) Unwrap() error { return e.Err }

// refuse returns a *ComposeError at path.
func refuse(path, format string, args ...any) error {
	return &ComposeError{Path: path, Err: fmt.Errorf(format, args...)}
}

// Composed is the Decider ComposeFromConfig assembled: the root of the
// tree, with the tree's named nodes. It answers, names itself and
// declares Capabilities as its root does.
//
// Reports: a decorator's report (per-answer provenance, escalations,
// failures, cache hits) stays reachable from a composed tree in two ways.
// The root's report: Root returns the concrete root decorator, whose
// DecideReport returns it per call. Every node's report, root or inner:
// the Observe hook its policy names receives it on every call that
// reaches the node, which is the one route for an inner node, since its
// report goes to its parent decorator and the parent keeps only the
// Response. A fallback node must name a hook; a cascade or cache node
// without one discards its reports, as its decorator does with a nil
// Observe. The composed Response never misattributes either way: a merge
// of several sources carries the merging decorator's Name and an empty
// Model. Through a cache over a cascade, the CachedReport says which
// answers were hits and the CascadeReport which stage produced each fresh
// one. Node finds a named decorator for what it exposes beyond Decide (a
// Recorder's Transcript, a Cascade's Stages); calling its DecideReport
// asks that node alone, outside the decorators above it.
//
// Concurrency: Composed holds no mutable state after ComposeFromConfig and
// is safe for concurrent use when its nodes are; every decorator is when
// the Deciders, stores and hooks it wraps are.
type Composed struct {
	root  Decider
	named map[string]Decider
}

// Root returns the root node's Decider: the decorator the policy's root
// names (*Cascade, *Fallback, *Cached, *Recorder or *Replay) or, for a
// provider root, the Registry's Decider.
func (c *Composed) Root() Decider { return c.root }

// Name returns the root's Name.
func (c *Composed) Name() string { return c.root.Name() }

// Decide answers req through the composed tree.
func (c *Composed) Decide(ctx context.Context, req Request) (Response, error) {
	return c.root.Decide(ctx, req)
}

// Capabilities declares what the root declares (CapabilitiesOf), which
// each decorator derives from what it wraps, so the declaration of the
// composed tree is the same as that of the tree built by hand. It makes
// no call.
func (c *Composed) Capabilities() Capabilities { return CapabilitiesOf(c.root) }

// Node returns the decorator whose policy node has Name name, and whether
// there is one. Unnamed nodes and provider leaves are not indexed.
func (c *Composed) Node(name string) (Decider, bool) {
	d, ok := c.named[name]
	return d, ok
}

// Names returns the names Node finds, sorted.
func (c *Composed) Names() []string { return slices.Sorted(maps.Keys(c.named)) }

// ComposeFromConfig assembles policy into a Decider from the existing
// decorators (NewCascade, NewFallback, NewCached, NewRecorder, NewReplay),
// resolving every key the policy names from reg: leaf Deciders, cache
// stores, transcripts and Observe hooks. Changing the cascade order, the
// fallback order, the escalation threshold or which node is cached or
// recorded is then a change of data, not a rebuild. It constructs no
// provider (no adapter is imported by name), and no store, cache, table or
// background work: a cache node uses the store the caller named.
//
// It refuses, with a *ComposeError naming the node's path (for example
// "policy.cascade.stages[1]") and never with a silent default: a node that
// sets no node type, more than one, or an unknown one; an unknown field in
// a decorator body; a provider, store, transcript or observe key reg does
// not hold, or holds as nil; an empty stage or fallback list; an
// escalate_below outside [0,1]; a cache node without a store, a template
// version or an inner node; a fallback node without an observe hook; a
// record node without a name or an inner node; a replay node without a
// transcript; two nodes with the same name; and anything a decorator
// constructor refuses (such as a transcript of another version), wrapped.
// See Composed for which reports stay reachable from the result.
func ComposeFromConfig(policy Policy, reg Registry) (*Composed, error) {
	c := &composer{reg: reg, named: map[string]Decider{}, namedAt: map[string]string{}}
	root, err := c.node(policy, "policy")
	if err != nil {
		return nil, err
	}
	return &Composed{root: root, named: c.named}, nil
}

// composer carries one ComposeFromConfig call's registry and the names
// claimed so far.
type composer struct {
	reg     Registry
	named   map[string]Decider
	namedAt map[string]string // name -> the path that claimed it
}

// node builds the Decider of the policy node at path.
func (c *composer) node(p Policy, path string) (Decider, error) {
	if len(p.unknown) > 0 {
		u := p.unknown[0]
		if u.body == "" {
			return nil, refuse(path, "unknown node type %q; want one of %s", u.key, strings.Join(policyNodeTypes, ", "))
		}
		return nil, refuse(path+"."+u.body, "unknown field %q", u.key)
	}
	switch set := p.nodeTypes(); len(set) {
	case 0:
		return nil, refuse(path, "sets no node type; set exactly one of %s", strings.Join(policyNodeTypes, ", "))
	case 1:
	default:
		return nil, refuse(path, "sets %d node types (%s); set exactly one", len(set), strings.Join(set, ", "))
	}
	switch {
	case p.Cascade != nil:
		return c.cascade(*p.Cascade, path+".cascade")
	case p.Fallback != nil:
		return c.fallback(*p.Fallback, path+".fallback")
	case p.Cache != nil:
		return c.cache(*p.Cache, path+".cache")
	case p.Record != nil:
		return c.record(*p.Record, path+".record")
	case p.Replay != nil:
		return c.replay(*p.Replay, path+".replay")
	}
	d, ok := c.reg.Deciders[p.Provider]
	switch {
	case !ok:
		return nil, refuse(path, "unknown provider %q: not a Registry.Deciders key", p.Provider)
	case isNilDecider(d):
		return nil, refuse(path, "provider %q is nil in Registry.Deciders", p.Provider)
	}
	return d, nil
}

// children builds the nodes of list field (at path.field[i]), refusing an
// empty list with why.
func (c *composer) children(nodes []Policy, path, field, why string) ([]Decider, error) {
	if len(nodes) == 0 {
		return nil, refuse(path, "%s is empty: %s", field, why)
	}
	out := make([]Decider, len(nodes))
	for i, n := range nodes {
		d, err := c.node(n, fmt.Sprintf("%s.%s[%d]", path, field, i))
		if err != nil {
			return nil, err
		}
		out[i] = d
	}
	return out, nil
}

// inner builds the inner node of the decorator at path.
func (c *composer) inner(p *Policy, path string) (Decider, error) {
	if p == nil {
		return nil, refuse(path, "inner is required")
	}
	return c.node(*p, path+".inner")
}

// claim reserves name for the node at path; an empty name claims nothing.
func (c *composer) claim(name, path string) error {
	if name == "" {
		return nil
	}
	if prev, ok := c.namedAt[name]; ok {
		return refuse(path, "name %q is already used by %s", name, prev)
	}
	c.namedAt[name] = path
	return nil
}

// built indexes d under name (when set) and returns it, or refuses with
// the constructor's err at path.
func (c *composer) built(d Decider, err error, name, path string) (Decider, error) {
	if err != nil {
		return nil, &ComposeError{Path: path, Err: err}
	}
	if name != "" {
		c.named[name] = d
	}
	return d, nil
}

// observer resolves an observe key from hooks (the Registry field named
// field); an empty key resolves to no hook.
func observer[R any](hooks map[string]func(R), key, path, field string) (func(R), error) {
	if key == "" {
		return nil, nil
	}
	fn, ok := hooks[key]
	switch {
	case !ok:
		return nil, refuse(path, "unknown observe hook %q: not a Registry.%s key", key, field)
	case fn == nil:
		return nil, refuse(path, "observe hook %q is nil in Registry.%s", key, field)
	}
	return fn, nil
}

// cascade builds a cascade node.
func (c *composer) cascade(p CascadePolicy, path string) (Decider, error) {
	if err := c.claim(p.Name, path); err != nil {
		return nil, err
	}
	below := 0.0 // NewCascade reads 0 as DefaultEscalateBelow
	if p.EscalateBelow != nil {
		t := *p.EscalateBelow
		if !(t >= 0 && t <= 1) {
			return nil, refuse(path, "escalate_below %v is outside [0,1]", t)
		}
		below = t
		if t == 0 {
			below = -1 // no confidence is below 0: never escalate on confidence
		}
	}
	observe, err := observer(c.reg.CascadeObservers, p.Observe, path, "CascadeObservers")
	if err != nil {
		return nil, err
	}
	stages, err := c.children(p.Stages, path, "stages", "a cascade needs at least one stage")
	if err != nil {
		return nil, err
	}
	d, err := NewCascade(CascadeOptions{Name: p.Name, Stages: stages, OrderByCost: p.OrderByCost, EscalateBelow: below, Observe: observe})
	return c.built(d, err, p.Name, path)
}

// fallback builds a fallback node.
func (c *composer) fallback(p FallbackPolicy, path string) (Decider, error) {
	if err := c.claim(p.Name, path); err != nil {
		return nil, err
	}
	if p.Observe == "" {
		return nil, refuse(path, "observe is required: a fallback is never silent, so name a Registry.FallbackObservers hook")
	}
	observe, err := observer(c.reg.FallbackObservers, p.Observe, path, "FallbackObservers")
	if err != nil {
		return nil, err
	}
	deciders, err := c.children(p.Deciders, path, "deciders", "a fallback needs at least one Decider")
	if err != nil {
		return nil, err
	}
	d, err := NewFallback(FallbackOptions{Name: p.Name, Deciders: deciders, Observe: observe})
	return c.built(d, err, p.Name, path)
}

// cache builds a cache node over the store the policy names.
func (c *composer) cache(p CachePolicy, path string) (Decider, error) {
	if err := c.claim(p.Name, path); err != nil {
		return nil, err
	}
	if p.Store == "" {
		return nil, refuse(path, "store is required: name a Registry.Stores key")
	}
	store, ok := c.reg.Stores[p.Store]
	switch {
	case !ok:
		return nil, refuse(path, "unknown store %q: not a Registry.Stores key", p.Store)
	case isNilValue(store):
		return nil, refuse(path, "store %q is nil in Registry.Stores", p.Store)
	}
	if p.TemplateVersion == "" {
		return nil, refuse(path, "template_version is required: it keys every entry, so bumping it re-grades")
	}
	observe, err := observer(c.reg.CachedObservers, p.Observe, path, "CachedObservers")
	if err != nil {
		return nil, err
	}
	inner, err := c.inner(p.Inner, path)
	if err != nil {
		return nil, err
	}
	d, err := NewCached(inner, CachedOptions{Name: p.Name, Store: store, TemplateVersion: p.TemplateVersion, Observe: observe})
	return c.built(d, err, p.Name, path)
}

// record builds a record node.
func (c *composer) record(p RecordPolicy, path string) (Decider, error) {
	if p.Name == "" {
		return nil, refuse(path, "name is required: the Recorder's Transcript is read through Composed.Node(name)")
	}
	if err := c.claim(p.Name, path); err != nil {
		return nil, err
	}
	inner, err := c.inner(p.Inner, path)
	if err != nil {
		return nil, err
	}
	d, err := NewRecorder(inner, RecorderOptions{Name: p.Name})
	return c.built(d, err, p.Name, path)
}

// replay builds a replay node over the transcript the policy names.
func (c *composer) replay(p ReplayPolicy, path string) (Decider, error) {
	if err := c.claim(p.Name, path); err != nil {
		return nil, err
	}
	if p.Transcript == "" {
		return nil, refuse(path, "transcript is required: name a Registry.Transcripts key")
	}
	t, ok := c.reg.Transcripts[p.Transcript]
	if !ok {
		return nil, refuse(path, "unknown transcript %q: not a Registry.Transcripts key", p.Transcript)
	}
	d, err := NewReplay(t, ReplayOptions{Name: p.Name})
	return c.built(d, err, p.Name, path)
}

// isNilValue reports whether v is nil or holds a nil pointer, map, slice,
// func, channel or interface.
func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return rv.IsNil()
	}
	return false
}
