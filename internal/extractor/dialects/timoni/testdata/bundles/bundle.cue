bundle: {
	apiVersion: "v1alpha1"
	name:       "demo"
	instances: {
		web: {
			module: url: "file://../app"
			namespace: "demo"
			values: message: "hi"
		}
		cache: {
			module: {
				url:     "oci://registry.example.test/modules/cache"
				version: "1.2.3"
			}
			namespace: "demo"
		}
		"shared.db": {
			module: url: "file://../../elsewhere"
			namespace: "demo"
		}
		climb: {
			module: url: "file://../../../outside"
			namespace: "demo"
		}
		dynamic: {
			module: url: "file://\(_base)/app"
			namespace: "demo"
		}
	}
}

_base: ".."
