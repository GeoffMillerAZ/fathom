// The authored orientation set (docs/design/fathom-dir.md section 7.6): where a
// session new to this repository starts. `fathom config audit` fails any anchor
// that no longer resolves.
orientation: [
	{anchor: "docs/architecture.md", why: "How the connector, fusion, query and store layers fit together."},
	{anchor: "internal/connector/connector.go#Connector", why: "The interface every source of files implements, the first layer of an ingest."},
	{anchor: "internal/fusion/ingest.go#Ingest", why: "The ingest pipeline that turns connector output into stored symbols, edges and facts."},
	{anchor: "internal/query/engine.go#Engine", why: "The concern-scoped query engine every surface answers from."},
	{anchor: "internal/store/store.go#Store", why: "The persistence contract the rest of the code reads and writes through."},
	{anchor: "internal/mcp/dispatch.go#registerTool", why: "Where an MCP tool is registered, with its descriptor and budget."},
	{anchor: "cmd/fathom/verbs.go#declareVerb", why: "How a CLI command registers itself in the verb table."},
	{anchor: "cmd/fathom/edithook.go#cmdEditHook", why: "The edit hook entry point that serves governing rules at a file edit."},
	{anchor: "cmd/fathom/prompthook.go#cmdPromptHook", why: "The prompt hook entry point that runs on each user prompt."},
	{anchor: "cmd/fathom/readouthook.go#cmdReadoutHook", why: "The readout hook entry point that compresses tool output."},
	{anchor: "cmd/fathom/servehttp.go#runServeHTTP", why: "The daemon's serve path: HTTP, MCP and the background refresh."},
	{anchor: "docs/plans/README.md", why: "The plan grammar a checklist must follow to be verifiable."},
	{anchor: "internal/verify/markdown.go#ParseMarkdownPlan", why: "The plan parser to read first for any verify question."},
	{anchor: "internal/extractor/tsextract.go#tsRegistry", why: "The language dialect registry that tags-engine extractors are built from."},
	{anchor: "docs/design/language-breadth.md", why: "The worker recipe (section 13) for adding a language or tool format."},
	{anchor: "internal/lint/tenancy/tenancy.go", why: "The tenancy lint that keeps one service's data out of another's."},
]
