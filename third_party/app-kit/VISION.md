# app-kit vision

**One person. Many exceptional apps. Every app improves the next.**

app-kit is Geoff's personal product platform: accumulated design judgment, typed
capabilities and working experiences that make ambitious applications easier to build,
more useful, more beautiful and more reliable. Its purpose is to raise the starting
point and the ceiling together.

The standard is demanding: a person should understand what matters, complete a task
confidently, recover from mistakes and enjoy the product's craft. An agent should find
the right capability, understand its tradeoffs, integrate it correctly and prove it
works without Geoff having to become a frontend specialist.

## What compounds

The unit of reuse is a solved experience: its design decision, typed contract,
implementation, behavior under real states, evidence and upgrade path. A collection
of snippets cannot carry those benefits by itself.

An app uncovers a need. The kit turns the reusable part into a capability, demonstrates
it in the Gallery, makes it discoverable through the Assistant, and carries it into
another app through an explicit upgrade. Feedback returns as a better default, a new
recipe, a test or a documented boundary. Improvements travel; product identity stays local.

## The experience we aim for

Precise, expressive and calm enough to think in. Dense where comparison benefits,
spacious where decisions need attention. Strong typography and composition, coherent
semantic color, purposeful motion, crisp input response and readable states in every
supported theme. No default aesthetic is right for every app.

Beauty and utility reinforce each other: hierarchy reveals the next action, motion
explains change, color encodes meaning, and empty/error states preserve momentum.
Dashboards should reveal what changed, why it matters, how trustworthy it is and what
can be done next. A wall of attractive numbers does not meet this bar.

Accessibility, responsiveness and performance are part of craft. Keyboard and pointer
paths are peers; narrow screens deserve designed navigation; data does not become
meaningless without color, motion, a hover tooltip or a fast network.

## One platform, focused sub-kits

| Family | Product promise |
|---|---|
| Foundation | CUE-authored semantic tokens and typed contracts reach web, Go, portable assets and terminals |
| Identity and style tools | Named themes, deliberate effects and app-owned living guides make customization previewable, reviewable and reversible |
| Application shell | Navigation, identity/profile, appearance/settings, help/about, search and common workflow states are easy to compose |
| Analytics | Trustworthy metrics, periods/comparisons, filters, tables, charts, temporal exploration, linked detail and export work together |
| Interaction | Forms, overlays, command/search surfaces, progress, notifications, approval and recovery behave consistently |
| CLI-kit | Go/CUE/Cobra and Charm deliver excellent help, config explanation, machine output and purposeful keyboard/mouse interfaces |
| Host adapters | A product can add native desktop capability to its web foundation without binding domain logic to a shell |

These are capability boundaries, not a demand for six frameworks or modules.
[Scope](docs/intent/scope.md) separates existing packages from accepted work.
Apps select the layers and profiles that fit them. Stopping is a complete adoption.
Shared capabilities free each app to express its own identity.

## The Gallery and the Assistant

The Gallery is the living style guide and experience laboratory. It runs the actual
kit, shows its offerings and their maturity, demonstrates complete tasks and difficult
states, and supplies deterministic specimens for visual and behavioral checks.
The existing Gallery is the starting point. Make it easy to launch locally and find a
working example before beginning a screen.

The Assistant is one coherent appkit MCP surface over the same catalog. It answers:
what solves this task, what does it require, how does it fit this app, where can I see
it work, what evidence supports it, and how do improvements reach the consumer?
First-party recipes, source dependencies and commercial references must be distinguishable.
Neither runtime applications nor ordinary kit guidance should require a paid corpus.

TailwindPlus and MatDash are valuable design inputs. Use them deliberately for
composition, interaction and visual ideas, with verified dependencies and provenance.
A purchased template does not supply domain semantics, task quality or unlimited
redistribution rights.

Geoff's own best work is a first-class design input too: especially the personal site's
white-neon, pink/purple Synthwave identity, with the newer indigo dashboard as another
choice. Preserve distinctive craft without forcing every application into that look.
Separate identity, scheme, typography/shape, density and optional effects. An app-owned
living guide should preview its real tasks and difficult states, compare candidate
tokens, expose font/contrast costs, reset safely and export an explicit reviewed change.
Use one source of design truth across app, guide and agent discovery, not copied catalogs.

Invest in a private, tested conversion pipeline so purchased design craft becomes
replayable Go/templ, static and client-native capabilities. Automate known transformations;
use agents to resolve bounded failures into reviewed rules, patches and regression tests.
Preserve original quality, expose unsupported targets honestly and measure the savings
across subsequent apps and vendor updates. Conversion count is not the success metric.
Finish an excellent core of complete patterns first. Convert and enhance remaining
variants when consumed or in bounded owner-selected batches. Versioned standards and
a CUE inventory make implementation, quality-gate evidence and consumer adoption
visible separately; deferred work never masquerades as passed quality or compatibility.

## Technical taste with measurable reasons

Prefer Go for typed, deterministic application logic; CUE for authored contracts and
generation; portable artifacts where they suffice; WASM when a measured client need
earns it. Keep the kit build reproducible and its outputs usable without a bundler.
Support other application stacks through selected artifacts and recipes.

Static sites deserve complete fast HTML without mandatory hydration. Heavy client
work deserves immediate local interaction and a single clear state/DOM owner, with
workers or Wasm where they prove value. Excellence is the fewest recurring problems
for the intended task, not the fewest technologies at any cost. [Runtime selection](docs/design/runtime-selection.md)
and the [conversion pipeline](docs/design/conversion-pipeline.md) make this concrete.

Go + templ + Datastar is the preferred dynamic operational web profile. NATS is a
capability for event needs that justify it. Charm is the terminal ecosystem. Wails v3
is the preferred desktop adapter candidate. None substitutes for design work, and none
is imposed on a consumer that only needs another layer.

Browser delivery is first-class for selected Wails apps: core workflows work without
a native window or bridge. Native features add value through explicit adapters, with
useful web equivalents or honest task-level exceptions. Local browser delivery does
not imply cloud hosting. [Web/desktop parity](docs/design/web-desktop-parity.md) owns
the contract; existing apps migrate when selected, not through a forced fleet rewrite.

Investigate new technology early. Pin what ships, measure it on the real workload and
retain an explicit fallback. Native mobile and advanced client rendering must earn
their support claims through separate proof. Share domain code and contracts across
targets; do not promise one identical binary across operating systems.

## How we know the kit is working

- A new application reaches a useful, deliberately designed first workflow in a day
  using a proven recipe; record hands-on effort and integration failures.
- A second comparable screen needs less app-specific behavior code and less correction,
  without losing the capabilities that make the app useful.
- A representative analytics task becomes faster and less error-prone with truthful
  numbers, context, drill-down and reproducible export.
- A kit fix reaches a second relevant consumer through a reviewed upgrade, with the
  original defect absent and app-specific identity intact.
- A fresh agent session finds and integrates the applicable recipe, recognizes an
  incompatible profile and verifies behavior without an oral tour of the kit.
- Browser, desktop and terminal promises are backed by target-specific evidence.
  Performance budgets are measured, missing checks are visible, and visual quality
  is reviewed in the running product.

These are acceptance ambitions, not measurements achieved by this document.
[Intent](docs/intent/README.md) owns the operating rules; [scope](docs/intent/scope.md)
owns current exit tests; [roadmap](ROADMAP.md) owns sequence.
