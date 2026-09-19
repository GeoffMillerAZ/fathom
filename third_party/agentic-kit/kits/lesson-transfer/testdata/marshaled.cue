// Concrete shapes emitted by Go JSON marshaling in the public fixture path.
package lessontransfer

import lesson "github.com/geoffmilleraz/agentic-kit/kits/lesson-transfer:lessontransfer"

receiverData: lesson.#Receiver & {
	workload: "resume_as_code"
	revision: "1111111111111111111111111111111111111111111111111111111111111111"
	config_digest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
}

outcomeData: lesson.#Outcome & {
	classification: "unknown"
	evidence: "9999999999999999999999999999999999999999999999999999999999999999"
	detail: "independent local oracle"
}

receipt: lesson.#Receipt & {
	version: "lesson-transfer/v1"
	payload_digest: "15c06d133d8c763c44c5af88bd7a49da1aed9b694a4df023bbfceec1788495ba"
	reproducer_digest: "c80ba1f5e23a700eb043d84608f2024b9cafe4c50a9903865955384ea7d47687"
	receiver: receiverData
	outcome: outcomeData
	integrity: "pass"
	reproduction: "pass"
	fresh: true
	checked_at: "2026-09-13T12:00:00Z"
}

cliValidate: lesson.#CLIOutput & {
	version: "kit-workflow/lesson/v1"
	operation: "lesson/validate"
	payload_digest: "15c06d133d8c763c44c5af88bd7a49da1aed9b694a4df023bbfceec1788495ba"
	reproducer_digest: "c80ba1f5e23a700eb043d84608f2024b9cafe4c50a9903865955384ea7d47687"
	integrity: "bound"
	reproduction: "not-run"
	fresh: false
	checked_at: "2026-09-13T12:00:00Z"
	boundary: "local only"
}

cliPending: lesson.#CLIOutput & {
	version: "kit-workflow/lesson/v1"
	operation: "lesson/adoption-check"
	payload_digest: "15c06d133d8c763c44c5af88bd7a49da1aed9b694a4df023bbfceec1788495ba"
	reproducer_digest: "c80ba1f5e23a700eb043d84608f2024b9cafe4c50a9903865955384ea7d47687"
	receiver: receiverData
	integrity: "bound"
	reproduction: "asserted-unverified"
	fresh: false
	checked_at: "2026-09-13T12:00:00Z"
	boundary: "pending"
}
