package repofacts

// The dialect basis: a file whose dialect is a tool's own format has the
// role that tool gives it, wherever the file sits. The caller supplies the
// dialect (an extractor dialect id, or connector.LanguageForPath's tag);
// this package never looks one up. A programming language or a bare data
// format (go, py, yaml, md, proto) decides nothing here: a Go file may be
// source or a test, a Markdown file may be a vendored tree's, so the
// convention basis reads those.
var dialectRoles = map[string]RoleName{
	// CI and the hooks that gate a push.
	"github-actions": CI, "gitlab-ci": CI, "jenkinsfile": CI, "git-hooks-config": CI,
	// Build tools and package manifests.
	"makefile": Build, "taskfile": Build, "cmake": Build, "meson": Build, "msbuild": Build, "gradle": Build,
	"maven": Build, "dockerfile": Build, "go-mod": Build, "cargo": Build, "package-json": Build, "composer": Build,
	"python-packaging": Build, "conan": Build, "vcpkg": Build, "compile-commands": Build,
	// Infrastructure as code.
	"terraform": IaC, "terragrunt": IaC, "bicep": IaC, "arm-templates": IaC, "cloudformation": IaC,
	"pulumi-yaml": IaC, "kubernetes": IaC, "kustomize": IaC, "helm": IaC, "crossplane": IaC, "flux": IaC,
	"argo": IaC, "ansible": IaC, "serverless": IaC, "timoni": IaC, "docker-compose": IaC, "aws-cdk": IaC,
	// Runtime and policy configuration.
	"nginx": Config, "haproxy": Config, "caddyfile": Config, "traefik": Config, "envoy": Config, "kong": Config,
	"otel-collector": Config, "prometheus": Config, "alertmanager": Config, "grafana-dashboard": Config,
	"fathom-config": Config, "hcl-tools": Config, "kyverno": Config, "gatekeeper": Config, "azure-policy": Config,
	"aws-iam-policy": Config, "cloud-custodian": Config,
	// Interface definitions and plan rows a dialect recognised by content.
	"openapi": Schema, "fathom-plan-rows": Docs,
}

func dialectRole(dialect string) (RoleName, bool) {
	r, ok := dialectRoles[dialect]
	return r, ok
}
