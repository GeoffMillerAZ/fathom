package testevidence

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"text/scanner"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/proto"
)

// UnityBinding is persisted alongside the standard run, never instead of it.
// Re-reading the active receipt and all bound inputs is mandatory before a
// runtime result. It is intentionally excluded from live certificates for now.
type UnityBinding struct {
	Root          string            `json:"root"`
	ReceiptPath   string            `json:"receipt_path"`
	ReceiptSHA256 string            `json:"receipt_sha256"`
	XMLPath       string            `json:"xml_path"`
	RunID         string            `json:"run_id"`
	CaseKeys      map[string]string `json:"case_keys"`
	Cohorts       map[string]string `json:"cohorts"`
}

var unityMethodDescriptor = regexp.MustCompile(`([\pL_][\pL\pN_]*)#([\pL_][\pL\pN_]*)\(\)\.$`)

// UnityJoinCases requires an exact source candidate, a SCIP definition at its
// position and a declared, bound generated-project assembly owner. No bare-name
// join, namespace suffix matching, argument stripping or framework edge exists.
func UnityJoinCases(e UnityEvidence, symbols []core.SymbolNode) ([]JoinedCase, UnityBinding, error) {
	b := UnityBinding{Root: e.Receipt.Root, ReceiptPath: e.ReceiptPath, ReceiptSHA256: e.ReceiptSHA256, XMLPath: e.Receipt.XML.Path, RunID: e.Receipt.RunID, CaseKeys: map[string]string{}, Cohorts: map[string]string{}}
	joined := make([]JoinedCase, 0, len(e.Run.Cases))
	for _, c := range e.Run.Cases {
		joined = append(joined, JoinedCase{Case: c, SuiteName: e.Run.Suite.Name})
	}
	// A selected native cohort count cannot prove the complete method family.
	// Conservatively retain filtered execution as an unjoined observation.
	if e.Receipt.Symbols == nil || e.Receipt.Filter != "" {
		return joined, b, nil
	}
	raw, err := unityFileBytes(b.Root, UnityFile{e.Receipt.Symbols.SCIPPath, e.Receipt.Symbols.SCIPSHA256}, 256<<20)
	if err != nil {
		return nil, b, err
	}
	var index scippb.Index
	if err = proto.Unmarshal(raw, &index); err != nil {
		return nil, b, err
	}
	if index.GetMetadata().GetToolInfo().GetName() != "scip-dotnet" {
		return nil, b, fmt.Errorf("native C# join requires scip-dotnet")
	}
	defs := map[string]map[int][]string{}
	for _, doc := range index.Documents {
		if doc.Language != "C#" && doc.Language != "csharp" {
			continue
		}
		if _, err := unityPath(b.Root, doc.RelativePath, false); err != nil {
			return nil, b, err
		}
		for _, occ := range doc.Occurrences {
			if occ.SymbolRoles&1 == 0 || !unityDefinitionRange(occ.Range) || !strings.HasPrefix(occ.Symbol, "scip-dotnet ") {
				continue
			}
			m := unityMethodDescriptor.FindStringSubmatch(occ.Symbol)
			if len(m) != 3 {
				continue
			}
			if defs[doc.RelativePath] == nil {
				defs[doc.RelativePath] = map[int][]string{}
			}
			defs[doc.RelativePath][int(occ.Range[0])+1] = append(defs[doc.RelativePath][int(occ.Range[0])+1], m[1]+"."+m[2])
		}
	}
	owners, err := unityProjectOwners(e.Receipt)
	if err != nil {
		return nil, b, err
	}
	byMethod := map[string][]core.SymbolNode{}
	for _, s := range symbols {
		if s.Kind != core.KindCSharpMethod {
			continue
		}
		qualified := s.Name
		if s.Package != "" {
			qualified = s.Package + "." + s.Name
		}
		byMethod[qualified] = append(byMethod[qualified], s)
	}
	cohortKeys := map[string]string{}
	for _, g := range e.Native.Cohorts {
		// This version refuses nested/generic classes and method overloads. The
		// runtime XML does not carry a source position for disambiguating them.
		if strings.ContainsAny(g.ClassName, "+`[]<>") {
			continue
		}
		candidates := byMethod[g.ClassName+"."+g.MethodName]
		if len(candidates) != 1 {
			continue
		}
		s := candidates[0]
		if !s.IsTest() || unityRetryAttribute(s.Signature) {
			continue
		}
		if owners[g.Assembly][s.FilePath] != 1 {
			continue
		}
		hits := 0
		for _, name := range defs[s.FilePath][s.Line] {
			if name == g.ClassName[strings.LastIndex(g.ClassName, ".")+1:]+"."+g.MethodName {
				hits++
			}
		}
		if hits != 1 {
			continue
		}
		key := s.Key()
		cohortKeys[g.ID] = key
		// Multiple native method cohorts mapping onto one declaration are ambiguous,
		// even if a single returned family happened to pass.
		if _, exists := b.Cohorts[key]; exists {
			b.Cohorts[key] = "unknown"
		} else {
			b.Cohorts[key] = string(g.ObservedVerdict)
		}
	}
	for _, c := range e.Native.Cases {
		if key := cohortKeys[c.Cohort]; key != "" {
			b.CaseKeys[c.Assembly+"::"+c.FullName] = key
		}
	}
	for i := range joined {
		joined[i].TestSymbolKey = b.CaseKeys[joined[i].ID]
	}
	return joined, b, nil
}

