# WA-101: connection pool leak on error path

Severity: high. Area: resource management. Class: resource-leak.
Close the pooled connection on every error return in internal/store/pool.go's
Acquire, not only the success path.

# WA-102: ignored write error silently drops data

Severity: high. Area: error handling. Class: error-handling.
Check and propagate the error returned by the flush call in
internal/query/engine.go's Flush instead of discarding it.

# WA-103: unsynchronized cache map under concurrent access

Severity: high. Area: concurrency. Class: concurrency.
Guard internal/cache/lru.go's Get/Set pair with the existing mutex; both
currently read/write the map without holding it.

# WA-104: auth token comparison is not constant-time

Severity: high. Area: security. Class: security.
Compare the bearer token in internal/auth/token.go's Verify using a
constant-time comparison, not the standard string equality operator.

# WA-105: O(n^2) rebuild on every dispatch call

Severity: medium. Area: performance. Class: performance.
Cache the routing table in internal/mcp/dispatch.go's Route instead of
rebuilding it from scratch on every call.

# WA-106: handler response shape violates the documented contract

Severity: medium. Area: API contract. Class: contract-violation.
Return the documented error envelope shape from internal/httpapi/handler.go's
WriteError instead of a bare string, matching the API contract doc.

# WA-107: retro note on the pool leak, already fixed

Severity: high. Area: resource management. Status: resolved.
This finding was WA-101 above; it has since been fixed and is kept here only
as a historical record, not an actionable remediation.

# WA-108: background context on the cache rewrite

Area: context. This section is background/context prose about why the cache
package was rewritten last quarter — it describes history, not an issue, and
states no remediation.
