/* budget.c — a deadline-bounded parse entry point compiled into every
 * fathom grammar blob (see build.sh; ADR-021 §4, critic B1).
 *
 * Why this exists: tree-sitter v0.26 removed ts_parser_set_timeout_micros;
 * the supported cancellation mechanism is a progress callback passed to
 * ts_parser_parse_with_options. Enforcing the deadline INSIDE the guest via
 * that callback costs one clock read per ~100 parser operations (the
 * OP_COUNT_PER_PARSER_CALLBACK_CHECK cadence in parser.c) — effectively
 * free — whereas the host-side alternative (wazero's WithCloseOnContextDone)
 * compiles a termination check into every loop back-edge of every module
 * and measured ~3x slower across the whole calibration corpus. The clock is
 * musl's clock_gettime(CLOCK_MONOTONIC), which lowers to WASI
 * clock_time_get — served by wazero's stock wasi_snapshot_preview1 module,
 * which the engine already instantiates.
 *
 * Concurrency: the static state below is per module INSTANCE (each
 * tsengine.Parser owns one), and a Parser is never used concurrently
 * (mutex-guarded by internal/extractor.TSExtractor), so plain statics are
 * safe.
 */

#include <stdbool.h>
#include <stdint.h>
#include <time.h>

#include "tree_sitter/api.h"

typedef struct {
  const char *data;
  uint32_t len;
} FathomBuf;

static const char *fathom_read(void *payload, uint32_t byte_index,
                               TSPoint position, uint32_t *bytes_read) {
  (void)position;
  FathomBuf *buf = (FathomBuf *)payload;
  if (byte_index >= buf->len) {
    *bytes_read = 0;
    return "";
  }
  *bytes_read = buf->len - byte_index;
  return buf->data + byte_index;
}

static uint64_t fathom_deadline_ns = 0;
static uint32_t fathom_cancelled = 0;

static uint64_t fathom_now_ns(void) {
  struct timespec ts;
  clock_gettime(CLOCK_MONOTONIC, &ts);
  return (uint64_t)ts.tv_sec * 1000000000ull + (uint64_t)ts.tv_nsec;
}

/* NOTE: no amortization of the clock read. It was tried (clock every 16th
 * callback) and reverted: on the pathological input the query cursor's
 * progress callbacks arrive at a slow REAL-TIME cadence (~8s between
 * deadline expiry and the next check under load), so widening the cadence
 * 16x pushed guest cancellation past the entire 45s scan — the budget
 * became decorative. The per-callback clock_gettime (a WASI host call)
 * costs ~2% user CPU corpus-wide: cheap insurance. */
static bool fathom_expired(void) {
  if (fathom_now_ns() >= fathom_deadline_ns) {
    fathom_cancelled = 1;
    return true;
  }
  return false;
}

/* Returning true cancels the parse (ts_parser_parse_with_options then
 * returns NULL). Called every ~100 parser operations. */
static bool fathom_progress(TSParseState *state) {
  (void)state;
  return fathom_expired();
}

/* Query-cursor twin of fathom_progress (called every ~100 query operations
 * from the cursor's advance loop). Cancellation here makes
 * ts_query_cursor_next_match return false — indistinguishable from normal
 * exhaustion at the C API — so the flag is the ONLY honest signal; the Go
 * caller must consult fathom_take_cancelled after the match loop ends. */
static bool fathom_query_progress(TSQueryCursorState *state) {
  (void)state;
  return fathom_expired();
}

/* fathom_take_cancelled returns 1 if the last bounded parse/query was
 * cancelled by its budget, clearing the flag. */
uint32_t fathom_take_cancelled(void) {
  uint32_t c = fathom_cancelled;
  fathom_cancelled = 0;
  return c;
}

/* fathom_node_end_byte reconstructs a TSNode from its six 32-bit words —
 * passed as plain scalars so the host needs NO guest-memory allocation —
 * and returns its end byte. Rationale: the host reads each query capture's
 * TSNode bytes anyway (they sit in the TSQueryMatch's capture array), and
 * start_byte/start_point are literally context[0..2] (node.c: ts_node_new),
 * so the ONLY node property requiring guest data is the end byte (it lives
 * in the referenced subtree). Making it a scalar-argument call turns the
 * per-capture marshaling cost from malloc+write+3 accessor calls+free into
 * one call — measured as the difference between busting and meeting the
 * calibration corpus's 1.5x ingest budget. */
uint32_t fathom_node_end_byte(uint32_t c0, uint32_t c1, uint32_t c2,
                              uint32_t c3, uint32_t id, uint32_t tree) {
  TSNode n = {{c0, c1, c2, c3}, (const void *)id, (const TSTree *)tree};
  return ts_node_end_byte(n);
}

/* fathom_parse_bounded parses data[0:len] with a wall-clock budget of
 * budget_ms milliseconds. Returns the tree, or NULL if the budget expired
 * (or the parser had no language). The parser itself SURVIVES an abort —
 * per tree-sitter's documented cancellation contract the next parse must be
 * preceded by ts_parser_reset, which the Go caller already does before
 * every parse. */
TSTree *fathom_parse_bounded(TSParser *parser, const char *data, uint32_t len,
                             uint32_t budget_ms) {
  FathomBuf buf = {data, len};
  TSInput input = {&buf, fathom_read, TSInputEncodingUTF8, NULL};
  TSParseOptions opts = {NULL, fathom_progress};
  fathom_deadline_ns = fathom_now_ns() + (uint64_t)budget_ms * 1000000ull;
  fathom_cancelled = 0;
  return ts_parser_parse_with_options(parser, NULL, input, opts);
}

/* fathom_query_cursor_exec_bounded starts a query execution whose match
 * iteration (the subsequent ts_query_cursor_next_match calls) is bounded by
 * budget_ms of wall clock. This matters more than the parse bound in
 * practice: on the pathological `({[` x 50,000 input the PARSE completes in
 * ~70ms but the query cursor's scan of the resulting error tree runs 45s+
 * (measured) — the query loop, not the parse loop, is where an unbounded
 * daemon actually wedges. node is passed by pointer because wasm exports
 * cannot receive tree-sitter's by-value TSNode struct directly. */
/* MUST be static-storage: ts_query_cursor_exec_with_options stores the
 * options POINTER on the cursor (query.c: `self->query_options =
 * query_options`) and dereferences it during every later
 * ts_query_cursor_next_match call — a stack-local here would dangle the
 * moment this export returns, and the stale progress_callback pointer then
 * traps with "invalid table access" on the first next_match. */
static TSQueryCursorOptions fathom_query_opts = {NULL, fathom_query_progress};

void fathom_query_cursor_exec_bounded(TSQueryCursor *cursor,
                                      const TSQuery *query, const TSNode *node,
                                      uint32_t budget_ms) {
  fathom_deadline_ns = fathom_now_ns() + (uint64_t)budget_ms * 1000000ull;
  fathom_cancelled = 0;
  ts_query_cursor_exec_with_options(cursor, query, *node, &fathom_query_opts);
}
