package certify

// CoverageMetaUsable exposes coverageMetaUsable to the external test that
// pins it to internal/conserve's bar (conserve imports this package, so
// only an external test package can hold both).
var CoverageMetaUsable = coverageMetaUsable
