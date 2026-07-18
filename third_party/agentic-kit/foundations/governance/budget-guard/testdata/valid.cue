// Package budgetguard — valid test data that must unify with schema.cue.
package budgetguard

import "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard:budgetguard"

// validDevelopment is a generous configuration for development environments.
validDevelopment: budgetguard.#Config & {
	MaxReqPerMin: 60
	MaxReqPerDay: 1000
	MaxUSDPerDay: 10.0
}

// validProduction is a conservative production configuration capping spend.
validProduction: budgetguard.#Config & {
	MaxReqPerMin: 10
	MaxReqPerDay: 500
	MaxUSDPerDay: 50.0
}

// validTight is a minimal configuration for cost-controlled batch jobs.
validTight: budgetguard.#Config & {
	MaxReqPerMin: 2
	MaxReqPerDay: 20
	MaxUSDPerDay: 1.0
}

// validMinimal is the smallest possible valid configuration.
validMinimal: budgetguard.#Config & {
	MaxReqPerMin: 1
	MaxReqPerDay: 1
	MaxUSDPerDay: 0.000001
}
