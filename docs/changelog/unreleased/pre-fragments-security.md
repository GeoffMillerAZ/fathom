### Security

- Requests to a configured decision provider now carry only the fields each site declares, redact secret-shaped strings (counted as the decision row's `redactions`), honour the new `[decide] exclude` path globs (every layer's union), refuse a question that names a security property, and can be printed without sending (`FATHOM_DECIDE_PLAN_ONLY=1`); provider `none` is unchanged.
