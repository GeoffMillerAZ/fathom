# elixir_ast_dump.exs — tier-B Elixir definitions and imports oracle (LB-Q-elixir, wave lang3; see README.md).
#
# Parses every .ex/.exs file under <root> with Elixir's own parser (Code.string_to_quoted/2, token_metadata on) and
# writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads.
#
#   --mode defs     {"tool","version","tier":"B","definitions":[{path,name,line,end,kind}]}
#   --mode imports  {"tool","version","tier":"B","disclose":[...],"imports":[{path,line,to_path,module}]}
#
# Definitions are the calls the language defines things with, read from the AST (never from text, so a `defmodule`
# inside a heredoc is a string and no definition): defmodule, defprotocol, defimpl, def/defp, defmacro/defmacrop,
# defguard/defguardp, defdelegate, defn/defnp, and the @type/@typep/@opaque/@callback/@macrocallback attributes. A
# definition whose name the source does not spell (`def unquote(name)(x)`, an operator definition `def a + b`)
# has no static name and is left out. `line` is the call's first line; `end` is its `end` keyword's line, else the
# last line any of its tokens sits on. Names are as written (a module is `Foo.Bar` as spelled in its defmodule).
#
# Imports are the alias, import, require and use statements whose module is a literal alias (`alias Foo.{A, B.C}`
# is one statement per member). A statement's module resolves to the file(s) of the subject whose defmodule
# defines it, with nested defmodules expanded (`defmodule A do defmodule B` defines A.B): the language's own
# naming, not the lib/foo/bar.ex convention the dialect's strategy assumes. A module no file of the subject defines
# (Logger, a dependency) names no subject file and is counted, never listed.
#
# usage: elixir elixir_ast_dump.exs --mode defs|imports <root> > out.json
defmodule AstDump do
  @defs ~w(def defp defmacro defmacrop defguard defguardp defdelegate defn defnp)a
  @kind %{def: "function", defp: "function", defn: "function", defnp: "function", defmacro: "macro", defmacrop: "macro",
          defguard: "guard", defguardp: "guard", defdelegate: "delegate"}

  def files(root) do
    Path.wildcard(Path.join(root, "**/*.{ex,exs}"), match_dot: true)
    |> Enum.reject(&String.contains?(&1, "/.git/"))
    |> Enum.sort()
  end

  def parse(path) do
    src = File.read!(path)
    case Code.string_to_quoted(src, token_metadata: true, file: path) do
      {:ok, ast} -> {:ok, ast}
      {:error, _} -> :error
    end
  end

  def rel(root, p), do: Path.relative_to(p, root)

  # the last line a node's tokens sit on
  def last_line(ast) do
    {_, max} =
      Macro.prewalk(ast, 0, fn
        {_, meta, _} = n, acc when is_list(meta) ->
          lines = [meta[:line], get_in_line(meta[:end]), get_in_line(meta[:closing]), get_in_line(meta[:end_of_expression])]
          {n, Enum.max([acc | Enum.filter(lines, &is_integer/1)])}

        n, acc ->
          {n, acc}
      end)

    max
  end

  defp get_in_line(kw) when is_list(kw), do: kw[:line]
  defp get_in_line(_), do: nil

  def alias_name({:__aliases__, _, [first | _] = parts}) when is_atom(first),
    do: if(Enum.all?(parts, &is_atom/1), do: Enum.map_join(parts, ".", &Atom.to_string/1), else: nil)

  def alias_name(_), do: nil

  # the static name a definition's head spells, or nil
  def head_name({:when, _, [head | _]}), do: head_name(head)
  def head_name({:unquote, _, _}), do: nil
  def head_name({name, _, args}) when is_atom(name) and is_list(args) do
    if Regex.match?(~r/^[a-z_][A-Za-z0-9_]*[?!]?$/, Atom.to_string(name)), do: Atom.to_string(name), else: nil
  end

  def head_name({name, _, ctx}) when is_atom(name) and is_atom(ctx),
    do: if(Regex.match?(~r/^[a-z_][A-Za-z0-9_]*[?!]?$/, Atom.to_string(name)), do: Atom.to_string(name), else: nil)

  def head_name(_), do: nil

  # ---- definitions
  def defs(ast, path, acc) do
    {_, acc} =
      Macro.prewalk(ast, acc, fn node, acc -> {node, def_of(node, path, acc)} end)

    acc
  end

  defp add(acc, path, name, kind, node, line) do
    [%{path: path, name: name, kind: kind, line: line, end: end_line(node)} | acc]
  end

  defp end_line({_, meta, _} = node) do
    case meta[:end] do
      kw when is_list(kw) -> kw[:line]
      _ -> last_line(node)
    end
  end

  defp def_of({d, meta, [name | _]} = node, path, acc) when d in [:defmodule, :defprotocol] do
    case alias_name(name) do
      nil -> acc
      n -> add(acc, path, n, if(d == :defmodule, do: "module", else: "protocol"), node, meta[:line])
    end
  end

  defp def_of({:defimpl, meta, [name | _]} = node, path, acc) do
    case alias_name(name) do
      nil -> acc
      n -> add(acc, path, n, "implementation", node, meta[:line])
    end
  end

  defp def_of({d, meta, [head | _]} = node, path, acc) when d in @defs do
    case head_name(head) do
      nil -> acc
      n -> add(acc, path, n, @kind[d], node, meta[:line])
    end
  end

  defp def_of({:@, meta, [{a, _, [spec | _]}]} = node, path, acc) when a in [:type, :typep, :opaque, :callback, :macrocallback] do
    spec = case spec do {:when, _, [s | _]} -> s; s -> s end
    case spec do
      {:"::", _, [lhs | _]} ->
        case head_name(lhs) do
          nil -> acc
          n -> add(acc, path, n, if(a in [:callback, :macrocallback], do: "callback", else: "type"), node, meta[:line])
        end

      _ ->
        acc
    end
  end

  defp def_of(_, _, acc), do: acc

  # ---- modules (full names, nested defmodules expanded), for the import truth
  def modules(ast, path, acc), do: walk_mods(ast, [], path, acc)

  defp walk_mods({:defmodule, _, [name, [{:do, body}]]}, stack, path, acc) do
    case alias_name(name) do
      nil -> walk_mods(body, stack, path, acc)
      n ->
        full = Enum.join(stack ++ [n], ".")
        walk_mods(body, stack ++ [n], path, [{full, path} | acc])
    end
  end

  defp walk_mods({a, _, args}, stack, path, acc) when is_list(args),
    do: Enum.reduce(args, walk_mods(a, stack, path, acc), &walk_mods(&1, stack, path, &2))

  defp walk_mods({a, b}, stack, path, acc), do: walk_mods(b, stack, path, walk_mods(a, stack, path, acc))
  defp walk_mods(l, stack, path, acc) when is_list(l), do: Enum.reduce(l, acc, &walk_mods(&1, stack, path, &2))
  defp walk_mods(_, _, _, acc), do: acc

  # ---- import statements
  @stmts ~w(alias import require use)a
  def stmts(ast, path, acc) do
    {_, acc} = Macro.prewalk(ast, acc, fn node, acc -> {node, stmt_of(node, path, acc)} end)
    acc
  end

  defp stmt_of({s, meta, [mod | _]}, path, {ok, dyn}) when s in @stmts do
    line = meta[:line]

    case mod do
      {{:., _, [base, :{}]}, _, members} ->
        b = alias_name(base)

        if b == nil do
          {ok, dyn + 1}
        else
          Enum.reduce(members, {ok, dyn}, fn m, {ok, dyn} ->
            case alias_name(m) do
              nil -> {ok, dyn + 1}
              n -> {[{path, line, b <> "." <> n} | ok], dyn}
            end
          end)
        end

      _ ->
        case alias_name(mod) do
          nil -> {ok, dyn + 1}
          n -> {[{path, line, n} | ok], dyn}
        end
    end
  end

  defp stmt_of(_, _, acc), do: acc
