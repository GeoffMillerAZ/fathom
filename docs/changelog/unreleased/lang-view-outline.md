### Added

- `view.Outline`, `view.OutlineOptions`, `view.OutlineDoc`, `view.OutlineNode` and `OutlineDoc.Text` (provisional) nest a `lang.Result`'s symbols by span containment into the shape of a file, with only strings the read carries. `MaxNodes` drops the deepest level first and the outline states how many nodes it omitted.
- `fathom-lang outline [-root DIR] [-format text|json] [-max-nodes N] [-deadline D] [-max-memory SIZE] [-in-process] PATH...` (provisional) prints that outline as a line-oriented text form or as `{"schema_version":1,"files":[...]}`, on the supervised read path of `extract`. `fathom-lang schema outline` prints the JSON Schema of the JSON form.
- `OutlineDoc.Text` writes a run of whitespace that holds a newline as one space, so a multi-line Go signature stays on its node's line. JSON strings are unchanged.
