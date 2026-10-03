runtime: {
	apiVersion: "v1alpha1"
	name:       "fleet"
	clusters: {
		staging: {
			group:       "staging"
			kubeContext: "kind-staging"
		}
		production: {
			group:       "production"
			kubeContext: "kind-production"
		}
	}
	values: [{
		query: "k8s:v1:Namespace:kube-system"
		for: "CLUSTER_ID": "obj.metadata.uid"
	}]
}
