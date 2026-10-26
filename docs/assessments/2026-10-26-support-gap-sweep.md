# Support-gap sweep: what real repositories contain that fathom reads badly or not at all

## Verdict

Fathom's board overstates what the runtime reads. The sharpest gaps are not exotic languages. They are file kinds that
sit in most Python, Ruby, iOS, Rails and .NET repositories: Python `requirements*.txt` and `setup.cfg`, Ruby `Gemfile`,
Swift plus Xcode project files, T-SQL and MySQL dialects, and the template engines (ERB, Razor, Twig, EJS and others).
A second finding matters as much: the board and `list_services` disagree. Formats the board calls "unsupported
(disclosed)" are often silent at runtime, because the runtime table lists only unambiguous extensions. The plan that
carries the work is `docs/plans/language-breadth.gap1.md`. This assessment is the evidence behind it and decides nothing.

Not claimed: no reach figure here is measured by fathom. Reach rests on dated secondhand surveys or on inference, and
each item says which. The row-level state of each gap (level, grade, refusal) is computed by `fathom support` and the
readings under `docs/bench/readings/lang/`, never held here (ADR-054, I54).

## Method

Four passes, in order.

1. **Eight domain sweeps** (measured, 2026-10-26): web templates, mobile and platform, data and database, build and
   packaging, infrastructure and CI, legacy enterprise, documents and diagrams, game and embedded. Each sweep listed
   candidate formats and a dated reach source. Sources were the Stack Overflow Developer Survey 2025 (secondhand
   figures), the TIOBE index (search-engine based, so it measures attention, not repository share), npm weekly
   download counts quoted by a package-security vendor and one blog (ejs about 26.7 million, handlebars about 12
   million), and inference from an ecosystem's defaults where no figure was retrieved. Inferred reach is graded
   unmeasured; the surveys are graded estimate.
2. **Repository verification** (measured, 2026-10-26): every candidate was checked against the tree for dispatch
   (`internal/connector/connector.go`), the runtime disclosure table (`internal/connector/unsupported_language_gen.go`),
   the board (`docs/languages.md`), the queue and the plan. A claim counted only with a file and line.
3. **Synthesis**: gaps ranked by how many repositories contain the item times how big the gap is (absent, refused,
   queued but unbuilt, disclosed on the board but silent at runtime, or weak), then by whether a good oracle exists,
   then by effort (S, M, L, XL). "Below target" alone is not a gap; it counts only where some file kind inside the
   language is not read at all.
4. **Completeness critic**: a second reader re-checked the synthesis and found two top-reach items the board hid,
   six wrong claims (W1 to W6) and twelve additions (A1 to A12). A later review of the roadmap commits found that
   Jinja2 had already landed in wave B.

## S0: the disclosure claim does not hold at runtime (systemic)

`unsupportedLangExtMap` lists only unambiguous extensions, by design. A format claimed by basename, glob or content
sniff never appears in it. None of these were in the runtime table when the sweep ran: `.env`, `.bat`/`.cmd`,
`.bas`/`.cls`/`.frm`, `.tex`, `.cshtml`, `.razor`, `.erb`, `.hbs`, `.ejs`, `.zsh`, `.fish`, `.Rmd`, `.qmd`, `.xml`,
`dependabot.yml`, `AndroidManifest.xml`, `Containerfile`. LB-F453 (open) promises the `.bat`/`.cmd`, Handlebars, EJS,
Liquid and ERB disclosure and had not delivered it. The critic widened S0 to the board itself, where rows named file
kinds nothing dispatches:

- W1: `requirements*.txt` and `setup.cfg` are not classified (connector.go classifies `pyproject.toml` only).
- W2: the Ruby row named `Gemfile`; connector.go says Gemfile, `*.gemspec`, Podfile and `*.podspec` are not claimed.
- W3: the Python row named `.pyi`; only `.py` is dispatched.

The board names were corrected in this wave (no reading moved). The check that keeps them honest is GAP1-1; the
runtime table for basename, glob and sniff formats is GAP1-2.

## Corrected top 10

1. **Python dependency manifests** (`requirements*.txt`, `setup.cfg`, then poetry, uv, Pipfile, `environment.yml`):
   absent, and the board overclaimed it. S to M. GAP1-27, GAP1-1.
2. **Swift plus Xcode project files**: Swift is refused (123 of 642 calibration files, 19.16%, parse with errors against
   a 5% limit); Xcode targets and file membership are not queued before this sweep. L. GAP1-30, GAP1-31.
