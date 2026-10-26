# Compact-output security review

Independent differential/sharp-edges review inspected current source after shared
Fathom declined refresh. It used concrete native counterexamples, focused tests and
CUE checks. No confirmed blocker remained in the frozen reread.

| Finding | Disposition and evidence |
|---|---|
| CV-F1: longer output prefix exceeded normalized scan budget | Compact selection checks both final output and normalized representation plus source bytes before scanning; it never shrinks after a finding. Zero-prefix total-budget failure retains `payload_limit`. |
| CV-F2: future header produced an invalid compact shape | Closed header eligibility falls back to ordinary JSON on unknown fields. New row kinds/fields, wire tags and custom value/pointer JSON methods also retain ordinary output. |
| INC-21: inspection failure ignored its accepted output cap | Effective cap survives failure reset, final write includes newline, and undersized failure output emits zero bytes. Native json/text/compact cap-one probes and exact-fit tests pass. |
| Prefix-size assumption | An independent native probe checked all 101 prefixes of a 100-line fixture at their exact caps: monotone sizes, correct returned count and zero overruns. This does not prove every possible text shape. |
| Encoding cost | Per-row JSON round trips were removed. Current Git source profiles permit at most 32 candidate rows, bounding their shrinking loop; no material budget violation was found. The encoding benchmark remains slower than ordinary JSON and is not production economics. |

The reviewer rebuilt current source, verified tiny-cap/ordinary-success behavior,
ran focused compact/inspection-failure tests and CUE fixtures, and checked current
reflection eligibility against the actual plain row fields. Full repository gate,
independent native Git corpus, installed hooks and other native OS behavior were
outside that critic's claim. Separate instruments appear in [the record](README.md).
