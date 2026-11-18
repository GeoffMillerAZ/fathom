package dependencyedges

#Path:      string & =~"^[A-Za-z0-9._@+~ -]+(/[A-Za-z0-9._@+~ -]+)*$" & !~"(^|/)\\.\\.?(/|$)" & =~"^.{1,512}$"
#Directory: "." | #Path
#Dialect:   "" | string & =~"^[a-z][a-z0-9-]{0,63}$"
#Ecosystem: string & =~"^[a-z][a-z0-9-]{0,63}$"
#External:  string & =~"^[a-z][a-z0-9-]{0,63}:@?[A-Za-z0-9._+~-]+(:[0-9]+/|/)?[A-Za-z0-9._+~/\\-]*$" & !~"^repo:|//|/$|[:/]\\.\\.?(/|$)" & =~"^.{1,512}$"
#Raw:       "" | "***" | (#Directory & !~" ")
#Version:   "" | string & =~"^([A-Za-z0-9][A-Za-z0-9._+!~-]*|sha(256|384|512):[0-9A-Fa-f]+)$" & =~"^.{1,512}$"

#Kind:        "import" | "include" | "embed" | "build-input" | "exec" | "config-ref"
#Resolution:  "file" | "directory" | "external" | "unknown"
#Provenance:  "builtin" | "pack" | "extension" | "declared"
#UnknownCode: "dynamic-import" | "computed-path" | "unresolved-alias" | "missing-config" |
	"unbounded-glob" | "unsupported-construct" | "parse-error" | "unmeasured-reader" |
		"external-unpinned" | "read-limit"
#DecidableBy: "toolchain" | "lockfile" | "config-file" | "observation" | "declaration"

#Edge: {
	from:       #Path
	kind:       #Kind
	raw:        #Raw
	to:         string
	resolution: #Resolution
	version:    #Version
	line:       int & >=1
	provenance: #Provenance

	if resolution == "file" {
		to:      #Path
		version: ""
	}
	if resolution == "directory" {
		to:      #Directory
		version: ""
	}
	if resolution == "external" {
		to: #External
	}
	if resolution == "unknown" {
		to:      ""
		version: ""
	}
}

#Unknown: {
	from: #Path
	code: #UnknownCode
	raw:  #Raw
	line: int & >=1
	decidable_by: [...#DecidableBy]
}

#FileDeps: {
	P=path:   #Path
	dialect:  #Dialect
	measured: bool
	trusted:  bool
	complete: bool
	edges: [...(#Edge & {from: P})]
	unknowns: [...(#Unknown & {from: P})]

	if measured {
		trusted: false
	}
	if complete {
		if !measured {
			trusted: true
		}
		edges: [...{resolution: !="unknown", ...}]
		unknowns: []
	}
	if !complete {
		unknowns: [#Unknown, ...#Unknown]
	}
}

#Batch: {
	schema_version: 1
	files: [...#FileDeps]
}

#Unit: {
	ecosystem: #Ecosystem
	name:      string
	if ecosystem == "repo" {
		name: #Directory
	}
	if ecosystem != "repo" {
		// Apply the same external-id rules after joining the wire fields.
		#id: "\(ecosystem):\(name)" & #External
	}
}

#DeclaredExport: [...{
	from: #Unit
	to:   #Unit
}]
