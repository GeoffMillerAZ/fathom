# Decision port: `KindNoul` becomes `KindPredicate`

`foundations/decision` names its three question kinds generically. The yes/no kind
was `KindNoul`, one vendor's term; it is now `KindPredicate`, and its canonical value
is `"predicate"` (plan row DP-12, [decision 0017](../intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md)).

## What changed

- `KindPredicate Kind = "predicate"` is the yes/no-probability kind. `Answer.Kind`
  and `Record.Kind` for such a question now hold `"predicate"`, where they held
  `"noul"` before. A consumer that writes `string(kind)` to a log or a store now
  writes `"predicate"` for new rows.
- `ParseKind(s)` decodes `"choice"`, `"score"`, `"predicate"`, and the alias
  `"noul"`, which becomes `KindPredicate`. Any other value is an error naming it.
  `Kind` implements `encoding.TextUnmarshaler` through it, so JSON, configuration and
  stored rows that say `"noul"` decode to `KindPredicate`. Empty text decodes to the
  zero `Kind`, which `Validate` still rejects.
- `Validate`, `LLM` and `TypeSafe` also accept a `Kind("noul")` built without
  `ParseKind` and answer it as `KindPredicate`.
- CUE `#Kind` is `"choice" | "score" | "predicate" | "noul"`; `"noul"` is the
  decode alias.
- The package doc no longer describes the kinds as a mirror of one vendor. The
  vendor mapping table lives on `TypeSafe`'s doc.
- `Answer.Noul` and `Record.Noul` keep their names.

## What did not change

- The adapters' wires. The `LLM` prompt still labels the kind `"noul"` and asks for
  a `"noul"` answer property; the `TypeSafe` request still sends the `"noul"`
  primitive, and replies carrying `"type":"noul"` still parse. The three golden
  fixtures (`testdata/prompt.golden`, `schema.golden`, `typesafe_request.golden`) are
  byte-identical, so no prompt calibration needs re-measuring.
- `KindNoul` still compiles. It is a deprecated constant equal to `KindPredicate`,
  so `a.Kind == decision.KindNoul` keeps working.

## What to do

1. Replace `decision.KindNoul` with `decision.KindPredicate`. Nothing else in the
   call site changes.
2. If you compare a stored or logged kind string, accept both `"noul"` (old rows)
   and `"predicate"` (new rows), or decode it with `ParseKind`.
3. Keep configuration that says `"noul"` as it is, or change it to `"predicate"`:
   both decode to `KindPredicate`.

## What breaks next

DP-14 moved the `LLM` and `TypeSafe` adapters into subpackages
(`foundations/decision/llm`, `foundations/decision/typesafe`), so the core imports no
vendor code, and removed the deprecated `KindNoul` constant. Its note is
[decision-adapter-subpackages.md](decision-adapter-subpackages.md): after step 1 above,
that step is only an import change.
