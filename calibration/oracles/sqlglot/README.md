# sqlglot — tier-C SQL foreign-key call pairs (D5)

DerekStride/tree-sitter-sql ships no `tags.scm`, so SQL's L2 truth comes from an
independent parser library (design §1.3, tier C): `sql_refs_oracle.py` emits the
neutral oracle file's `calls` — one `(table, referenced table)` pair per foreign key
declared inside a `CREATE TABLE` (inline `REFERENCES` or a table constraint). Keys added
by `ALTER TABLE … ADD CONSTRAINT` have no enclosing definition and are not pairs.

    python3 -m venv <tools>/pyts && <tools>/pyts/bin/pip install sqlglot==27.8.0
    <tools>/pyts/bin/python sql_refs_oracle.py --dialect '*sqlite*=sqlite' <sqitch> > sql-sqitch.json
    <tools>/pyts/bin/python sql_refs_oracle.py <pagila> pagila-schema.sql pagila-schema-jsonb.sql pagila-temporal-setup.sql > sql-pagila.json

Pass the output as `indexbench lang-oracle -tool-oracle`; the harness drops pairs on
files outside `-only`. A statement sqlglot cannot parse falls back to a command (a
warning on stderr) and yields no pair — never guessed.
