// Package recordids checks authored Markdown record identities without
// changing plan grammar or using the index. Cross-plan checks are opt-in.
package recordids

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

type Family struct {
	Name    string   `json:"name"`
	Files   []string `json:"files"`
	Pattern string   `json:"pattern,omitempty"`
	Plans   bool     `json:"plans,omitempty"`
	PerFile bool     `json:"per_file,omitempty"`
}

type Exception struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
}

type Config struct {
	Families    []Family    `json:"families,omitempty"`
	CrossPlan   bool        `json:"cross_plan"`
	Grandfather []Exception `json:"grandfather,omitempty"`
}

type Location struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}
type Duplicate struct {
	Family, ID    string
	First, Second Location
}

func (d Duplicate) String() string {
	return fmt.Sprintf("%s %s: %s:%d and %s:%d", d.Family, d.ID, d.First.Path, d.First.Line, d.Second.Path, d.Second.Line)
}

func Defaults() Config {
	return Config{Families: []Family{
		{Name: "adr", Files: []string{"ADR.md"}, Pattern: `^##\s+(ADR-[0-9]+)\b`},
		{Name: "invariant", Files: []string{"docs/intent/invariants.md"}, Pattern: `^\|\s*(I[0-9]+)\s*\|`},
		{Name: "future", Files: []string{"docs/intent/futures.md", "docs/intent/futures/*.md"}, Pattern: `^#{2,6}\s+(F[0-9]+)\b`},
		{Name: "incident", Files: []string{"docs/intent/incidents.md"}, Pattern: `^(?:#{2,6}\s+|-\s+|\*\*)(INC-[0-9]+)\b`},
		{Name: "observation", Files: []string{projectdir.ObservationLog.Rel(), projectdir.Observations.Rel("*.md")}, Pattern: `^#{2,6}\s+(O[0-9]+)\b`},
		{Name: "plan", Files: []string{"docs/plans/*.md"}, Plans: true, PerFile: true},
	}}
}

// Load overlays the default families; explicit [] disables all families.
func Load(path string) (Config, error) {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func archived(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(filepath.Clean(path)), "docs/plans/archive/")
}

func excepted(cfg Config, id string, a, b Location) bool {
	if a.Path == b.Path {
		return false
	}
	for _, e := range cfg.Grandfather {
		if e.ID == id && slices.Contains(e.Paths, a.Path) && slices.Contains(e.Paths, b.Path) {
			return true
		}
	}
	return false
}

// Inventory discovers every authored definition without collapsing repeats. Patterns
// use filepath.Glob syntax. Archived plans are always excluded, even if a
// configured pattern names them explicitly. Exceptions only cover named
// cross-plan pairs and never excuse a duplicate within one plan.
func Inventory(root string, cfg Config) ([]Definition, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository root is not a directory")
	}
	out := []Definition{}
	for _, family := range cfg.Families {
		var re *regexp.Regexp
		if !family.Plans {
			var err error
			re, err = regexp.Compile(family.Pattern)
			if err != nil {
				return nil, fmt.Errorf("%s pattern: %w", family.Name, err)
			}
			if re.NumSubexp() != 1 {
				return nil, fmt.Errorf("%s pattern must capture exactly one record id", family.Name)
			}
		}
		var files []string
		for _, pattern := range family.Files {
			if filepath.IsAbs(pattern) || strings.HasPrefix(filepath.Clean(pattern), "..") {
				return nil, fmt.Errorf("%s: file patterns must stay repository-relative", family.Name)
			}
			found, err := filepath.Glob(filepath.Join(root, pattern))
			if err != nil {
				return nil, err
			}
			files = append(files, found...)
		}
		slices.Sort(files)
		files = slices.Compact(files)
		for _, file := range files {
			rel, err := filepath.Rel(root, file)
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			if archived(rel) {
				continue
			}
			b, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			out = append(out, scanDefinitions(family, rel, b, re)...)

		}
	}
	if _, err := ReadLanes(root); err != nil {
		return nil, err
	}
	sortDefinitions(out)
	return out, nil
}

// Check preserves the legacy duplicate-only API.
func Check(root string, cfg Config) ([]Duplicate, error) {
	defs, err := Inventory(root, cfg)
	if err != nil {
		return nil, err
	}
	return CheckDefinitions(defs, cfg)
}

// FamilyName is the closed vocabulary of allocatable identity families.
type FamilyName string

const (
	FamilyADR         FamilyName = "adr"
	FamilyInvariant   FamilyName = "invariant"
	FamilyFuture      FamilyName = "future"
	FamilyIncident    FamilyName = "incident"
	FamilyObservation FamilyName = "observation"
	FamilyPlan        FamilyName = "plan"
)

