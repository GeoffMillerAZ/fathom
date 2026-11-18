package dependencyboundaries

import "list"

#GlobClass:     "\\[\\^?[A-Za-z0-9._@+~ -]+\\]"
#ExternalClass: "\\[\\^?[A-Za-z0-9._+~-]+\\]"
#PathPart:      "([A-Za-z0-9._@+~ *?-]|\(#GlobClass))+"
#ExternalPart:  "([A-Za-z0-9._+~*?-]|\(#ExternalClass))+"
#Ecosystem:     "[a-z][a-z0-9-]{0,63}"

#ID:   string & =~"^\(#Ecosystem)$"
#Glob: string & =~"^.{1,512}$" & !~"(^|[:/])\\.\\.?(/|:|$)" &
		!~"\\*\\*:|\\*\\*\\*|[^:/\\*]\\*\\*|\\*\\*[^:/\\*]"
#PathGlob:     #Glob & =~"^\(#PathPart)(/\(#PathPart))*$"
#ExternalGlob: #Glob & !~"^repo:" &
	=~"^\(#Ecosystem):@?\(#ExternalPart)(:[0-9]+/\(#ExternalPart))?(/\(#ExternalPart))*$"

#Sources: [#PathGlob, ...#PathGlob]
#Target: "." | #PathGlob | #ExternalGlob
#Targets: [#Target, ...#Target]

#Exception: {
	from: #Sources
	to:   #Targets
}

#Rule: {
	id:          #ID
	from:        #Sources
	deny?:       #Targets
	allow_only?: #Targets
	except?: [...#Exception]
	reason: string & =~"[^\\s\\p{Z}]" & !~"[\\p{Cc}\\p{Zl}\\p{Zp}]"
} & ({deny!: #Targets, allow_only?: _|_} | {allow_only!: #Targets, deny?: _|_})

#Rules: {
	schema_version: 1
	rules: [#Rule, ...#Rule]
	#unique: true & list.UniqueItems([for rule in rules {rule.id}])
}
