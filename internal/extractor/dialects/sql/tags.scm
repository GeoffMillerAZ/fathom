; dialects/sql/tags.scm — SQL DDL (LB-Q-sql, design §13.2, port of the
; incumbent regex sqlextract.go).
;
; source: DerekStride/tree-sitter-sql has no queries/tags.scm of its own
; (only highlights.scm/indents.scm), and neither aider nor nvim-treesitter
; carries a sql-tags.scm/locals.scm (checked, 2026-09-28) — see
; PROVENANCE.md. Every shape below comes from `indexbench lang-probe
; -dialect sql <fixture>` (§13 step 12), never from grammar.js/
; node-types.json.
;
; Grammar variance (PROVENANCE.md has the full account): this grammar
; (v0.3.11) has no create_procedure production at all — "CREATE PROCEDURE"
; fails to parse into any node (an ERROR blob) — so, unlike the incumbent,
; this dialect cannot detect stored procedures; and a table constraint
; named with CONSTRAINT before FOREIGN KEY ("CONSTRAINT fk FOREIGN KEY
; (...) REFERENCES ...") mis-parses the "CONSTRAINT <name>" prefix into an
; ERROR node, though the parser recovers and still yields a usable
; `constraint` node for the FOREIGN KEY clause itself (fixtures/broken.sql
; exercises this survival).

; --- table (its columns are @body: the incumbent's stripQuotes made a
; table "callable" of nothing, but this engine only attaches a
; @reference.call to a callable's smallest enclosing definition — see
; attachTSCalls, internal/extractor/tsextract.go — so a table is marked
; callable here purely to receive its FK call edges, LB-Q-sql's "FK
; references as L2 edges") ---------------------------------------------
(create_table
  (object_reference name: (identifier) @name)
  (column_definitions) @body) @definition.table

; --- view: plain (incl. OR REPLACE / IF NOT EXISTS) and MATERIALIZED are
; two distinct node types in this grammar --------------------------------
(create_view
  (object_reference name: (identifier) @name)) @definition.view

(create_materialized_view
  (object_reference name: (identifier) @name)) @definition.view

; --- index: the index's own name sits in the (oddly named, upstream)
; "column" field, directly after the INDEX keyword ------------------------
(create_index
  column: (identifier) @name) @definition.index

; --- function (CREATE PROCEDURE: see the grammar-variance note above —
; not captured, this grammar does not parse it) ---------------------------
(create_function
  (object_reference name: (identifier) @name)
  (function_body) @body) @definition.function

; --- type (CREATE TYPE ... AS ENUM/…) -------------------------------------
(create_type
  (object_reference name: (identifier) @name)) @definition.type

; --- column (nested inside a table, LB-Q-sql's "columns") ----------------
; Only a CREATE TABLE's column list or an explicit ALTER TABLE ... ADD
; [COLUMN]: tree-sitter-sql recovers an unsupported ALTER TABLE clause
; (PostgreSQL's `ATTACH PARTITION p FOR VALUES …`) into an add_column with
; no ADD keyword whose "column" is the clause's first word — 57 phantom
; `ATTACH` columns on pagila's schema (D2 re-measurement, 2026-09-28).
(column_definitions
  (column_definition
    name: (identifier) @name) @definition.column)
(add_column
  (keyword_add)
  (column_definition
    name: (identifier) @name) @definition.column)

; --- FK references as L2 call edges: inline "col TYPE REFERENCES t(c)" --
(column_definition
  (keyword_references)
  (object_reference name: (identifier) @name)) @reference.call

; --- FK references as L2 call edges: a table-level FOREIGN KEY constraint
; (named or bare — see the grammar-variance note) -------------------------
(constraint
  (keyword_references)
  (object_reference name: (identifier) @name)) @reference.call

; @body (LB-F3, design §3.3): a callable's Signature is its definition's
; text up to the start of this capture, whitespace collapsed.
