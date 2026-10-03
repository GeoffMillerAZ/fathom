package templates

import (
	timoniv1 "timoni.sh/core/v1alpha1"
)

// Config defines the schema and defaults for the Instance values.
#Config: {
	metadata: timoniv1.#Metadata
	message!: string
	replicas: *1 | int & >0
	image: {
		repository: string
		tag:        *"latest" | string
	}
	test: enabled: *false | bool
	extra: [...timoniv1.#Selector]
}

// Instance takes the config values and outputs the Kubernetes objects.
#Instance: {
	config: #Config

	objects: {
		svc: #Service & {#config: config}
		deploy: #Deployment & {
			#config: config
		}
	}

	tests: {
		"test-ping": #TestJob & {#config: config}
	}
}