type LaneKind string

const KindLane LaneKind = "lane"

type LaneClass string

const (
	ClassData    LaneClass = "L-D"
	ClassReading LaneClass = "L-R"
	ClassPackage LaneClass = "L-P"
	ClassFiles   LaneClass = "L-F"
	ClassEngine  LaneClass = "L-E"
	ClassSurface LaneClass = "L-S"
)

type ErrorCode string

const (
	ErrShape     ErrorCode = "shape"
	ErrPolicy    ErrorCode = "policy"
	ErrDuplicate ErrorCode = "duplicate"
	ErrOwnership ErrorCode = "ownership"
	ErrTrust     ErrorCode = "trust"
	ErrTarget    ErrorCode = "target"
	ErrExhausted ErrorCode = "exhausted"
)

type RefusalError struct {
	Code   ErrorCode `json:"code"`
	Detail string    `json:"detail"`
}

func (e *RefusalError) Error() string            { return string(e.Code) + ": " + e.Detail }
func Refuse(code ErrorCode, detail string) error { return &RefusalError{Code: code, Detail: detail} }

type Definition struct {
	Family FamilyName `json:"family"`
	ID     string     `json:"id"`
	At     Location   `json:"at"`
}
type IDRange struct {
	Family FamilyName `json:"family"`
	Prefix string     `json:"prefix"`
	First  int        `json:"first"`
	Last   int        `json:"last"`
}
type LaneGrant struct {
	ID         string    `json:"id"`
	OwnedPaths []string  `json:"owned_paths"`
	IDRanges   []IDRange `json:"id_ranges"`
}
type OwnershipReason string

const (
	ReasonUnreserved  OwnershipReason = "unreserved or ambiguous identity"
	ReasonOutsidePath OwnershipReason = "outside owned paths"
)

type OwnershipViolation struct {
	Definition Definition      `json:"definition"`
	LaneID     string          `json:"lane_id"`
	Reason     OwnershipReason `json:"reason"`
}
type TrustedInventory struct {
	Revision string       `json:"revision"`
	Lanes    []LaneGrant  `json:"lanes"`
	Legacy   []Definition `json:"legacy"`
}

// ValidatedInventory can only be constructed by the shared checker.
type ValidatedInventory struct {
	valid       bool
	definitions []Definition
	grants      []LaneGrant
}

func (v ValidatedInventory) Valid() bool { return v.valid }
func (v ValidatedInventory) Definitions() []Definition {
	return append([]Definition{}, v.definitions...)
}
func (v ValidatedInventory) Grant(id string) (LaneGrant, bool) {
	for _, g := range v.grants {
		if g.ID == id {
			g.OwnedPaths = append([]string{}, g.OwnedPaths...)
			g.IDRanges = append([]IDRange{}, g.IDRanges...)
			return g, true
		}
	}
	return LaneGrant{OwnedPaths: []string{}, IDRanges: []IDRange{}}, false
}
func sortDefinitions(ds []Definition) {
	slices.SortFunc(ds, func(a, b Definition) int {
		return cmp.Or(strings.Compare(string(a.Family), string(b.Family)), strings.Compare(a.ID, b.ID), strings.Compare(a.At.Path, b.At.Path), cmp.Compare(a.At.Line, b.At.Line))
	})
}

