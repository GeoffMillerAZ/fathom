package dependencypathfilters

sample: #CoverageMap & {
	schema_version: 1
	coverage: [
		{workflow: ".github/workflows/ci.yml", entries: ["src/main.ts", "src/util.ts"]},
		{workflow: ".github/workflows/ci.yml", job: "lint", entries: ["src/main.ts"]},
		{workflow: ".github/workflows/test.yaml", job: "unit-tests", entries: ["test/main.ts"]},
	]
}
