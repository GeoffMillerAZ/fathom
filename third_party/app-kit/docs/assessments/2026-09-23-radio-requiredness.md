# Native radio requiredness: rejected defect hypothesis

Task: preserve correct native form behavior while hardening reusable controls.
A review proposed moving `required` from a disabled first radio to an enabled peer.
Source inspection alone could not establish that the existing output was defective.
No production change was made. No API, consumer, dependency or quality status changes.

One synthetic Chrome control used four minimal native HTML shapes corresponding to
`RenderRadioGroup`, separate form owners and no scripts or checked values:

| Shape | Observed form validity | Enabled peer missing a value |
|---|---|---|
| First radio required and disabled; second enabled | invalid | yes |
| First radio required; both enabled | invalid | yes |
| Required group inside disabled fieldset | valid | no validating peer |
| Optional group, first radio disabled | valid | no |

The hypothesized requiredness loss was disproved for this browser control. Retain
the native renderer. `TestRadioGroup_DisabledRequiredStructure` pins the corresponding
attributes, unselected values, shared name and distinction between option and fieldset
disabling. It is structural proof, not an executed browser validation test.
Go1.26.3 `go test -race ./appkit/ui/form` passes, as does the subsequent uncached
four-package race run recorded in the [handoff check](2026-09-23-untracked-quality-handoff.md).
Independent review found no issue in the test or these evidence boundaries.

The prior acquisition passed four fixed checks in one cell, with no reported errors.
This follow-up reread the report and verified all three archive/report hashes; it did
not rerun the browser. Chrome153.0.8010.53, Playwright1.62.1, Node26.3.0; source driver
and prospective contract were retained before execution. Evidence root:
`/Volumes/m2ext/appkit-evidence/appkit-select-guide.of7K36NO`.

- `radio-validity-prospective.tar`: `b6a97420b7325a2fbe9bf69145e43d2c450b24fadec4474fcca3631492b24b46`
- `radio-validity-acquisition.tar`: `2fa70d6b1ed63091b22ddefb278637729e30405e0056867c65e9aa5b1005a4dc`
- `radio-validity-report.json`: `48dec8e69c94471ffe644293954f2e46b7a1edae70cd10e49e09a8aa105ec5b6`

Limits: minimal HTML, not a current Go-rendered fixture or native submission journey;
no other browser engine, screen-reader, visual, zoom or performance claim. The control
does not certify the renderer or replace app-owned server validation. Existing
[radio feedback evidence](2026-09-23-radio-feedback.md) retains its separate scope.
