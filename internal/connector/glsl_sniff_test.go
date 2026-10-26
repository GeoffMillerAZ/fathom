package connector_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func TestGLSLFSRealManifestSniff(t *testing.T) {
	dialects, err := extractor.LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var rules []connector.DialectRule
	for _, d := range dialects {
		rule := connector.DialectRule{ID: d.ID, Extensions: d.Match.Extensions, Basenames: d.Match.Basenames, Globs: d.Match.Globs, Shebangs: d.Match.Shebangs}
		if s := d.Match.Sniff; s != nil {
			rule.Sniff = &connector.DialectSniffRule{Extensions: s.Extensions, Pattern: s.Pattern, Priority: s.Priority, ScanBytes: s.ScanBytes}
		}
		rules = append(rules, rule)
	}
	for _, tc := range []struct {
		name, source, want string
	}{
		{"versionless fragment shader", "void main(){ gl_FragColor=vec4(1); }", "glsl"},
		{"versioned fragment shader", "#version 120\nvoid main(){ gl_FragColor=vec4(1); }", "glsl"},
		{"F sharp function", "let main() = printfn \"gl_FragColor=vec4(1)\"", ""},
		{"F sharp module", "module Shader\nlet main () = 1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := connector.ClassifyDialect(rules, "shader.fs", []byte(tc.source))
			if got.Dialect != tc.want {
				t.Errorf("dialect = %q (%s), want %q", got.Dialect, got.By, tc.want)
			}
		})
	}
}
