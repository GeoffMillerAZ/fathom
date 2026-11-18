package dependencypathfilters

import "list"

#RepoPath: string & =~"^.{1,512}$" &
	=~"^[A-Za-z0-9._@+~ -]+(/[A-Za-z0-9._@+~ -]+)*$" &
		!~"(^|/)(\\.\\.?|\\.git|\\.hg|\\.svn|\\.bzr)(/|$)"
#Workflow: #RepoPath & =~"^\\.github/workflows/[A-Za-z0-9._-]+\\.ya?ml$"
#Job:      string & =~"^[A-Za-z_][A-Za-z0-9_-]*$"

#Scope: {
	workflow: #Workflow
	job?:     #Job
	entries: [#RepoPath, ...#RepoPath]
	#uniqueEntries: true & list.UniqueItems(entries)
	#jobKey:        *"" | #Job
	if job != _|_ {
		#jobKey: job
	}
	#key: "\(workflow):\(#jobKey)"
}

#CoverageMap: {
	schema_version: 1
	coverage: [#Scope, ...#Scope]
	#uniqueScopes: true & list.UniqueItems([for scope in coverage {scope.#key}])
}
