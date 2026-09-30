package fusion_test

// vimscript_ingest_test.go — LB-Q-vimscript, refused (design §14.2): the vimscript dialect
// is built and measured (L1 excellent against universal-ctags on vim-plug and vim-fugitive),
// but 3 of those 7 files (vim-plug's plug.vim, fugitive's autoload/fugitive.vim and
// plugin/fugitive.vim) parse with an error under the pinned grammar
// (tree-sitter-grammars/tree-sitter-vim v0.8.1), over the 5% refusal line. A refused
// language "stays disclosed as unsupported": ingest does not route a Vim file to the
// dialect, whether it is claimed by extension (.vim) or by basename (vimrc, .vimrc, _vimrc,
// gvimrc, .gvimrc), extracts nothing from it, and names a .vim file in
// Report.UnsupportedLanguages. The dialect itself stays tested (TestDialectGoldens/vimscript)
// for the day a grammar parses the subjects under the line.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestIngest_VimscriptIsRefusedAndDisclosed(t *testing.T) {
	const vimSrc = "function! s:Toggle() abort\n  return 1\nendfunction\ncommand! Toggle call s:Toggle()\naugroup Acme\n  autocmd!\naugroup END\n"
	dir := t.TempDir()
	writeFile(t, dir, "plugin/acme.vim", vimSrc)
	writeFile(t, dir, "home/vimrc", vimSrc)
	writeFile(t, dir, "home/.vimrc", vimSrc)
	writeFile(t, dir, "home/_vimrc", vimSrc)
	writeFile(t, dir, "home/gvimrc", vimSrc)
	writeFile(t, dir, "README.md", "# Acme\n")

	if lang, ok := connector.UnsupportedLanguageForPath("plugin/acme.vim"); !ok || lang != "vimscript" {
		t.Fatalf("UnsupportedLanguageForPath(.vim) = %q, %v; want vimscript disclosed (refused, §14.2)", lang, ok)
	}
	obs, report := ingestDir(t, dir, "vimscript-refused")
	for _, o := range obs {
		ref := strings.SplitN(o.SourceRef, ":", 2)[0]
		if strings.HasSuffix(ref, ".vim") || strings.Contains(ref, "vimrc") {
			t.Errorf("a refused language was extracted: %s %s (%s)", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	if got, want := report.UnsupportedLanguages["vimscript"], 1; got != want {
		t.Errorf("UnsupportedLanguages[vimscript] = %d, want %d (%v)", got, want, report.UnsupportedLanguages)
	}
}
