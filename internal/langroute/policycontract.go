package langroute

// The policy contract every policy producer emits (docs/design/
// language-breadth.md §4): an extractor.DocCrossFileRef with this Edge, one
// of these Strategies, Raw the target as written, Target its normalized
// form (Raw when there is nothing to normalize), Scope and Labels for a
// selector. internal/fusion re-exports these names.
const (
	PolicyRefEdge       = "policy_ref"
	PolicyResourceKind  = "resource-kind"
	PolicyLabelSelector = "label-selector"
	PolicyResourceType  = "resource-type"
	PolicyAWSARN        = "aws-arn"
	PolicyServiceName   = "service-name"
)