func unityProjectOwners(r UnityReceipt) (map[string]map[string]int, error) {
	owners := map[string]map[string]int{}
	boundInputs := map[string]bool{}
	for _, input := range r.Source.Inputs {
		path, err := unityPath(r.Root, input.Path, true)
		if err != nil {
			return nil, err
		}
		boundInputs[path] = true
	}
	for _, input := range r.Source.Inputs {
		if !strings.HasSuffix(input.Path, ".csproj") {
			continue
		}
		path, err := unityPath(r.Root, input.Path, true)
		if err != nil {
			return nil, err
		}
		// Unity-generated project files in the selected project only. External
		// arbitrary projects cannot claim ownership of source in this Unity run.
		project, _ := unityPath(r.Root, r.Project, false)
		if filepath.Dir(path) != project {
			continue
		}
		raw, err := unityFileBytes(r.Root, input, 16<<20)
		if err != nil {
			return nil, err
		}
		d := xml.NewDecoder(bytes.NewReader(raw))
		assembly := ""
		compile := []string{}
		for {
			token, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			switch token := token.(type) {
			case xml.Directive:
				return nil, fmt.Errorf("project XML directives refused")
			case xml.StartElement:
				switch token.Name.Local {
				case "AssemblyName":
					var name string
					if err := d.DecodeElement(&name, &token); err != nil {
						return nil, err
					}
					if assembly != "" {
						return nil, fmt.Errorf("duplicate assembly name")
					}
					assembly = name
				case "Compile":
					for _, a := range token.Attr {
						if a.Name.Local == "Include" {
							compile = append(compile, a.Value)
						}
					}
				}
			}
		}
		if assembly == "" || !unitySafeText(assembly) || strings.ContainsAny(assembly, "/\\") {
			continue
		}
		if owners[assembly] == nil {
			owners[assembly] = map[string]int{}
		}
		for _, included := range compile {
			included = strings.ReplaceAll(included, "\\", "/")
			if included == "" || strings.ContainsAny(included, "$*?;") {
				continue
			}
			absolute := included
			if !filepath.IsAbs(absolute) {
				absolute = filepath.Join(project, filepath.FromSlash(included))
			}
			relative, err := filepath.Rel(r.Root, filepath.Clean(absolute))
			if err != nil {
				return nil, err
			}
			relative = filepath.ToSlash(relative)
			if _, err = unityPath(r.Root, relative, false); err != nil {
				continue
			}
			if boundInputs[filepath.Clean(absolute)] {
				owners[assembly][relative]++
			}
		}
	}
	return owners, nil
}

