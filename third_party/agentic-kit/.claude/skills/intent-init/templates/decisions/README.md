# Decisions

<!-- Index of decision records. One row per record. The "Revisit when" column is a
     short pointer; the record holds the full condition. Keep rows in id order.
     Records are general decision records: product, ux, domain, architecture, data,
     security, ops, dependency, platform, process, strategy. -->

Record a decision when any holds: reversal is expensive; it constrains code outside its
own package; it creates or removes an invariant, non-goal, or boundary; it accepts a
notable tradeoff or dependency; it defers something significant and names what would
un-defer it; it rejects an alternative someone will propose again; without the record a
future reader would rebuild this differently. Otherwise: a comment, a test, a commit.

Status: proposed · accepted · rejected · deferred · superseded by <id> · deprecated.

| Id | Status | Type | Title | Revisit when |
|---|---|---|---|---|
| 0001 | accepted | architecture | <title> | <short condition> |

<!-- Decisions recorded elsewhere (specs, audits, plans) that pass the threshold but
     were not written as records: point at them here rather than duplicating. -->
