package templates

import (
	corev1 "k8s.io/api/core/v1"
)

#Service: corev1.#Service & {
	#config:    #Config
	apiVersion: "v1"
	kind:       "Service"
	metadata:   #config.metadata
}

#TestJob: {
	#config:    #Config
	apiVersion: "batch/v1"
	kind:       "Job"
}

// Selector is a helper, not an object: it has no literal kind.
#Selector: {
	app: string
}
