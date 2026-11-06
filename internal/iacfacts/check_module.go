package iacfacts

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
)

// Per-answer bounds of CheckIaCModule (design section 9). Each is disclosed when hit.
const (
	// MaxCheckBlobs is the most catalog blobs one answer reads (design section 9: review_diff reads at most 256).
	MaxCheckBlobs = 256
	// maxCheckBindings is the most distinct provider bindings one answer resolves. A binding walks the repository's
	// lockfiles, so a module that names more providers than this is not an ordinary one.
	maxCheckBindings = 16
)

// IaCModuleQuery selects what CheckIaCModule checks in one module directory.
type IaCModuleQuery struct {
	Repo fs.FS
	// CatalogDir is where the catalogs in Catalogs live; contracts are read from it and nowhere else.
	CatalogDir string
	Catalogs   []IaCCatalogInfo
	// Dir is the module's repository-relative directory.
	Dir string
	// Blocks limits the check to these block addresses (aws_s3_bucket.logs, data.aws_ami.base). Empty checks every
	// resource, data source and ephemeral resource of the module.
	Blocks map[string]bool
	// Files limits the check to blocks and outputs declared in these repository-relative files. Empty means all.
	Files map[string]bool
	// Outputs also judges the module's outputs.
	Outputs bool
}

// IaCBlockCheck is the answer for one block: how it binds, and what its contract says about what it wrote.
type IaCBlockCheck struct {
	Block   string
	Span    Span
	Binding IaCBinding
	// Contract cites the blob the facts came from; nil when the block has no facts.
	Contract *IaCFactRef
	Check    IaCCheck
}

// IaCModuleCheck is the answer for a module.
type IaCModuleCheck struct {
	Blocks []IaCBlockCheck
	// Outputs is the check of the module's outputs, empty unless asked for.
	Outputs IaCCheck
	// BlobsRead is how many catalog blobs the answer read.
	BlobsRead   int
	Disclosures []string
}

// CheckIaCModule binds each block of one module directory, reads the contract of each bound type once and runs
// CheckAttributeUse on it. Catalogs are read from q.CatalogDir only: nothing is fetched (I56). With no catalog every
// block reads unbound and no fact is returned.
func CheckIaCModule(q IaCModuleQuery) (IaCModuleCheck, error) {
	var out IaCModuleCheck
	set, err := ReadTerraformModule(q.Repo, q.Dir)
	if err != nil {
		return out, err
	}
	if set.SyntaxErrors > 0 {
		out.Disclosures = append(out.Disclosures, fmt.Sprintf("%s has %d syntax errors, so a block there may be misread", cleanRepoDir(q.Dir), set.SyntaxErrors))
	}
	var outputs *IaCOutputSet
	if q.Outputs {
		if outputs, err = ReadTerraformOutputs(q.Repo, q.Dir); err != nil {
			return out, err
		}
		for _, s := range outputs.Skipped {
			out.Disclosures = append(out.Disclosures, fmt.Sprintf("%s was not read for outputs (JSON syntax or override file)", s))
		}
	}

	m := &moduleChecker{q: q, set: set, bindings: map[string]IaCBinding{}, contracts: map[string]contractEntry{}, notes: map[string]bool{}}
	for i := range set.Resources {
		r := &set.Resources[i]
		if r.Mode == ModeModule {
			continue
		}
		if len(q.Blocks) > 0 && !q.Blocks[blockAddress(r)] {
			continue
		}
		if len(q.Files) > 0 && !q.Files[r.Span.File] {
			continue
		}
		b, c := m.bindAndLoad(r)
		bc := IaCBlockCheck{Block: blockAddress(r), Span: r.Span, Binding: b}
		if c != nil {
			ref := c.ref("block")
			bc.Contract = &ref
		}
		bc.Check = CheckAttributeUse(IaCCheckInput{Resource: r, Binding: b, Contract: c, Resolve: m.resolve})
		out.Blocks = append(out.Blocks, bc)
	}
	if outputs != nil {
		var outs []IaCOutput
		for _, o := range outputs.Outputs {
			if len(q.Files) == 0 || q.Files[o.Span.File] {
				outs = append(outs, o)
			}
		}
		out.Outputs = CheckAttributeUse(IaCCheckInput{Resolve: m.resolve, Outputs: outs})
	}
	out.BlobsRead = m.blobs
	out.Disclosures = append(out.Disclosures, m.noteList()...)
	return out, nil
}

