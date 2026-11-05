package lang

import "strings"

// structural.go -- the kinds of a non-code dialect whose Signature is returned.
//
// A Signature from a dialect that is no code (Kind "tooling": a manifest, a
// template, a pipeline, a schema) is withheld unless its symbol's Kind is here,
// because a configuration file keeps its values where a program keeps its
// structure, and a Signature in it is mostly a value (a version, a type named by a
// `type:` key, a command, an image, a URL, a default, a title). A kind is listed
// only when its Signature is structure: a keyword, or a name the declaration
// introduces, written outside every quoted literal and every `key: value` value.
// A kind nobody listed reads empty, so a new kind fails closed. A kind is selected
// by its Kind, never by its Canonical, which says what a thing is and not where
// its Signature came from.
//
// Every entry carries a one-line reason. TestStructuralKindsHoldNoValue holds the
// list (each kind is one a fixture of a dialect that is no code produces, each
// reason is one line, and an unlisted kind reads empty) and
// TestSentinelPositionsAreNotReturned puts a sentinel in every quoted literal and
// every value of every fixture and requires it in none of these Signatures.

// structuralKinds maps a Symbol.Kind to why its Signature is structure.
var structuralKinds = map[string]string{
	"AlertmanagerReceiver":   "the integration keys of the receiver, which name the integrations and carry none of their settings",
	"BicepFunc":              "the keyword and the declared function name",
	"BicepMetadata":          "the keyword and the declared metadata name, before the value",
	"BicepOutput":            "the keyword, the declared name and its type, before the value",
	"BicepType":              "the keyword and the declared type name",
	"BicepVar":               "the keyword and the declared name, before the value",
	"CDKWatch":               "the include or exclude key of the watch block, a keyword",
	"CMakeTarget":            "the target's kind and its library type, keywords",
	"CaddyNamedRoute":        "the declared name of the route",
	"CaddySnippet":           "the declared name of the snippet",
	"CheckovCheck":           "the check class and its base class, a class header",
	"ConanRecipe":            "the recipe class and its base class, a class header",
	"DocChunk":               "the heading level of the chunk, from the number of # signs",
	"GraphQLFragment":        "the keyword, the fragment name and the type it is on",
	"JenkinsAgent":           "the agent directive, a keyword",
	"JenkinsEnvironment":     "the keyword of the environment block, before its values",
	"JenkinsOptions":         "the keyword of the options block",
	"JenkinsParameters":      "the keyword of the parameters block",
	"JenkinsPipeline":        "the keyword of the pipeline block",
	"JenkinsPost":            "the keyword of the post block",
	"JenkinsStages":          "the keyword of the stages block",
	"JenkinsSteps":           "the keyword of the steps block",
	"NginxUpstream":          "the keyword and the declared upstream name",
	"NixAttribute":           "the attribute path of an assignment, before its value",
	"NixOption":              "the option path of an assignment, before its value",
	"NixPackage":             "the attribute path of an assignment, before its value",
	"NixVariable":            "the variable of an assignment, before its value",
	"NomadLocal":             "the names a locals block declares, never their values",
	"PackerBuild":            "the keyword of the build block",
	"PackerLocal":            "the names a locals block declares, never their values",
	"PackerVarValue":         "the variable name, with its value already elided",
	"PrismaDatasource":       "the keyword and the declared datasource name",
	"PrismaEnum":             "the keyword and the declared enum name",
	"PrismaGenerator":        "the keyword and the declared generator name",
	"PrismaModel":            "the keyword and the declared model name",
	"PrismaType":             "the keyword and the declared type name",
	"PrismaView":             "the keyword and the declared view name",
	"ProtoRPC":               "the request and response message types",
	"TerraformLocal":         "the names a locals block declares, never their values",
	"TerraformVarValue":      "the variable name, with its value already elided",
	"TerragruntDependencies": "the keyword of the dependencies block",
	"TerragruntInput":        "the names the inputs block sets, never their values",
	"TerragruntLocal":        "the names a locals block declares, never their values",
	"TerragruntRemoteState":  "the keyword of the remote_state block",
	"TerragruntTerraform":    "the keyword of the terraform block",
	"ThriftEnumValue":        "the enum member's name",
	"ThriftService":          "the keyword, the service name and the service it extends",
}

// structuralCuts are the kinds whose Signature a parser may carry past the
// structure: a query that captures no body ends it at the end of the first line,
// and a definition written on one line holds its body there (a one-line Thrift
// service, with the default of a parameter), and a GraphQL fragment's directives
// follow its type condition and hold arguments. The Signature is cut at the first
// of the characters listed, which is where a value can start.
var structuralCuts = map[string]string{
	"GraphQLFragment": "@{",
	"ThriftService":   "{",
}

// structuralSignature is the part of a structural kind's Signature that is
// structure.
func structuralSignature(kind, sig string) string {
	if cut, ok := structuralCuts[kind]; ok {
		if i := strings.IndexAny(sig, cut); i >= 0 {
			return strings.TrimSpace(sig[:i])
		}
	}
	return sig
}

// structuralKind reports whether a Signature of this kind is returned from a
// dialect that is no code.
func structuralKind(kind string) bool {
	_, ok := structuralKinds[kind]
	return ok
}