// CheckDefinitions owns the duplicate policy, including exact grandfather pairs.
func CheckDefinitions(defs []Definition, cfg Config) ([]Duplicate, error) {
	ds := append([]Definition{}, defs...)
	sortDefinitions(ds)
	out := []Duplicate{}
	planIDs := map[string]map[string]bool{}
	families := map[string]Family{}
	for _, f := range cfg.Families {
		families[f.Name] = f
	}
	for i, d := range ds {
		if d.Family == FamilyPlan {
			if planIDs[d.ID] == nil {
				planIDs[d.ID] = map[string]bool{}
			}
			planIDs[d.ID][d.At.Path] = true
		}
		for _, first := range ds[:i] {
			if first.ID != d.ID {
				continue
			}
			same := first.Family == d.Family
			f := families[string(d.Family)]
			if same && f.PerFile && !(f.Plans && cfg.CrossPlan) && first.At.Path != d.At.Path {
				continue
			}
			if same && d.Family == FamilyPlan && cfg.CrossPlan && excepted(cfg, d.ID, first.At, d.At) {
				continue
			}
			out = append(out, Duplicate{Family: string(d.Family), ID: d.ID, First: first.At, Second: d.At})
		}
	}
	if cfg.CrossPlan {
		for _, e := range cfg.Grandfather {
			paths := slices.Clone(e.Paths)
			slices.Sort(paths)
			paths = slices.Compact(paths)
			if len(paths) < 2 {
				return nil, Refuse(ErrPolicy, "stale exception "+e.ID+": requires at least two distinct plan paths")
			}
			for _, p := range paths {
				if !planIDs[e.ID][p] {
					return nil, Refuse(ErrPolicy, "stale exception "+e.ID+": "+p+" no longer collides")
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b Duplicate) int {
		return cmp.Or(strings.Compare(a.Family, b.Family), strings.Compare(a.ID, b.ID), strings.Compare(a.First.Path, b.First.Path), cmp.Compare(a.First.Line, b.First.Line), strings.Compare(a.Second.Path, b.Second.Path), cmp.Compare(a.Second.Line, b.Second.Line))
	})
	return out, nil
}
func CanonicalPath(p string) bool {
	return p != "" && p != "." && !strings.ContainsAny(p, "\\\x00\r\n\t*?[]") && !filepath.IsAbs(p) && filepath.ToSlash(filepath.Clean(p)) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func (r IDRange) Validate() error {
	fixed := map[FamilyName]string{FamilyADR: "ADR-", FamilyInvariant: "I", FamilyFuture: "F", FamilyIncident: "INC-", FamilyObservation: "O"}
	if r.First < 1 || r.Last < r.First || !regexp.MustCompile(`^[A-Z][A-Z0-9]*-?$`).MatchString(r.Prefix) {
		return Refuse(ErrShape, "invalid range")
	}
	if r.Family == FamilyPlan {
		return nil
	}
	if p, ok := fixed[r.Family]; !ok || p != r.Prefix {
		return Refuse(ErrShape, "family/prefix mismatch")
	}
	return nil
}

// MatchesPrefix recognizes a canonical id with this prefix, irrespective of its range.
func (r IDRange) MatchesPrefix(id string) bool {
	if !strings.HasPrefix(id, r.Prefix) {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, r.Prefix))
	return err == nil && n >= 1 && r.Format(n) == id
}
func (r IDRange) Contains(d Definition) bool {
	if r.Family != d.Family || !r.MatchesPrefix(d.ID) {
		return false
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(d.ID, r.Prefix))
	return n >= r.First && n <= r.Last
}
func (r IDRange) Format(n int) string {
	if r.Family == FamilyADR {
		return fmt.Sprintf("%s%03d", r.Prefix, n)
	}
	return r.Prefix + strconv.Itoa(n)
}
func validateGrants(gs []LaneGrant) error {
	ids := map[string]bool{}
	owners := map[string]string{}
	var ranges []IDRange
	for _, g := range gs {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(g.ID) || ids[g.ID] || len(g.OwnedPaths) == 0 {
			return Refuse(ErrShape, "invalid or duplicate lane slug "+g.ID)
		}
		ids[g.ID] = true
		paths := map[string]bool{}
		for _, p := range g.OwnedPaths {
			if !CanonicalPath(p) || paths[p] {
				return Refuse(ErrShape, "noncanonical or duplicate owned path "+p)
			}
			if owner, exists := owners[p]; exists {
				return Refuse(ErrShape, "owned path "+p+" has multiple writers: "+owner+" and "+g.ID)
			}
			owners[p] = g.ID
			paths[p] = true
		}
		for _, r := range g.IDRanges {
			if e := r.Validate(); e != nil {
				return e
			}
			for _, prev := range ranges {
				if rangesIntersect(r, prev) {
					return Refuse(ErrShape, "overlapping lane ranges for "+r.Prefix)
				}
			}
			ranges = append(ranges, r)
		}
	}
	return nil
}
func CheckOwnership(defs []Definition, trusted TrustedInventory) ([]OwnershipViolation, error) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(trusted.Revision) {
		return nil, Refuse(ErrTrust, "missing trusted revision")
	}
	if e := validateGrants(trusted.Lanes); e != nil {
		return nil, e
	}
	out := []OwnershipViolation{}
	for _, d := range defs {
		legacy := false
		for _, old := range trusted.Legacy {
			if d.Family == old.Family && d.ID == old.ID && d.At.Path == old.At.Path {
				legacy = true
				break
			}
		}
		if legacy {
			continue
		}
		matches := []LaneGrant{}
		for _, g := range trusted.Lanes {
			for _, r := range g.IDRanges {
				if r.Contains(d) {
					matches = append(matches, g)
				}
			}
		}
		reason, lane := OwnershipReason(""), ""
		if len(matches) != 1 {
			reason = ReasonUnreserved
		} else {
			lane = matches[0].ID
			if !slices.Contains(matches[0].OwnedPaths, d.At.Path) {
				reason = ReasonOutsidePath
			}
		}
		if reason != "" {
			out = append(out, OwnershipViolation{Definition: d, LaneID: lane, Reason: reason})
		}
	}
	slices.SortFunc(out, func(a, b OwnershipViolation) int {
		c := cmp.Or(strings.Compare(string(a.Definition.Family), string(b.Definition.Family)), strings.Compare(a.Definition.ID, b.Definition.ID), strings.Compare(a.Definition.At.Path, b.Definition.At.Path), cmp.Compare(a.Definition.At.Line, b.Definition.At.Line))
		return cmp.Or(c, strings.Compare(a.LaneID, b.LaneID), strings.Compare(string(a.Reason), string(b.Reason)))
	})
	return out, nil
}
func ValidateInventory(defs []Definition, policy Config, trusted TrustedInventory) (ValidatedInventory, error) {
	// Legacy Check may scope any family per file. Lane validation always enforces
	// full keys through that same checker, retaining exact grandfather path sets.
	policy.CrossPlan = true
	policy.Families = slices.Clone(policy.Families)
	for i := range policy.Families {
		policy.Families[i].PerFile = false
	}
	d, e := CheckDefinitions(defs, policy)
	if e != nil {
		return ValidatedInventory{}, e
	}
	if len(d) > 0 {
		parts := []string{}
		for _, v := range d {
			parts = append(parts, v.String())
		}
		return ValidatedInventory{}, Refuse(ErrDuplicate, strings.Join(parts, "; "))
	}
	v, e := CheckOwnership(defs, trusted)
	if e != nil {
		return ValidatedInventory{}, e
	}
	if len(v) > 0 {
		parts := []string{}
		for _, x := range v {
			parts = append(parts, fmt.Sprintf("%s %s %s:%d lane %s: %s", x.Definition.Family, x.Definition.ID, x.Definition.At.Path, x.Definition.At.Line, x.LaneID, x.Reason))
		}
		return ValidatedInventory{}, Refuse(ErrOwnership, strings.Join(parts, "; "))
	}
	result := ValidatedInventory{valid: true, definitions: append([]Definition{}, defs...), grants: []LaneGrant{}}
	sortDefinitions(result.definitions)
	for _, g := range trusted.Lanes {
		g.OwnedPaths = append([]string{}, g.OwnedPaths...)
		g.IDRanges = append([]IDRange{}, g.IDRanges...)
		result.grants = append(result.grants, g)
	}
	return result, nil
}

// LaneDeclaration is the standalone lane shape. No derived state is authored.
type LaneDeclaration struct {
	Kind          LaneKind  `json:"kind"`
	ID            string    `json:"id"`
	Goal          string    `json:"goal"`
	Class         LaneClass `json:"class"`
	BaseCommit    string    `json:"base_commit"`
	Rows          []string  `json:"rows"`
	OwnedPaths    []string  `json:"owned_paths"`
	ReadOnlyPaths []string  `json:"read_only_paths"`
	ContractFiles []string  `json:"contract_files"`
	ProofCommand  string    `json:"proof_command"`
	Budget        string    `json:"budget"`
	HarnessClass  string    `json:"harness_class"`
	IDRanges      []IDRange `json:"id_ranges"`
}

func (l LaneDeclaration) Validate() error {
	if l.Kind != KindLane || l.Goal == "" || l.ProofCommand == "" || l.Budget == "" || l.HarnessClass == "" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(l.BaseCommit) || len(l.Rows) == 0 || len(l.ContractFiles) == 0 {
		return Refuse(ErrShape, "missing or invalid lane fields")
	}
	switch l.Class {
	case ClassData, ClassReading, ClassPackage, ClassFiles, ClassEngine, ClassSurface:
	default:
		return Refuse(ErrShape, "invalid lane class")
	}
	for _, p := range append(append([]string{}, l.ReadOnlyPaths...), l.ContractFiles...) {
		if !CanonicalPath(p) {
			return Refuse(ErrShape, "invalid lane path "+p)
		}
	}
	for _, r := range l.Rows {
		if strings.TrimSpace(r) == "" {
			return Refuse(ErrShape, "empty lane row")
		}
	}
	return validateGrants([]LaneGrant{{ID: l.ID, OwnedPaths: l.OwnedPaths, IDRanges: l.IDRanges}})
}

// ReadLanes reads only direct lane entries, resolving only the standalone schema
// in this root. It never asks CUE to resolve worktree or network imports.
func ReadLanes(root string) ([]LaneDeclaration, error) {
	if err := rejectLaneSymlinks(root); err != nil {
		return nil, err
	}
	result := []LaneDeclaration{}
	entries, e := os.ReadDir(filepath.Join(root, "docs/lanes"))
	if os.IsNotExist(e) {
		return result, nil
	}
	if e != nil {
		return nil, e
	}
	schema, e := os.ReadFile(filepath.Join(root, "schema/records/lane.cue"))
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cue") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, Refuse(ErrTrust, "symlink lane manifest")
		}
		raw, e := os.ReadFile(filepath.Join(root, "docs/lanes", entry.Name()))
		if e != nil {
			return nil, e
		}
		file, e := parser.ParseFile(entry.Name(), raw)
		if e != nil {
			return nil, Refuse(ErrShape, e.Error())
		}
		decls := []ast.Decl{}
		count := 0
		imports := 0
		for _, decl := range file.Decls {
			switch x := decl.(type) {
			case *ast.Package:
			case *ast.ImportDecl:
				for _, spec := range x.Specs {
					p, _ := strconv.Unquote(spec.Path.Value)
					if p != "github.com/geoffmilleraz/fathom/schema/records" || spec.Name != nil {
						return nil, Refuse(ErrShape, "only the standalone records import is allowed")
					}
					imports++
				}
			case *ast.Field:
				label, _, er := ast.LabelName(x.Label)
				binary, ok := x.Value.(*ast.BinaryExpr)
				if er != nil || label != "lane" || !ok || binary.Op != token.AND {
					return nil, Refuse(ErrShape, "expected lane: records.#Lane & {...}")
				}
				sel, ok := binary.X.(*ast.SelectorExpr)
				if !ok {
					return nil, Refuse(ErrShape, "expected records.#Lane")
				}
				ident, ok := sel.X.(*ast.Ident)
				name, _, _ := ast.LabelName(sel.Sel)
				if !ok || ident.Name != "records" || name != "#Lane" {
					return nil, Refuse(ErrShape, "expected records.#Lane")
				}
				if _, ok := binary.Y.(*ast.StructLit); !ok {
					return nil, Refuse(ErrShape, "expected literal lane struct")
				}
				binary.X = ast.NewIdent("#Lane")
				decls = append(decls, x)
				count++
			default:
				return nil, Refuse(ErrShape, "unexpected lane declaration")
			}
		}
		if count != 1 || imports != 1 {
			return nil, Refuse(ErrShape, "expected exactly one lane and schema import")
		}
		file.Decls = decls
		file.Imports = nil
		body, e := format.Node(file)
		if e != nil {
			return nil, e
		}
		source := append(append([]byte{}, schema...), append([]byte("\n"), body...)...)
		value := cuecontext.New().CompileBytes(source).LookupPath(cue.ParsePath("lane"))
		if e := value.Validate(cue.Concrete(true)); e != nil {
			return nil, Refuse(ErrShape, e.Error())
		}
		var lane LaneDeclaration
		if e := value.Decode(&lane); e != nil {
			return nil, Refuse(ErrShape, e.Error())
		}
		if e := lane.Validate(); e != nil {
			return nil, e
		}
		if entry.Name() != lane.ID+".cue" {
			return nil, Refuse(ErrShape, "lane filename must equal slug")
		}
		result = append(result, lane)
	}
	grants := []LaneGrant{}
	for _, l := range result {
		grants = append(grants, LaneGrant{ID: l.ID, OwnedPaths: l.OwnedPaths, IDRanges: l.IDRanges})
	}
	if e := validateGrants(grants); e != nil {
		return nil, e
	}
	return result, nil
}

