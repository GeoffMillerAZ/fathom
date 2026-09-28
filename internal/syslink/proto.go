package syslink

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"github.com/emicklei/proto"
)

// SourceFile is one file of a service, as a whole-service source reads it.
type SourceFile struct {
	Path    string
	Dialect string
	Content []byte
}

// protoProvides returns one rpc: provides row per RPC of every service a
// .proto file declares, at spec tier; the key is ADR-044's inventory
// spelling ("<package>.<Service>/<Method>"). An unparsable file provides
// nothing.
func protoProvides(file string, content []byte) []KeyRow {
	def, err := proto.NewParser(bytes.NewReader(content)).Parse()
	if err != nil || def == nil {
		return nil
	}
	pkg := ""
	for _, e := range def.Elements {
		if p, ok := e.(*proto.Package); ok {
			pkg = p.Name
			break
		}
	}
	var out []KeyRow
	for _, e := range def.Elements {
		svc, ok := e.(*proto.Service)
		if !ok {
			continue
		}
		name := svc.Name
		if pkg != "" {
			name = pkg + "." + svc.Name
		}
		for _, se := range svc.Elements {
			rpc, ok := se.(*proto.RPC)
			if !ok {
				continue
			}
			key, err := ParseContractKey("rpc:" + name + "/" + rpc.Name)
			if err != nil {
				continue
			}
			out = append(out, KeyRow{
				Key: key, Role: RoleProvides, NodeKind: "rpc", NodeID: key.Body,
				File: file, Line: rpc.Position.Line, TierHint: TierSpec,
			})
		}
	}
	SortRows(out)
	return out
}

var (
	stubCtorRe   = regexp.MustCompile(`(?m)^func (New(\w+)Client)\(`)
	stubMethodRe = regexp.MustCompile(`"/([\w.]+)/(\w+)"`)
)

// stubService is what a generated gRPC stub tells us about one client
// constructor: the proto service it calls and that service's methods.
type stubService struct {
	fullName string
	methods  map[string]bool
}

// grpcStubs indexes the generated Go gRPC stubs among files by client
// constructor name (NewOrdersClient -> shop.v1.Orders). A constructor two
// stubs declare for different services is ambiguous and dropped: the
// consumer then resolves nothing rather than a guess.
func grpcStubs(files []SourceFile) map[string]stubService {
	out := map[string]stubService{}
	ambiguous := map[string]bool{}
	for _, f := range files {
		if f.Dialect != "go" || !bytes.Contains(f.Content, []byte("protoc-gen-go-grpc")) {
			continue
		}
		src := string(f.Content)
		for _, m := range stubCtorRe.FindAllStringSubmatch(src, -1) {
			ctor, svc := m[1], m[2]
			st := stubService{methods: map[string]bool{}}
			for _, mm := range stubMethodRe.FindAllStringSubmatch(src, -1) {
				full := mm[1]
				if full != svc && !strings.HasSuffix(full, "."+svc) {
					continue
				}
				if st.fullName != "" && st.fullName != full {
					st.fullName = "" // two services named alike in one stub: unresolvable
					break
				}
				st.fullName = full
				st.methods[mm[2]] = true
			}
			if st.fullName == "" {
				continue
			}
			if prev, ok := out[ctor]; ok && prev.fullName != st.fullName {
				ambiguous[ctor] = true
			}
			out[ctor] = st
		}
	}
	for c := range ambiguous {
		delete(out, c)
	}
	return out
}

// grpcStubConsumes returns rpc: consumes rows at typed tier for every call
// of a stub client's method in one Go file: `c := pkg.NewXClient(conn)`
// then `c.Method(...)` in the same function, or the chained
// `pkg.NewXClient(conn).Method(...)`. The stub resolves the call to the
// proto declaration, which is what makes it typed rather than literal.
func grpcStubConsumes(file string, content []byte, stubs map[string]stubService) []KeyRow {
	if len(stubs) == 0 || bytes.Contains(content, []byte("protoc-gen-go-grpc")) {
		return nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, content, 0)
	if err != nil {
		return nil
	}
	ctorOf := func(e ast.Expr) (stubService, bool) {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return stubService{}, false
		}
		name := ""
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		case *ast.Ident:
			name = fn.Name
		}
		st, ok := stubs[name]
		return st, ok
	}
	var out []KeyRow
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		clients := map[string]stubService{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for i, rhs := range n.Rhs {
					if i >= len(n.Lhs) {
						break
					}
					if id, ok := n.Lhs[i].(*ast.Ident); ok {
						if st, ok := ctorOf(rhs); ok {
							clients[id.Name] = st
						}
					}
				}
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				var st stubService
				found := false
				if id, ok := sel.X.(*ast.Ident); ok {
					st, found = clients[id.Name]
				} else {
					st, found = ctorOf(sel.X)
				}
				if !found || !st.methods[sel.Sel.Name] {
					return true
				}
				key, err := ParseContractKey("rpc:" + st.fullName + "/" + sel.Sel.Name)
				if err != nil {
					return true
				}
				out = append(out, KeyRow{
					Key: key, Role: RoleConsumes, NodeKind: "call", NodeID: fd.Name.Name,
					File: file, Line: fset.Position(n.Pos()).Line, TierHint: TierTyped,
				})
			}
			return true
		})
	}
	SortRows(out)
	return out
}

// protoRPCKeys is SG-8 over one service's files: .proto service
// definitions provide rpc: keys at spec tier, and Go gRPC client calls
// resolved through the generated stubs among the same files consume them
// at typed tier. Rows are stamped with the proto sources' ids and epochs.
func protoRPCKeys(files []SourceFile) []KeyRow {
	stubs := grpcStubs(files)
	var out []KeyRow
	for _, f := range files {
		switch f.Dialect {
		case "proto":
			for _, r := range protoProvides(f.Path, f.Content) {
				r.Extractor, r.ExtractorEpoch = protoSource{}.ID(), protoSource{}.Epoch()
				out = append(out, r)
			}
		case "go":
			for _, r := range grpcStubConsumes(f.Path, f.Content, stubs) {
				r.Extractor, r.ExtractorEpoch = "go-grpc-stub", 1
				out = append(out, r)
			}
		}
	}
	SortRows(out)
	return out
}

type protoSource struct{}

func (protoSource) ID() string { return "proto" }
func (protoSource) Epoch() int { return 1 }
func (protoSource) Keys(file string, content []byte, _ string) []KeyRow {
	return protoProvides(file, content)
}
