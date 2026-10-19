package fusion

// elixirimport_test.go — LB-Q-elixir L3 (wave lang3): alias, import, require and use name a module, and the
// module is the file(s) whose defmodule defines it, wherever the file sits (the dialect's package is the enclosing
// defmodule, @scope.package, and namespace-dir resolves a name no directory spells to the files declaring it as
// their package). An `import` brings the target's functions in unqualified, an alias or a group member binds the
// name it chooses (`as: U`, `{Mailer}`), and a module nothing in the service defines (Logger, a dependency) or a
// computed one (`alias __MODULE__.Child`) resolves to no file.

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_ElixirImportsResolve(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"lib/app/helpers.ex":         "defmodule App.Helpers do\n  def slug(s), do: s\nend\n",
		"lib/app/repo.ex":            "defmodule App.Repo do\n  def all(q), do: q\nend\n",
		"lib/app/accounts/user.ex":   "defmodule App.Accounts.User do\n  def name(u), do: u\nend\n",
		"lib/app/mailer.ex":          "defmodule App.Mailer do\n  def deliver(m), do: m\nend\n",
		"lib/app/billing/invoice.ex": "defmodule App.Billing.Invoice do\n  def total(i), do: i\nend\n",
		"lib/app/unrelated.ex":       "defmodule App.Unrelated do\n  def slug(s), do: s\nend\n",
		// two modules in one file, neither named by its path: the definer is still the file
		"lib/app/bundle.ex": "defmodule App.Bundle.One do\n  def one, do: 1\nend\n\ndefmodule App.Bundle.Two do\n  def two, do: 2\nend\n",
		"lib/app/web.ex": "defmodule App.Web do\n  alias App.Repo\n  alias App.Accounts.User, as: U\n  alias App.{Mailer, Billing.Invoice}\n" +
			"  alias App.Bundle.Two\n  import App.Helpers\n  require Logger\n  alias __MODULE__.Child\n\n" +
			"  def page(x) do\n    slug(x)\n    App.Repo.all(x)\n    U.name(x)\n    Mailer.deliver(x)\n    App.Bundle.Two.two()\n    Logger.info(x)\n  end\nend\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "ex", Path: dir}); err != nil {
		t.Fatal(err)
	}
	all, err := st.TypedEdges(ctx, "ex")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	got := map[string]bool{}
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "lib/app/web.ex" {
			got[e.FromSymbol+" -> "+e.ToSymbol] = true
			files[e.ToSymbol] = true
		}
	}
	for _, w := range []string{
		"ex::lib/app/App.Web.page -> ex::lib/app/App.Helpers.slug",                // import App.Helpers: slug(x), unqualified
		"ex::lib/app/App.Web.page -> ex::lib/app/App.Repo.all",                    // alias App.Repo: App.Repo.all(x)
		"ex::lib/app/App.Web.page -> ex::lib/app/accounts/App.Accounts.User.name", // alias App.Accounts.User, as: U: U.name(x)
		"ex::lib/app/App.Web.page -> ex::lib/app/App.Mailer.deliver",              // alias App.{Mailer, ...}: Mailer.deliver(x)
		"ex::lib/app/App.Web.page -> ex::lib/app/App.Bundle.Two.two",              // alias App.Bundle.Two, defined in bundle.ex beside One
	} {
		if !got[w] {
			t.Errorf("missing file-resolved edge %s; have %v", w, got)
		}
	}
	for k := range got {
		for _, bad := range []string{"unrelated", "logger", "child"} {
			if strings.Contains(strings.ToLower(k), bad) {
				t.Errorf("a module nothing imports resolved: %s", k)
			}
		}
	}
}