type TrustedState struct {
	Inventory   TrustedInventory  `json:"inventory"`
	Policy      Config            `json:"policy"`
	Lanes       []LaneDeclaration `json:"lanes"`
	Manifests   map[string][]byte `json:"-"`
	definitions []Definition
}

// LoadTrusted reads immutable git objects. Candidate fields never establish trust.
// Its scratch snapshot lives under workspace and is removed on return.
func LoadTrusted(ctx context.Context, root, revision string) (TrustedState, error) {
	empty := TrustedState{Lanes: []LaneDeclaration{}, Manifests: map[string][]byte{}}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(revision) {
		return empty, Refuse(ErrTrust, "a full trusted commit id is required")
	}

	if _, err := readTrustedGit(ctx, root, trustedBlobLimit, "cat-file", "-e", revision+"^{commit}"); err != nil {
		return empty, err
	}
	list, err := readTrustedGit(ctx, root, trustedTreeLimit, "ls-tree", "-r", "-z", revision)
	if err != nil {
		return empty, err
	}
	tree := map[string]trustedBlob{}
	paths := []string{}
	objectID := regexp.MustCompile(`^[0-9a-f]{40}$`)
	for _, entry := range strings.Split(string(list), "\x00") {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "\t", 2)
		if len(parts) != 2 {
			return empty, Refuse(ErrTrust, "malformed trusted tree entry")
		}
		header := strings.Fields(parts[0])
		if len(header) != 3 || !objectID.MatchString(header[2]) {
			return empty, Refuse(ErrTrust, "malformed trusted tree object")
		}
		path := parts[1]
		if _, exists := tree[path]; exists {
			return empty, Refuse(ErrTrust, "duplicate trusted tree path")
		}
		tree[path] = trustedBlob{mode: header[0], kind: header[1], oid: header[2]}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	cache := map[string][]byte{}
	total := 0
	readBlob := func(path string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if b, ok := cache[path]; ok {
			return b, nil
		}
		if !CanonicalPath(path) {
			return nil, Refuse(ErrTrust, "noncanonical trusted path")
		}
		object, ok := tree[path]
		if !ok || object.kind != "blob" || (object.mode != "100644" && object.mode != "100755") {
			return nil, Refuse(ErrTrust, "nonregular trusted inventory file "+path)
		}
		if len(cache) >= trustedPathLimit {
			return nil, Refuse(ErrTrust, "trusted inventory path limit exceeded")
		}
		limit := min(trustedBlobLimit, trustedTotalLimit-total)
		b, err := readTrustedGit(ctx, root, limit, "cat-file", "blob", object.oid)
		if err != nil {
			return nil, err
		}
		total += len(b)
		cache[path] = b
		return b, nil
	}
	policy := Defaults()
	policyPath := projectdir.File("record-ids.json").Rel()
	if _, exists := tree[policyPath]; exists {
		config, err := readBlob(policyPath)
		if err != nil {
			return empty, err
		}
		dec := json.NewDecoder(bytes.NewReader(config))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&policy); err != nil {
			return empty, Refuse(ErrPolicy, err.Error())
		}
	}
	selected := []string{}
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		include := p == "cue.mod/module.cue" || strings.HasPrefix(p, "schema/records/") || strings.HasPrefix(p, "docs/lanes/") && strings.Count(p, "/") == 2 && strings.HasSuffix(p, ".cue") || p == policyPath
		for _, f := range policy.Families {
			for _, pattern := range f.Files {
				if !CanonicalPattern(pattern) {
					return empty, Refuse(ErrPolicy, "invalid family file pattern")
				}
				ok, e := filepath.Match(pattern, p)
				if e != nil {
					return empty, e
				}
				include = include || ok
			}
		}
		// Exact target paths are learned from trusted manifests after this snapshot.
		if include {
			selected = append(selected, p)
		}
	}
	if len(selected) == 0 {
		return empty, Refuse(ErrTrust, "trusted inventory is empty")
	}
	workspace := filepath.Join(root, "workspace")
	if info, e := os.Lstat(workspace); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return empty, Refuse(ErrTrust, "symlink workspace")
	}
	if e := os.MkdirAll(workspace, 0700); e != nil {
		return empty, e
	}
	scratch, e := os.MkdirTemp(workspace, "rec-trust-")
	if e != nil {
		return empty, e
	}
	defer os.RemoveAll(scratch)

	materialized := map[string]bool{}
	extract := func(paths []string) error {
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return err
			}
			if materialized[path] {
				continue
			}
			b, err := readBlob(path)
			if err != nil {
				return err
			}
			target := filepath.Join(scratch, path)
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(target, b, 0600); err != nil {
				return err
			}
			materialized[path] = true
		}
		return nil
	}

	if e := extract(selected); e != nil {
		return empty, e
	}
	lanes, e := ReadLanes(scratch)
	if e != nil {
		return empty, e
	}
	state := empty
	state.Lanes = lanes
	state.Inventory = TrustedInventory{Revision: revision, Lanes: []LaneGrant{}, Legacy: []Definition{}}
	targets := []string{}
	for _, l := range lanes {
		state.Inventory.Lanes = append(state.Inventory.Lanes, LaneGrant{ID: l.ID, OwnedPaths: l.OwnedPaths, IDRanges: l.IDRanges})
		raw, e := os.ReadFile(filepath.Join(scratch, "docs/lanes", l.ID+".cue"))
		if e != nil {
			return empty, e
		}
		state.Manifests[l.ID] = raw

		for _, r := range l.IDRanges {
			enabled := false
			for i := range policy.Families {
				if policy.Families[i].Name == string(r.Family) {
					enabled = true
					policy.Families[i].Files = append(policy.Families[i].Files, l.OwnedPaths...)
				}
			}
			if !enabled {
				return empty, Refuse(ErrPolicy, "lane reserves disabled family "+string(r.Family))
			}
		}
		for _, p := range l.OwnedPaths {
			if _, exists := tree[p]; exists {
				targets = append(targets, p)
			}
		}
	}
	slices.Sort(targets)
	targets = slices.Compact(targets)
	if e := extract(targets); e != nil {
		return empty, e
	}
	state.Policy = policy
	defs, e := Inventory(scratch, policy)
	if e != nil {
		return empty, e
	}
	for _, d := range defs {
		reserved := false
		for _, g := range state.Inventory.Lanes {
			for _, r := range g.IDRanges {
				if r.Contains(d) {
					reserved = true
				}
			}
		}
		if !reserved {
			state.Inventory.Legacy = append(state.Inventory.Legacy, d)
		}
	}
	if _, e := ValidateInventory(defs, policy, state.Inventory); e != nil {
		return empty, e
	}
	// Plan allocation may only extend a prefix already authored at a trusted target.
	for _, l := range lanes {
		for _, r := range l.IDRanges {
			if r.Family != FamilyPlan {
				continue
			}
			found := false
			for _, d := range defs {
				if d.Family == FamilyPlan && r.MatchesPrefix(d.ID) && slices.Contains(l.OwnedPaths, d.At.Path) {
					found = true
				}
			}
			if !found {
				return empty, Refuse(ErrTrust, "plan prefix has no trusted owned definition: "+r.Prefix)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	state.definitions = append([]Definition{}, defs...)
	return state, nil
}

// PlanTargetAllowed derives the target/prefix agreement only from trusted bytes.
func (s TrustedState) PlanTargetAllowed(laneID, rel string) bool {
	for _, lane := range s.Lanes {
		if lane.ID != laneID {
			continue
		}
		for _, r := range lane.IDRanges {
			if r.Family != FamilyPlan {
				continue
			}
			for _, d := range s.definitions {
				if d.Family == FamilyPlan && d.At.Path == rel && r.MatchesPrefix(d.ID) {
					return true
				}
			}
		}
	}
	return false
}
func CanonicalPattern(p string) bool {
	return p != "" && !filepath.IsAbs(p) && filepath.ToSlash(filepath.Clean(p)) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.Contains(p, "\\")
}

// CheckManifests refuses candidate edits, new grants and missing trusted grants.
func (s TrustedState) CheckManifests(root string) error {
	if err := rejectLaneSymlinks(root); err != nil {
		return err
	}
	entries, e := os.ReadDir(filepath.Join(root, "docs/lanes"))
	if os.IsNotExist(e) && len(s.Manifests) == 0 {
		return nil
	}
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cue") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".cue")
		expected, ok := s.Manifests[id]
		if !ok || entry.Type()&os.ModeSymlink != 0 {
			return Refuse(ErrTrust, "untrusted lane manifest "+entry.Name())
		}
		actual, e := os.ReadFile(filepath.Join(root, "docs/lanes", entry.Name()))
		if e != nil {
			return e
		}
		if !bytes.Equal(actual, expected) {
			return Refuse(ErrTrust, "lane manifest differs from trusted base: "+id)
		}
		seen[id] = true
	}
	for _, l := range s.Lanes {
		if !seen[l.ID] {
			return Refuse(ErrTrust, "missing trusted lane manifest "+l.ID)
		}
	}
	return nil
}

// Manifest bytes grant authority only through ordinary directories and files.
func rejectLaneSymlinks(root string) error {
	for _, p := range []string{"docs", "docs/lanes"} {
		info, err := os.Lstat(filepath.Join(root, p))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return Refuse(ErrTrust, "symlink in lane manifest path: "+p)
		}
	}
	return nil
}

