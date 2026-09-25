# Full official HTML syntax diagnostic — 2026-09-13

The current translator accepts **439/657 inputs (66.8%)** in a read-only, in-memory
coverage run. Every accepted/refused-by-grammar translation replayed exactly. This is
broader syntax evidence, **not by itself** compilation, visual proof or publication.
The separate [compiler/DOM follow-up](2026-09-13-html-compiler-dom-audit.md) now proves all
439 accepted outputs in both formatting phases, without target certification.
No translator rule, limit, assertion or compatibility gate changed to obtain this result.

## Denominator and identity

All 657 official HTML members from the existing application-ui, ecommerce and marketing
archives remain in scope. Their exact IDs and input-set hash are frozen in the
[structural inventory](2026-09-13-html-conversion-coverage.md). The probe verifies whole
archives and rederived metadata, then exact bounded source reads before translation.
Input-set SHA-256: `d57b19cec955cc89c094e304ab7bb6d39ec1188fc88b0d6cb4c99f50ec0580b0`.
The source catalog revision stayed unchanged throughout the run.

This denominator excludes React/Vue variants, the thirteen full templates, Catalyst,
and the MatDash codebase. It is not a percent of the entire purchased design collection.
The separate [12-case pilot](2026-09-13-conversion-pilot.md) still owns its target/behavior
families; the [saved pilot](2026-09-13-saved-conversions.md) still has five translated
artifacts and five unsupported records. This diagnostic adds no saved derivatives.

## Results

| Official HTML archive | Inputs | Syntax translated | Grammar unsupported | Source-size refusal |
|---|---:|---:|---:|---:|
| Application UI | 364 | 260 | 103 | 1 |
| Ecommerce | 114 | 60 | 54 | 0 |
| Marketing | 179 | 119 | 60 | 0 |
| Total | **657** | **439** | **217** | **1** |

656 translation envelopes passed exact current-rule replay; all retain `not-verified`.
The 439 temporary outputs total 4,267,017 bytes and were discarded after counting/hashing.
No vendor projects or scripts ran; no network, dependency install or LLM was used.

The single size refusal is `html/data-display/calendars/05-year-view.html`, 403,534 bytes,
larger than the existing 256 KiB text/translation boundary. Whole-archive verification
still checks its original bytes; the exact-read refusal is exercised. It is included
in totals but never counted as translated, grammar-tested or replayed. The initial
probe incorrectly treated this expected product limit as an operational failure;
the report now distinguishes it without weakening the access limit.

Diagnostic file counts overlap: `unsupported-element` 190, `unsupported-attribute` 151,
`unsafe-resource` 2, `source-size` 1. Exclusive groups make the denominator auditable:

| Diagnostic combination | Files |
|---|---:|
| Element only | 66 |
| Element + attribute | 122 |
| Attribute only | 27 |
| Element + attribute + resource | 2 |
| Source size | 1 |

Unsupported does not mean defective vendor source. The bounded rule deliberately does
not port custom runtimes, scripts, inline style or arbitrary expressions. Do not drop
these nodes/attributes, silently normalize a design, or enlarge limits just to raise a rate.

## Reproduce and inspect

```sh
APPKIT_PROBE_HTML_COVERAGE_KIT_ROOT=/absolute/path/to/app-kit \
  go test ./internal/designconvert -run '^TestHTMLInventorySyntaxCoverageLive$' -count=1 -v
```

Optional `APPKIT_PROBE_HTML_COVERAGE_REPORT=/absolute/new-file.json` writes metadata
only with mode 0600 and exclusive creation; existing files are never overwritten.
Default invocation writes no report or derivative. Missing opt-in configuration is a
skip, not a corpus pass. Missing/corrupt inputs and unexpected translation/replay errors
fail; they do not become unsupported rows. Counts and exact source-set identity are checked.

Observed with Go 1.26.3 darwin/arm64, x/net 0.54.0, required templ 0.3.1020:
88.08 seconds test time, including source verification and CUE validation/replay.
This is a local diagnostic elapsed time, not rendering performance or conversion ROI.
Rule digest: `0db161e2e0e567ae491661614e0115f4042c3ea76b188f867c8242e44e90eb7c`.
The 349,410-byte report is retained privately at
`data/design-assets/coverage/35067bfc3e591bfbecb2dd6ea20038b28159a97a02cf57fdda934004eddba79c.json`;
its filename is its SHA-256. It contains one source-bound row per input, status,
byte/output-hash accounting and bounded diagnostic codes—no source or generated body.

Independent review found no probe defect. New synthetic canaries also pass under the
existing pinned format/generate/compile/namespace-aware DOM oracle: generic attribute
newline/tab/entities, adjacent multiline/empty comments, empty/newline-only pre/textarea.
An unsupported SVG `textPath` refusal canary preserves the existing grammar boundary.
These strengthen generic compiler checks; they do not compile the 439 corpus outputs.

## Next evidence, not automatic promotion

Retain distinct counts for inventoried, syntax-translated, compiled/DOM-equal, behavior/
visually verified, saved/published and consumer-adopted. Earlier 5/657 (0.8%) described
completed saved/compiled pilot cases, not the translator's measured acceptance rate.
At this syntax checkpoint there were five compiled/DOM-equal pilot cases; the linked
follow-up extends compiler/DOM evidence to all 439 accepted HTML inputs. Graphical
certifications remain zero. Interactive families still need a pinned runtime/adapter
and browser lifecycle/focus evidence; remaining grammar refusals are unchanged.
Follow [the conversion pipeline](../design/conversion-pipeline.md) before publishing
bulk target compatibility. Originals remain first-class regardless of every result.
