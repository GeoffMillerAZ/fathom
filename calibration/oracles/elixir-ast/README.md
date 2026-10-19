# elixir-ast — tier-B Elixir definitions and imports oracle (LB-Q-elixir, wave lang3)

`elixir_ast_dump.exs` writes `indexbench lang-oracle -tool-oracle`'s neutral `definitions` (`--mode defs`) and `imports`
(`--mode imports`) for an Elixir subject. It parses every `.ex` and `.exs` file with Elixir's own parser
(`Code.string_to_quoted/2`, `token_metadata: true`), so a definition is a call in the syntax tree and never text:

- **Definitions**: `defmodule`, `defprotocol`, `defimpl`, `def`/`defp`, `defmacro`/`defmacrop`, `defguard`/`defguardp`,
  `defdelegate`, `defn`/`defnp`, and the `@type`/`@typep`/`@opaque`/`@callback`/`@macrocallback` attributes. A name the
  source does not spell (`def unquote(name)(x)`, an operator definition) is no definition. `end` is the `end` keyword's
  line, else the last line any token of the call sits on. ExUnit `test` blocks are not emitted.
- **Imports**: `alias`, `import`, `require` and `use` of a literal alias (`alias Foo.{A, B.C}` is one statement per
  member). A module resolves to the file(s) whose `defmodule` defines it, with nested `defmodule`s expanded
  (`defmodule A do defmodule B` defines `A.B`): the language's own naming, not a path convention. A module no subject
  file defines (Logger, a dependency) names no subject file and is counted in `disclose`, never listed.

Tier B, not A: it reads definitions and statements from the syntax tree without compiling, so a definition a macro
generates and a module alias a macro injects are outside it, and the module-to-file lookup is this script's, not the
compiler's. Tier B caps L3 at good.

Why this oracle and not universal-ctags (the row's named oracle): ctags tags Elixir code that sits inside a heredoc
(`@moduledoc """ ... defmodule MyApp.Endpoint do ... def action(conn, _) do ... """`: 119 `def`, 50 `defmodule` and 22
`defp` on Phoenix v1.8.9 alone) and its module end lines follow those false modules (`Phoenix.Socket`, lines 1-921, is
given `end:38`), so against ctags range_agreement reads .5965 on Phoenix and .5857 on Ecto (sym_recall .9302 and
.9617, sym_precision .9985 and .9997, 548 and 1,273 `test` names it writes with spaces, which the harness's
last-segment name rule cannot compare). Design D2 (a verified mis-tag) allows the stronger oracle; the reading keeps
the ctags numbers in its notes.

Calls are graded by `calibration/oracles/ts-tags` (tier C): the grammar's own `queries/tags.scm` run by py-tree-sitter.

    cd <tools> && curl -sLO https://github.com/erlef/otp_builds/releases/download/OTP-29.1.2/OTP-29.1.2-macos-arm64.tar.gz
    mkdir otp && tar xzf OTP-29.1.2-macos-arm64.tar.gz -C otp
    curl -sLO https://builds.hex.pm/builds/elixir/v1.19.6-otp-27.zip && unzip -q v1.19.6-otp-27.zip -d ex
    PATH=<tools>/otp/bin:<tools>/ex/bin:$PATH elixir elixir_ast_dump.exs --mode defs <clone> > out/elixir-phoenix-defs.json
    PATH=<tools>/otp/bin:<tools>/ex/bin:$PATH elixir elixir_ast_dump.exs --mode imports <clone> > out/elixir-phoenix-imports.json
    uv venv pyts && uv pip install --python pyts/bin/python tree-sitter==0.25.2 tree-sitter-elixir==0.3.5
    pyts/bin/python -I ../ts-tags/tags_oracle.py --lang elixir --query ../ts-tags/elixir-tags.scm --ext .ex --ext .exs \
        --honor-ignore --enclose-modules <clone> > out/elixir-phoenix-calls.json
    go run ./cmd/indexbench lang-oracle -dialect elixir -subject <clone> -subject-name phoenixframework/phoenix \
        -tool-oracle out/elixir-phoenix-defs.json -tool-oracle out/elixir-phoenix-calls.json \
        -tool-oracle out/elixir-phoenix-imports.json -out <reading>

Subjects: phoenixframework/phoenix v1.8.9 (177 files) and elixir-ecto/ecto v3.14.2 (126 files). Bench-only; never a
product dependency. Elixir 1.19.6 is compiled for OTP 27 and runs on OTP 29.1.2 (a macOS arm64 build from erlef/otp_builds).