// rangesIntersect checks rendered identities as well as numeric intervals. A
// prefix ending in digits can otherwise overlap another prefix's decimal suffix.
func rangesIntersect(a, b IDRange) bool {
	if a.Prefix == b.Prefix {
		return a.First <= b.Last && b.First <= a.Last
	}
	if len(a.Prefix) > len(b.Prefix) {
		a, b = b, a
	}
	if !strings.HasPrefix(b.Prefix, a.Prefix) {
		return false
	}
	tail := strings.TrimPrefix(b.Prefix, a.Prefix)
	if tail == "" || tail[0] == '0' {
		return false
	}
	for _, c := range tail {
		if c < '0' || c > '9' {
			return false
		}
	}
	first, last := strconv.Itoa(b.First), strconv.Itoa(b.Last)
	aFirst, aLast := strconv.Itoa(a.First), strconv.Itoa(a.Last)
	for width := len(first); width <= len(last); width++ {
		lo, hi := first, last
		if len(lo) < width {
			lo = "1" + strings.Repeat("0", width-1)
		}
		if len(hi) > width {
			hi = strings.Repeat("9", width)
		}
		// Decimal string comparisons avoid overflow even at the maximum int.
		if compareDecimal(tail+lo, aLast) <= 0 && compareDecimal(aFirst, tail+hi) <= 0 {
			return true
		}
	}
	return false
}
func compareDecimal(a, b string) int {
	return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
}

