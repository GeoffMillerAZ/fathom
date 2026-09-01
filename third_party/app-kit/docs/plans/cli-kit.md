# cli-kit — the terminal target, built and consumed

Turns ADR-0018 into gated work. The five packages shipped 2026-08-21 with
their first consumer (env-kit's `bosun`, platform-engineering) migrated the
same day; this plan records the claims so the index can gate them, and holds
the open tail.

Gate: `fathom verify --service app-kit --plan docs/plans/cli-kit.md`

Row grammar note: the V1 tests are table-driven (calls inside subtest
closures — edge-void mode), so rows carry skip_test with the proving tests
and gates named in the skip prose; every named test ran green in `task
check` on 2026-08-21.

## 0 — Shipped

- [x] The terminal palette is generated from the CUE tokens — all 23 color
  roles, OKLCH converted to sRGB, both polarities — and the byte-identity
  gate covers it. {id: CK-1, skip_test: true, skip: "appkit/cli/theme/palette_gen.go emitted by tokens.EmitTerminalPalette via appkit gen --terminal; task check's git diff --exit-code includes the file; TestPaletteCoversAllRoles, TestPaletteRegenByteIdentical, TestOKLCHConversionKnownValues"}
- [x] fang's help chrome draws the kit's own roles: a fixed role-to-field
  mapping, documented in place, adaptive to terminal background. {id: CK-2, skip_test: true, skip: "theme.FangColorScheme with the mapping table in its doc comment; consumed by command.Execute; visible in bosun --help"}
- [x] command wraps cobra + fang with house defaults and the fleet exit-code
  convention. {id: CK-3, skip_test: true, skip: "TestExitCodeMapping and TestExecuteVersionFlag in appkit/cli/command; bosun gets --version, completion, man for free"}
- [x] Output discipline: color-profile detection via colorprofile, NDJSON,
  and a status table whose plain rendering is byte-compatible with the
  doctor format — styled adds color to status and remedy only. {id: CK-4, skip_test: true, skip: "TestStatusTablePlainMatchesDoctorGolden, TestStatusTableStyledContainsANSI, TestNDJSONRowShape in appkit/cli/output; the consumer-side proof is env-kit's unchanged TestDoctorRosterGolden"}
- [x] Layered CUE config with provenance: schema, system, XDG user, project
  file bounded at the git root, overrides — conflicts refuse, Explain names
  every loaded AND absent layer with digests. {id: CK-5, skip_test: true, skip: "TestConfigLayerPrecedence, TestConfigProjectWalkBoundedAtGitRoot, TestConfigExplainNamesLayer, TestConfigConflictRefused in appkit/cli/config"}
- [x] A thin bubbletea v2 runner with deterministic teatest support. {id: CK-6, skip_test: true, skip: "TestRunSmokeModel, TestRunHonorsContextCancel in appkit/cli/tui; mouse and alt-screen are opt-in View decorations per the bubbletea v2 API"}
- [x] First consumer proven: bosun migrated with zero behavioral drift on
  plain output and exit codes. {id: CK-7, skip_test: true, skip: "platform-engineering kits/env-kit — its goldens and exit-code tests passed unchanged across the migration; docs/plans/env-kit.md EK-62 is the consumer-side row"}

## 1 — Open tail

- [ ] cmd/appkit (and appkit-mcp) migrate onto cli-kit, ending the admitted
  two-style drift. {id: CK-10, skip_test: true, skip: "ADR-0018 revisit trigger; not started"}
- [ ] Cut the app-kit v0.1.0 tag so consumers can drop replace directives.
  {id: CK-11, skip_test: true, skip: "ADR-0017 section 4 prerequisite; trigger per ADR-0018 is the third fleet CLI or the second external consumer"}
- [ ] Styled table polarity: resolve against the real terminal background
  instead of the documented dark-palette simplification. {id: CK-12, skip_test: true, skip: "DIVERGENCE noted in output.RenderStatusTable's doc; needs a background query a one-shot Detect cannot provide"}
- [ ] bubbles/huh/glamour/log each join on first consumer need, one pin at a
  time. {id: CK-13, skip_test: true, skip: "ADR-0018 boundary; a form-bearing bosun init wizard is the likely huh trigger"}
- [x] Fix the pre-existing synthwave theme.css generator drift found during
  this work (gen output missing the shipped data-theme light block). {id: CK-14, skip_test: true, skip: "root cause: the committed artifact predated the emitter's H1 symmetric-light fix and task check never watched the themes dir; regenerated, pinned by TestCSSMatchesTheCommittedArtifact (proved RED first), and check now covers appkit/ui/themes/synthwave — task check green 2026-08-21"}
