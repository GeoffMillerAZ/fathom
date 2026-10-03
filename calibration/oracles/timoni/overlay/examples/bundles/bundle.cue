// The overlay the timoni oracle's reading adds to stefanprodan/timoni (an
// untracked file, never part of the subject's own tree): a bundle whose
// instances name local modules by file://, the way Timoni's bundle documentation
// shows, so the bundle-to-module references have a subject. The subject's own
// bundles (podinfo.cue) all name oci:// modules and need a registry.
bundle: {
	apiVersion: "v1alpha1"
	name:       "local"
	instances: {
		minimal: {
			module: url: "file://../minimal"
			namespace: "demo"
			values: message: "hello"
		}
		redis: {
			module: url: "file://../redis"
			namespace: "demo"
		}
		starter: {
			module: url: "file://../../blueprints/starter"
			namespace: "demo"
		}
		podinfo: {
			module: {
				url:     "oci://ghcr.io/stefanprodan/modules/podinfo"
				version: "6.14.1"
			}
			namespace: "demo"
		}
	}
}