func scanDefinitions(family Family, rel string, b []byte, re *regexp.Regexp) []Definition {
	out := []Definition{}
	record := func(id string, line int) {
		out = append(out, Definition{Family: FamilyName(family.Name), ID: id, At: Location{Path: rel, Line: line}})
	}
	lines := strings.Split(string(b), "\n")
	fenced := map[int]bool{}
	fence := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			marker := trim[:3]
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
			}
			fenced[i+1] = true
			continue
		}
		if fence != "" {
			fenced[i+1] = true
			continue
		}
		if !family.Plans {
			if m := re.FindStringSubmatch(line); m != nil {
				record(m[1], i+1)
			}
		}
	}
	if family.Plans {
		for _, task := range verify.ParseMarkdownPlan("", b).Tasks {
			if fenced[task.Line] {
				continue
			}
			ann, _, _ := verify.ParseAnnotation(strings.Join(lines[task.Line-1:task.EndLine], " "))
			id := ann["id"]
			if id == "" {
				continue
			}

			record(id, task.Line)
		}
	}
	return out
}

// DefinitionsForFile uses the inventory's same parser on prospective file bytes.
// It lets a writer prove discoverability and validate the resulting inventory
// before an atomic rename, without exposing a second identity registry.
func DefinitionsForFile(rel string, raw []byte, cfg Config) ([]Definition, error) {
	out := []Definition{}
	if !CanonicalPath(rel) {
		return nil, Refuse(ErrTarget, "noncanonical inventory path")
	}
	if archived(rel) {
		return out, nil
	}
	for _, family := range cfg.Families {
		matched := false
		for _, pattern := range family.Files {
			ok, e := filepath.Match(pattern, rel)
			if e != nil {
				return nil, e
			}
			matched = matched || ok
		}
		if !matched {
			continue
		}
		var re *regexp.Regexp
		if !family.Plans {
			var e error
			re, e = regexp.Compile(family.Pattern)
			if e != nil {
				return nil, e
			}
			if re.NumSubexp() != 1 {
				return nil, Refuse(ErrPolicy, "family pattern must capture exactly one id")
			}
		}
		out = append(out, scanDefinitions(family, rel, raw, re)...)
	}
	sortDefinitions(out)
	return out, nil
}

// Trusted Git reads are raw object reads, never archive/export views. Limits
// bound the tree listing, selected regular paths, each blob and aggregate bytes.
const (
	trustedTreeLimit  = 32 << 20
	trustedBlobLimit  = 16 << 20
	trustedTotalLimit = 64 << 20
	trustedPathLimit  = 16384
)

type trustedBlob struct{ mode, kind, oid string }
type trustedGitOutput struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (w *trustedGitOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buffer.Len() {
		w.exceeded = true
		return 0, Refuse(ErrTrust, "trusted git output limit exceeded")
	}
	return w.buffer.Write(p)
}
func readTrustedGit(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, Refuse(ErrTrust, "trusted git output limit exceeded")
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "-C", root}, args...)...)
	// A trusted read may not fetch missing promisor objects or run a download.
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	out := &trustedGitOutput{limit: limit}
	cmd.Stdout = out
	err := cmd.Run()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if out.exceeded {
		return nil, Refuse(ErrTrust, "trusted git output limit exceeded")
	}
	if err != nil {
		return nil, Refuse(ErrTrust, fmt.Sprintf("git object read failed: %v", err))
	}
	return out.buffer.Bytes(), nil
}
