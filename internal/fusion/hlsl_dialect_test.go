package fusion

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// TestClassifyDialectHLSLFxSniff (LB-Q-hlsl, H5-R3): .fx is also used by other
// formats, so the hlsl manifest claims one only by shader structure (a cbuffer
// or tbuffer block, an SV_ semantic, a register binding, a technique block),
// never by a word a comment, an #include line or a struct name can carry.
func TestClassifyDialectHLSLFxSniff(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		name, path, src, want, by string
	}{
		{"shader with a cbuffer", "fx/Basic.fx", "#include \"Common.fxh\"\n\ncbuffer Parameters : register(b0)\n{\n    float4 DiffuseColor;\n};\n\nfloat4 PSBasic(float4 c : COLOR0) : SV_Target0 { return c; }\n", "hlsl", connector.ClassifiedBySniff},
		{"effect with a technique", "fx/Legacy.fx", "float4 PS(float4 c : COLOR0) : COLOR0 { return c; }\n\ntechnique11 Main\n{\n    pass P0 { }\n}\n", "hlsl", connector.ClassifiedBySniff},
		{"semantic only", "fx/Pass.fx", "float4 VS(float4 p : POSITION) : SV_Position { return p; }\n", "hlsl", connector.ClassifiedBySniff},
		{"binding only", "fx/Tex.fx", "Texture2D Tex : register( t0 );\n", "hlsl", connector.ClassifiedBySniff},
		{"JavaFX script, float in a comment", "ui/Main.fx", "// the float value below is a width\nimport javafx.stage.Stage;\n\nStage {\n    title: \"Main\"\n    width: 250\n}\n", "", connector.ClassifiedAmbiguous},
		{"Flux config, float and an include line", "cfg/app.fx", "#include \"base.fx\"\n# float tolerance for the VSOutput and PSInput stages\nname = \"app\"\nreplicas = 2\n", "", connector.ClassifiedAmbiguous},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, []byte(c.src))
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("%s: ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.name, c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
}