// UnityEvidenceRun binds a validated producer record to the currently indexed
// source declarations. Revalidation at ingestion and read preserves freshness.
func UnityEvidenceRun(e UnityEvidence, symbols []core.SymbolNode) (Run, error) {
	_, binding, err := UnityJoinCases(e, symbols)
	if err != nil {
		return Run{}, err
	}
	run := e.Run
	run.Unity = &binding
	return run, nil
}

// Read leading attribute heads only: a method name or a TestCase string value
// containing "Retry" is not a retry policy. Unknown qualified retry heads are
// conservatively unsupported rather than assumed to be the NUnit type.
func unityRetryAttribute(signature string) bool {
	var s scanner.Scanner
	s.Init(strings.NewReader(signature))
	s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanChars | scanner.ScanComments | scanner.SkipComments
	bad := false
	s.Error = func(*scanner.Scanner, string) { bad = true }
	isRetry := func(name string) bool {
		return name == "Retry" || name == "RetryAttribute" || name == "Repeat" || name == "RepeatAttribute"
	}
	for token := s.Scan(); token == '['; token = s.Scan() {
		depth, parens := 1, 0
		head := ""
		inHead := true
		for depth > 0 {
			token = s.Scan()
			if token == scanner.EOF || bad {
				return true
			}
			if depth == 1 && parens == 0 {
				if inHead && token == scanner.Ident {
					head = s.TokenText()
				}
				if token == '(' {
					if isRetry(head) {
						return true
					}
					inHead = false
				}
				if token == ',' {
					if isRetry(head) {
						return true
					}
					head = ""
					inHead = true
				}
				if token == ']' && isRetry(head) {
					return true
				}
			}
			switch token {
			case '[':
				depth++
			case ']':
				depth--
			case '(':
				parens++
			case ')':
				parens--
			}
		}
	}
	return bad
}

func unityDefinitionRange(r []int32) bool {
	if len(r) != 3 && len(r) != 4 {
		return false
	}
	for _, n := range r {
		if n < 0 {
			return false
		}
	}
	if len(r) == 3 {
		return r[2] >= r[1]
	}
	return r[2] > r[0] || r[2] == r[0] && r[3] >= r[1]
}

// UnityCurrentSymbols refuses a stale syntax index even if its old method name
// and line happen to coincide with a current typed definition. ContentHash is
// extraction provenance, not another witness or an inferred framework identity.
func UnityCurrentSymbols(e UnityEvidence, observations []core.Observation) []core.SymbolNode {
	inputs := map[string]UnityFile{}
	for _, input := range e.Receipt.Source.Inputs {
		path, err := unityPath(e.Receipt.Root, input.Path, true)
		if err == nil {
			inputs[path] = input
		}
	}
	// Observation.ContentHash is the extractor-epoch fingerprint, not a raw
	// SHA256. Verify the raw producer-bound bytes first, then compare only the
	// authoritative current fingerprint. Cache once per source within this call.
	fingerprints := map[string]string{}
	out := []core.SymbolNode{}
	for _, o := range observations {
		if o.SourceKind != core.KindCSharpMethod {
			continue
		}
		path, err := unityPath(e.Receipt.Root, o.SourceRef, false)
		input, bound := inputs[path]
		if err != nil || !bound || o.ContentHash == "" {
			continue
		}
		fingerprint, checked := fingerprints[path]
		if !checked {
			fingerprint = ""
			if source, readErr := unityFileBytes(e.Receipt.Root, input, 16<<20); readErr == nil {
				fingerprint = fusion.FingerprintHash(source)
			}
			fingerprints[path] = fingerprint
		}
		if fingerprint != "" && fingerprint == o.ContentHash {
			out = append(out, o.ToSymbolNode())
		}
	}
	return out
}