end

{opts, [root], _} = OptionParser.parse(System.argv(), strict: [mode: :string])
root = Path.expand(root)
version = "Elixir #{System.version()} Code.string_to_quoted/2 (OTP #{System.otp_release()})"
files = AstDump.files(root)

parsed =
  Enum.flat_map(files, fn f ->
    case AstDump.parse(f) do
      {:ok, ast} -> [{AstDump.rel(root, f), ast}]
      :error -> []
    end
  end)

errors = length(files) - length(parsed)

case opts[:mode] do
  "defs" ->
    defs =
      parsed
      |> Enum.flat_map(fn {rel, ast} -> AstDump.defs(ast, rel, []) end)
      |> Enum.sort_by(&{&1.path, &1.line, &1.name, &1.kind})

    IO.puts(JSON.encode!(%{tool: "elixir-ast", version: version, tier: "B", definitions: defs}))
    IO.puts(:stderr, "files=#{length(files)} parse_errors=#{errors} definitions=#{length(defs)}")

  "imports" ->
    mods =
      parsed
      |> Enum.flat_map(fn {rel, ast} -> AstDump.modules(ast, rel, []) end)
      |> Enum.group_by(&elem(&1, 0), &elem(&1, 1))

    {stmts, dyn} =
      Enum.reduce(parsed, {[], 0}, fn {rel, ast}, acc -> AstDump.stmts(ast, rel, acc) end)

    stmts = Enum.reverse(stmts)

    imports =
      for {path, line, mod} <- stmts, to <- Enum.uniq(Map.get(mods, mod, [])), to != path do
        %{path: path, line: line, module: mod, to_path: to}
      end
      |> Enum.sort_by(&{&1.path, &1.line, &1.to_path})

    outside = Enum.count(stmts, fn {_, _, m} -> not Map.has_key?(mods, m) end)

    disclose = [
      "elixir-ast (tier B): #{length(stmts)} alias, import, require and use statements naming a literal module in #{length(files)} files " <>
        "(#{errors} the parser rejected); #{length(imports)} edges to the file(s) whose defmodule defines the module (nested defmodules expanded; " <>
        "a self-reference is no edge), #{outside} statements name a module no subject file defines (Logger, a dependency), #{dyn} name no static " <>
        "module (`alias __MODULE__.Foo`, `use unquote(m)`), counted and never listed"
    ]

    IO.puts(JSON.encode!(%{tool: "elixir-ast", version: version, tier: "B", disclose: disclose, imports: imports}))
    IO.puts(:stderr, "files=#{length(files)} parse_errors=#{errors} statements=#{length(stmts)} edges=#{length(imports)} outside=#{outside} dynamic=#{dyn}")
end
