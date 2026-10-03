package templates

import (
	appsv1 "k8s.io/api/apps/v1"
)

#Deployment: appsv1.#Deployment & {
	#config:    #Config
	apiVersion: "apps/v1"
	kind:       "Deployment"
	metadata:   #config.metadata
	spec: appsv1.#DeploymentSpec & {
		replicas: #config.replicas
	}
}
