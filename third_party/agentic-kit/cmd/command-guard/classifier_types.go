package main

// A classifier invocation owns all state. No transport or receipt is shared.
type classifierPolicy struct {
	Profile               string `json:"profile"`
	Endpoint              string `json:"endpoint"`
	Model                 string `json:"model"`
	ExpectedResponseModel string `json:"expectedResponseModel"`
	Criterion             string `json:"criterion"`
	CriterionSHA256       string `json:"criterionSHA256"`
	TimeoutMillis         int    `json:"timeoutMillis"`
	MaxInputBytes         int    `json:"maxInputBytes"`
	MaxResponseBytes      int    `json:"maxResponseBytes"`
	MaxTokens             int    `json:"maxTokens"`
}
type classifierReceipt struct {
	Profile            string `json:"profile"`
	Status             string `json:"status"`
	Code               string `json:"code"`
	ExpectedFiles      int    `json:"expectedFiles"`
	SelectedFiles      int    `json:"selectedFiles"`
	SelectedBytes      int    `json:"selectedBytes"`
	AttemptedRequests  int    `json:"attemptedRequests"`
	CompletedDecisions int    `json:"completedDecisions"`
	RequestBytes       int    `json:"requestBytes"`
	ResponseBytes      int    `json:"responseBytes"`
}

func newClassifierReceipt(n int) *classifierReceipt {
	return &classifierReceipt{Profile: "loopback-concern/1", Status: "not-invoked", Code: "not_invoked", ExpectedFiles: n}
}
func classifierPermitted(s scope, h string) bool {
	for _, x := range s.PermittedClassifierSHA256 {
		if x == h {
			return true
		}
	}
	return false
}

func selectedClassifierControls(s scope, p policy) bool {
	return len(s.RequiredClassifierIDs) > 0 || len(s.PermittedClassifierSHA256) > 0 || s.ClassifierPolicySHA256 != "" || p.Classifier != nil
}
