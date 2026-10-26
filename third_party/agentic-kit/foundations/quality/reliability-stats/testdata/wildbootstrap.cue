package reliabilitystats

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats:reliabilitystats"

validWildCluster: reliabilitystats.#WildClusterInterval & {low: 0.26, high: 3.74, point: 2, tasks: 2, clusters: 2}
validWildConstant: reliabilitystats.#WildClusterInterval & {low: 3, high: 3, point: 3, tasks: 2, clusters: 2}
validWildRepository: reliabilitystats.#WildClusterInterval & {low: -1.2, high: 2.7, point: 0.75, tasks: 4, clusters: 2}
validClusteredObservation: reliabilitystats.#Observation & {task: "a", cluster: "repository", diff: 0.5}

// Invalid counterparts, uncomment individually to see cue vet reject them:
// invalidWildSingle: reliabilitystats.#WildClusterInterval & {low: 1, high: 1, point: 1, tasks: 2, clusters: 1}
// invalidWildInverted: reliabilitystats.#WildClusterInterval & {low: 2, high: 1, point: 1.5, tasks: 2, clusters: 2}
// invalidWildTooManyClusters: reliabilitystats.#WildClusterInterval & {low: 1, high: 2, point: 1.5, tasks: 2, clusters: 3}
// invalidClusterType: reliabilitystats.#Observation & {task: "a", cluster: 1, diff: 0.5}