type contractEntry struct {
	c   *IaCContract
	err error
}

type moduleChecker struct {
	q         IaCModuleQuery
	set       *ResourceSet
	bindings  map[string]IaCBinding
	contracts map[string]contractEntry
	blobs     int
	notes     map[string]bool
	noteOrder []string
}

func (m *moduleChecker) note(s string) {
	if !m.notes[s] {
		m.notes[s] = true
		m.noteOrder = append(m.noteOrder, s)
	}
}

func (m *moduleChecker) noteList() []string {
	out := append([]string(nil), m.noteOrder...)
	sort.Strings(out)
	return out
}

func sectionOf(mode string) string {
	switch mode {
	case ModeData:
		return SectionData
	case ModeEphemeral:
		return SectionEphemeral
	}
	return SectionResource
}

// bind resolves the block's provider binding, once per provider name and alias in the module.
func (m *moduleChecker) bind(r *Resource) IaCBinding {
	ref := IaCBlockRef{Dir: m.q.Dir, Type: r.Type, Provider: r.Provider}
	local, alias := providerLocalName(ref)
	key := local + "\x00" + alias
	if b, ok := m.bindings[key]; ok {
		return b
	}
	bq := IaCBindingQuery{Repo: m.q.Repo, Catalogs: m.q.Catalogs, Block: ref}
	var b IaCBinding
	if len(m.bindings) >= maxCheckBindings {
		b = bindingRefused(bq, fmt.Sprintf("the module names more than %d providers, the most one answer binds", maxCheckBindings))
	} else {
		b = ResolveIaCBinding(bq)
	}
	m.bindings[key] = b
	return b
}

// load reads the contract of one type at one binding, once.
func (m *moduleChecker) load(b IaCBinding, mode, typ string) *IaCContract {
	if b.Status != BindLocked && b.Status != BindConstraintMatched {
		return nil
	}
	key := b.Catalog + "\x00" + b.Version + "\x00" + sectionOf(mode) + "\x00" + typ
	if e, ok := m.contracts[key]; ok {
		return e.c
	}
	if m.blobs >= MaxCheckBlobs {
		m.note(fmt.Sprintf("the answer read %d catalog blobs, the most one answer reads, so later types carry no facts", MaxCheckBlobs))
		m.contracts[key] = contractEntry{}
		return nil
	}
	m.blobs++
	c, err := LoadIaCContract(m.q.CatalogDir, b, sectionOf(mode), typ)
	if err != nil {
		var absent *ErrIaCTypeAbsent
		if errors.As(err, &absent) {
			m.note(fmt.Sprintf("%s is not a type of %s at %s, so it carries no facts (it may belong to another provider)", typ, b.Catalog, b.Version))
		} else {
			m.note(fmt.Sprintf("%s was refused: %v", typ, err))
		}
	}
	m.contracts[key] = contractEntry{c: c, err: err}
	return c
}

func (m *moduleChecker) bindAndLoad(r *Resource) (IaCBinding, *IaCContract) {
	b := m.bind(r)
	return b, m.load(b, r.Mode, r.Type)
}

// resolve finds a block of the module by mode, type and name, for a traversal or an output.
func (m *moduleChecker) resolve(mode, typ, name string) (IaCBinding, *IaCContract) {
	for i := range m.set.Resources {
		r := &m.set.Resources[i]
		if r.Mode == mode && r.Type == typ && r.Name == name {
			return m.bindAndLoad(r)
		}
	}
	return IaCBinding{}, nil
}
