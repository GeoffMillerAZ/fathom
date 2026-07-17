// thirdlayer is a TEST-ONLY fixture pack (docs/plans/layer-catalog.md LC-8):
// not shipped, not embedded, not obligations, not findings. Its only purpose
// is proving the loader/compiler/server pipeline is generic — see
// thirdlayer_test.go, which loads this via semantic.LoadLayerIn against this
// directory's parent as a project root, exactly the way `fathom layers
// install` would leave a real pack.
name:    "thirdlayer"
version: "v1"
kind:    "third-kind"
corpus: include: ["governance/**"]
prompt_version: "thirdlayer/fixture"
prompt_file:    "thirdlayer/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 50000
	max_usd_per_run:    0.5
}
// Proves LC-2's drift_tracked generalization too: a third layer opts into
// remediation-drift anchoring by setting this ONE field, no Go anywhere.
serving: drift_tracked: true
