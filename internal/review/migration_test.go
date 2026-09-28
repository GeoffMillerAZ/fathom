package review

import "testing"

// TestParseMigrationForms proves ParseMigration (CX-9, design §4) handles
// every declared kind — symbol, import, route, config, pkg — and rejects
// the malformed forms a plan author or a generator bug could produce.
func TestParseMigrationForms(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want Migration
	}{
		{
			name: "symbol",
			raw:  "symbol OldWidget -> NewWidget",
			want: Migration{Kind: MigrationSymbol, From: "OldWidget", To: "NewWidget"},
		},
		{
			name: "import quoted",
			raw:  `import "internal/old/pkg" -> "internal/new/pkg"`,
			want: Migration{Kind: MigrationImport, From: "internal/old/pkg", To: "internal/new/pkg"},
		},
		{
			name: "import bare",
			raw:  "import internal/old/pkg -> internal/new/pkg",
			want: Migration{Kind: MigrationImport, From: "internal/old/pkg", To: "internal/new/pkg"},
		},
		{
			name: "route",
			raw:  "route GET /v1/widgets -> GET /v2/widgets",
			want: Migration{Kind: MigrationRoute, From: "GET /v1/widgets", To: "GET /v2/widgets"},
		},
		{
			name: "config",
			raw:  "config old.setting.key -> new.setting.key",
			want: Migration{Kind: MigrationConfig, From: "old.setting.key", To: "new.setting.key"},
		},
		{
			name: "pkg version",
			raw:  "pkg example.com/widgetmod@v1.2.0 -> example.com/widgetmod@v2.0.0",
			want: Migration{Kind: MigrationPkg, From: "example.com/widgetmod@v1.2.0", To: "example.com/widgetmod@v2.0.0"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMigration(tc.raw)
			if err != nil {
				t.Fatalf("ParseMigration(%q) error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("ParseMigration(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestParseMigrationRejectsMalformed proves every malformed input is a
// named error, never a zero-value Migration silently accepted.
func TestParseMigrationRejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"unknown kind", "widget OldName -> NewName"},
		{"missing arrow", "symbol OldName NewName"},
		{"missing to", "symbol OldName ->"},
		{"missing from", "symbol -> NewName"},
		{"identical from and to", "symbol SameName -> SameName"},
		{"kind only, no from/to", "symbol"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMigration(tc.raw)
			if err == nil {
				t.Fatalf("ParseMigration(%q) = %+v, want an error", tc.raw, got)
			}
		})
	}
}

// TestParseMigrationDeterministic proves the determinism contract
// (internal/CLAUDE.md): identical input always parses to an identical
// result, whatever order or how many times it runs.
func TestParseMigrationDeterministic(t *testing.T) {
	raw := `import "internal/old/pkg" -> "internal/new/pkg"`
	first, err := ParseMigration(raw)
	if err != nil {
		t.Fatalf("ParseMigration(%q) error: %v", raw, err)
	}
	for i := 0; i < 25; i++ {
		got, err := ParseMigration(raw)
		if err != nil {
			t.Fatalf("run %d: ParseMigration(%q) error: %v", i, raw, err)
		}
		if got != first {
			t.Fatalf("run %d: ParseMigration(%q) = %+v, want %+v (identical to the first run)", i, raw, got, first)
		}
	}
}
