package guardrailspipeline

policy: #RefusalPolicy & {
	id: "local-security-policy"
	version: 1
	on_uncertain: "review"
	on_error: "review"
	categories: [{
		name: "local-category"
		rubric: "A locally configured, trusted screening rule."
		action: "review"
	}]
}
