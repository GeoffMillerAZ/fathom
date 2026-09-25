package designadapt

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// Keep the original directory rule/model/schema untouched. An enhancement's
// identity can change without invalidating source-styled P04 artifacts.
//
//go:embed directory_enhanced.go directory_enhanced_model.go.txt directory_enhanced.templ.txt directory_enhanced.css directory_enhanced.cue directory.cue
var directoryEnhancedImplementation embed.FS

// DirectoryEnhancedArtifact composes an unchanged licensed base with an opt-in
// first-party styling companion. Keep the entire generated package private.
type DirectoryEnhancedArtifact struct {
	SchemaVersion int               `json:"schema_version"`
	Adapter       string            `json:"adapter"`
	Base          DirectoryArtifact `json:"base"`
	RuleSHA256    string            `json:"rule_sha256"`
	TemplSHA256   string            `json:"templ_sha256"`
	ModelSHA256   string            `json:"model_sha256"`
	CSSSHA256     string            `json:"css_sha256"`
	Templ         string            `json:"templ"`
	Model         string            `json:"model"`
	CSS           string            `json:"css"`
	Verification  string            `json:"verification"`
}

// AdaptDirectoryEnhanced performs only deterministic source generation. As for
// AdaptDirectory, the caller must independently verify original archive access.
func AdaptDirectoryEnhanced(source designconvert.Source, body []byte) (DirectoryEnhancedArtifact, error) {
	base, err := AdaptDirectory(source, body)
	if err != nil {
		return DirectoryEnhancedArtifact{}, err
	}
	return enhanceDirectory(base)
}

// VerifyDirectoryEnhancedReplay verifies both independent source-generation
// identities; it cannot certify the enhanced component's rendered quality.
func VerifyDirectoryEnhancedReplay(artifact DirectoryEnhancedArtifact, body []byte) error {
	if len(artifact.Templ) > 4096 || len(artifact.Model) > 32768 || len(artifact.CSS) > 16384 {
		return fmt.Errorf("directory enhancement: artifact exceeds replay bounds")
	}
	if err := VerifyDirectoryReplay(artifact.Base, body); err != nil {
		return err
	}
	want, err := enhanceDirectory(artifact.Base)
	if err != nil {
		return err
	}
	if artifact.SchemaVersion != want.SchemaVersion || artifact.Adapter != want.Adapter || artifact.RuleSHA256 != want.RuleSHA256 || artifact.TemplSHA256 != want.TemplSHA256 || artifact.ModelSHA256 != want.ModelSHA256 || artifact.CSSSHA256 != want.CSSSHA256 || artifact.Templ != want.Templ || artifact.Model != want.Model || artifact.CSS != want.CSS || artifact.Verification != want.Verification {
		return fmt.Errorf("directory enhancement: replay mismatch")
	}
	return nil
}

func enhanceDirectory(base DirectoryArtifact) (DirectoryEnhancedArtifact, error) {
	fail := func() (DirectoryEnhancedArtifact, error) {
		return DirectoryEnhancedArtifact{}, fmt.Errorf("directory enhancement: generated envelope failed schema")
	}
	names := []string{"directory_enhanced.go", "directory_enhanced_model.go.txt", "directory_enhanced.templ.txt", "directory_enhanced.css", "directory_enhanced.cue", "directory.cue"}
	inputs := make(map[string][]byte, len(names))
	h := sha256.New()
	for _, name := range names {
		b, err := directoryEnhancedImplementation.ReadFile(name)
		if err != nil {
			return fail()
		}
		inputs[name] = b
		fmt.Fprintf(h, "%d:", len(b))
		_, _ = h.Write(b)
	}
	digest := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	template, model, css := inputs["directory_enhanced.templ.txt"], inputs["directory_enhanced_model.go.txt"], inputs["directory_enhanced.css"]
	result := DirectoryEnhancedArtifact{SchemaVersion: 1, Adapter: "p04-directory-enhanced-v1", Base: base,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), TemplSHA256: digest(template), ModelSHA256: digest(model), CSSSHA256: digest(css),
		Templ: string(template), Model: string(model), CSS: string(css), Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail()
	}
	instance := &build.Instance{}
	for _, name := range []string{"directory.cue", "directory_enhanced.cue"} {
		if err := instance.AddFile(name, inputs[name]); err != nil {
			return fail()
		}
	}
	ctx := cuecontext.New()
	if err := ctx.BuildInstance(instance).LookupPath(cue.ParsePath("#DirectoryEnhancedArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail()
	}
	return result, nil
}