3. **T-SQL, MySQL and PL/SQL dialects**: `.sql` goes to a grammar that excludes these families, with no per-file
   disclosure. Sniff and disclosure first, M. GAP1-28.
4. **Ruby `Gemfile`, gemspec, Podfile**: queued, not built, overclaimed. S. GAP1-6.
5. **Jinja2**: done. The refusal was throughput only; wave B dispatched it (L1 to L3 excellent, 5.17 MB per CPU-second,
   0.58% of files in error). It is not a gap row.
6. **Containerfile, `Dockerfile.*`, `*.Dockerfile`**: not dispatched. S. GAP1-4.
7. **Razor and Blazor**, then XAML: absent and undisclosed. M. GAP1-25, GAP1-26.
8. **ERB, then Haml and Slim**: Rails views, absent. M. GAP1-16, GAP1-17.
9. **AndroidManifest.xml and `res/` XML**: queued, not built. S to M. GAP1-8, GAP1-9.
10. **The remaining template engines** (EJS, Handlebars, Liquid, Twig, Pug, Nunjucks, Go templates, Blade): one shared
    scanner, S each. GAP1-10, GAP1-14, GAP1-18 to GAP1-24.

Demoted by the critic: dotenv and Dependabot/Renovate (top reach, little code-graph value; fixing S0 makes them honest
and they fit the cheap-dispatch lane), SQL migrations, Azure Pipelines and dev containers (lower reach times gap).
Other additions filed as rows: F# `.fs` (silent, rides GAP1-2), game engines (GAP1-38), workflow DAGs (GAP1-39), the
Svelte 5 runes symbol rule (GAP1-40; the critic read the Svelte board number backwards, 0.3816 is the newer reading and
a re-time will not fix it). Lockfiles (A12) are a note only: top reach, low code-graph value.

## Supported well (people assume these are missing)

Next.js file routing, Jenkins (Jenkinsfile L3, shared libraries linked), Gradle Groovy DSL, Angular (TypeScript L4,
templates read as HTML; only Angular-specific meaning is missing), Flutter (Dart L4), React Native, Elixir and
Objective-C (L3), PostgreSQL SQL, API and schema formats (Prisma, OpenAPI, JSON Schema, Protobuf, GraphQL, Thrift),
Terraform, Kubernetes and Helm, Argo, Ansible, Istio, Nix, Jupyter, Bash, Markdown, Makefile, SCSS, Solidity, VHDL,
MATLAB (with a written ceiling) and R. Levels are read from `fathom support`, not from this list.

## The three proposed lanes (low merge conflict)

- **L1, honest disclosure and cheap dispatch**: S0 and its board check, `.pyi`, Containerfile, dotenv, Ruby DSL
  manifests, Dependabot and Renovate, AndroidManifest, Go templates, `.ino`, `.sbt`, Husky. Owns the disclosure
  generator and additive `connector.go` edits.
- **L2, template-engine scanner**: one shared hand-written Go scanner (ADR-066) with one owner, then ERB, Haml and
  Slim, EJS, Handlebars and Mustache, Liquid, Twig, Pug, Nunjucks, Blade, Razor. XAML needs no scanner. Every engine
  has an owner-tool oracle; Liquid's is the Ruby gem, not liquidjs.
- **L3, data and database**: SQL dialect sniff, application migrations (Alembic, Rails, Django, EF Core, offline
  oracles only), Python manifests. Owns the sql and python-packaging dialects.

Swift is not a lane: it needs the owner to rule on the product parser first. The recommendation is a hand-written Go
scanner with swift-syntax and `swiftc -frontend -dump-parse` as oracles only, because a helper binary adds a Swift
toolchain runtime dependency the product does not carry and needs an owner ruling (GAP1-30).

To keep merges clean: reserve LB-F id ranges per lane, keep `connector.go` and `register.go` edits additive, and let
only the integrator edit `language-breadth.queue.json` and regenerate `unsupported_language_gen.go` and
`docs/languages.md` (both are generated).

## Decisions it drove

Plan rows `docs/plans/language-breadth.gap1.md` (GAP1-1 to GAP1-40, GAP1-15 unused) and 16 new queue entries in
`docs/plans/language-breadth.queue.json` (wave 7), plus tiers on nine existing entries added at the owner's
2026-10-26 roadmap ask. Terraform and OpenTofu take tier T3 at target L3, matching the `#Tier` taxonomy and design
section 18.3 (every IaC entry is T3); ruling I-R1 is amended from T2. No ADR is changed; the Swift parser ruling, when it
comes, belongs in `ADR.md` (trigger on GAP1-30).
