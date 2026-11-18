package dependencypathfilters

// TestPathFiltersCoverageMapSchema checks each commented JSON example in CUE and Go.
// case: {"schema_version":2,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/main.ts"]}]}
// case: {"schema_version":1,"coverage":[]}
// case: {"schema_version":1,"coverage":[{"workflow":"ci.yml","entries":["src/main.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","job":null,"entries":["src/main.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","job":"build/test","entries":["src/main.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":[]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["/src/main.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/../main.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/.git/config"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/*.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/main.ts","src/main.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/main.ts"],"event":"push"}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","entries":["src/main.ts"]},{"workflow":".github/workflows/ci.yml","entries":["src/util.ts"]}]}
// case: {"schema_version":1,"coverage":[{"workflow":".github/workflows/ci.yml","job":"build","entries":["src/main.ts"]},{"workflow":".github/workflows/ci.yml","job":"build","entries":["src/util.ts"]}]}
