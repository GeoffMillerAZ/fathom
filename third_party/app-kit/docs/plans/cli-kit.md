# cli-kit — the terminal target, built and consumed

Turns ADR-0018 into gated work. The five app-kit packages shipped 2026-08-21;
their in-repo contracts are gated here. Consumer adoption remains separate:
env-kit's `bosun` is documented as a user, but its behavioral-retention claim
needs a revision-bound receipt owned with that consumer.

Gate: `fathom verify --service app-kit --plan docs/plans/cli-kit.md --strict`

Row grammar note: every contract stays on its checkbox line. Fathom resolves
calls inside `t.Run`, callbacks and goroutines, so test reachability is gated
directly; it is static evidence, not a claim that a test ran or passed.

## 0 — Shipped

- [x] Generate the terminal palette from CUE tokens with the checked OKLCH-to-sRGB role pipeline. {id: CK-1, symbol: EmitTerminalPalette, package: appkit/ui/tokens, reachable_from: TestPaletteRegenByteIdentical}
- [x] Supply fang help chrome from the kit's fixed light/dark terminal role mapping. {id: CK-2, symbol: FangColorScheme, package: appkit/cli/theme, reachable_from: TestFangColorSchemeUsesLightDarkFunc}
- [x] Execute cobra commands through fang while retaining the typed CLI exit-code contract. {id: CK-3, symbol: Execute, package: appkit/cli/command, reachable_from: TestExecuteVersionFlag}
- [x] Render the byte-stable plain doctor table and progressively styled form from one geometry. {id: CK-4, symbol: RenderStatusTable, package: appkit/cli/output, reachable_from: TestStatusTablePlainMatchesDoctorGolden}
- [x] Load layered CUE configuration with git-root-bounded project discovery and conflict refusal. {id: CK-5, symbol: Load, package: appkit/cli/config, reachable_from: TestConfigLayerPrecedence}
- [x] Run Bubble Tea v2 models with context cancellation and deterministic test support. {id: CK-6, symbol: Run, package: appkit/cli/tui, reachable_from: TestRunHonorsContextCancel}
- [-] Capture a revision-bound Bosun adoption receipt proving retained plain output and exit behavior against the consumer's own tests. {id: CK-7, disposition: deferred, reason: "app-kit can prove cross-repo imports but not the consumer test run or retained behavior", trigger: "the platform-engineering consumer revision and exact verification command are recorded in a separately named receipt"}

## 1 — Deferred tail

- [-] Migrate `cmd/appkit` or `appkit-mcp` onto cli-kit when richer CLI UX justifies ending the admitted two-style drift. {id: CK-10, disposition: deferred, reason: "ADR-0018 keeps the kit binaries hand-rolled; apps/terminal is a separate reference and does not show either binary needs migration", trigger: "either binary needs richer CLI UX"}
- [-] Cut an authorized app-kit release so external consumers can replace local module overrides with a real version. {id: CK-11, disposition: deferred, reason: "tagging and publishing require owner release authority", trigger: "the owner authorizes a release after the ADR-0018 third-fleet-CLI trigger or another evidenced release need"}
- [-] Resolve styled-table polarity from the actual terminal background instead of the documented dark-palette simplification. {id: CK-12, disposition: deferred, reason: "the current one-shot color-profile detector does not expose terminal background polarity", trigger: "a supported background-polarity signal is available or a consumer demonstrates the need"}
- [-] Admit bubbles, huh, glamour or log one dependency at a time. {id: CK-13, disposition: deferred, reason: "ADR-0018 excludes speculative terminal dependencies", trigger: "a consumer needs the corresponding interaction, form, markdown or structured-logging capability"}
- [x] Keep committed synthwave CSS byte-identical to regeneration, including the light-theme branch. {id: CK-14, symbol: CSS, package: appkit/ui/themes/synthwave, reachable_from: TestCSSMatchesTheCommittedArtifact}
